package eksconfig

import (
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnFluentd defines parameters for EKS cluster
// add-on Fluentd.
// ref. https://www.eksworkshop.com/intermediate/230_logging/deploy/
type AddOnFluentd struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnFluentd is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnFluentd = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_FLUENTD_"

// IsEnabledAddOnFluentd returns true if "AddOnFluentd" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnFluentd() bool {
	if cfg.AddOnFluentd == nil {
		return false
	}
	if cfg.AddOnFluentd.Enable {
		return true
	}
	cfg.AddOnFluentd = nil
	return false
}

func getDefaultAddOnFluentd() *AddOnFluentd {
	return &AddOnFluentd{
		Enable: false,
	}
}

func (cfg *Config) validateAddOnFluentd() error {
	if !cfg.IsEnabledAddOnFluentd() {
		return nil
	}

	return nil
}
