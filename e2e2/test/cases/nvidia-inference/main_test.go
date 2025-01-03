package inference

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/e2e2/internal/framework_extensions"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

//go:embed manifests/nvidia-device-plugin.yaml
var nvidiaDevicePluginManifest []byte

var (
	testenv            env.Environment
	bertInferenceImage *string
	inferenceMode      *string
	gpuRequested       *int
)

func TestMain(m *testing.M) {
	bertInferenceImage = flag.String("bertInferenceImage", "", "BERT inference container image")
	inferenceMode = flag.String("inferenceMode", "throughput", "Inference mode for BERT (throughput or latency)")
	gpuRequested = flag.Int("gpuRequested", 1, "Number of GPUs required for inference")

	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("[ERROR] Failed to create test environment: %v", err)
	}
	testenv = env.NewWithConfig(cfg)

	devicePluginManifests := [][]byte{nvidiaDevicePluginManifest}

	testenv.Setup(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("[INFO] Applying NVIDIA device plugin.")
			if applyErr := fwext.ApplyManifests(config.Client().RESTConfig(), devicePluginManifests...); applyErr != nil {
				return ctx, fmt.Errorf("failed to apply device plugin: %w", applyErr)
			}
			return ctx, nil
		},
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			ds := &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nvidia-device-plugin-daemonset",
					Namespace: "kube-system",
				},
			}
			err := wait.For(
				fwext.NewConditionExtension(config.Client().Resources()).DaemonSetReady(ds),
				wait.WithTimeout(5*time.Minute),
			)
			if err != nil {
				return ctx, fmt.Errorf("device plugin daemonset not ready: %w", err)
			}
			log.Println("[INFO] NVIDIA device plugin is ready.")
			return ctx, nil
		},
		checkGpuCapacity,
	)

	testenv.Finish(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("[INFO] Cleaning up NVIDIA device plugin.")
			slices.Reverse(devicePluginManifests)
			if delErr := fwext.DeleteManifests(config.Client().RESTConfig(), devicePluginManifests...); delErr != nil {
				return ctx, fmt.Errorf("failed to delete device plugin: %w", delErr)
			}
			log.Println("[INFO] Device plugin cleanup complete.")
			return ctx, nil
		},
	)

	exitCode := testenv.Run(m)
	log.Printf("[INFO] Test environment finished with exit code %d", exitCode)
	os.Exit(exitCode)
}

// checkGpuCapacity ensures at least one node has >= the requested number of GPUs,
// and logs each node's instance type.
func checkGpuCapacity(ctx context.Context, config *envconf.Config) (context.Context, error) {
	log.Printf("[INFO] Validating cluster has at least %d GPU(s).", *gpuRequested)

	cs, err := kubernetes.NewForConfig(config.Client().RESTConfig())
	if err != nil {
		return ctx, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	nodes, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return ctx, fmt.Errorf("failed to list nodes: %w", err)
	}
	if len(nodes.Items) == 0 {
		return ctx, fmt.Errorf("no nodes found in the cluster")
	}

	var found bool
	for _, node := range nodes.Items {
		instanceType := node.Labels["node.kubernetes.io/instance-type"]
		gpuCap, ok := node.Status.Capacity["nvidia.com/gpu"]
		if !ok {
			log.Printf("[INFO] Node %s (type: %s) has no GPU capacity.", node.Name, instanceType)
			continue
		}

		log.Printf("[INFO] Node %s (type: %s) reports %d GPU(s).", node.Name, instanceType, gpuCap.Value())

		if int(gpuCap.Value()) >= *gpuRequested {
			log.Printf("[INFO] Node %s (type: %s) meets the request of %d GPU(s).",
				node.Name, instanceType, *gpuRequested)
			found = true
		}
	}

	if !found {
		return ctx, fmt.Errorf("no node has >= %d GPU(s)", *gpuRequested)
	}

	log.Println("[INFO] GPU capacity check passed.")
	return ctx, nil
}
