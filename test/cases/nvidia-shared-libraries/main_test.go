//go:build e2e

package nvidia_shared_libraries

import (
	_ "embed"
	"log"
	"os"
	"testing"

	"github.com/aws/aws-k8s-tester/test/common"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

//go:embed manifests/daemonset-shared-libraries-check.yaml
var sharedLibrariesCheckDS []byte

const defaultRequiredLibs = "libcuda.so.1 libnvidia-ml.so.1 libnvidia-ptxjitcompiler.so.1 libnvidia-cfg.so.1"

const defaultAllowedLibDirsRegex = `^/(usr/)?lib(64)?/[^/]+$`

// Config is the flag surface for this test binary.
type Config struct {
	RequiredLibs        string `flag:"requiredLibs" desc:"space-separated list of SONAMEs that MUST be present in the container's ld.so cache and MUST resolve under -allowedLibDirsRegex"`
	AllowedLibDirsRegex string `flag:"allowedLibDirsRegex" desc:"extended regex applied to each required library's resolved path; a required library that resolves outside this pattern fails the test"`
	InstallDevicePlugin bool   `flag:"installDevicePlugin" desc:"install the NVIDIA k8s device plugin before the test and delete it after (default true)"`
}

var (
	testenv    env.Environment
	testConfig Config
)

func TestMain(m *testing.M) {
	testConfig = Config{
		RequiredLibs:        defaultRequiredLibs,
		AllowedLibDirsRegex: defaultAllowedLibDirsRegex,
	}

	if _, err := common.ParseFlags(&testConfig); err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to initialize test environment: %v", err)
	}
	if testConfig.RequiredLibs == "" {
		log.Fatalf("-requiredLibs must not be empty")
	}
	if testConfig.AllowedLibDirsRegex == "" {
		log.Fatalf("-allowedLibDirsRegex must not be empty")
	}
	testenv = env.NewWithConfig(cfg)

	os.Exit(testenv.Run(m))
}

// DaemonSetManifestTplVars is the template-variable set for the
// nvidia-shared-libraries-check DaemonSet manifest.
type DaemonSetManifestTplVars struct {
	RequiredLibs        string
	AllowedLibDirsRegex string
}
