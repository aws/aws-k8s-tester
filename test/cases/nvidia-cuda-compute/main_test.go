//go:build e2e

// Package nvidia_cuda_compute runs the CUDA sample binaries baked into
// the nvidia test image and asserts each exits 0.
// (excludes deviceQuery and vectorAdd which
// are already asserted by upstream's nvidia.test).
package nvidia_cuda_compute

import (
	"context"
	_ "embed"
	"log"
	"os"
	"os/signal"
	"slices"
	"testing"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/aws-k8s-tester/test/common"
	"github.com/aws/aws-k8s-tester/test/manifests"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

//go:embed manifests/pod-cuda-compute-check.yaml
var podCUDAComputeCheckManifest []byte

type Config struct {
	NvidiaTestImage     string `flag:"nvidiaTestImage" desc:"URL of the nvidia test image with the extended CUDA samples baked in -- required"`
	InstallDevicePlugin bool   `flag:"installDevicePlugin" desc:"install the NVIDIA k8s device plugin before the test and delete it after (default true)"`
	EfaEnabled          bool   `flag:"efaEnabled" desc:"also install the aws-efa-k8s device plugin (default false)"`
}

var (
	testenv    env.Environment
	testConfig Config
)

func TestMain(m *testing.M) {
	testConfig = Config{InstallDevicePlugin: true}

	if _, err := common.ParseFlags(&testConfig); err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to initialize test environment: %v", err)
	}
	if testConfig.NvidiaTestImage == "" {
		log.Fatalf("-nvidiaTestImage is required")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	testenv = env.NewWithConfig(cfg).WithContext(ctx)

	var pluginManifests [][]byte
	var setUp []env.Func
	if testConfig.InstallDevicePlugin {
		pluginManifests = append(pluginManifests, manifests.NvidiaDevicePluginManifest)
		setUp = append(setUp,
			func(ctx context.Context, config *envconf.Config) (context.Context, error) {
				if err := fwext.ApplyManifests(config.Client().RESTConfig(), manifests.NvidiaDevicePluginManifest); err != nil {
					return ctx, err
				}
				return ctx, nil
			},
			common.DeployDaemonSet("nvidia-device-plugin-daemonset", "kube-system"),
		)
	}
	if testConfig.EfaEnabled {
		pluginManifests = append(pluginManifests, manifests.EfaDevicePluginManifest)
		setUp = append(setUp,
			func(ctx context.Context, config *envconf.Config) (context.Context, error) {
				if err := fwext.ApplyManifests(config.Client().RESTConfig(), manifests.EfaDevicePluginManifest); err != nil {
					return ctx, err
				}
				return ctx, nil
			},
			common.DeployDaemonSet("aws-efa-k8s-device-plugin-daemonset", "kube-system"),
		)
	}
	if len(setUp) > 0 {
		testenv.Setup(setUp...)
	}
	if len(pluginManifests) > 0 {
		testenv.Finish(func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			slices.Reverse(pluginManifests)
			if err := fwext.DeleteManifests(config.Client().RESTConfig(), pluginManifests...); err != nil {
				return ctx, err
			}
			return ctx, nil
		})
	}

	os.Exit(testenv.Run(m))
}

type PodManifestTplVars struct {
	NvidiaTestImage string
}
