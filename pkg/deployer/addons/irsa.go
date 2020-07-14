package addons

import "github.com/aws/aws-k8s-tester/pkg/deployer/types"

// IAMRoleServiceAccount is an addon that sets up IRSA
type IAMRoleServiceAccount struct {
}

// NewIAMRoleServiceAccount initializes the addon
func NewIAMRoleServiceAccount() *IAMRoleServiceAccount {
	return &IAMRoleServiceAccount{}
}

// IsEnabled returns true if its configuration is not null
func (i *IAMRoleServiceAccount) IsEnabled(t *types.DeployerConfiguration) bool {
	return t.Spec.IAMRoleServiceAccount != nil
}

// Validate installs the addon
func (i *IAMRoleServiceAccount) Validate(t *types.DeployerConfiguration) error {
	return nil
}

// Default installs the addon
func (i *IAMRoleServiceAccount) Default(t *types.DeployerConfiguration) {
}

// Apply installs the addon
func (i *IAMRoleServiceAccount) Apply(t *types.DeployerConfiguration) error {
	return nil
}

// Finalize installs the addon
func (i *IAMRoleServiceAccount) Finalize(t *types.DeployerConfiguration) error {
	return nil
}
