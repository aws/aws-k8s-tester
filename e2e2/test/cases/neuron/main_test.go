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
	neuronTestImage     *string
	installDevicePlugin *bool
)

var (
	//go:embed manifests/k8s-neuron-device-plugin-rbac.yml
	neuronDevicePlugiRbacManifest []byte
	//go:embed manifests/k8s-neuron-device-plugin.yml
	neuronDevicePluginManifest []byte
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

func TestMain(m *testing.M) {
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

	var manifests [][]byte
	setUpFunctions := []env.Func{
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			err := fwext.ApplyManifests(config.Client().RESTConfig(), manifests...)
			if err != nil {
				return ctx, err
			}
			return ctx, nil
		},
	}

	if *installDevicePlugin {
		manifests = append(manifests, neuronDevicePluginManifest, neuronDevicePlugiRbacManifest)
		setUpFunctions = append(setUpFunctions, deployNeuronDevicePlugin)
	}

	testenv.Setup(setUpFunctions...)

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
