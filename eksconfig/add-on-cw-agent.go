package eksconfig

import (
	"errors"

	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnCWAgent defines parameters for EKS cluster
// add-on CloudWatch agent.
// Publishes worker nodes logs to:
//  - /aws/containerinsights/[CLUSTER-NAME]/performance
type AddOnCWAgent struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`
}

// AWS_K8S_TESTER_EKS_ADD_ON_CW_AGENT_PREFIX is the environment variable prefix used for "eksconfig".
const AWS_K8S_TESTER_EKS_ADD_ON_CW_AGENT_PREFIX = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_CW_AGENT_"

// IsEnabledAddOnCWAgent returns true if "AddOnCWAgent" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnCWAgent() bool {
	if cfg.AddOnCWAgent == nil {
		return false
	}
	if cfg.AddOnCWAgent.Enable {
		return true
	}
	cfg.AddOnCWAgent = nil
	return false
}

func getDefaultAddOnCWAgent() *AddOnCWAgent {
	return &AddOnCWAgent{
		Enable: false,
	}
}

func (cfg *Config) validateAddOnCWAgent() error {
	if !cfg.IsEnabledAddOnCWAgent() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnCWAgent.Enable true but no node group is enabled")
	}
	if cfg.AddOnCWAgent.Namespace == "" {
		cfg.AddOnCWAgent.Namespace = cfg.Name + "-cw-agent"
	}
	return nil
}
