package eksconfig

import (
	"fmt"
)

// ClusterLoaderSpec defines the spec for the Addon
type ClusterLoaderSpec struct {
	Image          string   `json:"image,omitempty"`
	Nodes          int32    `json:"nodes,omitempty"`
	TestConfigUris []string `json:"testConfigUris,omitempty"`
	TestOverrides  []string `json:"testOverrides,omitempty"`
	// Specifies which instance type the clusterloader2 pod will be able to be scheduled on
	// Leaving this empty will allow it to be scheduled on any instance type
	InstanceTypes []string `json:"instanceTypes,omitempty"`
}

// ClusterLoaderStatus defines the status for the Addon
type ClusterLoaderStatus struct {
	AddonStatus `json:",inline"`
}

// Validate installs the addon
func (spec *ClusterLoaderSpec) Validate(cfg *Config) error {
	if spec.Nodes <= 0 {
		return fmt.Errorf("ClusterLoaderSpec.Nodes must be greater than 0")
	}
	if len(spec.TestConfigUris) == 0 {
		return fmt.Errorf("ClusterLoaderSpec.TestConfigUris array must have length greater than 0")
	}
	return nil
}

// Default installs the addon
func (spec *ClusterLoaderSpec) Default(cfg *Config) {
	if spec.Image == "" {
		spec.Image = "197575167141.dkr.ecr.us-west-2.amazonaws.com/clusterloader2:latest"
	}
	if spec.InstanceTypes == nil {
		spec.InstanceTypes = []string{}
	}
}
