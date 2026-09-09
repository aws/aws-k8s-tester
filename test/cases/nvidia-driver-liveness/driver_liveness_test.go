//go:build e2e

package nvidia_driver_liveness

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
	podName      = "nvidia-driver-liveness-check"
	podNamespace = "default"
)

// TestDriverLiveness runs nvidia-smi -L, driver version match,
// and GPU model matches instance type in one pod. The pod's script
// exits non-zero with a specific code on the first failing assertion --
// see pod-driver-liveness-check.yaml for the exit-code catalog.
func TestDriverLiveness(t *testing.T) {
	feat := features.New("nvidia-driver-liveness").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			rendered, err := fwext.RenderManifests(podDriverLivenessCheckManifest, tplVars())
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
			err := e2ewait.For(
				fwext.NewConditionExtension(cfg.Client().Resources()).PodSucceeded(pod),
				e2ewait.WithTimeout(5*time.Minute),
			)
			if err != nil {
				if err == wait.ErrWaitTimeout {
					t.Fatalf("driver-liveness pod did not complete within 5 minutes: %v", err)
				}
				t.Fatalf("driver-liveness pod ended in Failed phase: %v (see pod logs; exit 10=smi, 11=version, 12=model)", err)
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
