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
		Assess("Neuron training Job succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			manifests := [][]byte{renderedCommServiceManifest, renderedManifest}
			maxAttempts := (*retries) + 1

			for attempt := 0; attempt < maxAttempts; attempt++ {
				log.Printf("Applying manifests for BERT training test (Attempt #%d)", attempt+1)

				if err := applyManifests(cfg, manifests); err != nil {
					log.Printf("Failed to apply manifests: %v", err)
					cleanupManifests(cfg, manifests)
					continue
				}

				job, err := waitForJobCreation(cfg)
				if err != nil {
					log.Printf("Failed to detect job creation: %v", err)
					cleanupManifests(cfg, manifests)
					continue
				}

				if err := waitForJobCompletion(job, cfg); err != nil {
					log.Printf("Job did not complete successfully: %v", err)
					logsBuf, err := gatherJobLogs(ctx, cfg, "default", "bert-training")
					if err != nil {
						log.Printf("failed to get logs: %v", err)
					} else {
						log.Println(logsBuf.String())
					}
					cleanupManifests(cfg, manifests)
					continue
				}

				// Job completed successfully
				if err := processJobLogs(ctx, cfg); err != nil {
					log.Printf("Failed to process job logs: %v", err)
					cleanupManifests(cfg, manifests)
					continue
				}

				// Test succeeded, clean up and return
				cleanupManifests(cfg, manifests)
				log.Printf("BERT training test succeeded on attempt #%d", attempt+1)
				return ctx
			}

			// If we've exhausted all attempts
			t.Fatalf("BERT training test did not succeed after %d attempts", maxAttempts)
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

func applyManifests(cfg *envconf.Config, manifests [][]byte) error {
	fwext.ApplyManifests(cfg.Client().RESTConfig(), manifests...)
	log.Println("Successfully applied test manifests.")
	return nil
}

func waitForJobCreation(cfg *envconf.Config) (*batchv1.Job, error) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bert-training",
			Namespace: "default",
		},
	}

	log.Println("Waiting for the 'bert-training' Job resource to be created...")
	return job, wait.For(
		conditions.New(cfg.Client().Resources()).ResourceMatch(job, func(object k8s.Object) bool {
			return true
		}),
		wait.WithTimeout(time.Minute*5),
	)
}

func waitForJobCompletion(job *batchv1.Job, cfg *envconf.Config) error {
	log.Println("Waiting for 'bert-training' Job to succeed...")
	return wait.For(
		fwext.NewConditionExtension(cfg.Client().Resources()).JobSucceeded(job),
		wait.WithTimeout(30*time.Minute),
	)
}

func processJobLogs(ctx context.Context, cfg *envconf.Config) error {
	logsBuf, err := gatherJobLogs(ctx, cfg, "default", "bert-training")
	if err != nil {
		return fmt.Errorf("failed to retrieve bert-training job logs: %v", err)
	}

	log.Println("== Raw Logs from the launcher pods ==")
	log.Println(logsBuf.String())

	processMetrics(logsBuf.String())
	return nil
}

func processMetrics(logs string) {
	// Process throughput
	avgThru, sumThru, countThru := aggregateMetricFromLogs(rankThroughputRegex, logs)
	if countThru == 0 {
		log.Printf("No throughput lines found. Possibly missing in logs.")
	} else {
		log.Printf("Parsed throughput from %d ranks. Total=%.2f samples/s, Average=%.2f samples/s",
			countThru, sumThru, avgThru)
		log.Printf("Average Throughput: %.2f samples/second", avgThru)
	}

	// Process epoch time
	avgEp, sumEp, countEp := aggregateMetricFromLogs(rankEpochTimeRegex, logs)
	if countEp == 0 {
		log.Printf("No epoch time lines found. Possibly missing in logs.")
	} else {
		log.Printf("Parsed average epoch time from %d ranks. Sum=%.2fs, Average=%.2fs",
			countEp, sumEp, avgEp)
	}
}

func cleanupManifests(cfg *envconf.Config, manifests [][]byte) {
	log.Println("Deleting test manifests.")
	if err := fwext.DeleteManifests(cfg.Client().RESTConfig(), manifests...); err != nil {
		log.Printf("Failed to delete manifests: %v", err)
	}
}

// getClientset creates a Kubernetes clientset from the given REST config
func getClientset(restConfig *rest.Config) (*kubernetes.Clientset, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}
	return clientset, nil
}
