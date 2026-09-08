//go:build e2e

package nvidia

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	//go:embed manifests/job-dcgm-diag.yaml
	jobDcgmDiagManifest         []byte
	renderedJobDcgmDiagManifest []byte
)

const dcgmDiagJobName = "dcgm-diag-job"

type dcgmDiagManifestTplVars struct {
	NvidiaTestImage       string
	NodeType              string
	GpuPerNode            int
	DiagLevel             int
	ActiveDeadlineSeconds int
}

// dcgmDiagProgressInterval is how often the waiting side polls the runner Job
// for new output. Short enough to notice a stall, long enough not to spam the
// API server across a multi-hour run.
const dcgmDiagProgressInterval = 2 * time.Minute

// deleteStaleJob removes a Job left behind by an earlier run and waits for it to
// disappear, so a fresh Job of the same name can be created. A missing Job is
// the expected case and is not an error.
func deleteStaleJob(ctx context.Context, cfg *envconf.Config, name string) error {
	res := cfg.Client().Resources()
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
	if err := res.Get(ctx, name, "default", job); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	log.Printf("[dcgm-diag] found a pre-existing %s from an earlier run; deleting it", name)
	// Propagate deletion so the Job's Pods go too, otherwise a finished Pod can
	// keep reporting logs for a Job that no longer exists.
	policy := metav1.DeletePropagationForeground
	if err := res.Delete(ctx, job, func(o *metav1.DeleteOptions) { o.PropagationPolicy = &policy }); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return wait.For(conditions.New(res).ResourceDeleted(job),
		wait.WithContext(ctx), wait.WithTimeout(2*time.Minute))
}

// requireJobExists verifies the Job is present after apply. ApplyManifests can
// return nil even when the underlying Create failed, so without this a
// swallowed error would surface much later as an opaque wait timeout.
func requireJobExists(ctx context.Context, cfg *envconf.Config, name string) error {
	job := &batchv1.Job{}
	return cfg.Client().Resources().Get(ctx, name, "default", job)
}

// followJobProgress polls a Job's logs and prints whatever is new, so a
// long-running run reports progress instead of going silent until it finishes.
// The returned function stops the poller.
func followJobProgress(ctx context.Context, cfg *envconf.Config, jobName string) func() {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)
		ticker := time.NewTicker(dcgmDiagProgressInterval)
		defer ticker.Stop()

		started := time.Now()
		var seen int
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				elapsed := time.Since(started).Round(time.Second)
				out, err := fwext.GetJobLogs(cfg.Client().RESTConfig(), &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: "default"},
				})
				if err != nil {
					// Expected early on, before the Pod is scheduled.
					log.Printf("[dcgm-diag +%s] no logs yet (%v)", elapsed, err)
					continue
				}
				if len(out) > seen {
					log.Printf("[dcgm-diag +%s] new output:\n%s", elapsed, out[seen:])
					seen = len(out)
				} else {
					log.Printf("[dcgm-diag +%s] no new output since last check", elapsed)
				}
			}
		}
	}()

	return func() {
		cancel()
		<-done
	}
}

// TestDcgmDiagnostics runs a deep DCGM diagnostic (level 4 by default) as a
// standalone feature.
//
// This is deliberately separate from the unit-test feature, which runs
// `dcgmi diag -r 2` as one of many quick checks. Levels 3 and 4 are invasive
// and slow: level 4 (`xlong`) adds memtest and pulse on top of level 3, and
// NVIDIA's guidance allows up to 2.25 hours on an eight-GPU node.
//
// Runtime grows with the number of GPUs on the node, though sub-linearly.
// Measured with DCGM 4.6.1 and driver 580: about 32 minutes on a single-GPU
// instance and about 80 minutes on an eight-GPU instance. Total HBM has
// little effect, so budget by GPU count.
//
// It therefore cannot be a per-build gate and belongs in a periodic or
// on-demand tier.
//
// IMPORTANT — this feature is opt-in and MUST stay that way. Some harnesses
// invoke this binary without -test.run (they pass only --skip-features), so any
// new Test function here runs by default in those pipelines. The
// --dcgmDiagEnabled gate is what prevents an hours-long GPU stress from
// silently attaching itself to a per-build path. Do not default it to true,
// and do not rely on --skip-features to hold it back: a harness that forgets
// the skip would inherit the whole run.
func TestDcgmDiagnostics(t *testing.T) {
	if !testConfig.DcgmDiagEnabled {
		t.Skip("skipping deep DCGM diagnostics; set --dcgmDiagEnabled to run (long-running, not suitable for a per-build gate)")
	}

	timeout := time.Duration(testConfig.DcgmDiagTimeoutMinutes) * time.Minute

	feat := features.New("dcgm-diag").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if testConfig.NvidiaTestImage == "" {
				t.Fatal(fmt.Errorf("nvidiaTestImage must be set to run dcgm-diag, use https://github.com/aws/aws-k8s-tester/blob/main/test/images/nvidia/Dockerfile to build the image and -nvidiaTestImage to set the image url"))
			}
			if testConfig.DcgmDiagLevel < 1 || testConfig.DcgmDiagLevel > 4 {
				t.Fatalf("dcgmDiagLevel must be 1-4, got %d", testConfig.DcgmDiagLevel)
			}
			deadlineSeconds, err := dcgmDiagDeadlineSeconds(testConfig.DcgmDiagTimeoutMinutes)
			if err != nil {
				t.Fatal(err)
			}
			renderedJobDcgmDiagManifest, err = fwext.RenderManifests(jobDcgmDiagManifest, dcgmDiagManifestTplVars{
				NvidiaTestImage:       testConfig.NvidiaTestImage,
				NodeType:              testConfig.NodeType,
				GpuPerNode:            gpuPerNode,
				DiagLevel:             testConfig.DcgmDiagLevel,
				ActiveDeadlineSeconds: deadlineSeconds,
			})
			if err != nil {
				t.Fatal(err)
			}
			// The Job name is fixed, so a leftover Job from an interrupted run on
			// a reused cluster would make the wait below observe that Job's
			// terminal state and report success without running anything. Clear
			// any such Job first. fwext.ApplyManifests cannot be relied on to
			// surface the resulting AlreadyExists error, because processObjects
			// discards the per-object Create error (internal/e2e/client.go).
			if err := deleteStaleJob(ctx, cfg, dcgmDiagJobName); err != nil {
				t.Fatalf("failed to clear a pre-existing %s: %v", dcgmDiagJobName, err)
			}
			if err := fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedJobDcgmDiagManifest); err != nil {
				t.Fatal(err)
			}
			// Confirm the Job actually exists, since a swallowed Create error
			// would otherwise leave the wait below polling a Job that is not
			// there.
			if err := requireJobExists(ctx, cfg, dcgmDiagJobName); err != nil {
				t.Fatalf("%s was not created: %v", dcgmDiagJobName, err)
			}
			return ctx
		}).
		// The verdict is dcgmi's exit code, so subtests that DCGM reports as
		// Skip do not fail this feature. That is intentional: NVIDIA enables
		// several plugins per GPU SKU in nvvs's built-in config and skips them
		// silently elsewhere, so which subtests actually run is a property of
		// the hardware, not of our setup. Read the result table in the Job log
		// to see what was exercised on a given instance type.
		Assess(fmt.Sprintf("DCGM level-%d diagnostic passes", testConfig.DcgmDiagLevel), func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: dcgmDiagJobName, Namespace: "default"},
			}

			// This run can take over an hour. Stream the Job's output while we
			// wait rather than only dumping it in Teardown, so a watcher can
			// tell a slow run from a stuck one without kubectl access. Use
			// log.Printf, not t.Logf: the stdlib logger writes to stderr
			// immediately, whereas t.Logf output is held until the test ends.
			stopProgress := followJobProgress(ctx, cfg, dcgmDiagJobName)
			defer stopProgress()

			log.Printf("[dcgm-diag] waiting up to %s for level %d on %d GPU(s)", timeout, testConfig.DcgmDiagLevel, gpuPerNode)
			err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).JobSucceeded(job),
				wait.WithContext(ctx),
				wait.WithTimeout(timeout))
			if err != nil {
				// Report rather than Fatal so Teardown still dumps the DCGM
				// output, which is the only place the failing subtest is named.
				t.Errorf("dcgm level-%d diagnostic did not succeed: %v", testConfig.DcgmDiagLevel, err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log, err := fwext.GetJobLogs(cfg.Client().RESTConfig(), &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: dcgmDiagJobName, Namespace: "default"},
			})
			if err != nil {
				t.Errorf("failed to get logs for %s: %v", dcgmDiagJobName, err)
			} else {
				t.Logf("Test log for %s:", dcgmDiagJobName)
				t.Log(log)
			}
			if err := fwext.DeleteManifests(cfg.Client().RESTConfig(), renderedJobDcgmDiagManifest); err != nil {
				t.Error(err)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
