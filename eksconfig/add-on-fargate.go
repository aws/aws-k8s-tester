package eksconfig

import "time"

// IsEnabledAddOnFargate returns true if "AddOnFargate" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnFargate() bool {
	if cfg.AddOnFargate == nil {
		return false
	}
	if cfg.AddOnFargate.Enable {
		return true
	}
	cfg.AddOnFargate = nil
	return false
}

// AddOnFargate defines parameters for EKS cluster
// add-on "EKS on Fargate".
type AddOnFargate struct {
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

	// RoleName is the role name for Fargate.
	RoleName string `json:"role-name"`
	// RoleCreate is true to auto-create and delete role.
	RoleCreate bool `json:"role-create"`
	// RoleARN is the role ARN for Fargate.
	RoleARN string `json:"role-arn"`
	// RoleServicePrincipals is the Fargate role Service Principals
	RoleServicePrincipals []string `json:"role-service-principals"`
	// RoleManagedPolicyARNs is Fargate role managed policy ARNs.
	RoleManagedPolicyARNs []string `json:"role-managed-policy-arns"`
	RoleCFNStackID        string   `json:"role-cfn-stack-id" read-only:"true"`

	// ProfileName is the profile name for Fargate.
	ProfileName string `json:"profile-name"`
	// SecretName is the secret name for Fargate.
	SecretName string `json:"secret-name"`
	// PodName is the name of the Fargate Pod.
	PodName string `json:"pod-name"`
	// ContainerName is the name of the Fargate container.
	ContainerName string `json:"container-name"`
}
