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
	testenv         env.Environment
	neuronTestImage *string
)

var (
	//go:embed manifests/k8s-neuron-device-plugin-rbac.yml
	neuronDevicePlugiRbacManifest []byte
	//go:embed manifests/k8s-neuron-device-plugin.yml
	neuronDevicePluginManifest []byte
)

func TestMain(m *testing.M) {
	neuronTestImage = flag.String("neuronTestImage", "", "image for neuron single node test")
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to initialize test environment: %v", err)
	}
	testenv = env.NewWithConfig(cfg)

	manifests := [][]byte{
		neuronDevicePluginManifest,
		neuronDevicePlugiRbacManifest,
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
			ds := appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Name: "neuron-device-plugin-daemonset", Namespace: "kube-system"},
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
