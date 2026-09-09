//go:build e2e

// Package nvidia_driver_liveness is an e2e test that asserts the NVIDIA
// driver is alive and reports the versions/models expected of the AMI
// under test. Covers three capability #1 sub-tests: nvidia-smi runs,
// driver version matches the -expectedDriverVersion flag, GPU
// model matches the instance type. Runs as one pod that performs all
// three assertions in a single bash script; the pod exits non-zero on
// any failure with a specific code (10=smi, 11=version, 12=model).
//
// Excludes tests already covered upstream in nvidia.test / test_sysinfo.sh:
// test_nvidia_gpu_count, test_nvidia_gpu_unused, test_nvidia_gpu_throttled.
package nvidia_driver_liveness

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

//go:embed manifests/pod-driver-liveness-check.yaml
var podDriverLivenessCheckManifest []byte

// Config is the flag surface for this test binary. Field tags become
// `-<flag>` on the command line (parsed via common.ParseFlags).
type Config struct {
	NvidiaTestImage       string `flag:"nvidiaTestImage" desc:"URL of the nvidia test image (built from upstream test/images/nvidia/Dockerfile) -- required"`
	ExpectedDriverVersion string `flag:"expectedDriverVersion" desc:"expected NVIDIA driver version (e.g. 570.86.15); when empty the 1.5 assertion is skipped"`
	NodeType              string `flag:"nodeType" desc:"EC2 instance type under qualification (e.g. p5.48xlarge) -- required for 1.8 GPU-model regex"`
	InstallDevicePlugin   bool   `flag:"installDevicePlugin" desc:"install the NVIDIA k8s device plugin before the test and delete it after (default true)"`
	EfaEnabled            bool   `flag:"efaEnabled" desc:"also install the aws-efa-k8s device plugin (default false)"`
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

	// Device-plugin manifests are applied in Setup and deleted (reversed)
	// in Finish so the cluster ends the run in the state it started in.
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
type PodManifestTplVars struct {
	NvidiaTestImage       string
	ExpectedDriverVersion string
	NodeType              string
}

func tplVars() PodManifestTplVars {
	return PodManifestTplVars{
		NvidiaTestImage:       testConfig.NvidiaTestImage,
		ExpectedDriverVersion: testConfig.ExpectedDriverVersion,
		NodeType:              testConfig.NodeType,
	}
}
