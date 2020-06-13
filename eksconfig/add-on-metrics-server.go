package eksconfig

import (
	"errors"

	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnMetricsServer defines parameters for EKS cluster
// add-on metrics server.
// ref. https://github.com/kubernetes-sigs/metrics-server/releases
type AddOnMetricsServer struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnMetricsServer is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnMetricsServer = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_METRICS_SERVER_"

// IsEnabledAddOnMetricsServer returns true if "AddOnMetricsServer" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnMetricsServer() bool {
	if cfg.AddOnMetricsServer == nil {
		return false
	}
	if cfg.AddOnMetricsServer.Enable {
		return true
	}
	cfg.AddOnMetricsServer = nil
	return false
}

func getDefaultAddOnMetricsServer() *AddOnMetricsServer {
	return &AddOnMetricsServer{
		Enable: false,
	}
}

func (cfg *Config) validateAddOnMetricsServer() error {
	if !cfg.IsEnabledAddOnMetricsServer() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnMetricsServer.Enable true but no node group is enabled")
	}
	return nil
}
