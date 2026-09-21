//go:build e2e

package nvidia

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"maps"
	"slices"
	"testing"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

var (
	//go:embed manifests/time-slicing-device-plugin.yaml
	timeSlicingDevicePluginManifest []byte
)

const (
	timeSlicingJobName          = "time-slicing-workload"
	timeSlicingConfigMapName    = "nvidia-time-slicing-config"
	timeSlicingWorkloadSelector = "app=time-slicing-workload"
)

type timeSlicingPluginTplVars struct {
	Replicas    int
	PluginImage string
}

// TestTimeSlicing validates NVIDIA GPU time-slicing: the device plugin advertising
// one physical GPU N times, with the driver time-sharing between the pods that land
// on it.
//
// Unlike MIG there is no hardware isolation here -- no dedicated memory, no fault
// containment -- so the only thing worth asserting is that oversubscription works:
// more concurrent pods than there are physical GPUs, each doing real CUDA work.
//
// IMPORTANT -- opt-in via --timeSlicingEnabled, and it must stay that way for a
// stronger reason than the DCGM feature. It mutates cluster-wide state: it replaces
// the device plugin, so while it runs every GPU on the node is oversubscribed. Some
// harnesses invoke this binary without -test.run, and on those paths an unguarded
// run would change GPU advertisement underneath the other features.
//
// It also requires --installDevicePlugin. Where the plugin is supplied by the
// platform rather than by this suite (EKS Auto ships its own), reconfiguring it is
// not ours to do, so the feature skips instead.
func TestTimeSlicing(t *testing.T) {
	if !testConfig.TimeSlicingEnabled {
		t.Skip("skipping time-slicing; set --timeSlicingEnabled to run (reconfigures the cluster's device plugin)")
	}
	if !testConfig.InstallDevicePlugin {
		t.Skip("skipping time-slicing; requires --installDevicePlugin so the plugin under reconfiguration is one this suite owns")
	}

	replicas := testConfig.TimeSlicingReplicas
	st := &sharingState{feature: sharingFeature{
		name:          "time-slicing",
		configMapName: timeSlicingConfigMapName,
	}}

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

			// Record the pre-change count so the assertion below compares against
			// what this cluster actually advertised, rather than assuming
			// gpuPerNode is still accurate.
			before, err := waitForAdvertisedGPUs(ctx, cfg)
			if err != nil {
				t.Fatalf("no allocatable nvidia.com/gpu within %v; the stock device plugin never advertised: %v",
					gpuSharingAdvertiseTimeout, err)
			}
			log.Printf("[time-slicing] allocatable nvidia.com/gpu before: %d", before)

			pluginImage, err := devicePluginImage()
			if err != nil {
				t.Fatal(err)
			}
			st.renderedPlugin, err = fwext.RenderManifests(
				timeSlicingDevicePluginManifest, timeSlicingPluginTplVars{
					Replicas:    replicas,
					PluginImage: pluginImage,
				})
			if err != nil {
				t.Fatal(err)
			}
			// Set before the delete is issued, not after: if the delete succeeds
			// but the wait for it times out, the stock plugin is already gone and
			// Teardown still has to put it back. Restoration is idempotent, so
			// recording a mutation that did not happen is safe while missing one is
			// not.
			//
			// Replace rather than patch: the stock DaemonSet has no config volume,
			// and two plugins advertising nvidia.com/gpu would conflict.
			st.swapped = true
			// A run killed before Teardown leaves the ConfigMap behind, and the
			// create below would then be a discarded AlreadyExists, mounting the
			// old replica count.
			if err := deleteConfigMapAndWait(ctx, cfg, timeSlicingConfigMapName); err != nil {
				t.Errorf("failed to clear a stale %s: %v", timeSlicingConfigMapName, err)
				return ctx
			}
			// From here on the cluster is modified, so failures must not use
			// t.Fatal: that aborts the goroutine and Teardown never runs, which
			// would leave the node without any device plugin.
			if err := deleteDaemonSetAndWait(ctx, cfg, devicePluginName); err != nil {
				t.Errorf("failed to remove the stock device plugin: %v", err)
				return ctx
			}
			ds := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: devicePluginName, Namespace: devicePluginNamespace}}
			if err := fwext.ApplyManifests(cfg.Client().RESTConfig(), st.renderedPlugin); err != nil {
				t.Errorf("failed to deploy the time-slicing device plugin: %v", err)
				return ctx
			}
			stopWatch := watchDevicePluginRollout(ctx, cfg, st.feature)
			readyErr := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).DaemonSetReady(ds),
				wait.WithContext(ctx), wait.WithTimeout(gpuSharingReadyTimeout))
			stopWatch()
			if readyErr != nil {
				t.Errorf("time-slicing device plugin did not become ready: %v", readyErr)
				dumpDevicePluginDiagnostics(ctx, cfg, st.feature)
				return ctx
			}
			st.ready = true
			ctx = context.WithValue(ctx, allocatableBeforeKey{}, before)
			return ctx
		}).
		Assess("node advertises time-sliced GPUs", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if !st.ready {
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
					// Transient read failures should not end the poll; see
					// waitForAdvertisedGPUs.
					log.Printf("[time-slicing] could not read allocatable GPUs, retrying: %v", err)
					return false, nil
				}
				return got == want, nil
			}, wait.WithContext(ctx), wait.WithTimeout(gpuSharingAdvertiseTimeout))
			if err != nil {
				t.Fatalf("allocatable nvidia.com/gpu = %d, want %d (%d physical x %d replicas): %v",
					got, want, before, replicas, err)
			}
			log.Printf("[time-slicing] allocatable nvidia.com/gpu after: %d (%d physical x %d replicas)", got, before, replicas)
			ctx = context.WithValue(ctx, allocatableAfterKey{}, got)
			return ctx
		}).
		Assess("oversubscribed pods share a physical GPU concurrently", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if !st.ready {
				t.Skip("time-slicing plugin never became ready; see the Setup failure above")
			}
			completions, _ := ctx.Value(allocatableAfterKey{}).(int)
			if completions == 0 {
				t.Fatal("no time-sliced GPU count available from the previous step")
			}
			physical, _ := ctx.Value(allocatableBeforeKey{}).(int)
			var err error
			st.renderedWorkload, err = fwext.RenderManifests(
				gpuSharingWorkloadManifest, gpuSharingWorkloadTplVars{
					Name:                  timeSlicingJobName,
					NvidiaTestImage:       testConfig.NvidiaTestImage,
					Completions:           completions,
					ActiveDeadlineSeconds: int(gpuSharingWorkloadTimeout.Seconds()) - 60,
					HoldSeconds:           int(gpuSharingHold.Seconds()),
				})
			if err != nil {
				t.Fatal(err)
			}
			// A Job of this name left by an interrupted run would otherwise be
			// inspected instead of the one rendered here, since the create error is
			// discarded (see requireJobExists).
			if err := deleteStaleJob(ctx, cfg, timeSlicingJobName); err != nil {
				t.Errorf("failed to remove a stale %s Job: %v", timeSlicingJobName, err)
				return ctx
			}
			if err := fwext.ApplyManifests(cfg.Client().RESTConfig(), st.renderedWorkload); err != nil {
				t.Errorf("failed to apply the time-slicing workload: %v", err)
				return ctx
			}
			if err := requireJobExists(ctx, cfg, timeSlicingJobName); err != nil {
				t.Errorf("%v", err)
				return ctx
			}

			// Observe the overlap while the pods are still alive; once the Job has
			// succeeded it is too late to tell concurrent execution from
			// sequential.
			log.Printf("[time-slicing] waiting for %d pods to run concurrently on %d physical GPU(s)", completions, physical)
			peak, err := observeConcurrentPods(ctx, cfg, st.feature.name, "default", timeSlicingWorkloadSelector, completions, gpuSharingConcurrencyTimeout)
			if err != nil {
				// Report rather than Fatal so Teardown still restores the plugin
				// and dumps the pod logs.
				t.Errorf("pods never ran concurrently on the time-sliced GPU (peak %d, want %d): %v", peak, completions, err)
			} else {
				log.Printf("[time-slicing] %d pods ran concurrently on %d physical GPU(s)", peak, physical)
			}

			job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: timeSlicingJobName, Namespace: "default"}}
			if err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).JobSucceeded(job),
				wait.WithContext(ctx), wait.WithTimeout(gpuSharingWorkloadTimeout)); err != nil {
				t.Errorf("oversubscribed workload did not complete: %v", err)
			}

			// Read the logs before Teardown deletes the Job.
			logsByPod := workloadPodLogs(ctx, cfg, "default", timeSlicingWorkloadSelector)
			uuids := markerValues(logsByPod, gpuUUIDMarker)
			if len(uuids) != completions {
				t.Errorf("only %d of %d pods reported a GPU UUID", len(uuids), completions)
			}
			for pod, uuid := range uuids {
				log.Printf("[time-slicing] pod %s used GPU %s", pod, uuid)
			}
			// With several physical GPUs the scheduler is free to spread the pods
			// across them, so identical UUIDs are only required when there is one
			// GPU for them all to share.
			if physical == 1 {
				if distinct := distinctValues(uuids); len(distinct) != 1 {
					t.Errorf("pods reported %d distinct GPU UUIDs %v on a single-GPU node; they did not share one physical GPU", len(distinct), distinct)
				}
			}
			ctx = context.WithValue(ctx, workloadLogsKey{}, logsByPod)
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if st.renderedWorkload != nil {
				// Prefer the logs the assertion already collected; re-reading after
				// a failure may find the pods gone.
				logsByPod, _ := ctx.Value(workloadLogsKey{}).(map[string]string)
				if len(logsByPod) == 0 {
					logsByPod = workloadPodLogs(ctx, cfg, "default", timeSlicingWorkloadSelector)
				}
				for _, pod := range slices.Sorted(maps.Keys(logsByPod)) {
					t.Logf("Test log for pod %s:\n%s", pod, logsByPod[pod])
				}
				if err := fwext.DeleteManifests(cfg.Client().RESTConfig(), st.renderedWorkload); err != nil {
					t.Errorf("failed to delete the time-slicing workload: %v", err)
				}
			}
			// Always attempt this, even if the assertions failed: leaving the
			// cluster oversubscribed would corrupt any feature that runs next.
			restoreStockDevicePlugin(ctx, cfg, t, st)
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
