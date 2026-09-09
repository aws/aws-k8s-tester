//go:build e2e

package nvidia_gsp_firmware

import (
	"context"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/aws-k8s-tester/test/common"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2ewait "sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const (
	podName      = "nvidia-gsp-firmware-check"
	podNamespace = "default"
)

// TestGSPFirmware runs the three folded GSP firmware assertions inside
// a single pod. Pod exit codes: 41 empty/N/A GSP version, 42 GSP
// version != driver version, 43 cross-GPU mismatch.
func TestGSPFirmware(t *testing.T) {
	feat := features.New("nvidia-gsp-firmware").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			rendered, err := fwext.RenderManifests(podGSPFirmwareCheckManifest, PodManifestTplVars{
				NvidiaTestImage: testConfig.NvidiaTestImage,
				GpuCount:        common.GPUCountForNodeType(testConfig.NodeType),
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
					t.Fatalf("gsp-firmware pod did not complete within 3 minutes: %v", err)
				}
				t.Fatalf("gsp-firmware pod ended in Failed phase: %v (exit 41=empty version, 42=driver mismatch, 43=cross-GPU mismatch)", err)
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
