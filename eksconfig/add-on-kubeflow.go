package eksconfig

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// AddOnKubeflow defines parameters for EKS cluster
// add-on Kubeflow.
// ref. https://www.kubeflow.org/docs/aws/deploy/install-kubeflow/
type AddOnKubeflow struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created bool `json:"created" read-only:"true"`
	// CreateTook is the duration that took to create the resource.
	CreateTook time.Duration `json:"create-took,omitempty" read-only:"true"`
	// CreateTookString is the duration that took to create the resource.
	CreateTookString string `json:"create-took-string,omitempty" read-only:"true"`
	// DeleteTook is the duration that took to create the resource.
	DeleteTook time.Duration `json:"delete-took,omitempty" read-only:"true"`
	// DeleteTookString is the duration that took to create the resource.
	DeleteTookString string `json:"delete-took-string,omitempty" read-only:"true"`

	// KfctlPath is the path to download the "kfctl".
	KfctlPath string `json:"kfctl-path,omitempty"`
	// KfctlDownloadURL is the download URL to download "kfctl" binary from.
	// ref. https://github.com/kubeflow/kfctl/releases
	KfctlDownloadURL string `json:"kfctl-download-url,omitempty"`

	// BaseDir is the base directory where you want to store one or more
	// Kubeflow deployments.
	BaseDir string `json:"base-dir"`
	KfDir   string `json:"kf-dir" read-only:"true"`
	// KfctlConfigPath is the path to write "kfctl" configuration.
	// The existing configuration file is overwritten.
	KfctlConfigPath string `json:"kfctl-config-path" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnKubeflow is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnKubeflow = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_KUBEFLOW_"

// IsEnabledAddOnKubeflow returns true if "AddOnKubeflow" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnKubeflow() bool {
	if cfg.AddOnKubeflow == nil {
		return false
	}
	if cfg.AddOnKubeflow.Enable {
		return true
	}
	cfg.AddOnKubeflow = nil
	return false
}

func getDefaultAddOnKubeflow() *AddOnKubeflow {
	addOn := &AddOnKubeflow{
		Enable:           false,
		KfctlPath:        "/tmp/kfctl-test-v1.0.2",
		KfctlDownloadURL: "https://github.com/kubeflow/kfctl/releases/download/v1.0.2/kfctl_v1.0.2-0-ga476281_linux.tar.gz",
	}
	if runtime.GOOS == "darwin" {
		addOn.KfctlDownloadURL = strings.Replace(addOn.KfctlDownloadURL, "linux", "darwin", -1)
	}
	return addOn
}

func (cfg *Config) validateAddOnKubeflow() error {
	if !cfg.IsEnabledAddOnKubeflow() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnKubeflow.Enable true but no node group is enabled")
	}
	if cfg.AddOnKubeflow.BaseDir == "" {
		cfg.AddOnKubeflow.BaseDir = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-kubeflow")
	}
	cfg.AddOnKubeflow.KfDir = filepath.Join(cfg.AddOnKubeflow.BaseDir, cfg.Name)
	cfg.AddOnKubeflow.KfctlConfigPath = filepath.Join(cfg.AddOnKubeflow.KfDir, "kfctl_aws.yaml")
	return nil
}
