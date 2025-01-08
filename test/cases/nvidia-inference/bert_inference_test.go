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

//go:embed manifests/bert-inference.yaml
var bertInferenceManifest []byte

var renderedBertInferenceManifest []byte

type bertInferenceManifestTplVars struct {
	BertInferenceImage string
	InferenceMode      string
	GPUPerNode         string
}

func TestBertInference(t *testing.T) {
	feature := features.New("bert-inference").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if *bertInferenceImage == "" {
				t.Fatalf("[ERROR] bertInferenceImage must be set")
			}

			log.Println("[INFO] Rendering BERT inference manifest...")
			var err error
			renderedBertInferenceManifest, err = fwext.RenderManifests(
				bertInferenceManifest,
				bertInferenceManifestTplVars{
					BertInferenceImage: *bertInferenceImage,
					InferenceMode:      *inferenceMode,
					GPUPerNode:         fmt.Sprintf("%d", *gpuRequested),
				},
			)
			if err != nil {
				t.Fatalf("[ERROR] Failed to render BERT inference manifest: %v", err)
			}

			log.Println("[INFO] Applying BERT inference manifest...")
			if applyErr := fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedBertInferenceManifest); applyErr != nil {
				t.Fatalf("[ERROR] Failed to apply BERT inference manifest: %v", applyErr)
			}
			log.Println("[INFO] BERT inference manifest applied successfully.")

			// Record time after applying the manifest
			ctx = context.WithValue(ctx, "applyTime", time.Now())
			return ctx
		}).
		Assess("BERT inference Job succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log.Println("[INFO] Checking BERT inference job completion...")
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "bert-inference", Namespace: "default"},
			}
			err := wait.For(
				fwext.NewConditionExtension(cfg.Client().Resources()).JobSucceeded(job),
				wait.WithTimeout(20*time.Minute),
			)
			if err != nil {
				t.Fatalf("[ERROR] BERT inference job did not succeed: %v", err)
			}
			log.Println("[INFO] BERT inference job succeeded. Gathering logs...")

			// Compute duration from manifest apply to job success
			startVal := ctx.Value("applyTime")
			if startVal != nil {
				if applyTime, ok := startVal.(time.Time); ok {
					duration := time.Since(applyTime)
					log.Printf("[INFO] BERT inference job completed in %s", duration)
				}
			}

			// Print logs (including node name) for the Pod
			if err := printJobLogs(ctx, cfg, "default", "bert-inference"); err != nil {
				t.Logf("[WARNING] Failed to retrieve BERT inference job logs: %v", err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log.Println("[INFO] Cleaning up BERT inference job resources...")
			if err := fwext.DeleteManifests(cfg.Client().RESTConfig(), renderedBertInferenceManifest); err != nil {
				t.Fatalf("[ERROR] Failed to delete BERT inference manifest: %v", err)
			}
			log.Println("[INFO] BERT inference job resources cleaned up.")
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

func printJobLogs(ctx context.Context, cfg *envconf.Config, namespace, jobName string) error {
	cs, err := getClientset(cfg.Client().RESTConfig())
	if err != nil {
		return fmt.Errorf("[ERROR] Failed to create kubernetes clientset: %w", err)
	}

	pods, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		return fmt.Errorf("[ERROR] Failed to list pods for job %s: %w", jobName, err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("[ERROR] No pods found for job %s", jobName)
	}

	for _, pod := range pods.Items {
		log.Printf("[INFO] Pod %s is running on node %s", pod.Name, pod.Spec.NodeName)

		log.Printf("[INFO] Retrieving logs from pod %s...", pod.Name)
		stream, err := cs.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{}).Stream(ctx)
		if err != nil {
			return fmt.Errorf("[ERROR] Failed to get logs from pod %s: %w", pod.Name, err)
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
				return fmt.Errorf("[ERROR] Failed to read logs from pod %s: %w", pod.Name, readErr)
			}
		}
	}
	return nil
}

func getClientset(restConfig *rest.Config) (*kubernetes.Clientset, error) {
	cs, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("[ERROR] Cannot create kubernetes clientset: %w", err)
	}
	return cs, nil
}
