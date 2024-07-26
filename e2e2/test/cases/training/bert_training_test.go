package training

import (
	"context"
	_ "embed"
	"fmt"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/e2e2/internal/framework_extensions"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	//go:embed manifests/bert-training.yaml
	bertTrainingManifest         []byte
	renderedBertTrainingManifest []byte
)

type bertTrainingManifestTplVars struct {
	BertTrainingImage string
	TrainingMode      string
}

func TestBertTraining(t *testing.T) {
	bertTraining := features.New("bert-training").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if *bertTrainingImage == "" {
				t.Fatal(fmt.Errorf("bertTrainingImage must be set to run the test"))
			}

			var err error
			renderedBertTrainingManifest, err = fwext.RenderManifests(bertTrainingManifest, bertTrainingManifestTplVars{
				BertTrainingImage: *bertTrainingImage,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedBertTrainingManifest)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("BERT training Job succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "bert-training-launcher", Namespace: "default"},
			}
			err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).JobSucceeded(job),
				wait.WithTimeout(time.Minute*20))
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Delete the manifest
			err := fwext.DeleteManifests(cfg.Client().RESTConfig(), renderedBertTrainingManifest)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, bertTraining)
}
