//go:build e2e

package nvidia

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/internal/e2e"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	_ "embed"
)

//go:embed manifests/daemonset-containerd-check.yaml
var containerdCheckDS []byte

func TestContainerdConfig(t *testing.T) {
	feat := features.New("containerd-config-check").
		WithLabel("suite", "nvidia").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log.Println("[Setup] Applying containerd-check DaemonSet manifest.")
			if err := e2e.ApplyManifests(cfg.Client().RESTConfig(), containerdCheckDS); err != nil {
				t.Fatalf("Failed to apply containerd-check DS: %v", err)
			}
			return ctx
		}).
		Assess("DaemonSet becomes ready", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			dsName := "containerd-check"
			dsNS := "default"

			log.Println("[Assess] Waiting up to 1 minute for containerd-check DS to become Ready...")
			ds := &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dsName,
					Namespace: dsNS,
				},
			}
			err := wait.For(
				e2e.NewConditionExtension(cfg.Client().Resources()).DaemonSetReady(ds),
				wait.WithTimeout(1*time.Minute),
			)
			if err != nil {
				t.Logf("[Assess] containerd-check DS did not become Ready: %v", err)
				e2e.PrintDaemonSetPodLogs(t, ctx, cfg.Client().RESTConfig(), dsNS, "app=containerd-check")
				t.Fatalf("containerd-check DS not Ready within 1 minute")
			}

			log.Println("[Assess] containerd-check DS is Ready.")
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Log("[Teardown] Removing containerd-check DS (no additional logs).")
			if err := e2e.DeleteManifests(cfg.Client().RESTConfig(), containerdCheckDS); err != nil {
				t.Fatalf("Failed to delete containerd-check DS: %v", err)
			}
			t.Log("[Teardown] containerd-check DS removed successfully.")
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
