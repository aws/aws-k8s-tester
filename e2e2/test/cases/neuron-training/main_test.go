package training

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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// Embedding Neuron-related manifests
var (
	// Neuron Device Plugin RBAC Manifest
	//go:embed manifests/k8s-neuron-device-plugin-rbac.yml
	neuronDevicePluginRbacManifest []byte

	// Neuron Device Plugin Manifest
	//go:embed manifests/k8s-neuron-device-plugin.yml
	neuronDevicePluginManifest []byte

	// MPI Operator Manifest
	//go:embed manifests/mpi-operator.yaml
	mpiOperatorManifest []byte

	// EFA Device Plugin Manifest
	//go:embed manifests/efa-device-plugin.yaml
	efaDevicePluginManifest []byte

	// Rendered manifests (if any preprocessing is needed)
	renderedNeuronSingleNodeManifest []byte
	renderedNeuronMultiNodeManifest  []byte
)

func TestMain(m *testing.M) {
	// Initialize flags
	flag.Parse()

	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to initialize test environment: %v", err)
	}

	testenv = env.NewWithConfig(cfg)

	// List of manifests to apply
	manifests := [][]byte{
		neuronDevicePluginRbacManifest,
		neuronDevicePluginManifest,
		mpiOperatorManifest,
		efaDevicePluginManifest,
	}

	testenv.Setup(
		// Apply all manifests
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("Applying Neuron RBAC, Neuron Device Plugin, MPI Operator, and EFA Device Plugin manifests.")
			err := fwext.ApplyManifests(config.Client().RESTConfig(), manifests...)
			if err != nil {
				return ctx, fmt.Errorf("failed to apply manifests: %w", err)
			}
			log.Println("Successfully applied all device plugin and MPI operator manifests.")
			return ctx, nil
		},

		// Wait for MPI Operator deployment
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

		// Wait for Neuron Device Plugin DaemonSet
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("Waiting for Neuron Device Plugin daemonset to be ready.")
			daemonset := appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Name: "neuron-device-plugin-daemonset", Namespace: "kube-system"},
			}
			err := wait.For(
				conditions.New(config.Client().Resources()).DaemonSetReady(&daemonset),
				wait.WithTimeout(time.Minute*5),
			)
			if err != nil {
				return ctx, fmt.Errorf("Neuron Device Plugin daemonset is not ready: %w", err)
			}
			log.Println("Neuron Device Plugin daemonset is ready.")
			return ctx, nil
		},

		// Wait for EFA Device Plugin DaemonSet
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("Waiting for EFA Device Plugin daemonset to be ready.")
			daemonset := appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Name: "aws-efa-k8s-device-plugin-daemonset", Namespace: "kube-system"},
			}
			err := wait.For(
				conditions.New(config.Client().Resources()).DaemonSetReady(&daemonset),
				wait.WithTimeout(time.Minute*5),
			)
			if err != nil {
				return ctx, fmt.Errorf("EFA Device Plugin daemonset is not ready: %w", err)
			}
			log.Println("EFA Device Plugin daemonset is ready.")
			return ctx, nil
		},

		checkNodeTypes, // Dynamically check node types and capacities after device plugins are ready
	)

	testenv.Finish(
		// Teardown: Delete all applied manifests
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("Deleting all device plugin and MPI operator manifests.")
			slices.Reverse(manifests)
			err := fwext.DeleteManifests(config.Client().RESTConfig(), manifests...)
			if err != nil {
				return ctx, fmt.Errorf("failed to delete manifests: %w", err)
			}
			log.Println("Successfully deleted all device plugin and MPI operator manifests.")
			return ctx, nil
		},
	)

	log.Println("Starting tests...")
	exitCode := testenv.Run(m)
	log.Printf("Tests finished with exit code %d", exitCode)
	os.Exit(exitCode)
}

// checkNodeTypes checks that all nodes are of the same type and records resource capacities
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

	singleNodeType := true
	for i := 1; i < len(nodes.Items); i++ {
		if nodes.Items[i].Labels["node.kubernetes.io/instance-type"] != nodes.Items[i-1].Labels["node.kubernetes.io/instance-type"] {
			singleNodeType = false
			break
		}
	}
	if !singleNodeType {
		return ctx, fmt.Errorf("node types are not the same, all node types must be the same in the cluster")
	}

	if *nodeType != "" {
		count := 0
		for _, v := range nodes.Items {
			if v.Labels["node.kubernetes.io/instance-type"] == *nodeType {
				count++
				if gpuCap, ok := v.Status.Capacity["nvidia.com/gpu"]; ok {
					gpuPerNode = int(gpuCap.Value())
				}
				if efaCap, ok := v.Status.Capacity["vpc.amazonaws.com/efa"]; ok {
					efaPerNode = int(efaCap.Value())
				}
				if neuronCap, ok := v.Status.Capacity["aws.amazon.com/neuron"]; ok {
					neuronPerNode = int(neuronCap.Value())
				}
			}
		}
		if count == 0 {
			return ctx, fmt.Errorf("no nodes match the specified nodeType: %s", *nodeType)
		}
		nodeCount = count
	} else {
		*nodeType = nodes.Items[0].Labels["node.kubernetes.io/instance-type"]
		nodeCount = len(nodes.Items)
		if gpuCap, ok := nodes.Items[0].Status.Capacity["nvidia.com/gpu"]; ok {
			gpuPerNode = int(gpuCap.Value())
		}
		if efaCap, ok := nodes.Items[0].Status.Capacity["vpc.amazonaws.com/efa"]; ok {
			efaPerNode = int(efaCap.Value())
		}
		if neuronCap, ok := nodes.Items[0].Status.Capacity["aws.amazon.com/neuron"]; ok {
			neuronPerNode = int(neuronCap.Value())
		}
	}

	log.Printf("[INFO] Node Type: %s", *nodeType)
	log.Printf("[INFO] Node Count: %d", nodeCount)
	log.Printf("[INFO] GPU Per Node: %d", gpuPerNode)
	log.Printf("[INFO] EFA Per Node: %d", efaPerNode)
	log.Printf("[INFO] Neuron Per Node: %d", neuronPerNode)

	return ctx, nil
}
