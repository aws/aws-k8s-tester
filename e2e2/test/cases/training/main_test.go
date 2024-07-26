package training

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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	testenv           env.Environment
	bertTrainingImage *string
)

var (
	//go:embed manifests/nvidia-device-plugin.yaml
	nvidiaDevicePluginManifest []byte
	//go:embed manifests/mpi-operator.yaml
	mpiOperatorManifest []byte
)

func TestMain(m *testing.M) {
	bertTrainingImage = flag.String("bertTrainingImage", "", "Docker image used for BERT training workload")
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
	)

	testenv.Finish(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
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
