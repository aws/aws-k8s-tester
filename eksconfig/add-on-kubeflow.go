package eksconfig

import "time"

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

// AddOnKubeflow defines parameters for EKS cluster
// add-on Kubeflow.
// ref. https://www.kubeflow.org/docs/aws/deploy/install-kubeflow/
type AddOnKubeflow struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`

	// Namespace is the namespace to create "AppMesh" controller/injector.
	Namespace string `json:"namespace"`

	// KfctlPath is the path to download the "kfctl".
	KfctlPath string `json:"kfctl-path,omitempty"`
	// KfctlDownloadURL is the download URL to download "kfctl" binary from.
	// https://github.com/kubeflow/kfctl/releases
	KfctlDownloadURL string `json:"kfctl-download-url,omitempty"`

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

	// BaseDir is the base directory where you want to store one or more
	// Kubeflow deployments.
	BaseDir string `json:"base-dir"`
	KfDir   string `json:"kf-dir" read-only:"true"`
	// KfctlConfigPath is the path to write "kfctl" configuration.
	// The existing configuration file is overwritten.
	KfctlConfigPath string `json:"kfctl-config-path" read-only:"true"`
}
