//go:build e2e

package nvidia

import (
	"context"
	"fmt"
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

//go:embed manifests/moderngl-script.py
var modernglScript []byte

const (
	PodName      = "moderngl-pod"
	PodNamespace = "default"
	ConfigMap    = "moderngl-script"
)

func TestNvidiaDriverCapabilities(t *testing.T) {
	feat := features.New("nvidia-driver-capabilities-check").
		WithLabel("suite", "nvidia").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log.Println("[Setup] Deploy the config map")
			e2e.CreateConfigMapFromFile(cfg.Client().RESTConfig(), PodNamespace, ConfigMap, fmt.Sprintf("%s.py", ConfigMap), modernglScript)
			log.Println("[Setup] Applying nvidia driver capabilities check pod manifest.")
			if err := e2e.ApplyManifests(cfg.Client().RESTConfig(), capabilitiesCheckPod); err != nil {
				t.Fatalf("Failed to apply capabilities check pod manifest: %v", err)
			}
			return ctx
		}).
		Assess("Check Pod becomes ready", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log.Println("[Assess] Waiting up to 5 minute for pod to become Running...")
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PodName,
					Namespace: PodNamespace,
				},
			}
			err := wait.For(
				e2e.NewConditionExtension(cfg.Client().Resources()).PodSucceeded(pod),
				wait.WithTimeout(5*time.Minute),
			)
			if err != nil {
				podPhase, err2 := e2e.NewConditionExtension(cfg.Client().Resources()).GetPodPhase(pod)
				if err2 != nil {
					t.Fatalf("[Assess] nvidia capabilities check pod not in succeeded phase within 5 minute: %v, failed to get current pod phase: %v", err, err2)
				} else if podPhase == v1.PodFailed {
					t.Fatalf("[Assess] nvidia capabilities pod in Failed status, ModernGL check failed. Could be caused by required library missing")
				} else {
					t.Fatalf("[Assess] nvidia capabilities check pod not in compeleted phase (succeeded or failed) within 5 minute: %v, current pod phase: %v", err, podPhase)
				}
			}
			log.Println("[Assess] nvidia driver capabilities check succeeded.")
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Log("[Teardown] Removing nvidia driver capabilities check pod.")
			if err := e2e.DeleteManifests(cfg.Client().RESTConfig(), capabilitiesCheckPod); err != nil {
				t.Fatalf("Failed to delete pod: %v", err)
			}
			t.Log("[Teardown] Removing nvidia driver capabilities script config map.")
			if err := e2e.DeleteConfigMap(cfg.Client().RESTConfig(), PodNamespace, ConfigMap); err != nil {
				t.Fatalf("Failed to delete configmap: %v", err)
			}
			t.Log("[Teardown] all test resources removed successfully.")
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
