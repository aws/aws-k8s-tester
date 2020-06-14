package eksconfig

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnIRSA defines parameters for EKS cluster
// add-on "IAM Roles for Service Accounts (IRSA)".
type AddOnIRSA struct {
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
	// e.g. "aws/aws-k8s-tester" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/aws/aws-k8s-tester"
	RepositoryAccountID string `json:"repository-account-id,omitempty"`
	// RepositoryName is the repositoryName for tester ECR image.
	// e.g. "aws/aws-k8s-tester" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/aws/aws-k8s-tester"
	RepositoryName string `json:"repository-name,omitempty"`
	// RepositoryImageTag is the image tag for tester ECR image.
	// e.g. "latest" for image URI "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/aws/aws-k8s-tester:latest"
	RepositoryImageTag string `json:"repository-image-tag,omitempty"`

	// RoleName is the role name for IRSA.
	RoleName string `json:"role-name"`
	// RoleARN is the role ARN for IRSA.
	RoleARN string `json:"role-arn"`
	// RoleManagedPolicyARNs is IRSA role managed policy ARNs.
	// ref. https://aws.amazon.com/blogs/opensource/introducing-fine-grained-iam-roles-service-accounts/
	RoleManagedPolicyARNs    []string `json:"role-managed-policy-arns"`
	RoleCFNStackID           string   `json:"role-cfn-stack-id" read-only:"true"`
	RoleCFNStackYAMLFilePath string   `json:"role-cfn-stack-yaml-file-path" read-only:"true"`

	// S3Key is the S3 key to write for IRSA tests.
	S3Key string `json:"s3-key"`

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

	if cfg.AddOnIRSA.Namespace == "" {
		cfg.AddOnIRSA.Namespace = cfg.Name + "-irsa"
	}

	if cfg.AddOnIRSA.RepositoryAccountID == "" {
		return errors.New("AddOnIRSA.RepositoryAccountID empty")
	}
	if cfg.AddOnIRSA.RepositoryName == "" {
		return errors.New("AddOnIRSA.RepositoryName empty")
	}
	if cfg.AddOnIRSA.RepositoryImageTag == "" {
		return errors.New("AddOnIRSA.RepositoryImageTag empty")
	}

	if cfg.S3BucketName == "" {
		return errors.New("AddOnIRSA requires S3 bucket but S3BucketName empty")
	}

	if cfg.AddOnIRSA.RoleName == "" {
		cfg.AddOnIRSA.RoleName = cfg.Name + "-add-on-irsa-role"
	}
	if cfg.AddOnIRSA.RoleCFNStackYAMLFilePath == "" {
		cfg.AddOnIRSA.RoleCFNStackYAMLFilePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".add-on-irsa.role.cfn.yaml"
	}

	if cfg.AddOnIRSA.S3Key == "" {
		cfg.AddOnIRSA.S3Key = path.Join(cfg.Name, "irsa-s3-key")
	}

	if cfg.AddOnIRSA.DeploymentResultPath == "" {
		cfg.AddOnIRSA.DeploymentResultPath = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-deployment-irsa-result.log")
	}
	return nil
}
