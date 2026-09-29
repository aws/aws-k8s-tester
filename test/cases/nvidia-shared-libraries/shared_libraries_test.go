//go:build e2e

package nvidia_shared_libraries

import (
	"context"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	e2ewait "sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const (
	dsName          = "nvidia-shared-libraries-check"
	dsNamespace     = "default"
	dsLabelSelector = "app=nvidia-shared-libraries-check"
)

func TestSharedLibraries(t *testing.T) {
	feat := features.New("nvidia-shared-libraries").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			rendered, err := fwext.RenderManifests(sharedLibrariesCheckDS, DaemonSetManifestTplVars{
				RequiredLibs:        testConfig.RequiredLibs,
				AllowedLibDirsRegex: testConfig.AllowedLibDirsRegex,
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
		Assess("all DaemonSet pods are Ready and the DS has at least one pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			ds := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: dsName, Namespace: dsNamespace}}
			err := e2ewait.For(
				fwext.NewConditionExtension(cfg.Client().Resources()).ResourceMatch(ds, func(obj k8s.Object) bool {
					s := obj.(*appsv1.DaemonSet).Status
					return s.DesiredNumberScheduled >= 1 &&
						s.NumberReady == s.DesiredNumberScheduled &&
						s.NumberUnavailable == 0
				}),
				e2ewait.WithTimeout(10*time.Minute),
			)
			if err != nil {
				// Dump logs from every pod
				fwext.PrintDaemonSetPodLogs(t, ctx, cfg.Client().RESTConfig(), dsNamespace, dsLabelSelector)
				t.Fatalf("shared-libraries DaemonSet did not reach Ready with >=1 pod within 10 minutes: %v", err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			rendered, _ := ctx.Value(renderedManifestKey{}).([]byte)
			if len(rendered) == 0 {
				return ctx
			}
			if err := fwext.DeleteManifests(cfg.Client().RESTConfig(), rendered); err != nil {
				t.Errorf("delete daemonset: %v", err)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}

type renderedManifestKey struct{}
