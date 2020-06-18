package eksconfig

import (
	"errors"

	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnNLBGuestbook defines parameters for EKS cluster
// add-on NLB guestbook service.
// ref. https://github.com/kubernetes/examples/tree/master/guestbook-go
// ref. https://docs.aws.amazon.com/eks/latest/userguide/eks-guestbook.html
type AddOnNLBGuestbook struct {
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
	// for NLB hello world deployment. If left empty, tester sets default selector.
	DeploymentNodeSelector map[string]string `json:"deployment-node-selector"`

	// NLBARN is the ARN of the NLB created from the service.
	NLBARN string `json:"nlb-arn" read-only:"true"`
	// NLBName is the name of the NLB created from the service.
	NLBName string `json:"nlb-name" read-only:"true"`
	// URL is the host name for guestbook service.
	URL string `json:"url" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnNLBGuestbook is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnNLBGuestbook = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_NLB_GUESTBOOK_"

// IsEnabledAddOnNLBGuestbook returns true if "AddOnNLBGuestbook" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnNLBGuestbook() bool {
	if cfg.AddOnNLBGuestbook == nil {
		return false
	}
	if cfg.AddOnNLBGuestbook.Enable {
		return true
	}
	cfg.AddOnNLBGuestbook = nil
	return false
}

func getDefaultAddOnNLBGuestbook() *AddOnNLBGuestbook {
	return &AddOnNLBGuestbook{
		Enable:                 false,
		DeploymentReplicas:     3,
		DeploymentNodeSelector: make(map[string]string),
	}
}

func (cfg *Config) validateAddOnNLBGuestbook() error {
	if !cfg.IsEnabledAddOnNLBGuestbook() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnNLBGuestbook.Enable true but no node group is enabled")
	}
	if cfg.AddOnNLBGuestbook.Namespace == "" {
		cfg.AddOnNLBGuestbook.Namespace = cfg.Name + "-nlb-guestbook"
	}
	return nil
}
