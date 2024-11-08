package neuron

import (
	"context"
	_ "embed"
	"fmt"
	"testing"

	fwext "github.com/aws/aws-k8s-tester/e2e2/internal/framework_extensions"
	kubeflowv2beta1 "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v2beta1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	//go:embed manifests/single-node-test-neuronx.yaml
	neuronSingleNodeManifest []byte
	//go:embed manifests/multi-node-test-neuron.yaml
	neuronMultiNodeManifest          []byte
	renderedNeuronSingleNodeManifest []byte
	renderedNeuronMultiNodeManifest  []byte
)

type neuronSingleNodeManifestTplVars struct {
	NeuronTestImage string
}

type neuronMultiNodeTestManifestTplVars struct {
	WorkerNodeCount       int
	WorkerNodeNeuronCount int
	NeuronPerNode         int
	NeuronCorePerNode     int
	NeuronTestImage       string
	EfaInterfacePerNode   int
}

func TestNeuronNodes(t *testing.T) {
	singleNode := features.New("single-node").
		WithLabel("suite", "neuron").
		WithLabel("hardware", "neuron").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if *neuronTestImage == "" {
				t.Fatal(fmt.Errorf("neuronTestImage must be set to run neuron single node test, use https://github.com/aws/aws-k8s-tester/blob/main/e2e2/test/images/neuron/Dockerfile to build the image and -neuronTestImage to set the image url"))
			}
			var err error
			renderedNeuronSingleNodeManifest, err = fwext.RenderManifests(neuronSingleNodeManifest, neuronSingleNodeManifestTplVars{
				NeuronTestImage: *neuronTestImage,
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Applying single node manifest")
			err = fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedNeuronSingleNodeManifest)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Manifest applied successfully")
			return ctx
		}).
		Assess("Single node test Job succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "neuronx-single-node", Namespace: "default"},
			}
			t.Log("Waiting for single node job to complete")
			err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).JobSucceeded(job),
				wait.WithContext(ctx))
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log, err := fwext.GetJobLogs(cfg.Client().RESTConfig(), &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "neuronx-single-node", Namespace: "default"},
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Test log for neuronx-single-node:")
			t.Log(log)
			err = fwext.DeleteManifests(cfg.Client().RESTConfig(), renderedNeuronSingleNodeManifest)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Feature()

	multiNode := features.New("multi-node").
		WithLabel("suite", "neuron").
		WithLabel("hardware", "neuron").
		WithLabel("hardware", "efa").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if *neuronTestImage == "" {
				t.Fatal(fmt.Errorf("neuronTestImage must be set to run unit test, use https://github.com/aws/aws-k8s-tester/blob/main/e2e2/test/images/neuron/Dockerfile to build the image and -neuronTestImage to set the image url"))
			}
			renderedNeuronMultiNodeManifest, err := fwext.RenderManifests(neuronMultiNodeManifest, neuronMultiNodeTestManifestTplVars{
				// one of the nodes will be used for the master pod
				WorkerNodeCount:       nodeCount,
				WorkerNodeNeuronCount: nodeCount * neuronPerNode,
				NeuronPerNode:         neuronPerNode,
				NeuronCorePerNode:     neuronCorePerNode,
				NeuronTestImage:       *neuronTestImage,
				EfaInterfacePerNode:   efaPerNode,
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Applying multi node manifest")
			err = fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedNeuronMultiNodeManifest)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Applied manifest successfully")
			return ctx
		}).
		Assess("NCCOM test succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			rsrc := cfg.Client().Resources()
			if err := kubeflowv2beta1.AddToScheme(rsrc.GetScheme()); err != nil {
				t.Fatal(err)
			}
			j := kubeflowv2beta1.MPIJob{
				ObjectMeta: metav1.ObjectMeta{Name: "multi-node-nccom-test", Namespace: "default"},
			}
			t.Log("Waiting for MPIJob to complete")
			err := wait.For(conditions.New(rsrc).ResourceMatch(&j, mpiJobSucceeded),
				wait.WithContext(ctx))
			if err != nil {
				t.Fatal(err)
			}

			// Verify GPU Direct RDMA is used on P4/P5
			log, err := fwext.GetJobLogs(cfg.Client().RESTConfig(), &kubeflowv2beta1.MPIJob{
				ObjectMeta: metav1.ObjectMeta{Name: "multi-node-nccom-test", Namespace: "default"},
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Test log for multi-node-nccom-test:")
			t.Log(log)
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			err := fwext.DeleteManifests(cfg.Client().RESTConfig(), renderedNeuronMultiNodeManifest)
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
