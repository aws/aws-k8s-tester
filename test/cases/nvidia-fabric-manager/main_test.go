//go:build e2e

// Package nvidia_fabric_manager asserts fabric-manager and NVLink health.
// Covers capability #11 test fabric-manager version matches driver
// version via a host-scoped DaemonSet, plus the in-container tests
// fabric state, NVLinks Active + expected count,
// and no NVLinks on non-NVLink hardware via one pod that
// dispatches by instance-family class.

package nvidia_fabric_manager

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

//go:embed manifests/daemonset-fabric-manager-version-check.yaml
var dsFabricManagerVersionCheckManifest []byte

//go:embed manifests/pod-fabric-nvlink-check.yaml
var podFabricNVLinkCheckManifest []byte

type Config struct {
	NvidiaTestImage     string `flag:"nvidiaTestImage" desc:"URL of the nvidia test image -- required for the in-container 5.3/5.4+5.5/6.8 checks"`
	NodeType            string `flag:"nodeType" desc:"EC2 instance type under qualification -- required for 5.3/5.4+5.5/6.8 dispatch"`
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
	if testConfig.NodeType == "" {
		log.Fatalf("-nodeType is required")
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

// PodManifestTplVars is the template-variable set for the in-container
// pod manifest. GpuCount is threaded into `nvidia.com/gpu:` so the pod
// pins the entire node's GPUs -- 5.3/5.4/5.5 assert node-wide fabric
// and NVLink counts, and the device plugin injects only the GPUs
// asked for, so a single-GPU request would make those counts
// unsatisfiable. The node type is not surfaced to the pod -- inside
// the container, classification and expected counts are derived from
// kernel state and GPU SKU respectively.
type PodManifestTplVars struct {
	NvidiaTestImage string
	GpuCount        int
}
