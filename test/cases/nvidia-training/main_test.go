//go:build e2e

package training

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/aws/aws-k8s-tester/test/common"
	"log"
	"os"
	"os/signal"
	"slices"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/aws-k8s-tester/test/manifests"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

func TestMain(m *testing.M) {
	_, err := common.ParseFlags(&testConfig)
	if err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to initialize test environment: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	testenv = env.NewWithConfig(cfg).WithContext(ctx)

	// Render CloudWatch Agent manifest with dynamic dimensions
	renderedCloudWatchAgentManifest, err := manifests.RenderCloudWatchAgentManifest(testConfig.MetricDimensions)
	if err != nil {
		log.Printf("Warning: failed to render CloudWatch Agent manifest: %v", err)
	}

	manifestsList := [][]byte{
		manifests.NvidiaDevicePluginManifest,
		manifests.MpiOperatorManifest,
		manifests.EfaDevicePluginManifest,
	}

	if len(testConfig.MetricDimensions) > 0 {
		manifestsList = append(manifestsList, manifests.DCGMExporterManifest, renderedCloudWatchAgentManifest)
	}

	testenv.Setup(
		// Apply all manifests
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("Applying manifests.")

			err := fwext.ApplyManifests(config.Client().RESTConfig(), manifestsList...)
			if err != nil {
				return ctx, fmt.Errorf("failed to apply manifests: %w", err)
			}
			log.Println("Successfully applied manifests.")
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

		// Wait for required DaemonSets
		common.DeployDaemonSet("nvidia-device-plugin-daemonset", "kube-system"),
		common.DeployDaemonSet("aws-efa-k8s-device-plugin-daemonset", "kube-system"),
		checkNodeTypes, // Dynamically check node types and capacities after device plugins are ready
	)

	if len(testConfig.MetricDimensions) > 0 {
		testenv.Setup(
			common.DeployDaemonSet("dcgm-exporter", "kube-system"),
			common.DeployDaemonSet("cwagent", "amazon-cloudwatch"),
		)
	}

	testenv.Finish(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("Deleting NVIDIA device plugin, MPI operator, EFA device plugin DCGM Exporter and CloudWatch Agent manifests.")
			slices.Reverse(manifestsList)
			err := fwext.DeleteManifests(config.Client().RESTConfig(), manifestsList...)
			if err != nil {
				return ctx, fmt.Errorf("failed to delete manifests: %w", err)
			}
			log.Println("Successfully deleted NVIDIA device plugin, MPI operator, EFA device plugin, DCGM Exporter and CloudWatch Agent manifests.")
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

	for i := 1; i < len(nodes.Items); i++ {
		if nodes.Items[i].Labels["node.kubernetes.io/instance-type"] != nodes.Items[i-1].Labels["node.kubernetes.io/instance-type"] {
			return ctx, fmt.Errorf("node types are not the same, all node types must be the same in the cluster")
		}
	}

	if testConfig.NodeType != "" {
		count := 0
		for _, v := range nodes.Items {
			if v.Labels["node.kubernetes.io/instance-type"] == testConfig.NodeType {
				count++
				if gpuCap, ok := v.Status.Capacity["nvidia.com/gpu"]; ok {
					gpuPerNode = int(gpuCap.Value())
				}
				if efaCap, ok := v.Status.Capacity["vpc.amazonaws.com/efa"]; ok {
					efaPerNode = int(efaCap.Value())
				}
			}
		}
		if count == 0 {
			return ctx, fmt.Errorf("no nodes match the specified nodeType: %s", testConfig.NodeType)
		}
		nodeCount = count
	} else {
		testConfig.NodeType = nodes.Items[0].Labels["node.kubernetes.io/instance-type"]
		nodeCount = len(nodes.Items)
		if gpuCap, ok := nodes.Items[0].Status.Capacity["nvidia.com/gpu"]; ok {
			gpuPerNode = int(gpuCap.Value())
		}
		if efaCap, ok := nodes.Items[0].Status.Capacity["vpc.amazonaws.com/efa"]; ok {
			efaPerNode = int(efaCap.Value())
		}
	}

	log.Printf("[INFO] Node Type: %s", testConfig.NodeType)
	log.Printf("[INFO] Node Count: %d", nodeCount)
	log.Printf("[INFO] GPU Per Node: %d", gpuPerNode)
	log.Printf("[INFO] EFA Per Node: %d", efaPerNode)

	return ctx, nil
}
