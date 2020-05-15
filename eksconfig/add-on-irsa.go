package eksconfig

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"time"
)

// AddOnIRSA defines parameters for EKS cluster
// add-on "IAM Roles for Service Accounts (IRSA)".
type AddOnIRSA struct {
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

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`

	// RoleName is the role name for IRSA.
	RoleName string `json:"role-name"`
	// RoleARN is the role ARN for IRSA.
	RoleARN string `json:"role-arn"`
	// RoleManagedPolicyARNs is IRSA role managed policy ARNs.
	// ref. https://aws.amazon.com/blogs/opensource/introducing-fine-grained-iam-roles-service-accounts/
	RoleManagedPolicyARNs []string `json:"role-managed-policy-arns"`
	RoleCFNStackID        string   `json:"role-cfn-stack-id" read-only:"true"`

	// S3Key is the S3 key to write for IRSA tests.
	S3Key string `json:"s3-key"`

	// RepositoryName is the repositoryName for tester.
	// e.g. "aws/aws-k8s-tester" for "[ACCOUNT_ID].dkr.ecr.us-west-2.amazonaws.com/aws/aws-k8s-tester"
	RepositoryName string `json:"repository-name,omitempty"`
	// RepositoryURI is the repositoryUri for tester.
	// e.g. "[ACCOUNT_ID].dkr.ecr.us-west-2.amazonaws.com/aws/aws-k8s-tester"
	RepositoryURI string `json:"repository-uri,omitempty"`
	// RepositoryImageTag is the image tag for tester.
	// e.g. "latest" for image URI "[ACCOUNT_ID].dkr.ecr.us-west-2.amazonaws.com/aws/aws-k8s-tester:latest"
	RepositoryImageTag string `json:"repository-image-tag,omitempty"`

	// DeploymentReplicas is the number of Deployment replicas.
	DeploymentReplicas int32 `json:"deployment-replicas"`
	// DeploymentResultPath is the output of "Deployment" run.
	DeploymentResultPath string `json:"deployment-result-path"`
	// DeploymentTook is the duration that took for Deployment resource.
	DeploymentTook time.Duration `json:"deployment-took,omitempty" read-only:"true"`
	// DeploymentTookString is the duration that took for Deployment resource.
	DeploymentTookString string `json:"deployment-took-string,omitempty" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnIRSA is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnIRSA = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_IRSA_"

// IsEnabledAddOnIRSA returns true if "AddOnIRSA" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnIRSA() bool {
	if cfg.AddOnIRSA == nil {
		return false
	}
	if cfg.AddOnIRSA.Enable {
		return true
	}
	cfg.AddOnIRSA = nil
	return false
}

func getDefaultAddOnIRSA() *AddOnIRSA {
	return &AddOnIRSA{
		Enable:             false,
		DeploymentReplicas: 10,
	}
}

func (cfg *Config) validateAddOnIRSA() error {
	if !cfg.IsEnabledAddOnIRSA() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnIRSA.Enable true but no node group is enabled")
	}
	if cfg.Parameters.VersionValue < 1.14 {
		return fmt.Errorf("Version %q not supported for AddOnIRSA", cfg.Parameters.Version)
	}
	if cfg.S3BucketName == "" {
		return errors.New("AddOnIRSA requires S3 bucket but S3BucketName empty")
	}
	if cfg.AddOnIRSA.Namespace == "" {
		cfg.AddOnIRSA.Namespace = cfg.Name + "-irsa"
	}
	if cfg.AddOnIRSA.RoleName == "" {
		cfg.AddOnIRSA.RoleName = cfg.Name + "-role-irsa"
	}
	if cfg.AddOnIRSA.S3Key == "" {
		cfg.AddOnIRSA.S3Key = path.Join(cfg.Name, "s3-key-irsa")
	}

	if cfg.AddOnIRSA.RepositoryName == "" {
		return errors.New("AddOnIRSA.RepositoryName empty")
	}
	if cfg.AddOnIRSA.RepositoryURI == "" {
		return errors.New("AddOnIRSA.RepositoryURI empty")
	}
	if cfg.AddOnIRSA.RepositoryImageTag == "" {
		return errors.New("AddOnIRSA.RepositoryImageTag empty")
	}

	if cfg.AddOnIRSA.DeploymentResultPath == "" {
		cfg.AddOnIRSA.DeploymentResultPath = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-deployment-irsa-result.log")
	}
	return nil
}
