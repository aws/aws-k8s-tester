package addons

import "github.com/aws/aws-k8s-tester/pkg/deployer/types"

// ManagedNodeGroups is an addon that installs EKS Managed Node Groups
type ManagedNodeGroups struct{}

// NewManagedNodeGroups initializes the addon
func NewManagedNodeGroups() *ManagedNodeGroups {
	return &ManagedNodeGroups{}
}

// IsEnabled returns true if its configuration is not null
func (m *ManagedNodeGroups) IsEnabled(t *types.DeployerConfiguration) bool {
	return t.Spec.ManagedNodeGroups != nil
}

// Validate installs the addon
func (m *ManagedNodeGroups) Validate(t *types.DeployerConfiguration) error {
	return nil
}

// Default installs the addon
func (m *ManagedNodeGroups) Default(t *types.DeployerConfiguration) {
}

// Apply installs the addon
func (m *ManagedNodeGroups) Apply(t *types.DeployerConfiguration) error {
	return nil
}

// Finalize installs the addon
func (m *ManagedNodeGroups) Finalize(t *types.DeployerConfiguration) error {
	return nil
}
