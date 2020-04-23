package eksconfig

import "time"

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

	// Namespace is the namespace to create "Secret" and "Pod" objects in.
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

	// ServiceAccountName is the IRSA ServiceAccount name.
	ServiceAccountName string `json:"service-account-name"`
	// ConfigMapName is the ConfigMap name.
	ConfigMapName string `json:"config-map-name"`
	// ConfigMapScriptFileName is the IRSA ConfigMap script name.
	ConfigMapScriptFileName string `json:"config-map-script-file-name"`
	// S3Key is the S3 key to write for IRSA tests.
	S3Key string `json:"s3-key"`

	// ProfileName is the profile name for Fargate.
	ProfileName string `json:"profile-name"`
	// PodName is the name of the Fargate Pod with IRSA.
	PodName string `json:"pod-name"`
	// ContainerName is the name of the Fargate container.
	ContainerName string `json:"container-name"`
}
