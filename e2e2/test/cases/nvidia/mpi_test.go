package nvidia

import (
	"context"
	"slices"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/e2e2/internal/framework_extensions"
	kubeflowv2beta1 "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v2beta1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMPIJobPytorchTraining(t *testing.T) {
	manifestSingleNode := "manifests/mpi-job-pytorch-training-single-node.yaml"

	singleNode := features.New("single-node").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			err := fwext.ApplyFile(cfg.Client().RESTConfig(), manifestSingleNode)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("MPIJob succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			rsrc := cfg.Client().Resources()
			if err := kubeflowv2beta1.AddToScheme(rsrc.GetScheme()); err != nil {
				t.Fatal(err)
			}
			j := kubeflowv2beta1.MPIJob{
				ObjectMeta: metav1.ObjectMeta{Name: "pytorch-training-single-node", Namespace: "default"},
			}
			timeout := time.Minute * 20
			err := wait.For(fwext.NewConditionExtension(rsrc).ResourceMatch(&j, mpiJobSucceeded),
				wait.WithTimeout(timeout))
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			err := fwext.DeleteFile(cfg.Client().RESTConfig(), manifestSingleNode)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Feature()

	manifestsMultiNode := []string{
		"manifests/efa-device-plugin.yaml",
		"manifests/mpi-job-pytorch-training-multi-node.yaml",
	}

	multiNode := features.New("multi-node").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		WithLabel("hardware", "efa").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			err := fwext.ApplyFiles(cfg.Client().RESTConfig(), manifestsMultiNode)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("EFA device plugin daemonset ready", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			ds := appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Name: "efa-device-plugin-daemonset", Namespace: "kube-system"},
			}
			err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).DaemonSetReady(&ds),
				wait.WithTimeout(time.Minute*5))
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("MPIJob succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			rsrc := cfg.Client().Resources()
			if err := kubeflowv2beta1.AddToScheme(rsrc.GetScheme()); err != nil {
				t.Fatal(err)
			}
			j := kubeflowv2beta1.MPIJob{
				ObjectMeta: metav1.ObjectMeta{Name: "pytorch-training-multi-node"},
			}
			timeout := time.Minute * 20
			err := wait.For(conditions.New(rsrc).ResourceMatch(&j, mpiJobSucceeded),
				wait.WithTimeout(timeout))
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			slices.Reverse(manifestsMultiNode)
			err := fwext.DeleteFiles(cfg.Client().RESTConfig(), manifestsMultiNode)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, singleNode, multiNode)
}

func mpiJobSucceeded(obj k8s.Object) bool {
	j := obj.(*kubeflowv2beta1.MPIJob)
	for _, c := range j.Status.Conditions {
		if c.Type == kubeflowv2beta1.JobSucceeded {
			return c.Status == v1.ConditionTrue
		}
	}
	return false
}
