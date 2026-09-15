//go:build e2e

package nvidia

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/aws-k8s-tester/test/manifests"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// Machinery shared by the GPU-sharing features -- time-slicing and MPS. Both work
// the same way: replace the device plugin with one that advertises each physical
// GPU several times, run more pods than there are GPUs, and check they really do
// run at once on the same device. Only how sharing is configured differs, so
// everything here takes the feature as a parameter rather than assuming one.

var (
	//go:embed manifests/job-gpu-sharing-workload.yaml
	gpuSharingWorkloadManifest []byte
)

const (
	devicePluginName        = "nvidia-device-plugin-daemonset"
	devicePluginNamespace   = "kube-system"
	devicePluginPodSelector = "name=nvidia-device-plugin-ds"

	// How long to wait for the node's allocatable count to reflect the
	// reconfigured plugin. The plugin re-registers with kubelet on startup and
	// kubelet then updates node status, so this is not instant.
	gpuSharingAdvertiseTimeout = 5 * time.Minute
	// The workload is vectorAdd per pod, so this only needs to cover image pull
	// plus scheduling all the pods onto one GPU.
	gpuSharingWorkloadTimeout = 15 * time.Minute
	// How long each pod keeps re-running vectorAdd. Long enough that every pod is
	// still working while the others start, so the concurrency check has a window
	// to observe; short enough not to dominate the Job's runtime.
	gpuSharingHold = 90 * time.Second
	// Covers image pull and pod startup before the overlap window opens.
	gpuSharingConcurrencyTimeout = 10 * time.Minute
	// How long a replaced DaemonSet gets to report ready.
	gpuSharingReadyTimeout = 5 * time.Minute

	// Emitted by each workload pod; see job-gpu-sharing-workload.yaml.
	gpuUUIDMarker = "GPU_UUID="
)

// sharingFeature is what the shared helpers need to know about the caller: which
// feature is running, and which ConfigMap carries its plugin configuration.
// Without it the diagnostics report the wrong feature's name and inspect the wrong
// ConfigMap, which is worse than no diagnostics -- an MPS failure previously
// printed "[time-slicing]" and complained about the time-slicing ConfigMap.
type sharingFeature struct {
	// name prefixes every log line, so a run is attributable to a feature.
	name string
	// configMapName is the ConfigMap the replacement plugin mounts, checked first
	// when the plugin fails to start.
	configMapName string
}

// sharingState is one feature invocation's mutable state. Kept here rather than in
// package-level variables so a second invocation in the same binary cannot inherit
// the previous run's flags, and so it is obvious what belongs to a run.
type sharingState struct {
	feature sharingFeature
	// swapped records that the stock plugin has been removed, so restoration
	// knows there is something to undo. Set before the delete is issued: if the
	// delete succeeds but the wait times out, the plugin is already gone.
	swapped bool
	// ready records that the replacement came up, so the assertions can skip
	// rather than each timing out against a plugin that never started.
	ready            bool
	renderedPlugin   []byte
	renderedWorkload []byte
}

// gpuSharingWorkloadTplVars renders job-gpu-sharing-workload.yaml. Name gives each
// feature its own Job and selector. ReportComputeMode is MPS-only: time-slicing has
// no use for it, since the mode it needs (DEFAULT) is also the mode a GPU with no
// sharing configured is in, so reading it would prove nothing.
type gpuSharingWorkloadTplVars struct {
	Name                  string
	NvidiaTestImage       string
	Completions           int
	ActiveDeadlineSeconds int
	HoldSeconds           int
	ReportComputeMode     bool
}

// devicePluginImage returns the plugin image the stock manifest pins, so the
// reconfigured plugins this package deploys cannot drift from it. Reading it out of
// the manifest rather than repeating the tag keeps one source of truth: a version
// bump in test/manifests/assets/nvidia-device-plugin.yaml carries automatically.
func devicePluginImage() (string, error) {
	m := regexp.MustCompile(`image:\s*(\S*k8s-device-plugin:\S+)`).FindSubmatch(manifests.NvidiaDevicePluginManifest)
	if m == nil {
		return "", fmt.Errorf("no k8s-device-plugin image found in the stock device plugin manifest")
	}
	return string(m[1]), nil
}

// allocatableGPUs returns the summed allocatable nvidia.com/gpu across nodes of the
// instance type under test.
func allocatableGPUs(ctx context.Context, cfg *envconf.Config) (int, error) {
	var nodes corev1.NodeList
	if err := cfg.Client().Resources().List(ctx, &nodes); err != nil {
		return 0, err
	}
	total := 0
	for _, n := range nodes.Items {
		if testConfig.NodeType != "" && n.Labels["node.kubernetes.io/instance-type"] != testConfig.NodeType {
			continue
		}
		q := n.Status.Allocatable["nvidia.com/gpu"]
		total += int(q.Value())
	}
	return total, nil
}

// restoreStockDevicePlugin puts the stock plugin back and removes the feature's
// ConfigMap. Shared because both features end the same way, and a divergence
// between two copies of this would leave one of them quietly not restoring.
//
// Callers with more to undo -- MPS also has a control daemon and the GPU compute
// mode -- do that first and then call this last.
func restoreStockDevicePlugin(ctx context.Context, cfg *envconf.Config, t *testing.T, st *sharingState) {
	if !st.swapped {
		// Setup failed before touching the plugin, so the stock one is still in
		// place and there is nothing to undo.
		return
	}
	// Same drain requirement as on the way in: the stock DaemonSet reuses this
	// name and selector.
	if err := deleteDaemonSetAndWait(ctx, cfg, devicePluginName); err != nil {
		t.Errorf("[%s] replacement device plugin did not go away: %v", st.feature.name, err)
		return
	}
	ds := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: devicePluginName, Namespace: devicePluginNamespace}}
	if err := fwext.ApplyManifests(cfg.Client().RESTConfig(), manifests.NvidiaDevicePluginManifest); err != nil {
		t.Errorf("[%s] failed to restore the stock device plugin: %v", st.feature.name, err)
		return
	}
	if err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).DaemonSetReady(ds),
		wait.WithContext(ctx), wait.WithTimeout(gpuSharingReadyTimeout)); err != nil {
		t.Errorf("[%s] stock device plugin did not become ready again: %v", st.feature.name, err)
	}
	// The ConfigMap is ours, so it goes too. Leaving it means a later run applying
	// the manifest gets AlreadyExists, which ApplyManifests discards, and the
	// replacement plugin then mounts the previous replica count.
	if err := deleteConfigMapAndWait(ctx, cfg, st.feature.configMapName); err != nil {
		t.Errorf("[%s] failed to remove %s: %v", st.feature.name, st.feature.configMapName, err)
	}
	log.Printf("[%s] stock device plugin restored", st.feature.name)
}

// deleteDaemonSetAndWait removes a DaemonSet in kube-system and waits for it to be
// gone.
//
// Foreground propagation keeps the object until its pods are gone, so waiting for
// the object covers both. That wait is not cosmetic: a replacement reusing the same
// name is created by applyManifests, which creates rather than patches, and
// processObjects discards per-object errors (internal/e2e/client.go) -- so racing
// the deletion would swallow an AlreadyExists and silently leave the old DaemonSet
// in place with nothing reporting a failure.
func deleteDaemonSetAndWait(ctx context.Context, cfg *envconf.Config, name string) error {
	res := cfg.Client().Resources()
	ds := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: devicePluginNamespace}}

	policy := metav1.DeletePropagationForeground
	if err := res.Delete(ctx, ds, func(o *metav1.DeleteOptions) { o.PropagationPolicy = &policy }); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("deleting %s: %w", name, err)
		}
	}
	if err := wait.For(func(ctx context.Context) (bool, error) {
		err := res.Get(ctx, ds.GetName(), ds.GetNamespace(), ds)
		return apierrors.IsNotFound(err), nil
	}, wait.WithContext(ctx), wait.WithTimeout(2*time.Minute)); err != nil {
		return fmt.Errorf("waiting for %s to be deleted: %w", name, err)
	}
	return nil
}

// deleteConfigMapAndWait removes a ConfigMap a feature created and waits for it to
// go, so a later run renders its own configuration rather than silently reusing
// this one's. A missing ConfigMap is the expected case.
func deleteConfigMapAndWait(ctx context.Context, cfg *envconf.Config, name string) error {
	res := cfg.Client().Resources()
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: devicePluginNamespace}}
	if err := res.Get(ctx, cm.GetName(), cm.GetNamespace(), cm); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := res.Delete(ctx, cm); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return wait.For(func(ctx context.Context) (bool, error) {
		err := res.Get(ctx, cm.GetName(), cm.GetNamespace(), cm)
		return apierrors.IsNotFound(err), nil
	}, wait.WithContext(ctx), wait.WithTimeout(1*time.Minute))
}

// observeConcurrentPods polls the workload pods and returns the peak number seen in
// Running at the same instant, returning as soon as want is reached.
//
// This is the check that separates real sharing from Kubernetes merely queueing the
// pods. A Job with parallelism N over one physical GPU succeeds either way --
// without sharing the pods just take turns -- so only simultaneous Running
// demonstrates that the device is genuinely shared.
func observeConcurrentPods(ctx context.Context, cfg *envconf.Config, logPrefix, namespace, selector string, want int, timeout time.Duration) (int, error) {
	res := cfg.Client().Resources(namespace)
	peak := 0
	deadline := time.Now().Add(timeout)
	for {
		var pods corev1.PodList
		if err := res.List(ctx, &pods, resources.WithLabelSelector(selector)); err != nil {
			return peak, err
		}
		running, finished := 0, 0
		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case corev1.PodRunning:
				running++
			case corev1.PodSucceeded, corev1.PodFailed:
				finished++
			}
		}
		if running > peak {
			peak = running
			log.Printf("[%s] concurrent Running pods: %d (want %d)", logPrefix, peak, want)
		}
		if peak >= want {
			return peak, nil
		}
		// Once every pod is terminal the peak can no longer rise, so report the
		// shortfall now instead of burning the whole timeout.
		if len(pods.Items) >= want && finished == len(pods.Items) {
			return peak, fmt.Errorf("all %d pods reached a terminal phase having never exceeded %d running at once", len(pods.Items), peak)
		}
		if !time.Now().Before(deadline) {
			return peak, fmt.Errorf("timed out after %v with a peak of %d concurrent pods", timeout, peak)
		}
		select {
		case <-ctx.Done():
			return peak, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// workloadPodLogs returns the logs of every pod matching selector, keyed by pod
// name. fwext.GetJobLogs only reads pods.Items[0], which is not enough here: the
// point of these features is what the *other* pods did.
func workloadPodLogs(ctx context.Context, cfg *envconf.Config, namespace, selector string) map[string]string {
	out := map[string]string{}
	var pods corev1.PodList
	if err := cfg.Client().Resources(namespace).List(ctx, &pods, resources.WithLabelSelector(selector)); err != nil {
		return out
	}
	for _, pod := range pods.Items {
		out[pod.Name] = podLogs(ctx, cfg, namespace, pod.Name, false)
	}
	return out
}

// podLogs fetches a pod's container output. fwext only exposes GetJobLogs, and a
// DaemonSet pod is not a Job, so go through the clientset directly. Best effort: a
// pod stuck in ContainerCreating has no logs yet, and that absence is itself part
// of the diagnosis.
func podLogs(ctx context.Context, cfg *envconf.Config, namespace, name string, previous bool) string {
	clientset, err := kubernetes.NewForConfig(cfg.Client().RESTConfig())
	if err != nil {
		return ""
	}
	stream, err := clientset.CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{Previous: previous}).Stream(ctx)
	if err != nil {
		return fmt.Sprintf("(no logs available: %v)", err)
	}
	defer stream.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, stream); err != nil {
		return fmt.Sprintf("(log read failed: %v)", err)
	}
	return buf.String()
}

// markerValues extracts the value each pod printed for marker, keyed by pod name.
func markerValues(logsByPod map[string]string, marker string) map[string]string {
	out := map[string]string{}
	for pod, logs := range logsByPod {
		for _, line := range strings.Split(logs, "\n") {
			line = strings.TrimSpace(line)
			// An empty value is not a reading: it would otherwise count towards
			// the per-pod total and collapse to a single distinct value, so a pod
			// that never reached the GPU would satisfy both assertions.
			if v, ok := strings.CutPrefix(line, marker); ok && v != "" {
				out[pod] = v
				break
			}
		}
	}
	return out
}

// distinctValues returns the set of distinct values in m.
func distinctValues(m map[string]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range m {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func mapKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// Context keys for passing the observed GPU counts between steps, rather than
// recomputing and risking the two assertions disagreeing.
type allocatableBeforeKey struct{}
type allocatableAfterKey struct{}

// Carries the workload pod logs from the assertion to Teardown.
type workloadLogsKey struct{}
