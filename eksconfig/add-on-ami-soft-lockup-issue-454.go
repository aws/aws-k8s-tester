package eksconfig

import (
	"errors"

	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnAmiSoftLockupIssue454 defines parameters for EKS cluster
// add-on NLB hello-world service.
type AddOnAmiSoftLockupIssue454 struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`

	// DeploymentReplicas is the number of replicas to deploy using "Deployment" object.
	DeploymentReplicas int32 `json:"deployment-replicas"`
	// DeploymentNodeSelector is configured to overwrite existing node selector
	// for deployment. If left empty, tester sets default selector.
	DeploymentNodeSelector map[string]string `json:"deployment-node-selector"`
}

// EnvironmentVariablePrefixAddOnAmiSoftLockupIssue454 is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnAmiSoftLockupIssue454 = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_AMI_SOFT_LOCKUP_ISSUE_454_"

// IsEnabledAddOnAmiSoftLockupIssue454 returns true if "AddOnAmiSoftLockupIssue454" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnAmiSoftLockupIssue454() bool {
	if cfg.AddOnAmiSoftLockupIssue454 == nil {
		return false
	}
	if cfg.AddOnAmiSoftLockupIssue454.Enable {
		return true
	}
	cfg.AddOnAmiSoftLockupIssue454 = nil
	return false
}

func getDefaultAddOnAmiSoftLockupIssue454() *AddOnAmiSoftLockupIssue454 {
	return &AddOnAmiSoftLockupIssue454{
		Enable:                 false,
		DeploymentReplicas:     8,
		DeploymentNodeSelector: make(map[string]string),
	}
}

func (cfg *Config) validateAddOnAmiSoftLockupIssue454() error {
	if !cfg.IsEnabledAddOnAmiSoftLockupIssue454() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnAmiSoftLockupIssue454.Enable true but no node group is enabled")
	}
	if cfg.AddOnAmiSoftLockupIssue454.Namespace == "" {
		cfg.AddOnAmiSoftLockupIssue454.Namespace = cfg.Name + "-ami-soft-lockup-issue-454"
	}
	return nil
}
