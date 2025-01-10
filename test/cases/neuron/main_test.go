//go:build e2e

package neuron

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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	testenv             env.Environment
	nodeType            *string
	efaEnabled          *bool
	nodeCount           int
	neuronPerNode       int
	neuronCorePerNode   int
	efaPerNode          int
	neuronTestImage     *string
	installDevicePlugin *bool
)

var (
	//go:embed manifests/k8s-neuron-device-plugin-rbac.yml
	neuronDevicePluginRbacManifest []byte
	//go:embed manifests/k8s-neuron-device-plugin.yml
	neuronDevicePluginManifest []byte
	//go:embed manifests/mpi-operator.yaml
	mpiOperatorManifest []byte
	//go:embed manifests/efa-device-plugin.yaml
	efaDevicePluginManifest []byte
)

func deployNeuronDevicePlugin(ctx context.Context, config *envconf.Config) (context.Context, error) {
	ds := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "neuron-device-plugin-daemonset", Namespace: "kube-system"},
	}
	err := wait.For(fwext.NewConditionExtension(config.Client().Resources()).DaemonSetReady(&ds),
		wait.WithContext(ctx))
	if err != nil {
		return ctx, err
	}
	return ctx, nil
}

func deployMPIOperator(ctx context.Context, config *envconf.Config) (context.Context, error) {
	dep := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "mpi-operator", Namespace: "mpi-operator"},
	}
	err := wait.For(conditions.New(config.Client().Resources()).DeploymentConditionMatch(&dep, appsv1.DeploymentAvailable, v1.ConditionTrue),
		wait.WithContext(ctx))
	if err != nil {
		return ctx, fmt.Errorf("failed to deploy mpi-operator: %v", err)
	}
	return ctx, nil
}

func deployEFAPlugin(ctx context.Context, config *envconf.Config) (context.Context, error) {
	err := fwext.ApplyManifests(config.Client().RESTConfig(), efaDevicePluginManifest)
	if err != nil {
		return ctx, err
	}

	ds := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "aws-efa-k8s-device-plugin-daemonset", Namespace: "kube-system"},
	}
	err = wait.For(fwext.NewConditionExtension(config.Client().Resources()).DaemonSetReady(&ds),
		wait.WithContext(ctx))
	if err != nil {
		return ctx, fmt.Errorf("failed to deploy efa-device-plugin: %v", err)
	}

	return ctx, nil
}

func checkNodeTypes(ctx context.Context, config *envconf.Config) (context.Context, error) {
	clientset, err := kubernetes.NewForConfig(config.Client().RESTConfig())
	if err != nil {
		return ctx, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return ctx, fmt.Errorf("failed to list nodes: %w", err)
	}

	if len(nodes.Items) == 0 {
		return ctx, fmt.Errorf("no nodes found in the cluster")
	}

	// Check if all nodes have the same instance type
	for i := 1; i < len(nodes.Items); i++ {
		if nodes.Items[i].Labels["node.kubernetes.io/instance-type"] != nodes.Items[i-1].Labels["node.kubernetes.io/instance-type"] {
			return ctx, fmt.Errorf("inconsistent node types detected, all nodes must have the same instance type")
		}
	}

	// Calculate capacities for all nodes
	totalNeuronCount := 0
	totalNeuronCoreCount := 0
	totalEfaCount := 0
	nodeCount = len(nodes.Items) // Store global node count

	for _, node := range nodes.Items {
		log.Printf("[INFO] Processing node %s", node.Name)

		// Check for Neuron capacity
		neuron, ok := node.Status.Capacity["aws.amazon.com/neuron"]
		if ok {
			totalNeuronCount += int(neuron.Value())
		} else {
			log.Printf("[WARN] Node %s does not have 'aws.amazon.com/neuron' capacity", node.Name)
		}

		// Check for NeuronCore capacity
		neuronCore, ok := node.Status.Capacity["aws.amazon.com/neuroncore"]
		if ok {
			totalNeuronCoreCount += int(neuronCore.Value())
		} else {
			log.Printf("[WARN] Node %s does not have 'aws.amazon.com/neuroncore' capacity", node.Name)
		}

		// Check for EFA capacity
		efa, ok := node.Status.Capacity["vpc.amazonaws.com/efa"]
		if ok {
			totalEfaCount += int(efa.Value())
		} else {
			log.Printf("[WARN] Node %s does not have 'vpc.amazonaws.com/efa' capacity", node.Name)
		}
	}

	// Update global capacities
	if nodeCount > 0 {
		neuronPerNode = totalNeuronCount / nodeCount
		neuronCorePerNode = totalNeuronCoreCount / nodeCount
		efaPerNode = totalEfaCount / nodeCount
	} else {
		log.Printf("[WARN] No nodes found, setting capacities to 0")
		neuronPerNode = 0
		neuronCorePerNode = 0
		efaPerNode = 0
	}

	log.Printf("[INFO] Total Nodes: %d", nodeCount)
	log.Printf("[INFO] Total Neuron Count: %d, Neuron Per Node: %d", totalNeuronCount, neuronPerNode)
	log.Printf("[INFO] Total Neuron Core Count: %d, Neuron Core Per Node: %d", totalNeuronCoreCount, neuronCorePerNode)
	log.Printf("[INFO] Total EFA Count: %d, EFA Per Node: %d", totalEfaCount, efaPerNode)

	return ctx, nil
}

func TestMain(m *testing.M) {
	nodeType = flag.String("nodeType", "", "node type for the tests")
	efaEnabled = flag.Bool("efaEnabled", false, "enable efa tests")
	neuronTestImage = flag.String("neuronTestImage", "", "image for neuron single node test")
	installDevicePlugin = flag.Bool("installDevicePlugin", true, "install neuron device plugin")
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to initialize test environment: %v", err)
	}
	testenv = env.NewWithConfig(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Minute)
	defer cancel()
	testenv = testenv.WithContext(ctx)

	manifests := [][]byte{
		mpiOperatorManifest,
	}
	setUpFunctions := []env.Func{
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			err := fwext.ApplyManifests(config.Client().RESTConfig(), manifests...)
			if err != nil {
				return ctx, err
			}
			return ctx, nil
		},
		deployMPIOperator,
		checkNodeTypes,
	}

	if *installDevicePlugin {
		manifests = append(manifests, neuronDevicePluginManifest, neuronDevicePluginRbacManifest)
		setUpFunctions = append(setUpFunctions, deployNeuronDevicePlugin)
	}

	if *efaEnabled {
		setUpFunctions = append(setUpFunctions, deployEFAPlugin)
	}

	testenv.Setup(setUpFunctions...)

	testenv.Finish(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			err := fwext.DeleteManifests(cfg.Client().RESTConfig(), efaDevicePluginManifest)
			if err != nil {
				return ctx, err
			}
			slices.Reverse(manifests)
			err = fwext.DeleteManifests(config.Client().RESTConfig(), manifests...)
			if err != nil {
				return ctx, err
			}
			return ctx, nil
		},
	)

	os.Exit(testenv.Run(m))
}
