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

	fwext "github.com/aws/aws-k8s-tester/e2e2/internal/framework_extensions"
	"github.com/aws/aws-sdk-go-v2/aws"
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
	testenv           env.Environment
	neuronTestImage   *string
	nodeType          *string
	nodeCount         int
	neuronPerNode     int
	neuronCorePerNode int
	efaPerNode        int
)

var (
	//go:embed manifests/k8s-neuron-device-plugin-rbac.yml
	neuronDevicePlugiRbacManifest []byte
	//go:embed manifests/k8s-neuron-device-plugin.yml
	neuronDevicePluginManifest []byte
	//go:embed manifests/efa-device-plugin.yaml
	efaDevicePluginManifest []byte
	//go:embed manifests/mpi-operator.yaml
	mpiOperatorManifest []byte
)

func TestMain(m *testing.M) {
	nodeType = flag.String("nodeType", "", "node type for the tests")
	neuronTestImage = flag.String("neuronTestImage", "", "image for neuron single node test")
	efaEnabled := flag.Bool("efaEnabled", false, "enable efa tests")
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to initialize test environment: %v", err)
	}
	testenv = env.NewWithConfig(cfg)

	manifests := [][]byte{
		neuronDevicePluginManifest,
		neuronDevicePlugiRbacManifest,
		mpiOperatorManifest,
	}

	testenv.Setup(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			err := fwext.ApplyManifests(config.Client().RESTConfig(), manifests...)
			if err != nil {
				return ctx, err
			}
			return ctx, nil
		},
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			neuronDs := appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Name: "neuron-device-plugin-daemonset", Namespace: "kube-system"},
			}
			err := wait.For(fwext.NewConditionExtension(config.Client().Resources()).DaemonSetReady(&neuronDs),
				wait.WithTimeout(time.Minute*5))
			if err != nil {
				return ctx, err
			}
			dep := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "mpi-operator", Namespace: "mpi-operator"},
			}
			err = wait.For(conditions.New(config.Client().Resources()).DeploymentConditionMatch(&dep, appsv1.DeploymentAvailable, v1.ConditionTrue),
				wait.WithTimeout(time.Minute*5))
			if err != nil {
				return ctx, err
			}
			if *efaEnabled {
				err := fwext.ApplyManifests(cfg.Client().RESTConfig(), efaDevicePluginManifest)
				if err != nil {
					return ctx, err
				}
				efaDs := appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{Name: "aws-efa-k8s-device-plugin-daemonset", Namespace: "kube-system"},
				}
				err = wait.For(fwext.NewConditionExtension(config.Client().Resources()).DaemonSetReady(&efaDs),
					wait.WithTimeout(time.Minute*5))
				if err != nil {
					return ctx, err
				}
			}
			return ctx, nil
		},
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			if *neuronTestImage == "" {
				log.Fatal(fmt.Errorf("neuronTestImage must be set to run neuron single node test, use https://github.com/aws/aws-k8s-tester/blob/main/e2e2/test/images/neuron to build the image and -neuronTestImage to set the image url"))
			}
			clientset, err := kubernetes.NewForConfig(cfg.Client().RESTConfig())
			if err != nil {
				return ctx, err
			}
			nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if err != nil {
				return ctx, err
			}
			err = fwext.ApplyManifests(cfg.Client().RESTConfig(), efaDevicePluginManifest)
			if err != nil {
				return ctx, err
			}
			ds := appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Name: "aws-efa-k8s-device-plugin-daemonset", Namespace: "kube-system"},
			}
			err = wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).DaemonSetReady(&ds),
				wait.WithTimeout(time.Minute*5))
			if err != nil {
				return ctx, err
			}

			if *nodeType == "" {
				log.Printf("No node type specified. Using the node type %s in the node groups.", nodes.Items[0].Labels["node.kubernetes.io/instance-type"])
				nodeType = aws.String(nodes.Items[0].Labels["node.kubernetes.io/instance-type"])
			}
			for _, v := range nodes.Items {
				if v.Labels["node.kubernetes.io/instance-type"] == *nodeType {
					nodeCount++
					neuron := v.Status.Capacity["aws.amazon.com/neuron"]
					neuronPerNode = int(neuron.Value())
					neuronCore := v.Status.Capacity["aws.amazon.com/neuroncore"]
					neuronCorePerNode = int(neuronCore.Value())
					efa := v.Status.Capacity["vpc.amazonaws.com/efa"]
					efaPerNode = int(efa.Value())
				}
			}
			return ctx, nil
		},
	)

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
