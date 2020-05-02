package eksconfig

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

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

	// Namespace is the namespace to create objects in.
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

// EnvironmentVariablePrefixAddOnFargate is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnFargate = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_FARGATE_"

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

// only letters and numbers for Secret key names
var fargateSecretRegex = regexp.MustCompile("[^a-zA-Z0-9]+")

func (cfg *Config) validateAddOnFargate() error {
	if !cfg.IsEnabledAddOnFargate() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnFargate.Enable true but no node group is enabled")
	}
	if cfg.Parameters.VersionValue < 1.14 {
		return fmt.Errorf("Version %q not supported for AddOnFargate", cfg.Parameters.Version)
	}
	if cfg.AddOnFargate.Namespace == "" {
		cfg.AddOnFargate.Namespace = cfg.Name + "-fargate"
	}
	// do not prefix with "eks-"
	// e.g. "The fargate profile name starts with the reserved prefix: 'eks-'."
	if cfg.AddOnFargate.ProfileName == "" {
		cfg.AddOnFargate.ProfileName = cfg.Name + "-fargate-profile"
	}
	if strings.HasPrefix(cfg.AddOnFargate.ProfileName, "eks-") {
		cfg.AddOnFargate.ProfileName = strings.Replace(cfg.AddOnFargate.ProfileName, "eks-", "", 1)
	}
	if cfg.AddOnFargate.SecretName == "" {
		cfg.AddOnFargate.SecretName = cfg.Name + "addonfargatesecret"
	}
	if cfg.AddOnFargate.PodName == "" {
		cfg.AddOnFargate.PodName = cfg.Name + "-pod-fargate"
	}
	if cfg.AddOnFargate.ContainerName == "" {
		cfg.AddOnFargate.ContainerName = cfg.Name + "-" + randString(10)
	}
	cfg.AddOnFargate.SecretName = strings.ToLower(fargateSecretRegex.ReplaceAllString(cfg.AddOnFargate.SecretName, ""))

	switch cfg.AddOnFargate.RoleCreate {
	case true: // need create one, or already created
		if cfg.AddOnFargate.RoleName == "" {
			cfg.AddOnFargate.RoleName = cfg.Name + "-role-fargate"
		}
		if cfg.AddOnFargate.RoleARN != "" {
			// just ignore...
			// could be populated from previous run
			// do not error, so long as RoleCreate false, role won't be deleted
		}
	case false: // use existing one
		if cfg.AddOnFargate.RoleARN == "" {
			return fmt.Errorf("AddOnFargate.RoleCreate false; expect non-empty RoleARN but got %q", cfg.AddOnFargate.RoleARN)
		}
		if cfg.AddOnFargate.RoleName == "" {
			cfg.AddOnFargate.RoleName = getNameFromARN(cfg.AddOnFargate.RoleARN)
		}
		if len(cfg.AddOnFargate.RoleManagedPolicyARNs) > 0 {
			return fmt.Errorf("AddOnFargate.RoleCreate false; expect empty RoleManagedPolicyARNs but got %q", cfg.AddOnFargate.RoleManagedPolicyARNs)
		}
		if len(cfg.AddOnFargate.RoleServicePrincipals) > 0 {
			return fmt.Errorf("AddOnFargate.RoleCreate false; expect empty RoleServicePrincipals but got %q", cfg.AddOnFargate.RoleServicePrincipals)
		}
	}

	return nil
}
