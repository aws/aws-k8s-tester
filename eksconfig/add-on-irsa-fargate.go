package eksconfig

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"time"
)

// AddOnIRSAFargate defines parameters for EKS cluster
// add-on "IAM Roles for Service Accounts (IRSA)" with Fargate.
type AddOnIRSAFargate struct {
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
	// RoleServicePrincipals is the Fargate role Service Principals
	RoleServicePrincipals []string `json:"role-service-principals"`
	// RoleManagedPolicyARNs is IRSA role managed policy ARNs.
	// ref. https://aws.amazon.com/blogs/opensource/introducing-fine-grained-iam-roles-service-accounts/
	RoleManagedPolicyARNs []string `json:"role-managed-policy-arns"`
	RoleCFNStackID        string   `json:"role-cfn-stack-id" read-only:"true"`

	// S3Key is the S3 key to write for IRSA tests.
	S3Key string `json:"s3-key"`

	// ProfileName is the profile name for Fargate.
	ProfileName string `json:"profile-name"`

	// RepositoryAccountID is the account ID for tester ECR image.
	// e.g. "aws/aws-k8s-tester" for "[ACCOUNT_ID].dkr.ecr.us-west-2.amazonaws.com/aws/aws-k8s-tester"
	RepositoryAccountID string `json:"repository-account-id,omitempty"`
	// RepositoryName is the repositoryName for tester ECR image.
	// e.g. "aws/aws-k8s-tester" for "[ACCOUNT_ID].dkr.ecr.us-west-2.amazonaws.com/aws/aws-k8s-tester"
	RepositoryName string `json:"repository-name,omitempty"`
	// RepositoryURI is the repositoryUri for tester ECR image.
	// e.g. "[ACCOUNT_ID].dkr.ecr.us-west-2.amazonaws.com/aws/aws-k8s-tester"
	RepositoryURI string `json:"repository-uri,omitempty"`
	// RepositoryImageTag is the image tag for tester ECR image.
	// e.g. "latest" for image URI "[ACCOUNT_ID].dkr.ecr.us-west-2.amazonaws.com/aws/aws-k8s-tester:latest"
	RepositoryImageTag string `json:"repository-image-tag,omitempty"`
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
	if cfg.S3BucketName == "" {
		return errors.New("AddOnIRSAFargate requires S3 bucket but S3BucketName empty")
	}
	if cfg.AddOnIRSAFargate.Namespace == "" {
		cfg.AddOnIRSAFargate.Namespace = cfg.Name + "-irsa-fargate"
	}
	if cfg.AddOnIRSAFargate.RoleName == "" {
		cfg.AddOnIRSAFargate.RoleName = cfg.Name + "-role-irsa-fargate"
	}
	if cfg.AddOnIRSAFargate.S3Key == "" {
		cfg.AddOnIRSAFargate.S3Key = path.Join(cfg.Name, "s3-key-irsa-fargate")
	}
	// do not prefix with "eks-"
	// e.g. "The fargate profile name starts with the reserved prefix: 'eks-'."
	if cfg.AddOnIRSAFargate.ProfileName == "" {
		cfg.AddOnIRSAFargate.ProfileName = cfg.Name + "-irsa-fargate-profile"
	}

	if cfg.AddOnIRSAFargate.RepositoryAccountID == "" {
		return errors.New("AddOnIRSAFargate.RepositoryAccountID empty")
	}
	if cfg.AddOnIRSAFargate.RepositoryName == "" {
		return errors.New("AddOnIRSAFargate.RepositoryName empty")
	}
	if cfg.AddOnIRSAFargate.RepositoryURI == "" {
		return errors.New("AddOnIRSAFargate.RepositoryURI empty")
	}
	if strings.Contains(cfg.AddOnIRSAFargate.RepositoryURI, cfg.AddOnIRSAFargate.RepositoryAccountID) {
		return fmt.Errorf("AddOnIRSAFargate.RepositoryURI %q does not have AddOnIRSAFargate.RepositoryAccountID %q", cfg.AddOnIRSAFargate.RepositoryURI, cfg.AddOnIRSAFargate.RepositoryAccountID)
	}
	if cfg.AddOnIRSAFargate.RepositoryImageTag == "" {
		return errors.New("AddOnIRSAFargate.RepositoryImageTag empty")
	}

	if strings.HasPrefix(cfg.AddOnIRSAFargate.ProfileName, "eks-") {
		cfg.AddOnIRSAFargate.ProfileName = strings.Replace(cfg.AddOnIRSAFargate.ProfileName, "eks-", "", 1)
	}

	return nil
}
