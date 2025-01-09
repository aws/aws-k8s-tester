package training

import (
	"context"
	_ "embed"
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
	//go:embed manifests/k8s-neuron-device-plugin-rbac.yml
	neuronDevicePluginRbacManifest []byte
	//go:embed manifests/k8s-neuron-device-plugin.yml
	neuronDevicePluginManifest []byte
	//go:embed manifests/mpi-operator.yaml
	mpiOperatorManifest []byte
	//go:embed manifests/efa-device-plugin.yaml
	efaDevicePluginManifest []byte
)

func TestMain(m *testing.M) {
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to initialize test environment: %v", err)
	}
	testenv = env.NewWithConfig(cfg)

	manifests := [][]byte{
		neuronDevicePluginRbacManifest,
		neuronDevicePluginManifest,
		mpiOperatorManifest,
		efaDevicePluginManifest,
	}

	testenv.Setup(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("Applying Neuron device plugin RBAC, Neuron device plugin, MPI operator, and EFA device plugin manifests.")
			err := fwext.ApplyManifests(config.Client().RESTConfig(), manifests...)
			if err != nil {
				return ctx, fmt.Errorf("failed to apply manifests: %w", err)
			}
			log.Println("Successfully applied Neuron device plugin RBAC, Neuron device plugin, MPI operator, and EFA device plugin manifests.")
			return ctx, nil
		},
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("Waiting for MPI Operator deployment to be available.")
			deployment := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "mpi-operator", Namespace: "mpi-operator"},
			}
			err := wait.For(
				conditions.New(config.Client().Resources()).DeploymentConditionMatch(
					&deployment, appsv1.DeploymentAvailable, v1.ConditionTrue,
				),
				wait.WithTimeout(time.Minute*5),
			)
			if err != nil {
				return ctx, fmt.Errorf("MPI Operator deployment is not available: %w", err)
			}
			log.Println("MPI Operator deployment is available.")
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
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("Waiting for EFA Device Plugin daemonset to be ready.")
			daemonset := appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Name: "aws-efa-k8s-device-plugin-daemonset", Namespace: "kube-system"},
			}
			err := wait.For(
				fwext.NewConditionExtension(config.Client().Resources()).DaemonSetReady(&daemonset),
				wait.WithTimeout(time.Minute*5),
			)
			if err != nil {
				return ctx, fmt.Errorf("EFA Device Plugin daemonset is not ready: %w", err)
			}
			log.Println("EFA Device Plugin daemonset is ready.")
			return ctx, nil
		},
		checkNodeTypes,
	)

	testenv.Finish(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("Deleting Neuron device plugin, MPI operator, and EFA device plugin manifests.")
			slices.Reverse(manifests)
			err := fwext.DeleteManifests(config.Client().RESTConfig(), manifests...)
			if err != nil {
				return ctx, fmt.Errorf("failed to delete manifests: %w", err)
			}
			log.Println("Successfully deleted Neuron device plugin, MPI operator, and EFA device plugin manifests.")
			return ctx, nil
		},
	)

	log.Println("Starting tests...")
	exitCode := testenv.Run(m)
	log.Printf("Tests finished with exit code %d", exitCode)
	os.Exit(exitCode)
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
