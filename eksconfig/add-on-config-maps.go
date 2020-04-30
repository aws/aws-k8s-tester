package eksconfig

import "time"

// IsEnabledAddOnConfigMaps returns true if "AddOnConfigMaps" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnConfigMaps() bool {
	if cfg.AddOnConfigMaps == nil {
		return false
	}
	if cfg.AddOnConfigMaps.Enable {
		return true
	}
	cfg.AddOnConfigMaps = nil
	return false
}

// AddOnConfigMaps defines parameters for EKS cluster
// add-on "ConfigMap".
type AddOnConfigMaps struct {
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

	// Namespace is the namespace to create "ConfigMap" objects in.
	Namespace string `json:"namespace"`

	// Objects is the number of "ConfigMap" objects to create.
	Objects int `json:"objects"`
	// Size is the "ConfigMap" value size in bytes.
	Size int `json:"size"`
	// CreatedNames is the list of created "ConfigMap" object names.
	CreatedNames []string `json:"created-names" read-only:"true"`

	// FailThreshold is the number of write failures to allow.
	FailThreshold int `json:"fail-threshold"`
}
