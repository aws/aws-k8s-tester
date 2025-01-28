//go:build e2e

package training

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

var (
	//go:embed manifests/bert-training.yaml
	bertTrainingJobManifest []byte

	//go:embed manifests/training-comm-service.yaml
	trainingPodCommServiceManifest []byte

	// Regex to match lines like:
	// local_throughput=5.00 samples/s
	rankThroughputRegex = regexp.MustCompile(
		`local_throughput\s*=\s*([\d\.]+)\s+samples\/s`,
	)

	// Regex to match lines like:
	// local_avg_epoch_time=12.50s
	rankEpochTimeRegex = regexp.MustCompile(
		`local_avg_epoch_time=([\d\.]+)s`,
	)
)

// TestBertTraining runs the Neuron-based BERT training test
func TestBertTraining(t *testing.T) {
	if *bertTrainingImage == "" {
		t.Fatal("bertTrainingImage must be set to run the test")
	}

	// Render the templated manifest with dynamic variables
	renderVars := map[string]string{
		"BertTrainingImage": *bertTrainingImage,
		"NodeType":          *nodeType,
		"SlotsPerWorker":    fmt.Sprintf("%d", nodeCount),
		"NodeCount":         fmt.Sprintf("%d", nodeCount),
		"NeuronPerNode":     fmt.Sprintf("%d", neuronPerNode),
		"NeuronCorePerNode": fmt.Sprintf("%d", neuronCorePerNode),
		"EFAPerNode":        fmt.Sprintf("%d", efaPerNode),
	}

	// Render the manifest
	renderedManifest, err := fwext.RenderManifests(bertTrainingJobManifest, renderVars)
	if err != nil {
		t.Fatalf("failed to render neuron BERT training manifest: %v", err)
	}

	renderedCommServiceManifest, err := fwext.RenderManifests(trainingPodCommServiceManifest, renderVars)
	if err != nil {
		t.Fatalf("failed to render pod communication manifest: %v", err)
	}

	// Define a feature for the Neuron BERT training
	neuronTraining := features.New("bert-training").
		WithLabel("suite", "neuron").
		WithLabel("hardware", "neuron").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log.Println("Applying pod communication service manifest.")
			err := fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedCommServiceManifest)
			if err != nil {
				t.Fatalf("failed to apply communication service manifest: %v", err)
			}
			log.Println("Applying rendered Neuron training manifest.")
			err = fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedManifest)
			if err != nil {
				t.Fatalf("failed to apply Neuron training manifest: %v", err)
			}
			log.Println("Successfully applied Neuron training manifest.")
			return ctx
		}).
		Assess("Neuron training Job succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bert-training",
					Namespace: "default",
				},
			}

			// Step 1: Wait for the Job resource to appear
			log.Println("Waiting for the 'bert-training' Job resource to be created...")
			err := wait.For(
				conditions.New(cfg.Client().Resources()).ResourceMatch(job, func(object k8s.Object) bool {
					return true
				}),
				wait.WithTimeout(time.Minute*5),
			)
			if err != nil {
				t.Fatalf("Failed to detect creation of Job 'bert-training': %v", err)
			}
			log.Println("Job 'bert-training' is created in the cluster.")

			// Step 2: Wait for the Job to succeed (i.e., complete)
			log.Println("Waiting for 'bert-training' Job to succeed...")
			err = wait.For(
				fwext.NewConditionExtension(cfg.Client().Resources()).JobSucceeded(job),
				// Bake in large margin b/c compile time. TODO: pre-compile and find best fit
				wait.WithTimeout(60*time.Minute),
			)
			if err != nil {
				t.Fatalf("Neuron training Job did not succeed: %v", err)
			}
			log.Println("Job 'bert-training' succeeded!")

			// Gather logs from the training pods (launcher)
			logsBuf, logErr := gatherJobLogs(ctx, cfg, "default", "bert-training")
			if logErr != nil {
				log.Printf("Warning: failed to retrieve bert-training job logs: %v", logErr)
				return ctx
			}

			log.Println("== Raw Logs from the launcher pods ==")
			log.Println(logsBuf.String())

			// 1) Throughput Aggregation
			avgThru, sumThru, countThru := aggregateMetricFromLogs(rankThroughputRegex, logsBuf.String())
			if countThru == 0 {
				log.Printf("No throughput lines found. Possibly missing in logs.")
			} else {
				log.Printf("Parsed throughput from %d ranks. Total=%.2f samples/s, Average=%.2f samples/s",
					countThru, sumThru, avgThru)
				// Same log line format as nvidia training for parsing.
				log.Printf("Average Throughput: %.2f samples/second", avgThru)
			}

			// 2) Average Epoch Time Aggregation
			avgEp, sumEp, countEp := aggregateMetricFromLogs(rankEpochTimeRegex, logsBuf.String())
			if countEp == 0 {
				log.Printf("No epoch time lines found. Possibly missing in logs.")
			} else {
				log.Printf("Parsed average epoch time from %d ranks. Sum=%.2fs, Average=%.2fs",
					countEp, sumEp, avgEp)
			}

			return ctx
		}).
		Feature()

	// Run the feature
	testenv.Test(t, neuronTraining)
}

// gatherJobLogs retrieves logs from all pods of the specified jobName, returning them as a buffer.
func gatherJobLogs(ctx context.Context, cfg *envconf.Config, namespace, jobName string) (*bytes.Buffer, error) {
	clientset, err := getClientset(cfg.Client().RESTConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods for job %s: %w", jobName, err)
	}
	if len(podList.Items) == 0 {
		return nil, fmt.Errorf("no pods found for job %s", jobName)
	}

	var out bytes.Buffer
	for _, pod := range podList.Items {
		req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{})
		logStream, err := req.Stream(ctx)
		if err != nil {
			return &out, fmt.Errorf("failed to get logs from pod %s: %w", pod.Name, err)
		}
		defer logStream.Close()

		// Copy logs into our buffer
		if _, err := out.ReadFrom(logStream); err != nil {
			return &out, fmt.Errorf("failed to read logs from pod %s: %w", pod.Name, err)
		}
	}

	return &out, nil
}

// aggregateMetricFromLogs scans the log output for lines based on a provided RegEx.
// The RegEx is assumed to take a sufficiently unique form like <metric>=<value> to avoid
// collisions, but also to simplify parsing.
//
// returns the average, sum, and count for all occurrences of the metric.
func aggregateMetricFromLogs(metricRegex *regexp.Regexp, logs string) (avg float64, sum float64, count int) {
	matches := metricRegex.FindAllStringSubmatch(logs, -1)
	for _, match := range matches {
		val, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			sum += val
			count++
		}
	}
	if count > 0 {
		avg = sum / float64(count)
	}
	return avg, sum, count
}

// getClientset creates a Kubernetes clientset from the given REST config
func getClientset(restConfig *rest.Config) (*kubernetes.Clientset, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}
	return clientset, nil
}
