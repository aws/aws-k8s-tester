package training

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	fwext "github.com/aws/aws-k8s-tester/e2e2/internal/framework_extensions"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

// Embedding Neuron test manifests
var (
	//go:embed manifests/neuron-bert-training.yaml
	neuronBertTrainingManifest []byte
)

// TestNeuronTraining runs the Neuron-based BERT training test
func TestNeuronTraining(t *testing.T) {
	if *bertTrainingImage == "" {
		t.Fatal("bertTrainingImage must be set to run the test")
	}

	// Render the manifests with dynamic variables
	renderVars := map[string]string{
		"BertTrainingImage": *bertTrainingImage,
		"SlotsPerWorker":    fmt.Sprintf("%d", neuronPerNode),
		"WorkerReplicas":    fmt.Sprintf("%d", nodeCount),
		"NP":                fmt.Sprintf("%d", neuronPerNode*nodeCount),
		"NeuronPerNode":     fmt.Sprintf("%d", neuronPerNode),
		"EFARequested":      fmt.Sprintf("%d", 0),
	}

	var renderedManifest []byte
	var err error
	if nodeCount == 1 {
		renderedNeuronSingleNodeManifest, err = fwext.RenderManifests(neuronSingleNodeManifest, renderVars)
		if err != nil {
			t.Fatalf("failed to render single-node manifest: %v", err)
		}
		renderedManifest = renderedNeuronSingleNodeManifest
	} else {
		renderedNeuronMultiNodeManifest, err = fwext.RenderManifests(neuronMultiNodeManifest, renderVars)
		if err != nil {
			t.Fatalf("failed to render multi-node manifest: %v", err)
		}
		renderedManifest = renderedNeuronMultiNodeManifest
	}

	// Define the feature
	neuronTraining := features.New("neuron-training").
		WithLabel("suite", "neuron").
		WithLabel("hardware", "neuron").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log.Println("Applying Neuron training manifest.")
			err := fwext.ApplyManifests(cfg.Client().RESTConfig(), neuronBertTrainingManifest)
			if err != nil {
				t.Fatalf("failed to apply neuron training manifest: %v", err)
			}
			log.Println("Successfully applied Neuron training manifest.")
			return ctx
		}).
		Assess("Neuron training Job succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "neuron-training-launcher", Namespace: "default"},
			}
			err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).JobSucceeded(job),
				wait.WithTimeout(time.Minute*30))
			if err != nil {
				return ctx
			}

			err = printJobLogs(ctx, cfg, "default", "neuron-training-launcher")
			if err != nil {
				t.Logf("Warning: failed to retrieve neuron-training job logs: %v", err)
			}

			return ctx
		}).
		Feature()

	// Run the feature
	testenv.Test(t, neuronTraining)
}

// printJobLogs retrieves and prints the logs of the specified job's pods
func printJobLogs(ctx context.Context, cfg *envconf.Config, namespace, jobName string) error {
	clientset, err := getClientset(cfg.Client().RESTConfig())
	if err != nil {
		return fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods for job %s: %w", jobName, err)
	}

	if len(podList.Items) == 0 {
		return fmt.Errorf("no pods found for job %s", jobName)
	}

	for _, pod := range podList.Items {
		req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{})
		logStream, err := req.Stream(ctx)
		if err != nil {
			return fmt.Errorf("failed to get logs from pod %s: %w", pod.Name, err)
		}
		defer logStream.Close()

		buf := make([]byte, 4096)
		for {
			n, err := logStream.Read(buf)
			if n > 0 {
				fmt.Printf("Logs from Pod %s:\n%s\n", pod.Name, string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}

	return nil
}

// getClientset creates a Kubernetes clientset from the given REST config
func getClientset(restConfig *rest.Config) (*kubernetes.Clientset, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}
	return clientset, nil
}
