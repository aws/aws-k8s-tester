package eksconfig

import "time"

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

	// Namespace is the namespace to create "Secret" and "Pod" objects in.
	Namespace string `json:"namespace"`

	// RoleName is the role name for IRSA.
	RoleName string `json:"role-name"`
	// RoleARN is the role ARN for IRSA.
	RoleARN string `json:"role-arn"`
	// RoleManagedPolicyARNs is IRSA role managed policy ARNs.
	// ref. https://aws.amazon.com/blogs/opensource/introducing-fine-grained-iam-roles-service-accounts/
	RoleManagedPolicyARNs []string `json:"role-managed-policy-arns"`
	RoleCFNStackID        string   `json:"role-cfn-stack-id" read-only:"true"`

	// ServiceAccountName is the ServiceAccount name.
	ServiceAccountName string `json:"service-account-name"`
	// ConfigMapName is the ConfigMap name.
	ConfigMapName string `json:"config-map-name"`
	// ConfigMapScriptFileName is the ConfigMap script name.
	ConfigMapScriptFileName string `json:"config-map-script-file-name"`
	// S3Key is the S3 key to write for IRSA tests.
	S3Key string `json:"s3-key"`

	// DeploymentName is the Deployment name.
	DeploymentName string `json:"deployment-name"`
	// DeploymentReplicas is the number of Deployment replicas.
	DeploymentReplicas int32 `json:"deployment-replicas"`
	// DeploymentResultPath is the output of "Deployment" run.
	DeploymentResultPath string `json:"deployment-result-path"`
	// DeploymentTook is the duration that took for Deployment resource.
	DeploymentTook time.Duration `json:"deployment-took,omitempty" read-only:"true"`
	// DeploymentTookString is the duration that took for Deployment resource.
	DeploymentTookString string `json:"deployment-took-string,omitempty" read-only:"true"`
}
