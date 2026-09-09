//go:build e2e

package nvidia_fabric_manager

import (
	"context"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/aws-k8s-tester/test/common"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2ewait "sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

// On every NVSwitch node, the running fabric-manager daemon's version must match the
// installed NVIDIA driver version. DaemonSet-based host check that reads
// /host/var/log/fabricmanager.log for the FM version and
// /host/proc/driver/nvidia/version for the driver version. Skips on
// non-NVSwitch instances (tail -f). Soft-skips (still Ready) if the log
// is missing or unparseable.
func TestFabricManagerVersionMatchesDriver(t *testing.T) {
	feat := features.New("nvidia-fabric-manager-version-check").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if err := fwext.ApplyManifests(cfg.Client().RESTConfig(), dsFabricManagerVersionCheckManifest); err != nil {
				t.Fatalf("apply DS: %v", err)
			}
			return ctx
		}).
		Assess("DS becomes Ready", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			ds := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "nvidia-fabric-manager-version-check", Namespace: "default"}}
			err := e2ewait.For(
				fwext.NewConditionExtension(cfg.Client().Resources()).DaemonSetReady(ds),
				e2ewait.WithTimeout(3*time.Minute),
			)
			if err != nil {
				t.Logf("DS not Ready: %v", err)
				fwext.PrintDaemonSetPodLogs(t, ctx, cfg.Client().RESTConfig(), "default", "app=nvidia-fabric-manager-version-check")
				t.Fatalf("nvidia-fabric-manager-version-check DS not Ready within 3 minutes -- see pod logs for the 5.2 failure")
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if err := fwext.DeleteManifests(cfg.Client().RESTConfig(), dsFabricManagerVersionCheckManifest); err != nil {
				t.Errorf("delete DS: %v", err)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}

// TestFabricAndNVLinkInContainer covers the three in-container fabric/
// NVLink assertions in one pod that dispatches by instance-family class:
// fabric state, NVLinks Active + expected count, no NVLinks on non-NVLink hardware.
// Each check either runs, prints
// SKIP for its class, or fails with a distinct exit code (50, 51, 52,
// 53 -- see the pod manifest).
func TestFabricAndNVLinkInContainer(t *testing.T) {
	feat := features.New("nvidia-fabric-nvlink-check").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			rendered, err := fwext.RenderManifests(podFabricNVLinkCheckManifest, PodManifestTplVars{
				NvidiaTestImage: testConfig.NvidiaTestImage,
				GpuCount:        common.GPUCountForNodeType(testConfig.NodeType),
			})
			if err != nil {
				t.Fatalf("render manifest: %v", err)
			}
			if err := fwext.ApplyManifests(cfg.Client().RESTConfig(), rendered); err != nil {
				t.Fatalf("apply pod: %v", err)
			}
			ctx = context.WithValue(ctx, renderedManifestKey{}, rendered)
			return ctx
		}).
		Assess("pod reaches Succeeded", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "nvidia-fabric-nvlink-check", Namespace: "default"}}
			err := e2ewait.For(
				fwext.NewConditionExtension(cfg.Client().Resources()).PodSucceeded(pod),
				e2ewait.WithTimeout(3*time.Minute),
			)
			if err != nil {
				if err == wait.ErrWaitTimeout {
					t.Fatalf("fabric-nvlink pod did not complete within 3 minutes: %v", err)
				}
				t.Fatalf("fabric-nvlink pod ended in Failed phase: %v (exit 50=fabric state, 51=nvlink not Active, 52=nvlink count, 53=nvlinks on non-NVLink hw)", err)
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
