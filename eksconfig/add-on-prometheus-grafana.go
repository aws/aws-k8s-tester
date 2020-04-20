package eksconfig

import "time"

// IsEnabledAddOnPrometheusGrafana returns true if "AddOnPrometheusGrafana" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnPrometheusGrafana() bool {
	if cfg.AddOnPrometheusGrafana == nil {
		return false
	}
	if cfg.AddOnPrometheusGrafana.Enable {
		return true
	}
	cfg.AddOnPrometheusGrafana = nil
	return false
}

// AddOnPrometheusGrafana defines parameters for EKS cluster
// add-on Prometheus/Grafana.
type AddOnPrometheusGrafana struct {
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
