package eksconfig

import (
	"errors"

	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnKubernetesDashboard defines parameters for EKS cluster
// add-on Kubernetes Dashboard.
// ref. https://docs.aws.amazon.com/eks/latest/userguide/dashboard-tutorial.html
type AddOnKubernetesDashboard struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// AuthenticationToken is the authentication token for eks-admin service account.
	AuthenticationToken string `json:"authentication-token,omitempty" read-only:"true"`
	// URL is the host name for Kubernetes Dashboard service.
	URL string `json:"url" read-only:"true"`

	// KubectlProxyPID is the PID for kubectl proxy.
	KubectlProxyPID int `json:"kubectl-proxy-pid" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnKubernetesDashboard is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnKubernetesDashboard = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_KUBERNETES_DASHBOARD_"

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

func getDefaultAddOnKubernetesDashboard() *AddOnKubernetesDashboard {
	return &AddOnKubernetesDashboard{
		Enable: false,
		URL:    defaultKubernetesDashboardURL,
	}
}

// ref. https://docs.aws.amazon.com/eks/latest/userguide/dashboard-tutorial.html
const defaultKubernetesDashboardURL = "http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/#/login"

func (cfg *Config) validateAddOnKubernetesDashboard() error {
	if !cfg.IsEnabledAddOnKubernetesDashboard() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnKubernetesDashboard.Enable true but no node group is enabled")
	}
	if !cfg.IsEnabledAddOnMetricsServer() {
		return errors.New("AddOnKubernetesDashboard.Enable true but AddOnMetricsServer.Enable false")
	}
	if cfg.AddOnKubernetesDashboard.URL == "" {
		cfg.AddOnKubernetesDashboard.URL = defaultKubernetesDashboardURL
	}
	return nil
}
