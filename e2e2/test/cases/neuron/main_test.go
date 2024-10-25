package neuron

import (
	"context"
	_ "embed"
	"flag"
	"log"
	"os"
	"slices"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/e2e2/internal/framework_extensions"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	testenv             env.Environment
	nodeType            *string
	efaEnabled          *bool
	nodeCount           int
	neuronPerNode       int
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
		return ctx, err
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return ctx, err
	}

	singleNodeType := true
	for i := 1; i < len(nodes.Items)-1; i++ {
		if nodes.Items[i].Labels["node.kubernetes.io/instance-type"] != nodes.Items[i-1].Labels["node.kubernetes.io/instance-type"] {
			singleNodeType = false
		}
	}
	if !singleNodeType {
		return ctx, fmt.Errorf("Node types are not the same, all node types must be the same in the cluster")
	}

	if *nodeType != "" {
		for _, v := range nodes.Items {
			if v.Labels["node.kubernetes.io/instance-type"] == *nodeType {
				nodeCount++
				neuron := v.Status.Capacity["aws.amazon.com/neuron"]
				neuronPerNode = int(neuron.Value())
				efa := v.Status.Capacity["vpc.amazonaws.com/efa"]
				efaPerNode = int(efa.Value())
			}
		}
	} else {
		log.Printf("No node type specified. Using the node type %s in the node groups.", nodes.Items[0].Labels["node.kubernetes.io/instance-type"])
		nodeType = aws.String(nodes.Items[0].Labels["node.kubernetes.io/instance-type"])
		nodeCount = len(nodes.Items)
		neuron := nodes.Items[0].Status.Capacity["aws.amazon.com/neuron"]
		neuronPerNode = int(gpu.Value())
		efa := nodes.Items[0].Status.Capacity["vpc.amazonaws.com/efa"]
		efaPerNode = int(efa.Value())
	}

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
		manifests = append(manifests, neuronDevicePluginManifest, neuronDevicePlugiRbacManifest)
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
