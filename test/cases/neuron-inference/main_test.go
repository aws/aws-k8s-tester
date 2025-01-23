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

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	//go:embed manifests/k8s-neuron-device-plugin-rbac.yml
	neuronDevicePluginRbacManifest []byte
	//go:embed manifests/k8s-neuron-device-plugin.yml
	neuronDevicePluginManifest []byte
)

func TestMain(m *testing.M) {

	flag.Parse()

	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("[ERROR] Failed to create test environment: %v", err)
	}
	testenv = env.NewWithConfig(cfg)

	manifests := [][]byte{
		neuronDevicePluginRbacManifest,
		neuronDevicePluginManifest,
	}

	// Setup steps: apply the device plugin, wait for DS readiness, discover capacity
	testenv.Setup(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("Applying Neuron device plugin RBAC and Neuron device plugin manifests.")
			err := fwext.ApplyManifests(config.Client().RESTConfig(), manifests...)
			if err != nil {
				return ctx, fmt.Errorf("failed to apply manifests: %w", err)
			}
			log.Println("Successfully applied Neuron device plugin RBAC and Neuron device plugin manifests.")
			return ctx, nil
		},
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("Waiting for Neuron Device Plugin daemonset to be ready.")
			daemonset := appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Name: "neuron-device-plugin-daemonset", Namespace: "kube-system"},
			}
			err := wait.For(
				fwext.NewConditionExtension(config.Client().Resources()).DaemonSetReady(&daemonset),
				wait.WithTimeout(time.Minute*5),
			)
			if err != nil {
				return ctx, fmt.Errorf("Neuron Device Plugin daemonset is not ready: %w", err)
			}
			log.Println("Neuron Device Plugin daemonset is ready.")
			return ctx, nil
		},
		discoverNeuronCoreCapacity,
	)

	// Finish steps: remove device plugin if desired
	testenv.Finish(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("[INFO] Cleaning up Neuron device plugin.")
			slices.Reverse(manifests)
			if err := fwext.DeleteManifests(config.Client().RESTConfig(), manifests...); err != nil {
				return ctx, fmt.Errorf("failed to delete neuron device plugin: %w", err)
			}
			log.Println("[INFO] Neuron device plugin cleanup complete.")
			return ctx, nil
		},
	)

	exitCode := testenv.Run(m)
	log.Printf("[INFO] Test environment finished with exit code %d", exitCode)
	os.Exit(exitCode)
}

// discoverNeuronCoreCapacity sets neuronPerNode and neuronCorePerNode by scanning the cluster
func discoverNeuronCoreCapacity(ctx context.Context, config *envconf.Config) (context.Context, error) {
	log.Println("[INFO] Discovering cluster's Neuron capacity...")

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

	var totalNeuron, totalNeuronCore int
	for _, node := range nodes.Items {
		instanceType := node.Labels["node.kubernetes.io/instance-type"]
		neuronCap, hasNeuron := node.Status.Capacity["aws.amazon.com/neuron"]
		neuronCoreCap, hasNeuronCore := node.Status.Capacity["aws.amazon.com/neuroncore"]

		if hasNeuron {
			totalNeuron += int(neuronCap.Value())
		} else {
			log.Printf("[WARN] Node %s (type=%s) lacks 'aws.amazon.com/neuron'.", node.Name, instanceType)
		}

		if hasNeuronCore {
			totalNeuronCore += int(neuronCoreCap.Value())
		} else {
			log.Printf("[WARN] Node %s (type=%s) lacks 'aws.amazon.com/neuroncore'.", node.Name, instanceType)
		}
	}

	nodeCount := len(nodes.Items)
	if nodeCount > 0 {
		neuronPerNode = totalNeuron / nodeCount
		neuronCorePerNode = totalNeuronCore / nodeCount
	}

	log.Printf("[INFO] Discovered neuronPerNode=%d, neuronCorePerNode=%d (across %d node(s))",
		neuronPerNode, neuronCorePerNode, nodeCount)

	if neuronCorePerNode <= 0 {
		return ctx, fmt.Errorf("discovered %d neuronCorePerNode => no Neuron capacity found", neuronCorePerNode)
	}

	log.Println("[INFO] Neuron capacity discovery complete.")
	return ctx, nil
}
