//go:build e2e

package training

import (
	"context"
	_ "embed"
	"fmt"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

// Use the parameterized manifest
var (
	//go:embed manifests/bert-training.yaml
	bertTrainingManifest []byte
)

func TestBertTraining(t *testing.T) {
	if *bertTrainingImage == "" {
		t.Fatal(fmt.Errorf("bertTrainingImage must be set to run the test"))
	}

	slotsPerWorker := gpuPerNode
	workerReplicas := nodeCount
	np := slotsPerWorker * workerReplicas
	efaRequested := 0
	if *efaEnabled && efaPerNode > 0 {
		efaRequested = 1
	}

	bertTraining := features.New("bert-training").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			renderVars := map[string]string{
				"BertTrainingImage": *bertTrainingImage,
				"SlotsPerWorker":    fmt.Sprintf("%d", slotsPerWorker),
				"NP":                fmt.Sprintf("%d", np),
				"WorkerReplicas":    fmt.Sprintf("%d", workerReplicas),
				"GPUPerNode":        fmt.Sprintf("%d", gpuPerNode),
				"EFARequested":      fmt.Sprintf("%d", efaRequested),
			}

			renderedManifest, err := fwext.RenderManifests(bertTrainingManifest, renderVars)
			if err != nil {
				t.Fatal(err)
			}

			err = fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedManifest)
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
				wait.WithTimeout(time.Minute*20),
				wait.WithContext(ctx),
			)
			if err != nil {
				t.Error(err)
			}

			err = printJobLogs(ctx, cfg, "default", "bert-training-launcher")
			if err != nil {
				t.Logf("Warning: failed to retrieve bert-training job logs: %v", err)
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, bertTraining)
}

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
				fmt.Printf("Logs from Pod %s: \n%s\n", pod.Name, string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}

	return nil
}

func getClientset(restConfig *rest.Config) (*kubernetes.Clientset, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}
	return clientset, nil
}
