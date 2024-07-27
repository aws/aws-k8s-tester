package neuron

import (
	"context"
	_ "embed"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/e2e2/internal/framework_extensions"
	kubeflowv2beta1 "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v2beta1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	//go:embed manifests/unit-test.yaml
	neuronUnitTestManifest []byte
	//go:embed manifests/bert-infer.yaml
	neuronInferManifest []byte
	//go:embed manifests/bert-train.yaml
	neuronTrainManifest            []byte
	renderedNeuronUnitTestManifest []byte
	renderedNeuronInferManifest    []byte
	renderedNeuronTrainManifest    []byte
)

type neuronUnitTestManifestTplVars struct {
	NeuronTestImage string
	NodeType        string
}

type neuronInferManifestTplVars struct {
	NeuronTestImage   string
	NodeType          string
	NeuronPerNode     int
	NeuronCorePerNode int
}

type neuronTrainManifestTplVars struct {
	NeuronTestImage     string
	NodeType            string
	WorkerNodeCount     int
	NeuronPerNode       int
	NeuronCorePerNode   int
	EfaInterfacePerNode int
}

func TestNeuron(t *testing.T) {
	singleNode := features.New("single-node-unit-test").
		WithLabel("hardware", "neuron").
		WithLabel("task", "unit-test").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var err error
			renderedNeuronUnitTestManifest, err = fwext.RenderManifests(neuronUnitTestManifest, neuronUnitTestManifestTplVars{
				NeuronTestImage: *neuronTestImage,
				NodeType:        *nodeType,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedNeuronUnitTestManifest)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("Single node unit test succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "neuron-unit-test", Namespace: "default"},
			}
			err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).JobSucceeded(job),
				wait.WithTimeout(time.Minute*20))
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			err := fwext.DeleteManifests(cfg.Client().RESTConfig(), renderedNeuronUnitTestManifest)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).Feature()

	singleNodeInfer := features.New("single-node-inference").
		WithLabel("hardware", "neuron").
		WithLabel("task", "inference").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var err error
			renderedNeuronInferManifest, err = fwext.RenderManifests(neuronInferManifest, neuronInferManifestTplVars{
				NeuronTestImage:   *neuronTestImage,
				NodeType:          *nodeType,
				NeuronPerNode:     neuronPerNode,
				NeuronCorePerNode: neuronCorePerNode,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedNeuronInferManifest)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("Single node bert inference Job succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "bert-inference", Namespace: "default"},
			}
			err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).JobSucceeded(job),
				wait.WithTimeout(time.Minute*20))
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			err := fwext.DeleteManifests(cfg.Client().RESTConfig(), renderedNeuronInferManifest)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).Feature()

	multiNodeTrain := features.New("multi-node-training").
		WithLabel("hardware", "neuron").
		WithLabel("task", "training").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var err error
			renderedNeuronTrainManifest, err = fwext.RenderManifests(neuronTrainManifest, neuronTrainManifestTplVars{
				NeuronTestImage:     *neuronTestImage,
				NodeType:            *nodeType,
				WorkerNodeCount:     nodeCount,
				NeuronPerNode:       neuronPerNode,
				NeuronCorePerNode:   neuronCorePerNode,
				EfaInterfacePerNode: efaPerNode,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedNeuronTrainManifest)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("Multi node bert training MPIJob succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			rsrc := cfg.Client().Resources()
			if err := kubeflowv2beta1.AddToScheme(rsrc.GetScheme()); err != nil {
				t.Fatal(err)
			}
			job := kubeflowv2beta1.MPIJob{
				ObjectMeta: metav1.ObjectMeta{Name: "bert-mpi-training", Namespace: "default"},
			}
			err := wait.For(fwext.NewConditionExtension(rsrc).MpiJobSucceeded(&job),
				wait.WithTimeout(time.Minute*20))
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			err := fwext.DeleteManifests(cfg.Client().RESTConfig(), renderedNeuronTrainManifest)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).Feature()

	testenv.Test(t, singleNode, singleNodeInfer, multiNodeTrain)
}
