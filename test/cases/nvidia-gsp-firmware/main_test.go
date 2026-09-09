//go:build e2e

// Package nvidia_gsp_firmware asserts the GPU System Processor firmware
// has loaded correctly and its version matches the NVIDIA driver
// version. Covers capability #1 test : non-empty per
// GPU, matches driver version, consistent across GPUs. Skips on
// pre-Ampere hardware.
package nvidia_gsp_firmware

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

//go:embed manifests/pod-gsp-firmware-check.yaml
var podGSPFirmwareCheckManifest []byte

type Config struct {
	NvidiaTestImage     string `flag:"nvidiaTestImage" desc:"URL of the nvidia test image -- required"`
	NodeType            string `flag:"nodeType" desc:"EC2 instance type under qualification; drives GPU count so the cross-GPU consistency check (exit 43) sees every GPU on the node"`
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

// PodManifestTplVars is the template-variable set for the pod manifest.
// GpuCount is threaded into `nvidia.com/gpu:` so the cross-GPU
// consistency check (exit 43) sees every GPU on the node -- the device
// plugin injects only the GPUs requested, so a single-GPU request
// would leave that assertion trivially satisfied.
type PodManifestTplVars struct {
	NvidiaTestImage string
	GpuCount        int
}
