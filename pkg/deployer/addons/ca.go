package addons

import "github.com/aws/aws-k8s-tester/pkg/deployer/types"

// ClusterAutoscaler is an addon that installs Cluster Autoscaler
type ClusterAutoscaler struct{}

// NewClusterAutoscaler initializes the addon
func NewClusterAutoscaler() *ClusterAutoscaler {
	return &ClusterAutoscaler{}
}

// IsEnabled returns true if its configuration is not null
func (c *ClusterAutoscaler) IsEnabled(t *types.DeployerConfiguration) bool {
	return t.Spec.ClusterAutoscaler != nil
}

// Validate installs the addon
func (c *ClusterAutoscaler) Validate(t *types.DeployerConfiguration) error {
	return nil
}

// Default installs the addon
func (c *ClusterAutoscaler) Default(t *types.DeployerConfiguration) {
}

// Apply installs the addon
func (c *ClusterAutoscaler) Apply(t *types.DeployerConfiguration) error {
	return nil
}

// Finalize installs the addon
func (c *ClusterAutoscaler) Finalize(t *types.DeployerConfiguration) error {
	return nil
}
