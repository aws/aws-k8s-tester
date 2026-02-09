//go:build e2e

package inference

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/signal"
	"slices"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/aws-k8s-tester/test/common"
	"github.com/aws/aws-k8s-tester/test/manifests"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

type TestConfig struct {
	common.MetricOps
	BertInferenceImage string `flag:"bertInferenceImage" desc:"BERT inference container image"`
	InferenceMode      string `flag:"inferenceMode" desc:"Inference mode for BERT (throughput or latency)"`
	GpuRequested       int    `flag:"gpuRequested" desc:"Number of GPUs required for inference"`
}

var (
	testenv    env.Environment
	testConfig TestConfig
)

func TestMain(m *testing.M) {
	// Initialize testConfig with default values
	testConfig = TestConfig{
		InferenceMode: "throughput",
		GpuRequested:  1,
	}

	_, err := common.ParseFlags(&testConfig)
	if err != nil {
		log.Fatalf("[ERROR] Failed to parse flags: %v", err)
	}
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("[ERROR] Failed to initialize test environment: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	testenv = env.NewWithConfig(cfg).WithContext(ctx)

	manifestsList := [][]byte{
		manifests.NvidiaDevicePluginManifest,
	}

	if len(testConfig.MetricDimensions) > 0 {
		// Render CloudWatch Agent manifest with dynamic dimensions
		renderedCloudWatchAgentManifest, err := manifests.RenderCloudWatchAgentManifest(testConfig.MetricDimensions)
		if err != nil {
			log.Printf("Warning: Failed to render CloudWatch Agent manifest: %v", err)
		}
		manifestsList = append(manifestsList, manifests.DCGMExporterManifest, renderedCloudWatchAgentManifest)
	}

	testenv.Setup(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("[INFO] Applying manifests.")
			err := fwext.ApplyManifests(config.Client().RESTConfig(), manifestsList...)
			if err != nil {
				return ctx, fmt.Errorf("[ERROR] Failed to apply manifests: %w", err)
			}
			log.Println("[INFO] Successfully applied manifests.")
			return ctx, nil
		},
		common.DeployDaemonSet("nvidia-device-plugin-daemonset", "kube-system"),
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			if len(testConfig.MetricDimensions) > 0 {
				if ctx, err := common.DeployDaemonSet("dcgm-exporter", "kube-system")(ctx, config); err != nil {
					return ctx, err
				}
				if ctx, err := common.DeployDaemonSet("cwagent", "amazon-cloudwatch")(ctx, config); err != nil {
					return ctx, err
				}
			}
			return ctx, nil
		},
		checkGpuCapacity,
	)

	testenv.Finish(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("[INFO] Deleting manifests.")
			slices.Reverse(manifestsList)
			err := fwext.DeleteManifests(config.Client().RESTConfig(), manifestsList...)
			if err != nil {
				return ctx, fmt.Errorf("[ERROR] failed to delete manifests: %w", err)
			}
			log.Println("[INFO] Successfully deleted manifests.")
			return ctx, nil
		},
	)

	exitCode := testenv.Run(m)
	log.Printf("[INFO] Tests finished with exit code %d", exitCode)
	os.Exit(exitCode)
}

// checkGpuCapacity ensures at least one node has >= the requested number of GPUs,
// and logs each node's instance type.
func checkGpuCapacity(ctx context.Context, config *envconf.Config) (context.Context, error) {
	log.Printf("[INFO] Validating cluster has at least %d GPU(s).", testConfig.GpuRequested)

	cs, err := kubernetes.NewForConfig(config.Client().RESTConfig())
	if err != nil {
		return ctx, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	err = wait.For(func(ctx context.Context) (bool, error) {
		nodes, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to list nodes: %w", err)
		} else if len(nodes.Items) == 0 {
			return false, fmt.Errorf("no nodes found in the cluster")
		}
		for _, node := range nodes.Items {
			instanceType := node.Labels["node.kubernetes.io/instance-type"]
			gpuCap, ok := node.Status.Capacity["nvidia.com/gpu"]
			if ok && int(gpuCap.Value()) >= testConfig.GpuRequested {
				log.Printf("[INFO] Node %s (type: %s) meets the request of %d GPU(s).",
					node.Name, instanceType, testConfig.GpuRequested)
				return true, nil
			}
			log.Printf("[INFO] Node %s (type: %s) has no GPU capacity.", node.Name, instanceType)
		}
		log.Printf("[INFO] No node meets the GPU requirement. The GPU info might not be propagated yet. Retrying...")
		return false, nil
	}, wait.WithTimeout(5*time.Minute), wait.WithInterval(10*time.Second))

	if err != nil {
		return ctx, fmt.Errorf("no node has >= %d GPU(s)", testConfig.GpuRequested)
	}

	log.Println("[INFO] GPU capacity check passed.")
	return ctx, nil
}
