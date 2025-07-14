//go:build e2e

package nvidia

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/internal/e2e"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/wait"
	e2ewait "sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	_ "embed"
)

//go:embed manifests/nvidia-driver-capabilities-check.yaml
var capabilitiesCheckPod []byte

const (
	PodName      = "moderngl-pod"
	PodNamespace = "default"
)

func TestNvidiaDriverCapabilities(t *testing.T) {
	feat := features.New("nvidia-driver-capabilities-check").
		WithLabel("suite", "nvidia").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Log("Applying nvidia driver capabilities check pod manifest.")
			// capabilitiesCheckPod only run moderngl.create_standalone_context() with NVIDIA_DRIVER_CAPABILITIES=all to load all capabilities enabled by nvidia driver.
			// If any lib required by any of nvidia driver capabilities is missing, it will failed with exception.
			if err := e2e.ApplyManifests(cfg.Client().RESTConfig(), capabilitiesCheckPod); err != nil {
				t.Fatalf("Failed to apply capabilities check pod manifest: %v", err)
			}
			return ctx
		}).
		Assess("Check Pod becomes ready", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Log("Waiting up to 5 minute for pod to complete...")
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PodName,
					Namespace: PodNamespace,
				},
			}
			err := e2ewait.For(
				e2e.NewConditionExtension(cfg.Client().Resources()).PodSucceeded(pod),
				e2ewait.WithTimeout(5*time.Minute),
			)
			if err != nil {
				if err == wait.ErrWaitTimeout {
					t.Fatalf("nvidia capabilities check pod not in compeleted phase (succeeded or failed) within 5 minute and waiter timeout: %v", err)
				}
				t.Fatalf("nvidia capabilities pod in Failed status, ModernGL check failed. Could be caused by required library missing")
			}
			t.Log("nvidia driver capabilities check succeeded.")
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Log("Removing nvidia driver capabilities check pod.")
			if err := e2e.DeleteManifests(cfg.Client().RESTConfig(), capabilitiesCheckPod); err != nil {
				t.Errorf("Failed to delete pod: %v", err)
			}
			t.Log("all test resources removed successfully.")
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
