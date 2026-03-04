//go:build e2e

package inference

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"log"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

//go:embed manifests/neuron-bert-inference.yaml
var neuronBertInferenceManifest []byte

var renderedManifest []byte

func TestNeuronInference(t *testing.T) {
	if *bertInferenceImage == "" {
		t.Fatal("bertInferenceImage must be set to run the test")
	}

	log.Printf("[INFO] Using nodeType=%s, inferenceMode=%s", *nodeType, *inferenceMode)
	log.Printf("[INFO] Discovered neuronPerNode=%d, neuronCorePerNode=%d", neuronPerNode, neuronCorePerNode)

	renderVars := map[string]string{
		"BertInferenceImage": *bertInferenceImage,
		"NodeType":           *nodeType,      // e.g. "inf2.xlarge"
		"InferenceMode":      *inferenceMode, // "throughput" or "latency"
		"NeuronPerNode":      fmt.Sprintf("%d", neuronPerNode),
		"NeuronCorePerNode":  fmt.Sprintf("%d", neuronCorePerNode),
	}

	// Render the manifest
	renderedManifest, err := fwext.RenderManifests(neuronBertInferenceManifest, renderVars)
	if err != nil {
		t.Fatalf("[ERROR] Failed to render Neuron inference manifest: %v", err)
	}
	log.Printf("[DEBUG] Rendered manifest:\n%s", string(renderedManifest))

	feature := features.New("neuron-inference").
		WithLabel("suite", "neuron").
		WithLabel("hardware", "neuron").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log.Println("[INFO] Applying rendered Neuron inference manifest.")
			err := fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedManifest)
			if err != nil {
				t.Fatalf("[ERROR] Failed to apply Neuron inference manifest: %v", err)
			}
			log.Println("[INFO] Successfully applied Neuron inference manifest.")
			return ctx
		}).
		Assess("BERT inference Job succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log.Println("[INFO] Checking 'neuron-inference' job completion...")

			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "neuron-inference", Namespace: "default"},
			}
			if err := wait.For(
				fwext.NewConditionExtension(cfg.Client().Resources()).JobSucceeded(job),
				wait.WithTimeout(60*time.Minute),
			); err != nil {
				log.Println("[ERROR] Neuron inference job failed. Gathering diagnostics...")
				if diagErr := printJobDiagnostics(ctx, cfg, "default", "neuron-inference"); diagErr != nil {
					t.Logf("[WARNING] Failed to retrieve job diagnostics: %v", diagErr)
				}
				if logErr := printJobLogs(ctx, cfg, "default", "neuron-inference"); logErr != nil {
					t.Logf("[WARNING] Failed to retrieve job logs: %v", logErr)
				}
				t.Fatalf("[ERROR] Neuron inference job did not succeed: %v", err)
			}

			log.Println("[INFO] Neuron inference job succeeded. Gathering logs...")
			applyTime := ctx.Value("applyTime")
			if applyTime != nil {
				if start, ok := applyTime.(time.Time); ok {
					duration := time.Since(start)
					log.Printf("[INFO] Neuron inference job completed in %s", duration)
				}
			}

			if err := printJobLogs(ctx, cfg, "default", "neuron-inference"); err != nil {
				t.Logf("[WARNING] Failed to retrieve neuron-inference job logs: %v", err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log.Println("[INFO] Cleaning up neuron-inference job resources...")
			if err := fwext.DeleteManifests(cfg.Client().RESTConfig(), renderedManifest); err != nil {
				t.Fatalf("[ERROR] Failed to delete inference job resources: %v", err)
			}
			log.Println("[INFO] Inference job cleanup complete.")
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

func printJobDiagnostics(ctx context.Context, cfg *envconf.Config, namespace, jobName string) error {
	cs, err := getClientset(cfg.Client().RESTConfig())
	if err != nil {
		return fmt.Errorf("[ERROR] failed to create kubernetes client: %w", err)
	}

	// Get job status
	job, err := cs.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("[ERROR] failed to get job %s: %w", jobName, err)
	}

	log.Printf("[INFO] Job Status: Active=%d, Succeeded=%d, Failed=%d",
		job.Status.Active, job.Status.Succeeded, job.Status.Failed)

	for _, condition := range job.Status.Conditions {
		log.Printf("[INFO] Job Condition: Type=%s, Status=%s, Reason=%s, Message=%s",
			condition.Type, condition.Status, condition.Reason, condition.Message)
	}

	// Get events for the job
	events, err := cs.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Job", jobName),
	})
	if err != nil {
		log.Printf("[WARNING] Failed to get events for job: %v", err)
	} else {
		for _, event := range events.Items {
			log.Printf("[INFO] Job Event: Type=%s, Reason=%s, Message=%s",
				event.Type, event.Reason, event.Message)
		}
	}

	// Get pods and their events
	pods, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		log.Printf("[WARNING] Failed to list pods for job %s: %v", jobName, err)
		return nil
	}

	if len(pods.Items) == 0 {
		log.Printf("[WARNING] No pods found for job %s", jobName)
		log.Printf("[INFO] Job spec details:")
		log.Printf("[INFO]   NodeSelector: %v", job.Spec.Template.Spec.NodeSelector)
		log.Printf("[INFO]   Tolerations: %v", job.Spec.Template.Spec.Tolerations)
		if len(job.Spec.Template.Spec.Containers) > 0 {
			log.Printf("[INFO]   Resource requests: %v", job.Spec.Template.Spec.Containers[0].Resources.Requests)
			log.Printf("[INFO]   Resource limits: %v", job.Spec.Template.Spec.Containers[0].Resources.Limits)
		}

		// Check if matching nodes exist
		if nodeSelector := job.Spec.Template.Spec.NodeSelector; len(nodeSelector) > 0 {
			nodes, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if err != nil {
				log.Printf("[WARNING] Failed to list nodes: %v", err)
			} else {
				matchingNodes := 0
				for _, node := range nodes.Items {
					matches := true
					for key, value := range nodeSelector {
						if node.Labels[key] != value {
							matches = false
							break
						}
					}
					if matches {
						matchingNodes++
						log.Printf("[INFO] Found matching node: %s (capacity: %v)", node.Name, node.Status.Capacity)
					}
				}
				if matchingNodes == 0 {
					log.Printf("[ERROR] No nodes found matching nodeSelector: %v", nodeSelector)
				}
			}
		}

		// List all pods in namespace to see cluster-wide scheduling status
		allPods, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			log.Printf("[WARNING] Failed to list all pods in namespace: %v", err)
		} else {
			log.Printf("[INFO] Total pods in namespace %s: %d", namespace, len(allPods.Items))
			pendingCount := 0
			runningCount := 0
			for _, pod := range allPods.Items {
				if pod.Status.Phase == v1.PodPending {
					pendingCount++
					log.Printf("[INFO] Pending pod: %s (reason: %s)", pod.Name, pod.Status.Reason)
				} else if pod.Status.Phase == v1.PodRunning {
					runningCount++
				}
			}
			log.Printf("[INFO] Pod summary: %d Running, %d Pending, %d Other", runningCount, pendingCount, len(allPods.Items)-runningCount-pendingCount)
		}
		return nil
	}

	for _, pod := range pods.Items {
		log.Printf("[INFO] Pod %s: Phase=%s, Node=%s", pod.Name, pod.Status.Phase, pod.Spec.NodeName)

		for _, condition := range pod.Status.Conditions {
			log.Printf("[INFO] Pod %s Condition: Type=%s, Status=%s, Reason=%s, Message=%s",
				pod.Name, condition.Type, condition.Status, condition.Reason, condition.Message)
		}

		// Get pod events
		podEvents, err := cs.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", pod.Name),
		})
		if err != nil {
			log.Printf("[WARNING] Failed to get events for pod %s: %v", pod.Name, err)
		} else {
			for _, event := range podEvents.Items {
				log.Printf("[INFO] Pod %s Event: Type=%s, Reason=%s, Message=%s",
					pod.Name, event.Type, event.Reason, event.Message)
			}
		}
	}

	return nil
}

func printJobLogs(ctx context.Context, cfg *envconf.Config, namespace, jobName string) error {
	cs, err := getClientset(cfg.Client().RESTConfig())
	if err != nil {
		return fmt.Errorf("[ERROR] failed to create kubernetes client: %w", err)
	}

	pods, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		return fmt.Errorf("[ERROR] failed to list pods for job %s: %w", jobName, err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("[ERROR] no pods found for job %s", jobName)
	}

	for _, pod := range pods.Items {
		log.Printf("[INFO] Pod %s is on node %s", pod.Name, pod.Spec.NodeName)
		stream, err := cs.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{}).Stream(ctx)
		if err != nil {
			return fmt.Errorf("[ERROR] failed to get logs from pod %s: %w", pod.Name, err)
		}
		defer stream.Close()

		buf := make([]byte, 4096)
		for {
			n, readErr := stream.Read(buf)
			if n > 0 {
				log.Printf("[INFO] Logs from Pod %s:\n%s", pod.Name, string(buf[:n]))
			}
			if readErr == io.EOF {
				log.Printf("[INFO] Completed log stream for pod %s.", pod.Name)
				break
			}
			if readErr != nil {
				return fmt.Errorf("[ERROR] reading logs from pod %s: %w", pod.Name, readErr)
			}
		}
	}
	return nil
}

func getClientset(restConfig *rest.Config) (*kubernetes.Clientset, error) {
	cs, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot create kubernetes clientset: %w", err)
	}
	return cs, nil
}
