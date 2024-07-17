package bert_inference

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
	//go:embed manifests/bert-inference.yaml
	bertInferenceManifest         []byte
	renderedBertInferenceManifest []byte
)

type bertInferenceManifestTplVars struct {
	BertInferenceImage string
	InferenceMode      string
}

func TestBertInference(t *testing.T) {
	bertInference := features.New("bert-inference").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if *bertInferenceImage == "" {
				t.Fatal(fmt.Errorf("bertInferenceImage must be set to run the test"))
			}

			var err error
			renderedBertInferenceManifest, err = fwext.RenderManifests(bertInferenceManifest, bertInferenceManifestTplVars{
				BertInferenceImage: *bertInferenceImage,
				InferenceMode:      *inferenceMode,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedBertInferenceManifest)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("BERT inference Job succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
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
			// Delete the manifest
			err := fwext.DeleteManifests(cfg.Client().RESTConfig(), renderedBertInferenceManifest)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, bertInference)
}
