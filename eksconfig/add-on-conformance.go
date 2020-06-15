package eksconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnConformance defines parameters for EKS cluster
// add-on Conformance.
// ref. https://github.com/cncf/k8s-conformance/blob/master/instructions.md
// ref. https://github.com/vmware-tanzu/sonobuoy
type AddOnConformance struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`

	// SonobuoyPath is the path to download the "sonobuoy".
	SonobuoyPath string `json:"sonobuoy-path,omitempty"`
	// SonobuoyDownloadURL is the download URL to download "sonobuoy" binary from.
	// ref. https://github.com/vmware-tanzu/sonobuoy/releases
	SonobuoyDownloadURL string `json:"sonobuoy-download-url,omitempty"`

	SonobuoyDeleteTimeout       time.Duration `json:"sonobuoy-delete-timeout"`
	SonobuoyDeleteTimeoutString string        `json:"sonobuoy-delete-timeout-string" read-only:"true"`
	SonobuoyRunTimeout          time.Duration `json:"sonobuoy-run-timeout"`
	SonobuoyRunTimeoutString    string        `json:"sonobuoy-run-timeout-string" read-only:"true"`

	// SonobuoyRunMode is the mode to run sonobuoy in.
	// Valid modes are 'non-disruptive-conformance', 'quick', 'certified-conformance'.
	// The default is 'certified-conformance'.
	// ref. https://github.com/vmware-tanzu/sonobuoy
	SonobuoyRunMode                 string `json:"sonobuoy-run-mode"`
	SonobuoyRunKubeConformanceImage string `json:"sonobuoy-run-kube-conformance-image"`

	SonobuoyResultTarGzPath    string `json:"sonobuoy-result-tar-gz-path" read-only:"true"`
	SonobuoyResultDir          string `json:"sonobuoy-result-dir" read-only:"true"`
	SonobuoyResultE2eLogPath   string `json:"sonobuoy-result-e2e-log-path" read-only:"true"`
	SonobuoyResultJunitXMLPath string `json:"sonobuoy-result-junit-xml-path" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnConformance is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnConformance = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_CONFORMANCE_"

// IsEnabledAddOnConformance returns true if "AddOnConformance" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnConformance() bool {
	if cfg.AddOnConformance == nil {
		return false
	}
	if cfg.AddOnConformance.Enable {
		return true
	}
	cfg.AddOnConformance = nil
	return false
}

func getDefaultAddOnConformance() *AddOnConformance {
	addOn := &AddOnConformance{
		Enable:              false,
		SonobuoyPath:        "/tmp/sonobuoy-v0.18.3",
		SonobuoyDownloadURL: "https://github.com/vmware-tanzu/sonobuoy/releases/download/v0.18.3/sonobuoy_0.18.3_linux_amd64.tar.gz",
	}
	if runtime.GOOS == "darwin" {
		addOn.SonobuoyDownloadURL = strings.Replace(addOn.SonobuoyDownloadURL, "linux", "darwin", -1)
	}
	return addOn
}

/*
TODO: fix this... conformance with managed node group does not work

Plugin: e2e
Status: failed
Total: 4897
Passed: 267
Failed: 9
Skipped: 4621

Failed tests:
[sig-network] Services should be able to change the type from ClusterIP to ExternalName [Conformance]
[sig-network] Services should be able to create a functioning NodePort service [Conformance]
[sig-api-machinery] Aggregator Should be able to support the 1.10 Sample API Server using the current Aggregator [Conformance]
[sig-network] Networking Granular Checks: Pods should function for intra-pod communication: udp [LinuxOnly] [NodeConformance] [Conformance]
[sig-cli] Kubectl client Kubectl run --rm job should create a job from an image, then delete the job  [Conformance]
[sig-network] Services should be able to change the type from ExternalName to NodePort [Conformance]
[sig-network] DNS should provide DNS for ExternalName services [Conformance]
[sig-network] Networking Granular Checks: Pods should function for node-pod communication: udp [LinuxOnly] [NodeConformance] [Conformance]
[sig-network] DNS should provide DNS for pods for Hostname [LinuxOnly] [Conformance]

Plugin: systemd-logs
Status: passed
Total: 14
Passed: 14
Failed: 0
Skipped: 0
*/

func (cfg *Config) validateAddOnConformance() error {
	if !cfg.IsEnabledAddOnConformance() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnConformance.Enable true but no node group is enabled")
	}

	// TODO: fix this...
	if cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnConformance.Enable true with AddOnManagedNodeGroups.Enable true")
	}

	if cfg.AddOnConformance.Namespace == "" {
		cfg.AddOnConformance.Namespace = cfg.Name + "-conformance"
	}

	if cfg.AddOnConformance.SonobuoyDeleteTimeout == time.Duration(0) {
		cfg.AddOnConformance.SonobuoyDeleteTimeout = 5 * time.Minute
	}
	cfg.AddOnConformance.SonobuoyDeleteTimeoutString = cfg.AddOnConformance.SonobuoyDeleteTimeout.String()

	// "certified-conformance" takes >=3-hour
	if cfg.AddOnConformance.SonobuoyRunTimeout == time.Duration(0) {
		cfg.AddOnConformance.SonobuoyRunTimeout = 5 * time.Hour
	}
	cfg.AddOnConformance.SonobuoyRunTimeoutString = cfg.AddOnConformance.SonobuoyRunTimeout.String()

	if cfg.AddOnConformance.SonobuoyRunMode == "" {
		cfg.AddOnConformance.SonobuoyRunMode = "certified-conformance"
	}
	switch cfg.AddOnConformance.SonobuoyRunMode {
	case "non-disruptive-conformance":
	case "quick":
	case "certified-conformance":
	default:
		return fmt.Errorf("unknown AddOnConformance.SonobuoyRunMode %q", cfg.AddOnConformance.SonobuoyRunMode)
	}

	if cfg.AddOnConformance.SonobuoyRunKubeConformanceImage == "" {
		cfg.AddOnConformance.SonobuoyRunKubeConformanceImage = fmt.Sprintf("gcr.io/google-containers/conformance:v%s.0", cfg.Parameters.Version)
	}

	if cfg.AddOnConformance.SonobuoyResultTarGzPath == "" {
		cfg.AddOnConformance.SonobuoyResultTarGzPath = filepath.Join(
			filepath.Dir(cfg.ConfigPath),
			fmt.Sprintf("%s-sonobuoy-result.tar.gz", cfg.Name),
		)
		os.RemoveAll(cfg.AddOnConformance.SonobuoyResultTarGzPath)
	}
	if !strings.HasSuffix(cfg.AddOnConformance.SonobuoyResultTarGzPath, ".tar.gz") {
		return fmt.Errorf("AddOnConformance.SonobuoyResultTarGzPath[%q] must have '.tar.gz' extension", cfg.AddOnConformance.SonobuoyResultTarGzPath)
	}

	cfg.AddOnConformance.SonobuoyResultDir = filepath.Join(
		filepath.Dir(cfg.ConfigPath),
		fmt.Sprintf("%s-sonobuoy-results", cfg.Name),
	)

	if cfg.AddOnConformance.SonobuoyResultE2eLogPath == "" {
		cfg.AddOnConformance.SonobuoyResultE2eLogPath = filepath.Join(
			filepath.Dir(cfg.ConfigPath),
			fmt.Sprintf("%s-sonobuoy-result.e2e.log", cfg.Name),
		)
		os.RemoveAll(cfg.AddOnConformance.SonobuoyResultE2eLogPath)
	}
	if !strings.HasSuffix(cfg.AddOnConformance.SonobuoyResultE2eLogPath, ".log") {
		return fmt.Errorf("AddOnConformance.SonobuoyResultE2eLogPath[%q] must have '.log' extension", cfg.AddOnConformance.SonobuoyResultTarGzPath)
	}

	if cfg.AddOnConformance.SonobuoyResultJunitXMLPath == "" {
		cfg.AddOnConformance.SonobuoyResultJunitXMLPath = filepath.Join(
			filepath.Dir(cfg.ConfigPath),
			fmt.Sprintf("%s-sonobuoy-result.junit.xml", cfg.Name),
		)
		os.RemoveAll(cfg.AddOnConformance.SonobuoyResultJunitXMLPath)
	}
	if !strings.HasSuffix(cfg.AddOnConformance.SonobuoyResultJunitXMLPath, ".xml") {
		return fmt.Errorf("AddOnConformance.SonobuoyResultJunitXMLPath[%q] must have '.xml' extension", cfg.AddOnConformance.SonobuoyResultTarGzPath)
	}

	return nil
}
