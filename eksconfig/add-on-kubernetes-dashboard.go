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

func (cfg *Config) getAddOnKubernetesDashboardAuthenticationToken() string {
	if cfg.AddOnKubernetesDashboard == nil {
		return ""
	}
	return cfg.AddOnKubernetesDashboard.AuthenticationToken
}

func (cfg *Config) getAddOnKubernetesDashboardURL() string {
	if cfg.AddOnKubernetesDashboard == nil {
		return ""
	}
	return cfg.AddOnKubernetesDashboard.URL
}

// AddOnKubernetesDashboard defines parameters for EKS cluster
// add-on Kubernetes Dashboard.
// ref. https://docs.aws.amazon.com/eks/latest/userguide/dashboard-tutorial.html
type AddOnKubernetesDashboard struct {
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

	// AuthenticationToken is the authentication token for eks-admin service account.
	AuthenticationToken string `json:"authentication-token,omitempty" read-only:"true"`
	// URL is the host name for Kubernetes Dashboard service.
	// TODO: convert this to read-only, and auto-populate this with NLB
	URL string `json:"url"`

	// KubectlProxyPID is the PID for kubectl proxy.
	KubectlProxyPID int `json:"kubectl-proxy-pid" read-only:"true"`
}
