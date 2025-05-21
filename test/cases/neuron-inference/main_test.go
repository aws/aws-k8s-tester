//go:build e2e

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
	"github.com/aws/aws-k8s-tester/test/manifests"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

func TestMain(m *testing.M) {

	flag.Parse()

	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("[ERROR] Failed to create test environment: %v", err)
	}
	testenv = env.NewWithConfig(cfg)

	deploymentManifests := [][]byte{
		manifests.NeuronDevicePluginRbacManifest,
		manifests.NeuronDevicePluginManifest,
	}

	// Setup steps: apply the device plugin, wait for DS readiness, discover capacity
	testenv.Setup(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("Applying Neuron device plugin RBAC and Neuron device plugin manifests.")
			err := fwext.ApplyManifests(config.Client().RESTConfig(), deploymentManifests...)
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
		getNodeCapacity,
	)

	// Finish steps: remove device plugin if desired
	testenv.Finish(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("[INFO] Cleaning up Neuron device plugin.")
			slices.Reverse(deploymentManifests)
			if err := fwext.DeleteManifests(config.Client().RESTConfig(), deploymentManifests...); err != nil {
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

	// Check Neuron devices
	log.Println("[INFO] Checking Neuron device capacity on nodes")
	err := wait.For(
		fwext.NewConditionExtension(config.Client().Resources()).AllNodesHaveNonZeroResourceCapacity("aws.amazon.com/neuron"),
		wait.WithTimeout(time.Second*60),
		wait.WithInterval(time.Second*5),
	)
	if err != nil {
		return ctx, fmt.Errorf("failed to verify Neuron device capacity on nodes: %w", err)
	}
	log.Println("[INFO] Neuron devices check passed - all nodes have non-zero capacity")

	// Check Neuron cores
	log.Println("[INFO] Checking Neuron core capacity on nodes")
	err = wait.For(
		fwext.NewConditionExtension(config.Client().Resources()).AllNodesHaveNonZeroResourceCapacity("aws.amazon.com/neuroncore"),
		wait.WithTimeout(time.Second*60),
		wait.WithInterval(time.Second*5),
	)
	if err != nil {
		return ctx, fmt.Errorf("failed to verify Neuron core capacity on nodes: %w", err)
	}
	log.Println("[INFO] Neuron cores check passed - all nodes have non-zero capacity")

	log.Println("[INFO] Neuron capacity discovery complete.")
	return ctx, nil
}

func getNodeCapacity(ctx context.Context, config *envconf.Config) (context.Context, error) {
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
	var totalNeuron, totalNeuronCore, nodeCount int
	// if nodeType not set, use the instance type discovered
	if *nodeType == "" {
		*nodeType = nodes.Items[0].Labels["node.kubernetes.io/instance-type"]
	}
	for _, node := range nodes.Items {
		instanceType := node.Labels["node.kubernetes.io/instance-type"]
		neuronCap, hasNeuron := node.Status.Capacity["aws.amazon.com/neuron"]
		neuronCoreCap, hasNeuronCore := node.Status.Capacity["aws.amazon.com/neuroncore"]
		if instanceType == *nodeType {
			nodeCount++
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
	}
	if nodeCount > 0 {
		neuronPerNode = totalNeuron / nodeCount
		neuronCorePerNode = totalNeuronCore / nodeCount
	} else {
		return ctx, fmt.Errorf("no nodes with %s node type found in the cluster", *nodeType)
	}
	log.Printf("[INFO] Discovered neuronPerNode=%d, neuronCorePerNode=%d (across %d node(s))", neuronPerNode, neuronCorePerNode, nodeCount)
	return ctx, nil
}
