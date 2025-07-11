//go:build e2e

package nvidia

import (
	"bytes"
	"context"
	"log"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/internal/e2e"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	_ "embed"
)

//go:embed manifests/nvidia-driver-capabilities-check.yaml
var capabilitiesCheckPod []byte

func TestNvidiaDriverCapabilities(t *testing.T) {
	feat := features.New("nvidia-driver-capabilities-check").
		WithLabel("suite", "nvidia").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log.Println("[Setup] Applying nvidia driver capabilities check pod manifest.")
			if err := e2e.ApplyManifests(cfg.Client().RESTConfig(), capabilitiesCheckPod); err != nil {
				t.Fatalf("Failed to apply capabilities check pod manifest: %v", err)
			}
			return ctx
		}).
		Assess("Pod becomes ready", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			podName := "moderngl-container"
			podNS := "default"

			log.Println("[Assess] Waiting up to 1 minute for pod to become Running...")
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: podNS,
				},
			}
			err := wait.For(
				e2e.NewConditionExtension(cfg.Client().Resources()).PodRunning(pod),
				wait.WithTimeout(1*time.Minute),
			)
			if err != nil {
				t.Fatalf("nvidia capabilities check pod not in Running within 1 minute")
			}
			log.Println("[Assess] nvidia capabilities check pod is Ready.")

			// Check ModernGL functionality
			pythonCmd := `python3 -c "
import moderngl
try:
    ctx = moderngl.create_standalone_context(backend='egl')
    print('ModernGL context created successfully')
    ctx.release()
except Exception as e:
    print(f'ModernGL error: {e}')
    exit(1)
"`
			e2e.ApplyManifests(cfg.Client().RESTConfig(), capabilitiesCheckPod)
			stdout, stderr, err := e2e.ExecuteInPod(cfg.Client().RESTConfig(), podName, podNS, pythonCmd)
			if err != nil {
				t.Fatalf("Failed to execute nvidia driver capabilites check in pod: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
			}

			if !bytes.Contains(stdout, []byte("ModernGL context created successfully")) {
				t.Fatalf("nvidia driver capabilites check failed:\nStdout: %s\nStderr: %s", stdout, stderr)
			}
			log.Println("[Assess] nvidia capabilities check pass")
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Log("[Teardown] Removing nvidia driver capabilities check pod.")
			if err := e2e.DeleteManifests(cfg.Client().RESTConfig(), capabilitiesCheckPod); err != nil {
				t.Fatalf("Failed to delete pod: %v", err)
			}
			t.Log("[Teardown] check pod removed successfully.")
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
