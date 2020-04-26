package eksconfig

import "time"

// IsEnabledAddOnClusterLoader returns true if "AddOnClusterLoader" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnClusterLoader() bool {
	if cfg.AddOnClusterLoader == nil {
		return false
	}
	if cfg.AddOnClusterLoader.Enable {
		return true
	}
	cfg.AddOnClusterLoader = nil
	return false
}

// AddOnClusterLoader defines parameters for EKS cluster
// add-on Cluster Loader.
// ref. https://github.com/kubernetes/perf-tests
type AddOnClusterLoader struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`

	// Namespace is the namespace to create cluster loads.
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

	// Duration is the duration to run load testing.
	Duration       time.Duration `json:"duration,omitempty"`
	DurationString string        `json:"duration-string,omitempty" read-only:"true"`
	// HollowNodes is the number of hollow nodes to create.
	// Writes happen concurrently with multiple clients
	// e.g. 10 clients with each client 5 QPS can create 50 objects per second.
	HollowNodes int `json:"hollow-nodes"`
}
