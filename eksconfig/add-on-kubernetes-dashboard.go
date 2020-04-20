package eksconfig

import "time"

// IsEnabledAddOnKubernetesDashboard returns true if "AddOnKubernetesDashboard" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnKubernetesDashboard() bool {
	if cfg.AddOnKubernetesDashboard == nil {
		return false
	}
	if cfg.AddOnKubernetesDashboard.Enable {
		return true
	}
	cfg.AddOnKubernetesDashboard = nil
	return false
}

// AddOnKubernetesDashboard defines parameters for EKS cluster
// add-on Kubernetes Dashboard.
type AddOnKubernetesDashboard struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`

	// Namespace is the namespace to create "AppMesh" controller/injector.
	Namespace string `json:"namespace"`

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

	// NLBARN is the ARN of the NLB created from the service.
	NLBARN string `json:"nlb-arn" read-only:"true"`
	// NLBName is the name of the NLB created from the service.
	NLBName string `json:"nlb-name" read-only:"true"`
	// URL is the host name for WordPress service.
	URL string `json:"url" read-only:"true"`
}
