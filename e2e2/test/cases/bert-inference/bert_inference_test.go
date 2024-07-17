package bert_inference

import (
	"context"
	_ "embed"
	"fmt"
	"os/exec"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/e2e2/internal/framework_extensions"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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
	var labeledNodeName string
	bertInference := features.New("bert-inference").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if *bertInferenceImage == "" {
				t.Fatal(fmt.Errorf("bertInferenceImage must be set to run the test"))
			}

			// Label a node with GPU present
			labelNodeWithGPU(ctx, t, cfg)

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
			// Unlabel the node
			if labeledNodeName != "" {
				unlabelNode(ctx, t, cfg, labeledNodeName)
			}
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

func labelNodeWithGPU(ctx context.Context, t *testing.T, cfg *envconf.Config) {
	t.Helper()

	// Fetch the list of nodes
	nodes := &corev1.NodeList{}
	err := cfg.Client().Resources().List(ctx, nodes)
	if err != nil {
		t.Fatalf("Failed to list nodes: %v", err)
	}

	if len(nodes.Items) == 0 {
		t.Fatal("No nodes found in the cluster")
	}

	// Label the first node with the GPU label
	nodeName := nodes.Items[0].Name
	cmd := exec.Command("kubectl", "label", "node", nodeName, "nvidia.com/gpu.present=true")
	err = cmd.Run()
	if err != nil {
		t.Fatalf("Failed to label node: %v", err)
	}

	t.Logf("Labeled node %s with nvidia.com/gpu.present=true", nodeName)
}

func unlabelNode(ctx context.Context, t *testing.T, cfg *envconf.Config, nodeName string) {
	t.Helper()
	cmd := exec.Command("kubectl", "label", "node", nodeName, "nvidia.com/gpu.present-")
	err := cmd.Run()
	if err != nil {
		t.Errorf("Failed to unlabel node: %v", err)
	} else {
		t.Logf("Unlabeled node %s", nodeName)
	}
}
