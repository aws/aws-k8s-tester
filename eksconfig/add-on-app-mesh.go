package eksconfig

import "time"

// IsEnabledAddOnAppMesh returns true if "AddOnAppMesh" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnAppMesh() bool {
	if cfg.AddOnAppMesh == nil {
		return false
	}
	if cfg.AddOnAppMesh.Enable {
		return true
	}
	cfg.AddOnAppMesh = nil
	return false
}

// AddOnAppMesh defines parameters for EKS cluster
// add-on "EKS App Mesh Integration".
type AddOnAppMesh struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`

	// Namespace is the namespace to create "AppMesh" controller/injector.
	Namespace string `json:"namespace"`

	// The image of appMesh controller
	ControllerImage string `json:"controller-image"`

	// The image of appMesh injector
	InjectorImage string `json:"injector-image"`

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

	// PolicyCFNStackID is the CFN stack ID for policy.
	PolicyCFNStackID string `json:"policy-cfn-stack-id,omitempty" read-only:"true"`
}
