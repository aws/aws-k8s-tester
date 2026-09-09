//go:build e2e

package nvidia_persistence_device_nodes

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
	podName      = "nvidia-persistence-device-nodes-check"
	podNamespace = "default"
)

// TestDeviceNodes verifies /dev/nvidia0..N, /dev/nvidiactl,
// /dev/nvidia-uvm, /dev/nvidia-uvm-tools are all present as character
// devices with root ownership. Pod exit codes: 30 (/dev missing),
// 31 (a /dev/nvidia<N> is bad), 32 (nvidiactl), 33 (nvidia-uvm),
// 34 (nvidia-uvm-tools), 35 (ownership).
func TestDeviceNodes(t *testing.T) {
	feat := features.New("nvidia-persistence-device-nodes").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			rendered, err := fwext.RenderManifests(podDeviceNodesCheckManifest, PodManifestTplVars{
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
			err := e2ewait.For(
				fwext.NewConditionExtension(cfg.Client().Resources()).PodSucceeded(pod),
				e2ewait.WithTimeout(3*time.Minute),
			)
			if err != nil {
				if err == wait.ErrWaitTimeout {
					t.Fatalf("device-nodes pod did not complete within 3 minutes: %v", err)
				}
				t.Fatalf("device-nodes pod ended in Failed phase: %v (see pod logs; exit 30-35 indicates which node/check failed)", err)
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
