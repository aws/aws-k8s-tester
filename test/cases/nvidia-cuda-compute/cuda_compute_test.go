//go:build e2e

package nvidia_cuda_compute

import (
	"context"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2ewait "sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const (
	podName      = "nvidia-cuda-compute-check"
	podNamespace = "default"
)

// TestCUDACompute runs the 9 CUDA compute samples in a single pod. See
// pod-cuda-compute-check.yaml for the run_sample helper that gates
// compute-cap-dependent samples and fails with exit 20 if a required
// binary is missing from the image.
func TestCUDACompute(t *testing.T) {
	feat := features.New("nvidia-cuda-compute").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			rendered, err := fwext.RenderManifests(podCUDAComputeCheckManifest, PodManifestTplVars{
				NvidiaTestImage: testConfig.NvidiaTestImage,
			})
			if err != nil {
				t.Fatalf("render manifest: %v", err)
			}
			if err := fwext.ApplyManifests(cfg.Client().RESTConfig(), rendered); err != nil {
				t.Fatalf("apply manifest: %v", err)
			}
			ctx = context.WithValue(ctx, renderedManifestKey{}, rendered)
			return ctx
		}).
		Assess("pod reaches Succeeded", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: podNamespace}}
			// Timeout is generous -- some samples (UnifiedMemoryPerf,
			// immaTensorCoreGemm) are minutes-long on their own.
			err := e2ewait.For(
				fwext.NewConditionExtension(cfg.Client().Resources()).PodSucceeded(pod),
				e2ewait.WithTimeout(15*time.Minute),
			)
			if err != nil {
				if err == wait.ErrWaitTimeout {
					t.Fatalf("cuda-compute pod did not complete within 15 minutes: %v", err)
				}
				t.Fatalf("cuda-compute pod ended in Failed phase: %v (exit 20=binary missing from image, exit 21=sample self-check failed)", err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			rendered, _ := ctx.Value(renderedManifestKey{}).([]byte)
			if len(rendered) == 0 {
				return ctx
			}
			if err := fwext.DeleteManifests(cfg.Client().RESTConfig(), rendered); err != nil {
				t.Errorf("delete pod: %v", err)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}

type renderedManifestKey struct{}
