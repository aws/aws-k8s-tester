//go:build e2e

package nvidia

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"log"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/aws-k8s-tester/test/manifests"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	//go:embed manifests/time-slicing-device-plugin.yaml
	timeSlicingDevicePluginManifest         []byte
	renderedTimeSlicingDevicePluginManifest []byte
	//go:embed manifests/job-time-slicing-workload.yaml
	timeSlicingWorkloadManifest         []byte
	renderedTimeSlicingWorkloadManifest []byte
)

const (
	timeSlicingJobName       = "time-slicing-workload"
	devicePluginName         = "nvidia-device-plugin-daemonset"
	devicePluginNamespace    = "kube-system"
	devicePluginPodSelector  = "name=nvidia-device-plugin-ds"
	timeSlicingConfigMapName = "nvidia-time-slicing-config"

	// How long to wait for the node's allocatable count to reflect the
	// reconfigured plugin. The plugin re-registers with kubelet on startup and
	// kubelet then updates node status, so this is not instant.
	timeSlicingAdvertiseTimeout = 5 * time.Minute
	// The workload is vectorAdd per pod, so this only needs to cover image pull
	// plus scheduling all the pods onto one GPU.
	timeSlicingWorkloadTimeout = 15 * time.Minute
	// How long each pod keeps re-running vectorAdd. Long enough that every pod
	// is still working while the others start, so the concurrency check has a
	// window to observe; short enough not to dominate the Job's runtime.
	timeSlicingHold = 90 * time.Second
	// Covers image pull and pod startup before the overlap window opens.
	timeSlicingConcurrencyTimeout = 10 * time.Minute

	timeSlicingWorkloadSelector = "app=time-slicing-workload"
	// Emitted by each workload pod; see job-time-slicing-workload.yaml.
	gpuUUIDMarker = "GPU_UUID="
)

// Whether Setup got as far as removing the stock plugin, and whether the
// replacement came up. Teardown keys off the first to decide if there is
// anything to restore; the assertions key off the second so they fail fast
// rather than timing out against a plugin that never started.
var (
	pluginSwapped bool
	pluginReady   bool
)

type timeSlicingPluginTplVars struct {
	Replicas int
}

type timeSlicingWorkloadTplVars struct {
	NvidiaTestImage       string
	Completions           int
	ActiveDeadlineSeconds int
	HoldSeconds           int
}

// allocatableGPUs returns the summed allocatable nvidia.com/gpu across nodes of
// the instance type under test.
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

// restoreDevicePlugin puts the stock device plugin back. Called from Teardown so
// later features do not inherit a time-sliced GPU count: an oversubscribed node
// would let a test that expects exclusive access schedule alongside something
// else and produce nonsense.
func restoreDevicePlugin(ctx context.Context, cfg *envconf.Config, t *testing.T) {
	if !pluginSwapped {
		// Setup failed before touching the plugin, so the stock one is still in
		// place and there is nothing to undo.
		return
	}
	// Same drain requirement as on the way in: the stock DaemonSet reuses this
	// name and selector.
	if err := deleteDevicePluginAndWait(ctx, cfg); err != nil {
		t.Errorf("time-slicing device plugin did not go away: %v", err)
		return
	}
	ds := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: devicePluginName, Namespace: devicePluginNamespace}}
	if err := fwext.ApplyManifests(cfg.Client().RESTConfig(), manifests.NvidiaDevicePluginManifest); err != nil {
		t.Errorf("failed to restore the stock device plugin: %v", err)
		return
	}
	if err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).DaemonSetReady(ds),
		wait.WithContext(ctx), wait.WithTimeout(5*time.Minute)); err != nil {
		t.Errorf("stock device plugin did not become ready again: %v", err)
	}
	// The ConfigMap is ours, so it goes too. Leaving it means a later run
	// applying this manifest gets AlreadyExists, which ApplyManifests discards,
	// and the replacement plugin then mounts the previous replica count.
	if err := deleteTimeSlicingConfigMap(ctx, cfg); err != nil {
		t.Errorf("failed to remove %s: %v", timeSlicingConfigMapName, err)
	}
	log.Printf("[time-slicing] stock device plugin restored")
}

// deleteDevicePluginAndWait removes the device-plugin DaemonSet and waits for
// both the object and its pods to be gone.
//
// Draining the pods is the load-bearing part. The replacement DaemonSet reuses
// the same name and selector, so if a pod from the old one is still Terminating
// when the new one is created, the new controller adopts that dying pod on the
// node and reports NumberReady=0 forever. Deleting with background propagation
// and waiting only for the DaemonSet object is therefore not enough -- that is
// exactly how this failed the first time it ran.
func deleteDevicePluginAndWait(ctx context.Context, cfg *envconf.Config) error {
	res := cfg.Client().Resources()
	ds := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: devicePluginName, Namespace: devicePluginNamespace}}

	policy := metav1.DeletePropagationForeground
	if err := res.Delete(ctx, ds, func(o *metav1.DeleteOptions) { o.PropagationPolicy = &policy }); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("deleting %s: %w", devicePluginName, err)
		}
	}

	// Foreground propagation keeps the object until its pods are gone, so
	// waiting for the object covers both. The wait is not cosmetic: the
	// replacement reuses this name, applyManifests creates rather than patches,
	// and processObjects discards per-object errors (internal/e2e/client.go), so
	// racing the deletion would swallow an AlreadyExists and silently leave the
	// stock plugin in place.
	if err := wait.For(func(ctx context.Context) (bool, error) {
		err := res.Get(ctx, ds.GetName(), ds.GetNamespace(), ds)
		return apierrors.IsNotFound(err), nil
	}, wait.WithContext(ctx), wait.WithTimeout(2*time.Minute)); err != nil {
		return fmt.Errorf("waiting for %s to be deleted: %w", devicePluginName, err)
	}
	return nil
}

// deleteTimeSlicingConfigMap removes the ConfigMap this feature creates and
// waits for it to go, so a later run renders its own replica count rather than
// silently reusing this one's. A missing ConfigMap is the expected case.
func deleteTimeSlicingConfigMap(ctx context.Context, cfg *envconf.Config) error {
	res := cfg.Client().Resources()
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: timeSlicingConfigMapName, Namespace: devicePluginNamespace}}
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

// watchDevicePluginRollout logs DaemonSet counters and pod state every 15s while
// the caller waits, so a failure comes with a timeline instead of one snapshot.
// The returned function stops it.
func watchDevicePluginRollout(ctx context.Context, cfg *envconf.Config) func() {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		t := time.NewTicker(15 * time.Second)
		defer t.Stop()
		res := cfg.Client().Resources()
		started := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				el := time.Since(started).Round(time.Second)
				var ds appsv1.DaemonSet
				if err := res.Get(ctx, devicePluginName, devicePluginNamespace, &ds); err != nil {
					log.Printf("[time-slicing +%s] DaemonSet not readable: %v", el, err)
					continue
				}
				st := ds.Status
				var pods corev1.PodList
				desc := "no pods"
				if err := res.List(ctx, &pods, resources.WithLabelSelector(devicePluginPodSelector)); err == nil {
					parts := []string{}
					for _, p := range pods.Items {
						if p.Namespace != devicePluginNamespace {
							continue
						}
						reason := string(p.Status.Phase)
						for _, cs := range p.Status.ContainerStatuses {
							if w := cs.State.Waiting; w != nil {
								reason = w.Reason
							}
						}
						parts = append(parts, fmt.Sprintf("%s=%s", p.Name, reason))
					}
					if len(parts) > 0 {
						desc = strings.Join(parts, " ")
					}
				}
				log.Printf("[time-slicing +%s] desired=%d ready=%d unavailable=%d | %s",
					el, st.DesiredNumberScheduled, st.NumberReady, st.NumberUnavailable, desc)
			}
		}
	}()
	return func() { cancel(); <-done }
}

// dumpDevicePluginDiagnostics prints everything needed to explain a readiness
// failure: the DaemonSet's own counters, each pod's phase and container waiting
// reason, recent events, and any container output.
//
// Without this a failure is just "did not become ready", which is what the first
// two runs of this test produced -- enough to know it broke, not enough to know
// why. fwext.ApplyManifests cannot be relied on to surface the cause either,
// because processObjects discards per-object Create errors
// (internal/e2e/client.go), so a missing ConfigMap looks identical to a
// crashlooping container from the caller's side.
func dumpDevicePluginDiagnostics(ctx context.Context, cfg *envconf.Config) {
	res := cfg.Client().Resources()

	// Did the ConfigMap the pod mounts actually get created? A swallowed Create
	// error here leaves the pod stuck in ContainerCreating indefinitely.
	var cm corev1.ConfigMap
	if err := res.Get(ctx, timeSlicingConfigMapName, devicePluginNamespace, &cm); err != nil {
		log.Printf("[time-slicing][diag] ConfigMap %s/%s NOT FOUND: %v -- the pod cannot mount its config",
			devicePluginNamespace, timeSlicingConfigMapName, err)
	} else {
		log.Printf("[time-slicing][diag] ConfigMap %s/%s exists, keys=%v",
			devicePluginNamespace, timeSlicingConfigMapName, mapKeys(cm.Data))
	}

	var ds appsv1.DaemonSet
	if err := res.Get(ctx, devicePluginName, devicePluginNamespace, &ds); err != nil {
		log.Printf("[time-slicing][diag] DaemonSet not readable: %v", err)
	} else {
		st := ds.Status
		log.Printf("[time-slicing][diag] DaemonSet desired=%d current=%d ready=%d available=%d unavailable=%d misscheduled=%d",
			st.DesiredNumberScheduled, st.CurrentNumberScheduled, st.NumberReady,
			st.NumberAvailable, st.NumberUnavailable, st.NumberMisscheduled)
	}

	var pods corev1.PodList
	if err := res.List(ctx, &pods, resources.WithLabelSelector(devicePluginPodSelector)); err != nil {
		log.Printf("[time-slicing][diag] could not list plugin pods: %v", err)
		return
	}
	found := 0
	for _, pod := range pods.Items {
		if pod.Namespace != devicePluginNamespace {
			continue
		}
		found++
		log.Printf("[time-slicing][diag] pod %s phase=%s node=%s", pod.Name, pod.Status.Phase, pod.Spec.NodeName)
		for _, c := range pod.Status.Conditions {
			log.Printf("[time-slicing][diag]   condition %s=%s %s %s", c.Type, c.Status, c.Reason, c.Message)
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if w := cs.State.Waiting; w != nil {
				log.Printf("[time-slicing][diag]   container %s WAITING reason=%s message=%s", cs.Name, w.Reason, w.Message)
			}
			if tm := cs.State.Terminated; tm != nil {
				log.Printf("[time-slicing][diag]   container %s TERMINATED exit=%d reason=%s", cs.Name, tm.ExitCode, tm.Reason)
			}
			if cs.RestartCount > 0 {
				log.Printf("[time-slicing][diag]   container %s restarts=%d", cs.Name, cs.RestartCount)
			}
		}
		if out := podLogs(ctx, cfg, pod.Namespace, pod.Name, false); out != "" {
			log.Printf("[time-slicing][diag] pod %s logs:\n%s", pod.Name, out)
		}
		// A crashlooping container has no current logs; the useful output is
		// from the instance that already died.
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.RestartCount > 0 {
				if out := podLogs(ctx, cfg, pod.Namespace, pod.Name, true); out != "" {
					log.Printf("[time-slicing][diag] pod %s PREVIOUS logs:\n%s", pod.Name, out)
				}
				break
			}
		}
		// If it never got scheduled, the reason is on the node, not the pod.
		if pod.Spec.NodeName == "" || pod.Status.Phase == corev1.PodPending {
			dumpNodeState(ctx, cfg)
		}
	}
	if found == 0 {
		log.Printf("[time-slicing][diag] NO pods matched %q in %s -- the DaemonSet controller created none, so look at the DaemonSet events above",
			devicePluginPodSelector, devicePluginNamespace)
	}

	// Include Normal events, not just Warnings: Scheduled / Pulling / Pulled /
	// Created / Started are precisely what show how far the pod got before it
	// stalled, and their absence is as informative as their content.
	var events corev1.EventList
	if err := res.List(ctx, &events); err == nil {
		for _, e := range events.Items {
			if e.Namespace != devicePluginNamespace {
				continue
			}
			if !strings.Contains(e.InvolvedObject.Name, "nvidia-device-plugin") {
				continue
			}
			log.Printf("[time-slicing][diag] event %s %s/%s %s: %s",
				e.Type, e.InvolvedObject.Kind, e.InvolvedObject.Name, e.Reason, e.Message)
		}
	}
}

// podLogs fetches a pod's container output. fwext only exposes GetJobLogs, and a
// DaemonSet pod is not a Job, so go through the clientset directly. Best effort:
// a pod stuck in ContainerCreating has no logs yet, and that absence is itself
// part of the diagnosis.
// dumpNodeState explains a Pending pod: taints it may not tolerate, and whether
// the node is actually schedulable.
func dumpNodeState(ctx context.Context, cfg *envconf.Config) {
	var nodes corev1.NodeList
	if err := cfg.Client().Resources().List(ctx, &nodes); err != nil {
		return
	}
	for _, n := range nodes.Items {
		gpuQ := n.Status.Allocatable["nvidia.com/gpu"]
		log.Printf("[time-slicing][diag] node %s unschedulable=%v allocatable-gpu=%d",
			n.Name, n.Spec.Unschedulable, gpuQ.Value())
		for _, tn := range n.Spec.Taints {
			log.Printf("[time-slicing][diag]   taint %s=%s:%s", tn.Key, tn.Value, tn.Effect)
		}
		for _, c := range n.Status.Conditions {
			if c.Type == corev1.NodeReady || c.Status != corev1.ConditionFalse {
				log.Printf("[time-slicing][diag]   condition %s=%s %s", c.Type, c.Status, c.Reason)
			}
		}
	}
}

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

// observeConcurrentPods polls the workload pods and returns the peak number seen
// in Running at the same instant, returning as soon as want is reached.
//
// This is the check that separates working time-slicing from Kubernetes merely
// queueing the pods. A Job with parallelism N over one physical GPU succeeds
// either way -- without slicing the pods just take turns -- so only simultaneous
// Running demonstrates that the device is genuinely shared.
func observeConcurrentPods(ctx context.Context, cfg *envconf.Config, namespace, selector string, want int, timeout time.Duration) (int, error) {
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
			log.Printf("[time-slicing] concurrent Running pods: %d (want %d)", peak, want)
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
// name. fwext.GetJobLogs only reads pods.Items[0], which is not enough here:
// the point of the feature is what the *other* pods did.
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

// gpuUUIDs extracts the GPU each pod reported opening, keyed by pod name.
func gpuUUIDs(logsByPod map[string]string) map[string]string {
	out := map[string]string{}
	for pod, logs := range logsByPod {
		for _, line := range strings.Split(logs, "\n") {
			line = strings.TrimSpace(line)
			// An empty value is not a reading: it would otherwise count towards
			// the per-pod total and collapse to a single distinct value, so a
			// pod that never reached the GPU would satisfy both assertions.
			if uuid, ok := strings.CutPrefix(line, gpuUUIDMarker); ok && uuid != "" {
				out[pod] = uuid
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

// TestTimeSlicing validates NVIDIA GPU time-slicing: the device plugin
// advertising one physical GPU N times, with the driver time-sharing between the
// pods that land on it.
//
// Unlike MIG there is no hardware isolation here -- no dedicated memory, no
// fault containment -- so the only thing worth asserting is that
// oversubscription works: more concurrent pods than there are physical GPUs,
// each doing real CUDA work.
//
// IMPORTANT -- opt-in via --timeSlicingEnabled, and it must stay that way for a
// stronger reason than the DCGM feature. This is the only feature here that
// mutates cluster-wide state: it replaces the device plugin, so while it runs
// every GPU on the node is oversubscribed. Some harnesses invoke this binary
// without -test.run, and on those paths an unguarded run would change GPU
// advertisement underneath the other features.
//
// It also requires --installDevicePlugin. Where the plugin is supplied by the
// platform rather than by this suite (EKS Auto ships its own), reconfiguring it
// is not ours to do, so the feature skips instead.
func TestTimeSlicing(t *testing.T) {
	if !testConfig.TimeSlicingEnabled {
		t.Skip("skipping time-slicing; set --timeSlicingEnabled to run (reconfigures the cluster's device plugin)")
	}
	if !testConfig.InstallDevicePlugin {
		t.Skip("skipping time-slicing; requires --installDevicePlugin so the plugin under reconfiguration is one this suite owns")
	}

	replicas := testConfig.TimeSlicingReplicas

	feat := features.New("time-slicing").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if testConfig.NvidiaTestImage == "" {
				t.Fatal(fmt.Errorf("nvidiaTestImage must be set to run time-slicing"))
			}
			if replicas < 2 {
				t.Fatalf("timeSlicingReplicas must be at least 2 to oversubscribe, got %d", replicas)
			}

			// Record the pre-change count so the assertion below compares
			// against what this cluster actually advertised, rather than
			// assuming gpuPerNode is still accurate.
			before, err := allocatableGPUs(ctx, cfg)
			if err != nil {
				t.Fatalf("failed to read allocatable GPUs: %v", err)
			}
			if before == 0 {
				t.Fatal("no allocatable nvidia.com/gpu before reconfiguring; the stock device plugin is not advertising")
			}
			log.Printf("[time-slicing] allocatable nvidia.com/gpu before: %d", before)

			renderedTimeSlicingDevicePluginManifest, err = fwext.RenderManifests(
				timeSlicingDevicePluginManifest, timeSlicingPluginTplVars{Replicas: replicas})
			if err != nil {
				t.Fatal(err)
			}
			// Set before the delete is issued, not after: if the delete
			// succeeds but the wait for it times out, the stock plugin is
			// already gone and Teardown still has to put it back. Restoration is
			// idempotent, so recording a mutation that did not happen is safe
			// while missing one is not.
			//
			// Replace rather than patch: the stock DaemonSet has no config
			// volume, and two plugins advertising nvidia.com/gpu would conflict.
			pluginSwapped = true
			// A run killed before Teardown leaves the ConfigMap behind, and the
			// create below would then be a discarded AlreadyExists, mounting the
			// old replica count.
			if err := deleteTimeSlicingConfigMap(ctx, cfg); err != nil {
				t.Errorf("failed to clear a stale %s: %v", timeSlicingConfigMapName, err)
				return ctx
			}
			// From here on the cluster is modified, so failures must not use
			// t.Fatal: that aborts the goroutine and Teardown never runs, which
			// would leave the node without any device plugin.
			if err := deleteDevicePluginAndWait(ctx, cfg); err != nil {
				t.Errorf("failed to remove the stock device plugin: %v", err)
				return ctx
			}
			ds := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: devicePluginName, Namespace: devicePluginNamespace}}
			if err := fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedTimeSlicingDevicePluginManifest); err != nil {
				t.Errorf("failed to deploy the time-slicing device plugin: %v", err)
				return ctx
			}
			// Report state while waiting rather than only after the timeout. A
			// single end-state snapshot cannot distinguish "never started" from
			// "started and died" from "nearly ready"; the progression can.
			stopWatch := watchDevicePluginRollout(ctx, cfg)
			readyErr := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).DaemonSetReady(ds),
				wait.WithContext(ctx), wait.WithTimeout(5*time.Minute))
			stopWatch()
			if readyErr != nil {
				t.Errorf("time-slicing device plugin did not become ready: %v", readyErr)
				dumpDevicePluginDiagnostics(ctx, cfg)
				return ctx
			}
			pluginReady = true
			ctx = context.WithValue(ctx, allocatableBeforeKey{}, before)
			return ctx
		}).
		Assess("node advertises time-sliced GPUs", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if !pluginReady {
				t.Skip("time-slicing plugin never became ready; see the Setup failure above")
			}
			before, _ := ctx.Value(allocatableBeforeKey{}).(int)
			want := before * replicas

			// kubelet updates node status only after the plugin re-registers, so
			// poll rather than reading once.
			var got int
			err := wait.For(func(ctx context.Context) (bool, error) {
				var err error
				got, err = allocatableGPUs(ctx, cfg)
				if err != nil {
					return false, err
				}
				return got == want, nil
			}, wait.WithContext(ctx), wait.WithTimeout(timeSlicingAdvertiseTimeout))
			if err != nil {
				t.Fatalf("allocatable nvidia.com/gpu = %d, want %d (%d physical x %d replicas): %v",
					got, want, before, replicas, err)
			}
			log.Printf("[time-slicing] allocatable nvidia.com/gpu after: %d (%d physical x %d replicas)", got, before, replicas)
			ctx = context.WithValue(ctx, allocatableAfterKey{}, got)
			return ctx
		}).
		Assess("oversubscribed pods share a physical GPU concurrently", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if !pluginReady {
				t.Skip("time-slicing plugin never became ready; see the Setup failure above")
			}
			completions, _ := ctx.Value(allocatableAfterKey{}).(int)
			if completions == 0 {
				t.Fatal("no time-sliced GPU count available from the previous step")
			}
			physical, _ := ctx.Value(allocatableBeforeKey{}).(int)
			var err error
			renderedTimeSlicingWorkloadManifest, err = fwext.RenderManifests(
				timeSlicingWorkloadManifest, timeSlicingWorkloadTplVars{
					NvidiaTestImage:       testConfig.NvidiaTestImage,
					Completions:           completions,
					ActiveDeadlineSeconds: int(timeSlicingWorkloadTimeout.Seconds()) - 60,
					HoldSeconds:           int(timeSlicingHold.Seconds()),
				})
			if err != nil {
				t.Fatal(err)
			}
			// A Job of this name left by an interrupted run would otherwise be
			// inspected instead of the one rendered here, since the create error
			// is discarded (see requireJobExists).
			if err := deleteStaleJob(ctx, cfg, timeSlicingJobName); err != nil {
				t.Errorf("failed to remove a stale %s Job: %v", timeSlicingJobName, err)
				return ctx
			}
			if err := fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedTimeSlicingWorkloadManifest); err != nil {
				t.Errorf("failed to apply the time-slicing workload: %v", err)
				return ctx
			}
			if err := requireJobExists(ctx, cfg, timeSlicingJobName); err != nil {
				t.Errorf("%v", err)
				return ctx
			}

			// Observe the overlap while the pods are still alive; once the Job
			// has succeeded it is too late to tell concurrent execution from
			// sequential.
			log.Printf("[time-slicing] waiting for %d pods to run concurrently on %d physical GPU(s)", completions, physical)
			peak, err := observeConcurrentPods(ctx, cfg, "default", timeSlicingWorkloadSelector, completions, timeSlicingConcurrencyTimeout)
			if err != nil {
				// Report rather than Fatal so Teardown still restores the plugin
				// and dumps the pod logs.
				t.Errorf("pods never ran concurrently on the time-sliced GPU (peak %d, want %d): %v", peak, completions, err)
			} else {
				log.Printf("[time-slicing] %d pods ran concurrently on %d physical GPU(s)", peak, physical)
			}

			job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: timeSlicingJobName, Namespace: "default"}}
			if err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).JobSucceeded(job),
				wait.WithContext(ctx), wait.WithTimeout(timeSlicingWorkloadTimeout)); err != nil {
				t.Errorf("oversubscribed workload did not complete: %v", err)
			}

			// Read the logs before Teardown deletes the Job.
			logsByPod := workloadPodLogs(ctx, cfg, "default", timeSlicingWorkloadSelector)
			uuids := gpuUUIDs(logsByPod)
			if len(uuids) != completions {
				t.Errorf("only %d of %d pods reported a GPU UUID", len(uuids), completions)
			}
			for pod, uuid := range uuids {
				log.Printf("[time-slicing] pod %s used GPU %s", pod, uuid)
			}
			// With several physical GPUs the scheduler is free to spread the
			// pods across them, so identical UUIDs are only required when there
			// is one GPU for them all to share.
			if physical == 1 {
				if distinct := distinctValues(uuids); len(distinct) != 1 {
					t.Errorf("pods reported %d distinct GPU UUIDs %v on a single-GPU node; they did not share one physical GPU", len(distinct), distinct)
				}
			}
			ctx = context.WithValue(ctx, workloadLogsKey{}, logsByPod)
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if renderedTimeSlicingWorkloadManifest != nil {
				// Prefer the logs the assertion already collected; re-reading
				// after a failure may find the pods gone.
				logsByPod, _ := ctx.Value(workloadLogsKey{}).(map[string]string)
				if len(logsByPod) == 0 {
					logsByPod = workloadPodLogs(ctx, cfg, "default", timeSlicingWorkloadSelector)
				}
				for _, pod := range slices.Sorted(maps.Keys(logsByPod)) {
					t.Logf("Test log for pod %s:\n%s", pod, logsByPod[pod])
				}
				if err := fwext.DeleteManifests(cfg.Client().RESTConfig(), renderedTimeSlicingWorkloadManifest); err != nil {
					t.Errorf("failed to delete the time-slicing workload: %v", err)
				}
			}
			// Always attempt this, even if the assertions failed: leaving the
			// cluster oversubscribed would corrupt any feature that runs next.
			restoreDevicePlugin(ctx, cfg, t)
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}

// Context keys for passing the observed GPU counts between steps, rather than
// recomputing and risking the two assertions disagreeing.
type allocatableBeforeKey struct{}
type allocatableAfterKey struct{}

// Carries the workload pod logs from the assertion to Teardown.
type workloadLogsKey struct{}
