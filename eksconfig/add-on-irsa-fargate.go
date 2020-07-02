package eksconfig

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnIRSAFargate defines parameters for EKS cluster
// add-on "IAM Roles for Service Accounts (IRSA)" with Fargate.
type AddOnIRSAFargate struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// S3Dir is the S3 directory to store all test results.
	// It is under the bucket "eksconfig.Config.S3BucketName".
	S3Dir string `json:"s3-dir"`

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`

	// RepositoryAccountID is the account ID for tester ECR image.
	// e.g. "aws/aws-k8s-tester" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/aws/aws-k8s-tester"
	RepositoryAccountID string `json:"repository-account-id,omitempty"`
	// RepositoryRegion is the ECR repository region to pull from.
	RepositoryRegion string `json:"repository-region,omitempty"`
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
	// RoleServicePrincipals is the Fargate role Service Principals
	RoleServicePrincipals []string `json:"role-service-principals"`
	// RoleManagedPolicyARNs is IRSA role managed policy ARNs.
	// ref. https://aws.amazon.com/blogs/opensource/introducing-fine-grained-iam-roles-service-accounts/
	RoleManagedPolicyARNs []string `json:"role-managed-policy-arns"`
	RoleCFNStackID        string   `json:"role-cfn-stack-id" read-only:"true"`
	RoleCFNStackYAMLPath  string   `json:"role-cfn-stack-yaml-path" read-only:"true"`
	RoleCFNStackYAMLS3Key string   `json:"role-cfn-stack-yaml-s3-key" read-only:"true"`

	// S3Key is the S3 key to write for IRSA tests.
	S3Key string `json:"s3-key"`

	// ProfileName is the profile name for Fargate.
	ProfileName string `json:"profile-name"`
}

// EnvironmentVariablePrefixAddOnIRSAFargate is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnIRSAFargate = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_IRSA_FARGATE_"

// IsEnabledAddOnIRSAFargate returns true if "AddOnIRSAFargate" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnIRSAFargate() bool {
	if cfg.AddOnIRSAFargate == nil {
		return false
	}
	if cfg.AddOnIRSAFargate.Enable {
		return true
	}
	cfg.AddOnIRSAFargate = nil
	return false
}

func getDefaultAddOnIRSAFargate() *AddOnIRSAFargate {
	return &AddOnIRSAFargate{
		Enable: false,
	}
}

func (cfg *Config) validateAddOnIRSAFargate() error {
	if !cfg.IsEnabledAddOnIRSAFargate() {
		return nil
	}

	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnIRSAFargate.Enable true but no node group is enabled")
	}

	if cfg.Parameters.VersionValue < 1.14 {
		return fmt.Errorf("Version %q not supported for AddOnIRSAFargate", cfg.Parameters.Version)
	}

	if cfg.AddOnIRSAFargate.S3Dir == "" {
		cfg.AddOnIRSAFargate.S3Dir = path.Join(cfg.Name, "add-on-irsa-fargate")
	}

	if cfg.AddOnIRSAFargate.Namespace == "" {
		cfg.AddOnIRSAFargate.Namespace = cfg.Name + "-irsa-fargate"
	}

	if cfg.AddOnIRSAFargate.RepositoryAccountID == "" {
		return errors.New("AddOnIRSAFargate.RepositoryAccountID empty")
	}
	if cfg.AddOnIRSAFargate.RepositoryRegion == "" {
		cfg.AddOnIRSAFargate.RepositoryRegion = cfg.Region
	}
	if cfg.AddOnIRSAFargate.RepositoryName == "" {
		return errors.New("AddOnIRSAFargate.RepositoryName empty")
	}
	if cfg.AddOnIRSAFargate.RepositoryImageTag == "" {
		return errors.New("AddOnIRSAFargate.RepositoryImageTag empty")
	}

	if cfg.AddOnIRSAFargate.RoleName == "" {
		cfg.AddOnIRSAFargate.RoleName = cfg.Name + "add-on-irsa-fargate-role"
	}
	if cfg.AddOnIRSAFargate.RoleCFNStackYAMLPath == "" {
		cfg.AddOnIRSAFargate.RoleCFNStackYAMLPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".add-on-irsa-fargate.role.cfn.yaml"
	}
	if cfg.AddOnIRSAFargate.RoleCFNStackYAMLS3Key == "" {
		cfg.AddOnIRSAFargate.RoleCFNStackYAMLS3Key = path.Join(
			cfg.AddOnIRSAFargate.S3Dir,
			filepath.Base(cfg.AddOnIRSAFargate.RoleCFNStackYAMLPath),
		)
	}

	if cfg.AddOnIRSAFargate.S3Key == "" {
		cfg.AddOnIRSAFargate.S3Key = path.Join(cfg.Name, "add-on-irsa-fargate-s3-key")
	}
	// do not prefix with "eks-"
	// e.g. "The fargate profile name starts with the reserved prefix: 'eks-'."
	if cfg.AddOnIRSAFargate.ProfileName == "" {
		cfg.AddOnIRSAFargate.ProfileName = cfg.Name + "-add-on-irsa-fargate-profile"
	}
	if strings.HasPrefix(cfg.AddOnIRSAFargate.ProfileName, "eks-") {
		cfg.AddOnIRSAFargate.ProfileName = strings.Replace(cfg.AddOnIRSAFargate.ProfileName, "eks-", "", 1)
	}

	return nil
}
