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

	testConfig = Config{
		MetricOps:              testConfig.MetricOps,
		NodeType:               testConfig.NodeType,
		InstallDevicePlugin:    true,
		EfaEnabled:             testConfig.EfaEnabled,
		NvidiaTestImage:        testConfig.NvidiaTestImage,
		PytorchImage:           "763104351884.dkr.ecr.us-west-2.amazonaws.com/pytorch-training:2.1.0-gpu-py310-cu121-ubuntu20.04-ec2",
		SkipUnitTestSubcommand: testConfig.SkipUnitTestSubcommand,
	}

	manifestsList := [][]byte{
		manifests.MpiOperatorManifest,
	}

	setUpFunctions := []env.Func{
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
		setUpFunctions = append(setUpFunctions, func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			return common.DeployDaemonSet("nvidia-device-plugin-daemonset", "kube-system")(ctx, config)
		})
	}

	if testConfig.EfaEnabled {
		manifestsList = append(manifestsList, manifests.EfaDevicePluginManifest)
		setUpFunctions = append(setUpFunctions, func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			return common.DeployDaemonSet("aws-efa-k8s-device-plugin-daemonset", "kube-system")(ctx, config)
		})
	}

	if len(testConfig.MetricDimensions) > 0 {
		renderedCloudWatchAgentManifest, err := manifests.RenderCloudWatchAgentManifest(testConfig.MetricDimensions)
		if err != nil {
			log.Printf("Warning: failed to render CloudWatch Agent manifest: %v", err)
		}
		manifestsList = append(manifestsList, manifests.DCGMExporterManifest, renderedCloudWatchAgentManifest)
		setUpFunctions = append(setUpFunctions, func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			if ctx, err := common.DeployDaemonSet("dcgm-exporter", "kube-system")(ctx, config); err != nil {
				return ctx, err
			}
			if ctx, err := common.DeployDaemonSet("cwagent", "amazon-cloudwatch")(ctx, config); err != nil {
				return ctx, err
			}
			return ctx, nil
		})
	}

	setUpFunctions = append(setUpFunctions, checkNodeTypes)
	testenv.Setup(setUpFunctions...)

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
