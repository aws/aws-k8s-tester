package eksconfig

import (
	"time"
)

// AddOnClusterLoader defines parameters for EKS cluster
// add-on Cluster Loader.
// ref. https://github.com/kubernetes/perf-tests
type AddOnClusterLoader struct {
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

	// Duration is the duration to run load testing.
	// The cluster loader waits "one" "Duration" for hollow ones.
	// And other one for cluster loader.
	Duration       time.Duration `json:"duration,omitempty"`
	DurationString string        `json:"duration-string,omitempty" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnClusterLoader is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnClusterLoader = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_CLUSTER_LOADER_"

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

func getDefaultAddOnClusterLoader() *AddOnClusterLoader {
	return &AddOnClusterLoader{
		Enable:   false,
		Duration: time.Minute,
	}
}

func (cfg *Config) validateAddOnClusterLoader() error {
	if !cfg.IsEnabledAddOnClusterLoader() {
		return nil
	}

	if cfg.AddOnClusterLoader.Duration == time.Duration(0) {
		cfg.AddOnClusterLoader.Duration = time.Minute
	}
	cfg.AddOnClusterLoader.DurationString = cfg.AddOnClusterLoader.Duration.String()

	return nil
}
