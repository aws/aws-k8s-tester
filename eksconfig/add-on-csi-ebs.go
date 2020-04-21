package eksconfig

import "time"

// IsEnabledAddOnCSIEBS returns true if "AddOnCSIEBS" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnCSIEBS() bool {
	if cfg.AddOnCSIEBS == nil {
		return false
	}
	if cfg.AddOnCSIEBS.Enable {
		return true
	}
	cfg.AddOnCSIEBS = nil
	return false
}

// AddOnCSIEBS defines parameters for EKS cluster
// add-on AWS EBS CSI Driver.
// ref. https://github.com/kubernetes-sigs/aws-ebs-csi-driver#deploy-driver
type AddOnCSIEBS struct {
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

	// ChartRepoURL is the chart repo URL.
	// e.g. https://github.com/kubernetes-sigs/aws-ebs-csi-driver/releases/download/v0.5.0/helm-chart.tgz
	ChartRepoURL string `json:"chart-repo-url"`
}
