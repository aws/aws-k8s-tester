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
	//go:embed manifests/nvidia-device-plugin.yaml
	nvidiaDevicePluginManifest []byte
	//go:embed manifests/mpi-operator.yaml
	mpiOperatorManifest []byte
)

func TestMain(m *testing.M) {
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to initialize test environment: %v", err)
	}
	testenv = env.NewWithConfig(cfg)

	manifests := [][]byte{
		nvidiaDevicePluginManifest,
		mpiOperatorManifest,
	}

	testenv.Setup(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("Applying NVIDIA device plugin and MPI operator manifests.")
			err := fwext.ApplyManifests(config.Client().RESTConfig(), manifests...)
			if err != nil {
				return ctx, fmt.Errorf("failed to apply manifests: %w", err)
			}
			log.Println("Successfully applied NVIDIA device plugin and MPI operator manifests.")
			return ctx, nil
		},

		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("Waiting for MPI Operator deployment to be available.")
			deployment := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "mpi-operator", Namespace: "mpi-operator"},
			}
			err := wait.For(conditions.New(config.Client().Resources()).DeploymentConditionMatch(&deployment, appsv1.DeploymentAvailable, v1.ConditionTrue),
				wait.WithTimeout(time.Minute*5))
			if err != nil {
				return ctx, fmt.Errorf("MPI Operator deployment is not available: %w", err)
			}
			log.Println("MPI Operator deployment is available.")
			return ctx, nil
		},

		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("Waiting for NVIDIA Device Plugin daemonset to be ready.")
			daemonset := appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Name: "nvidia-device-plugin-daemonset", Namespace: "kube-system"},
			}
			err := wait.For(fwext.NewConditionExtension(config.Client().Resources()).DaemonSetReady(&daemonset),
				wait.WithTimeout(time.Minute*5))
			if err != nil {
				return ctx, fmt.Errorf("NVIDIA Device Plugin daemonset is not ready: %w", err)
			}
			log.Println("NVIDIA Device Plugin daemonset is ready.")
			return ctx, nil
		},
	)

	testenv.Finish(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			log.Println("Deleting NVIDIA device plugin and MPI operator manifests.")
			slices.Reverse(manifests)
			err := fwext.DeleteManifests(config.Client().RESTConfig(), manifests...)
			if err != nil {
				return ctx, fmt.Errorf("failed to delete manifests: %w", err)
			}
			log.Println("Successfully deleted NVIDIA device plugin and MPI operator manifests.")
			return ctx, nil
		},
	)

	log.Println("Starting tests...")
	exitCode := testenv.Run(m)
	log.Printf("Tests finished with exit code %d", exitCode)
	os.Exit(exitCode)
}
