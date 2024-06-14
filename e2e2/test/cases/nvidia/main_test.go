package nvidia

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

var (
	testenv       env.Environment
	nodeType      *string
	efaEnabled    *bool
	ncclTestImage *string
	nodeCount     int
	gpuPerNode    int
	efaPerNode    int
)

var (
	//go:embed manifests/nvidia-device-plugin.yaml
	nvidiaDevicePluginManifest []byte
	//go:embed manifests/mpi-operator.yaml
	mpiOperatorManifest []byte
	//go:embed manifests/efa-device-plugin.yaml
	efaDevicePluginManifest []byte
)

func TestMain(m *testing.M) {
	nodeType = flag.String("nodeType", "", "node type for the tests")
	ncclTestImage = flag.String("ncclTestImage", "", "nccl test image for nccl tests")
	efaEnabled = flag.Bool("efaEnabled", false, "enable efa tests")
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to initialize test environment: %v", err)
	}
	testenv = env.NewWithConfig(cfg)

	// all NVIDIA tests require the device plugin and MPI operator
	manifests := [][]byte{
		nvidiaDevicePluginManifest,
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
			dep := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "mpi-operator", Namespace: "mpi-operator"},
			}
			err := wait.For(conditions.New(config.Client().Resources()).DeploymentConditionMatch(&dep, appsv1.DeploymentAvailable, v1.ConditionTrue),
				wait.WithTimeout(time.Minute*5))
			if err != nil {
				return ctx, err
			}
			return ctx, nil
		},
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			ds := appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Name: "nvidia-device-plugin-daemonset", Namespace: "kube-system"},
			}
			err := wait.For(fwext.NewConditionExtension(config.Client().Resources()).DaemonSetReady(&ds),
				wait.WithTimeout(time.Minute*5))
			if err != nil {
				return ctx, err
			}
			return ctx, nil
		},
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			clientset, err := kubernetes.NewForConfig(cfg.Client().RESTConfig())
			if err != nil {
				return ctx, err
			}
			nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if err != nil {
				return ctx, err
			}
			if *efaEnabled {
				err := fwext.ApplyManifests(cfg.Client().RESTConfig(), efaDevicePluginManifest)
				if err != nil {
					return ctx, err
				}
				ds := appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{Name: "efa-device-plugin-daemonset", Namespace: "kube-system"},
				}
				err = wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).DaemonSetReady(&ds),
					wait.WithTimeout(time.Minute*5))
				if err != nil {
					return ctx, err
				}
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
						gpu := v.Status.Capacity["nvidia.com/gpu"]
						gpuPerNode = int(gpu.Value())
						efa := v.Status.Capacity["vpc.amazonaws.com/efa"]
						efaPerNode = int(efa.Value())
					}
				}
			} else {
				log.Printf("No node type specified. Using the node type %s in the node groups.", nodes.Items[0].Labels["node.kubernetes.io/instance-type"])
				nodeCount = len(nodes.Items)
				gpu := nodes.Items[0].Status.Capacity["nvidia.com/gpu"]
				gpuPerNode = int(gpu.Value())
				efa := nodes.Items[0].Status.Capacity["vpc.amazonaws.com/efa"]
				efaPerNode = int(efa.Value())
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
