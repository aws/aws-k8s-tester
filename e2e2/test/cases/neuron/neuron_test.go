package neuron

import (
	"context"
	_ "embed"
	"fmt"
	"testing"

	fwext "github.com/aws/aws-k8s-tester/e2e2/internal/framework_extensions"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	//go:embed manifests/single-node-test-neuronx.yaml
	neuronSingleNodeManifest         []byte
	renderedNeuronSingleNodeManifest []byte
)

type neuronSingleNodeManifestTplVars struct {
	NeuronTestImage string
}

func TestMPIJobPytorchTraining(t *testing.T) {
	singleNode := features.New("single-node").
		WithLabel("suite", "neuron").
		WithLabel("hardware", "gpu").
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
			err = fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedNeuronSingleNodeManifest)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("Single node test Job succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "neuronx-single-node", Namespace: "default"},
			}
			err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).JobSucceeded(job),
				wait.WithContext(ctx))
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			err := fwext.DeleteManifests(cfg.Client().RESTConfig(), renderedNeuronSingleNodeManifest)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, singleNode)
}
