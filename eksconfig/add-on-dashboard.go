package eksconfig

import "time"

// IsEnabledAddOnDashboard returns true if "AddOnDashboard" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnDashboard() bool {
	if cfg.AddOnDashboard == nil {
		return false
	}
	if cfg.AddOnDashboard.Enable {
		return true
	}
	cfg.AddOnDashboard = nil
	return false
}

// AddOnDashboard defines parameters for EKS cluster
// add-on Dashboard.
type AddOnDashboard struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`

	// Namespace is the namespace to create "AppMesh" controller/injector.
	Namespace string `json:"namespace"`

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
}
