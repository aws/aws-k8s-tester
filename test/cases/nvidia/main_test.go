//go:build e2e

package nvidia

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/signal"
	"slices"
	"testing"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/aws-k8s-tester/test/common"
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

type Config struct {
	common.MetricOps
	NodeType               string `flag:"nodeType" desc:"node type for the tests"`
	InstallDevicePlugin    bool   `flag:"installDevicePlugin" desc:"install nvidia device plugin"`
	EfaEnabled             bool   `flag:"efaEnabled" desc:"enable efa tests"`
	NvidiaTestImage        string `flag:"nvidiaTestImage" desc:"nccl test image for nccl tests"`
	PytorchImage           string `flag:"pytorchImage" desc:"pytorch cuda image for single node tests"`
	SkipUnitTestSubcommand string `flag:"skipUnitTestSubcommand" desc:"optional command to skip specified unit test"`
}

var (
	testenv    env.Environment
	testConfig Config
	nodeCount  int
	gpuPerNode int
	efaPerNode int
)

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

func checkNodeTypes(ctx context.Context, config *envconf.Config) (context.Context, error) {
	clientset, err := kubernetes.NewForConfig(config.Client().RESTConfig())
	if err != nil {
		return ctx, err
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return ctx, err
	}

	for i := 1; i < len(nodes.Items)-1; i++ {
		if nodes.Items[i].Labels["node.kubernetes.io/instance-type"] != nodes.Items[i-1].Labels["node.kubernetes.io/instance-type"] {
			return ctx, fmt.Errorf("Node types are not the same, all node types must be the same in the cluster")
		}
	}

	if testConfig.NodeType != "" {
		for _, v := range nodes.Items {
			if v.Labels["node.kubernetes.io/instance-type"] == testConfig.NodeType {
				nodeCount++
				gpu := v.Status.Capacity["nvidia.com/gpu"]
				gpuPerNode = int(gpu.Value())
				efa := v.Status.Capacity["vpc.amazonaws.com/efa"]
				efaPerNode = int(efa.Value())
			}
		}
	} else {
		log.Printf("No node type specified. Using the node type %s in the node groups.", nodes.Items[0].Labels["node.kubernetes.io/instance-type"])
		testConfig.NodeType = nodes.Items[0].Labels["node.kubernetes.io/instance-type"]
		nodeCount = len(nodes.Items)
		gpu := nodes.Items[0].Status.Capacity["nvidia.com/gpu"]
		gpuPerNode = int(gpu.Value())
		efa := nodes.Items[0].Status.Capacity["vpc.amazonaws.com/efa"]
		efaPerNode = int(efa.Value())
	}

	return ctx, nil
}

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

	// Set default values
	if testConfig.PytorchImage == "" {
		testConfig.PytorchImage = "763104351884.dkr.ecr.us-west-2.amazonaws.com/pytorch-training:2.1.0-gpu-py310-cu121-ubuntu20.04-ec2"
	}
	if !testConfig.InstallDevicePlugin {
		testConfig.InstallDevicePlugin = true
	}

	renderedCloudWatchAgentManifest, err := manifests.RenderCloudWatchAgentManifest(testConfig.MetricDimensions)
	if err != nil {
		log.Printf("Warning: failed to render CloudWatch Agent manifest: %v", err)
	}

	manifestsList := [][]byte{
		manifests.MpiOperatorManifest,
	}
	setupFunctions := []env.Func{
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			err := fwext.ApplyManifests(config.Client().RESTConfig(), manifestsList...)
			if err != nil {
				return ctx, err
			}
			return ctx, nil
		},
		deployMPIOperator,
	}

	if testConfig.InstallDevicePlugin {
		manifestsList = append(manifestsList, manifests.NvidiaDevicePluginManifest)
		setupFunctions = append(setupFunctions, common.DeployDaemonSet("nvidia-device-plugin-daemonset", "kube-system"))
	}

	if testConfig.EfaEnabled {
		manifestsList = append(manifestsList, manifests.EfaDevicePluginManifest)
		setupFunctions = append(setupFunctions, common.DeployDaemonSet("aws-efa-k8s-device-plugin-daemonset", "kube-system"))
	}

	setupFunctions = append(setupFunctions, checkNodeTypes)

	if len(testConfig.MetricDimensions) > 0 {
		manifestsList = append(manifestsList, manifests.DCGMExporterManifest, renderedCloudWatchAgentManifest)
		setupFunctions = append(setupFunctions,
			common.DeployDaemonSet("dcgm-exporter", "kube-system"),
			common.DeployDaemonSet("cwagent", "amazon-cloudwatch"),
		)
	}

	testenv.Setup(setupFunctions...)

	testenv.Finish(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			slices.Reverse(manifestsList)
			err := fwext.DeleteManifests(config.Client().RESTConfig(), manifestsList...)
			if err != nil {
				return ctx, err
			}
			return ctx, nil
		},
	)

	os.Exit(testenv.Run(m))
}
