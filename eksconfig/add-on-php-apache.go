package eksconfig

import (
	"errors"

	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnPHPApache defines parameters for EKS cluster
// add-on PHP Apache.
type AddOnPHPApache struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`

	// RepositoryAccountID is the account ID for tester ECR image.
	// e.g. "php-apache" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/php-apache"
	RepositoryAccountID string `json:"repository-account-id,omitempty"`
	// RepositoryRegion is the ECR repository region to pull from.
	RepositoryRegion string `json:"repository-region,omitempty"`
	// RepositoryName is the repositoryName for tester ECR image.
	// e.g. "php-apache" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/php-apache"
	RepositoryName string `json:"repository-name,omitempty"`
	// RepositoryImageTag is the image tag for tester ECR image.
	// e.g. "latest" for image URI "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/php-apache:latest"
	RepositoryImageTag string `json:"repository-image-tag,omitempty"`

	// DeploymentReplicas is the number of replicas to deploy using "Deployment" object.
	DeploymentReplicas int32 `json:"deployment-replicas"`
	// DeploymentNodeSelector is configured to overwrite existing node selector
	// for PHP Apache deployment. If left empty, tester sets default selector.
	DeploymentNodeSelector map[string]string `json:"deployment-node-selector"`
}

// EnvironmentVariablePrefixAddOnPHPApache is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnPHPApache = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_PHP_APACHE_"

// IsEnabledAddOnPHPApache returns true if "AddOnPHPApache" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnPHPApache() bool {
	if cfg.AddOnPHPApache == nil {
		return false
	}
	if cfg.AddOnPHPApache.Enable {
		return true
	}
	cfg.AddOnPHPApache = nil
	return false
}

func getDefaultAddOnPHPApache() *AddOnPHPApache {
	return &AddOnPHPApache{
		Enable:                 false,
		DeploymentReplicas:     3,
		DeploymentNodeSelector: make(map[string]string),
	}
}

func (cfg *Config) GetAddOnPHPApacheRepositoryRegion() string {
	if !cfg.IsEnabledAddOnPHPApache() {
		return cfg.Region
	}
	return cfg.AddOnPHPApache.RepositoryRegion
}

func (cfg *Config) validateAddOnPHPApache() error {
	if !cfg.IsEnabledAddOnPHPApache() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnPHPApache.Enable true but no node group is enabled")
	}
	if cfg.AddOnPHPApache.Namespace == "" {
		cfg.AddOnPHPApache.Namespace = cfg.Name + "-php-apache"
	}
	return nil
}
