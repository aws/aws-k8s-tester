//go:build e2e

package nvidia

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

var (
	//go:embed manifests/mps-device-plugin.yaml
	mpsDevicePluginManifest []byte
	//go:embed manifests/daemonset-mps-compute-mode-reset.yaml
	mpsComputeModeResetManifest []byte
)

const (
	mpsJobName            = "mps-workload"
	mpsResetName          = "mps-compute-mode-reset"
	mpsResetLabel         = "name=mps-compute-mode-reset"
	mpsControlDaemonName  = "nvidia-device-plugin-mps-control-daemon"
	mpsControlDaemonLabel = "name=nvidia-device-plugin-mps-control-daemon"
	mpsConfigMapName      = "nvidia-mps-config"
	mpsWorkloadSelector   = "app=mps-workload"

	// Where the control daemon keeps its pipe, log and shm directories, and where
	// the plugin looks for them. The two must agree: if they do not, the plugin
	// cannot reach the daemon and MPS does not engage, with nothing reporting an
	// error.
	mpsRoot = "/run/nvidia/mps"

	mpsResetTimeout = 5 * time.Minute
	// How long to wait for the workload pods to be gone before removing the daemon
	// and resetting compute mode; they are killed, not waited out, so this only
	// covers termination.
	mpsDrainTimeout = 3 * time.Minute
	// Bound on restoration when it runs outside the test's own context.
	mpsCleanupTimeout = 15 * time.Minute

	// Emitted by each workload pod when ReportComputeMode is set; see
	// job-gpu-sharing-workload.yaml.
	computeModeMarker = "COMPUTE_MODE="
	// What nvidia-smi reports once the control daemon has taken the GPU. MPS
	// requires it, and it is the one signal that distinguishes MPS from the plugin
	// merely advertising more devices.
	exclusiveProcessComputeMode = "Exclusive_Process"
)

// mpsState is one TestMps invocation's state: the shared sharing lifecycle plus what
// only MPS has.
type mpsState struct {
	sharingState
	replicas    int
	physical    int
	completions int
	// restoreOnce guards restoration, which both Teardown and t.Cleanup call.
	// Scoped to the invocation so a second one in the same binary is unaffected.
	restoreOnce   *sync.Once
	renderedReset []byte
}

type mpsPluginTplVars struct {
	Replicas    int
	MpsRoot     string
	PluginImage string
}

type mpsResetTplVars struct {
	NvidiaTestImage string
}

// TestMps validates NVIDIA Multi-Process Service: the device plugin advertises each
// physical GPU several times and a control daemon funnels every client through one
// shared CUDA context, so their kernels run at the same time instead of the driver
// switching between them as it does under time-slicing.
//
// Opt-in via --mpsEnabled. Beyond replacing the cluster's device plugin, which is
// reason enough on harnesses that invoke this binary without -test.run, the control
// daemon sets the GPU compute mode to EXCLUSIVE_PROCESS. Deleting the DaemonSet is
// enough to undo that -- the daemon restores DEFAULT as it shuts down, confirmed on
// hardware -- but only when it exits gracefully, so the feature resets the mode
// itself rather than relying on that.
func TestMps(t *testing.T) {
	if !testConfig.MpsEnabled {
		t.Skip("--mpsEnabled not set; skipping the MPS feature")
	}
	if !testConfig.InstallDevicePlugin {
		t.Skip("--installDevicePlugin=false; MPS reconfigures the plugin this suite deploys, so there is nothing for it to replace")
	}

	st := &mpsState{
		sharingState: sharingState{feature: sharingFeature{
			name:          "mps",
			configMapName: mpsConfigMapName,
		}},
		replicas:    testConfig.MpsReplicas,
		restoreOnce: new(sync.Once),
	}

	feature := features.New("mps").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			return setupMps(ctx, t, cfg, st)
		}).
		Assess("node advertises MPS-shared GPUs", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			return assessMpsAdvertisement(ctx, t, cfg, st)
		}).
		Assess("oversubscribed pods share a physical GPU concurrently", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			return assessMpsConcurrentSharing(ctx, t, cfg, st)
		}).
		Assess("GPU is in EXCLUSIVE_PROCESS compute mode", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			return assessMpsComputeMode(ctx, t, st)
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Deliberately not ctx. An interrupted run cancels the test context
			// before Teardown, so restoring with it would fail every API call and
			// still consume the one-shot guard, leaving nothing for t.Cleanup to
			// retry with.
			cleanupCtx, cancel := cleanupContext()
			defer cancel()
			restoreFromMps(cleanupCtx, cfg, t, st)
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

// setupMps replaces the stock device plugin with the MPS one and its control daemon,
// and registers restoration before anything is changed.
func setupMps(ctx context.Context, t *testing.T, cfg *envconf.Config, st *mpsState) context.Context {
	if testConfig.NvidiaTestImage == "" {
		t.Fatal(fmt.Errorf("nvidiaTestImage must be set to run MPS"))
	}
	if st.replicas < 2 {
		t.Fatalf("mpsReplicas must be at least 2 to oversubscribe, got %d", st.replicas)
	}

	// Compare against what this cluster actually advertised rather than assuming
	// gpuPerNode is still accurate.
	before, err := allocatableGPUs(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to read allocatable GPUs: %v", err)
	}
	if before == 0 {
		t.Fatal("no allocatable nvidia.com/gpu before reconfiguring; the stock device plugin is not advertising")
	}
	st.physical = before
	st.completions = before * st.replicas
	log.Printf("[mps] allocatable nvidia.com/gpu before: %d", before)

	pluginImage, err := devicePluginImage()
	if err != nil {
		t.Fatal(err)
	}
	st.renderedPlugin, err = fwext.RenderManifests(
		mpsDevicePluginManifest, mpsPluginTplVars{
			Replicas:    st.replicas,
			MpsRoot:     mpsRoot,
			PluginImage: pluginImage,
		})
	if err != nil {
		t.Fatal(err)
	}

	// Set before the delete is issued, not after: if the delete succeeds but the
	// wait for it times out, the stock plugin is already gone and restoration still
	// has to put it back. Undoing a mutation that did not happen is harmless;
	// missing one is not.
	st.swapped = true
	// Registered here as well as in Teardown: Teardown does not run when the
	// framework aborts the test early, and without a restoration path the node
	// would be left with the MPS plugin and its GPUs in EXCLUSIVE_PROCESS.
	// Whichever runs first wins; the other is a no-op.
	t.Cleanup(func() {
		cleanupCtx, cancel := cleanupContext()
		defer cancel()
		restoreFromMps(cleanupCtx, cfg, t, st)
	})
	// A run killed before Teardown leaves the ConfigMap behind, and the create
	// below would then be a discarded AlreadyExists, leaving the plugin on the old
	// replica count.
	if err := deleteConfigMapAndWait(ctx, cfg, mpsConfigMapName); err != nil {
		t.Errorf("failed to clear a stale %s: %v", mpsConfigMapName, err)
		return ctx
	}
	// From here on the cluster is modified, so Setup must not use t.Fatal: that
	// aborts the goroutine and Teardown never runs, which would leave the node with
	// no device plugin and its GPUs in EXCLUSIVE_PROCESS.
	if err := deleteDaemonSetAndWait(ctx, cfg, devicePluginName); err != nil {
		t.Errorf("failed to remove the stock device plugin: %v", err)
		return ctx
	}
	if err := fwext.ApplyManifests(cfg.Client().RESTConfig(), st.renderedPlugin); err != nil {
		t.Errorf("failed to deploy the MPS device plugin: %v", err)
		return ctx
	}

	stop := watchDevicePluginRollout(ctx, cfg, st.feature)
	defer stop()

	// Both DaemonSets have to be ready. The plugin alone advertising the multiplied
	// count proves nothing: without the control daemon there is no shared context,
	// so the pods would be sharing the GPU the way they do with no sharing
	// configured at all. An ordered slice rather than a map so the log reads the
	// same way every run.
	cond := fwext.NewConditionExtension(cfg.Client().Resources())
	for _, name := range []string{devicePluginName, mpsControlDaemonName} {
		ds := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: devicePluginNamespace}}
		if err := wait.For(cond.DaemonSetReady(ds),
			wait.WithContext(ctx), wait.WithTimeout(gpuSharingReadyTimeout)); err != nil {
			t.Errorf("%s did not become ready: %v", name, err)
			dumpDevicePluginDiagnostics(ctx, cfg, st.feature)
			dumpMpsControlDaemonDiagnostics(ctx, cfg)
			return ctx
		}
		// DaemonSetReady compares NumberReady against DesiredNumberScheduled, so a
		// DaemonSet that matched no nodes at all reports ready at 0/0. For the
		// control daemon that is the difference between MPS working and the plugin
		// advertising a multiplied count with nothing behind it.
		if ds.Status.DesiredNumberScheduled == 0 {
			t.Errorf("%s is scheduled on no nodes; it reports ready only because 0 of 0 pods are ready", name)
			dumpMpsControlDaemonDiagnostics(ctx, cfg)
			return ctx
		}
		log.Printf("[mps] %s ready on %d node(s)", name, ds.Status.DesiredNumberScheduled)
	}
	st.ready = true
	return ctx
}

// assessMpsAdvertisement waits for the node to report physical x replicas GPUs.
func assessMpsAdvertisement(ctx context.Context, t *testing.T, cfg *envconf.Config, st *mpsState) context.Context {
	if !st.ready {
		t.Skip("MPS plugin never became ready; see the Setup failure above")
	}
	var after int
	if err := wait.For(func(ctx context.Context) (bool, error) {
		n, err := allocatableGPUs(ctx, cfg)
		if err != nil {
			return false, err
		}
		after = n
		return n == st.completions, nil
	}, wait.WithContext(ctx), wait.WithTimeout(gpuSharingAdvertiseTimeout)); err != nil {
		t.Fatalf("allocatable nvidia.com/gpu = %d, want %d (%d physical x %d replicas): %v",
			after, st.completions, st.physical, st.replicas, err)
	}
	log.Printf("[mps] allocatable nvidia.com/gpu after: %d (%d physical x %d replicas)", after, st.physical, st.replicas)
	return ctx
}

// assessMpsConcurrentSharing runs the oversubscription workload and checks the pods
// really did run at the same time on the same physical GPU.
func assessMpsConcurrentSharing(ctx context.Context, t *testing.T, cfg *envconf.Config, st *mpsState) context.Context {
	if !st.ready {
		t.Skip("MPS plugin never became ready; see the Setup failure above")
	}
	var err error
	st.renderedWorkload, err = fwext.RenderManifests(
		gpuSharingWorkloadManifest, gpuSharingWorkloadTplVars{
			Name:                  mpsJobName,
			NvidiaTestImage:       testConfig.NvidiaTestImage,
			Completions:           st.completions,
			ActiveDeadlineSeconds: int(gpuSharingWorkloadTimeout.Seconds()) - 60,
			HoldSeconds:           int(gpuSharingHold.Seconds()),
			ReportComputeMode:     true,
		})
	if err != nil {
		t.Errorf("failed to render the MPS workload: %v", err)
		return ctx
	}
	// A Job of this name left by an interrupted run would otherwise be inspected
	// instead of the one rendered here, since the create error is discarded (see
	// requireJobExists).
	if err := deleteStaleJob(ctx, cfg, mpsJobName); err != nil {
		t.Errorf("failed to remove a stale %s Job: %v", mpsJobName, err)
		return ctx
	}
	if err := fwext.ApplyManifests(cfg.Client().RESTConfig(), st.renderedWorkload); err != nil {
		t.Errorf("failed to apply the MPS workload: %v", err)
		return ctx
	}
	if err := requireJobExists(ctx, cfg, mpsJobName); err != nil {
		t.Errorf("%v", err)
		return ctx
	}

	// Observe the overlap while the pods are still alive. Once the Job completes
	// there is nothing left to look at, and a Job with parallelism N over one GPU
	// completes even with no sharing at all, because each pod frees the GPU when it
	// exits and the next is then admitted.
	log.Printf("[mps] waiting for %d pods to run concurrently on %d physical GPU(s)", st.completions, st.physical)
	peak, err := observeConcurrentPods(ctx, cfg, st.feature.name, "default", mpsWorkloadSelector, st.completions, gpuSharingConcurrencyTimeout)
	if err != nil {
		t.Errorf("pods never ran concurrently on the MPS-shared GPU (peak %d, want %d): %v", peak, st.completions, err)
	} else {
		log.Printf("[mps] %d pods ran concurrently on %d physical GPU(s)", peak, st.physical)
	}

	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: mpsJobName, Namespace: "default"}}
	if err := wait.For(conditions.New(cfg.Client().Resources()).JobCompleted(job),
		wait.WithContext(ctx), wait.WithTimeout(gpuSharingWorkloadTimeout)); err != nil {
		t.Errorf("oversubscribed workload did not complete: %v", err)
	}

	logsByPod := workloadPodLogs(ctx, cfg, "default", mpsWorkloadSelector)
	uuids := markerValues(logsByPod, gpuUUIDMarker)
	if len(uuids) != st.completions {
		t.Errorf("only %d of %d pods reported a GPU UUID", len(uuids), st.completions)
	}
	for pod, uuid := range uuids {
		log.Printf("[mps] pod %s used GPU %s", pod, uuid)
	}
	// With several physical GPUs the scheduler may spread the pods, so identical
	// UUIDs are only required when there is one GPU to share.
	if st.physical == 1 {
		if distinct := distinctValues(uuids); len(distinct) != 1 {
			t.Errorf("pods reported %d distinct GPU UUIDs %v on a single-GPU node; they did not share one physical GPU", len(distinct), distinct)
		}
	}
	// Printed, not just parsed. The workload dumps nvidia-smi at the end, which is
	// the only view of what the GPU looked like under load. Its process list is
	// empty because the pod has no hostPID rather than because of anything MPS
	// does, so a per-process assertion -- the M+C type Bottlerocket checks -- would
	// need an observer with host PID visibility, not a change to this workload.
	for _, pod := range slices.Sorted(maps.Keys(logsByPod)) {
		t.Logf("Test log for pod %s:\n%s", pod, logsByPod[pod])
	}
	return context.WithValue(ctx, workloadLogsKey{}, logsByPod)
}

// assessMpsComputeMode is what separates MPS from time-slicing. Both multiply
// allocatable and both let pods share a GPU, but only MPS routes the clients through
// a control daemon, and the daemon takes the GPU in EXCLUSIVE_PROCESS. A run where
// the plugin came up with the MPS config but the daemon never took effect passes the
// two assertions above and fails here.
func assessMpsComputeMode(ctx context.Context, t *testing.T, st *mpsState) context.Context {
	if !st.ready {
		t.Skip("MPS plugin never became ready; see the Setup failure above")
	}
	logsByPod, _ := ctx.Value(workloadLogsKey{}).(map[string]string)
	if len(logsByPod) == 0 {
		t.Skip("no workload logs available from the previous step")
	}
	modes := markerValues(logsByPod, computeModeMarker)
	if len(modes) != len(logsByPod) {
		t.Errorf("only %d of %d pods reported a compute mode", len(modes), len(logsByPod))
	}
	for pod, mode := range modes {
		if !strings.EqualFold(mode, exclusiveProcessComputeMode) {
			t.Errorf("pod %s saw compute mode %q, want %q; the MPS control daemon did not take the GPU",
				pod, mode, exclusiveProcessComputeMode)
		}
	}
	if len(modes) > 0 {
		log.Printf("[mps] compute mode reported by %d pod(s): %v", len(modes), distinctValues(modes))
	}
	return ctx
}

// cleanupContext returns a context for restoration work that does not inherit the
// test's context. Restoration has to run when the test is being torn down by
// --fail-fast or an interrupt, and in both of those the test context is already
// cancelled, so every API call made with it would fail immediately.
func cleanupContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), mpsCleanupTimeout)
}

// restoreFromMps undoes everything setupMps changed. Both Teardown and t.Cleanup
// call it; restoreOnce ensures only the first does the work.
//
// Order is not cosmetic. The workload pods go first so nothing still holds a CUDA
// context on the GPU. Then the control daemon, which would otherwise re-apply
// EXCLUSIVE_PROCESS under the reset. Then the reset, then the stock plugin.
func restoreFromMps(ctx context.Context, cfg *envconf.Config, t *testing.T, st *mpsState) {
	st.restoreOnce.Do(func() {
		if !st.swapped {
			// Setup failed before touching anything, so there is nothing to undo.
			return
		}
		drainMpsWorkload(ctx, cfg, t, st)
		if err := deleteDaemonSetAndWait(ctx, cfg, mpsControlDaemonName); err != nil {
			t.Errorf("MPS control daemon did not go away: %v", err)
		}
		resetComputeMode(ctx, cfg, t, st)
		restoreStockDevicePlugin(ctx, cfg, t, &st.sharingState)
	})
}

// drainMpsWorkload removes the workload and waits for its pods to actually be gone.
// DeleteManifests propagates in the background and returns as soon as the Job object
// is accepted for deletion, so without this the daemon removal and the reset below
// would race pods that still hold CUDA contexts on the GPU.
func drainMpsWorkload(ctx context.Context, cfg *envconf.Config, t *testing.T, st *mpsState) {
	if st.renderedWorkload == nil {
		return
	}
	res := cfg.Client().Resources()
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: mpsJobName, Namespace: "default"}}
	policy := metav1.DeletePropagationForeground
	if err := res.Delete(ctx, job, func(o *metav1.DeleteOptions) { o.PropagationPolicy = &policy }); err != nil {
		if !apierrors.IsNotFound(err) {
			t.Errorf("failed to delete the MPS workload: %v", err)
		}
		return
	}
	if err := wait.For(func(ctx context.Context) (bool, error) {
		var pods corev1.PodList
		if err := res.List(ctx, &pods, resources.WithLabelSelector(mpsWorkloadSelector)); err != nil {
			return false, err
		}
		remaining := 0
		for _, pod := range pods.Items {
			if pod.Namespace == "default" {
				remaining++
			}
		}
		if remaining > 0 {
			log.Printf("[mps] waiting for %d workload pod(s) to terminate before resetting compute mode", remaining)
		}
		return remaining == 0, nil
	}, wait.WithContext(ctx), wait.WithTimeout(mpsDrainTimeout)); err != nil {
		t.Errorf("MPS workload pods did not terminate; the compute-mode reset may fail with the GPU still in use: %v", err)
	}
}

// resetComputeMode puts every GPU node back in DEFAULT compute mode. On hardware the
// control daemon already restores DEFAULT when it shuts down, so this is normally a
// no-op that reports "already set to DEFAULT"; it exists for the case where the
// daemon does not exit cleanly and the mode is left applied. A DaemonSet rather than
// a Job because the daemon ran on every GPU node, so each one needs resetting;
// readiness is gated on the reset having succeeded, so ready means every node is back
// in DEFAULT.
func resetComputeMode(ctx context.Context, cfg *envconf.Config, t *testing.T, st *mpsState) {
	var err error
	st.renderedReset, err = fwext.RenderManifests(
		mpsComputeModeResetManifest, mpsResetTplVars{NvidiaTestImage: testConfig.NvidiaTestImage})
	if err != nil {
		t.Errorf("failed to render the compute-mode reset DaemonSet: %v", err)
		return
	}
	if err := deleteDaemonSetAndWait(ctx, cfg, mpsResetName); err != nil {
		t.Errorf("failed to remove a stale %s DaemonSet: %v", mpsResetName, err)
		return
	}
	if err := fwext.ApplyManifests(cfg.Client().RESTConfig(), st.renderedReset); err != nil {
		t.Errorf("failed to apply the compute-mode reset DaemonSet: %v", err)
		return
	}
	ds := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: mpsResetName, Namespace: devicePluginNamespace}}
	if err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).DaemonSetReady(ds),
		wait.WithContext(ctx), wait.WithTimeout(mpsResetTimeout)); err != nil {
		// Worth more than one line: the nodes are now in a state that breaks
		// unrelated features, and each pod's own output names the GPUs still left
		// exclusive.
		t.Errorf("compute-mode reset did not complete; nodes may be left in EXCLUSIVE_PROCESS: %v", err)
		for pod, out := range workloadPodLogs(ctx, cfg, devicePluginNamespace, mpsResetLabel) {
			t.Logf("compute-mode reset log for %s:\n%s", pod, out)
		}
		return
	}
	if ds.Status.DesiredNumberScheduled == 0 {
		t.Errorf("compute-mode reset ran on no nodes; GPUs may be left in EXCLUSIVE_PROCESS")
		return
	}
	for pod, out := range workloadPodLogs(ctx, cfg, devicePluginNamespace, mpsResetLabel) {
		log.Printf("[mps] compute-mode reset (%s): %s", pod, strings.TrimSpace(out))
	}
	log.Printf("[mps] compute mode reset to Default on %d node(s)", ds.Status.DesiredNumberScheduled)
	if err := deleteDaemonSetAndWait(ctx, cfg, mpsResetName); err != nil {
		t.Errorf("failed to delete the compute-mode reset DaemonSet: %v", err)
	}
}

// dumpMpsControlDaemonDiagnostics reports why the control daemon did not start. Its
// failures look different from the plugin's: it is privileged, it mounts a hostPath
// the init container has to create first, and it competes for the GPU with anything
// already holding a context, so the container state and the init container's own
// logs are usually where the answer is.
func dumpMpsControlDaemonDiagnostics(ctx context.Context, cfg *envconf.Config) {
	res := cfg.Client().Resources()
	var ds appsv1.DaemonSet
	if err := res.Get(ctx, mpsControlDaemonName, devicePluginNamespace, &ds); err != nil {
		log.Printf("[mps] control daemon DaemonSet not found: %v", err)
	} else {
		log.Printf("[mps] control daemon desired=%d current=%d ready=%d available=%d misscheduled=%d",
			ds.Status.DesiredNumberScheduled, ds.Status.CurrentNumberScheduled,
			ds.Status.NumberReady, ds.Status.NumberAvailable, ds.Status.NumberMisscheduled)
		// DesiredNumberScheduled of 0 means it matched no nodes, which is what a
		// scheduling constraint no node satisfies looks like.
		if ds.Status.DesiredNumberScheduled == 0 {
			log.Printf("[mps] control daemon matched no nodes; check its tolerations against the node's taints")
		}
	}
	var pods corev1.PodList
	if err := res.List(ctx, &pods, resources.WithLabelSelector(mpsControlDaemonLabel)); err != nil {
		log.Printf("[mps] failed to list control daemon pods: %v", err)
		return
	}
	for _, pod := range pods.Items {
		log.Printf("[mps] control daemon pod %s phase=%s node=%s", pod.Name, pod.Status.Phase, pod.Spec.NodeName)
		for _, cs := range pod.Status.InitContainerStatuses {
			log.Printf("[mps]   init %s ready=%t state=%+v", cs.Name, cs.Ready, cs.State)
		}
		for _, cs := range pod.Status.ContainerStatuses {
			log.Printf("[mps]   ctr %s ready=%t restarts=%d state=%+v", cs.Name, cs.Ready, cs.RestartCount, cs.State)
		}
		if out := podLogs(ctx, cfg, pod.Namespace, pod.Name, false); out != "" {
			log.Printf("[mps]   logs for %s:\n%s", pod.Name, out)
		}
	}
}
