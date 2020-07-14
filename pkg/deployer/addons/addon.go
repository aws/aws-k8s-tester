package addons

import "github.com/aws/aws-k8s-tester/pkg/deployer/types"

// Addon is implemented by every addon and executed generically
type Addon interface {
	// IsEnabled returns true if the addon is enabled
	IsEnabled(t *types.DeployerConfiguration) bool
	// Apply idempotently creates addon resources
	Apply(t *types.DeployerConfiguration) error
	// Finalize idempotently deletes addon resources
	Finalize(t *types.DeployerConfiguration) error
	// Validate checks if the addon is valid for the testrun
	Validate(t *types.DeployerConfiguration) error
	// Default sets default values for the addon
	Default(t *types.DeployerConfiguration)
}
