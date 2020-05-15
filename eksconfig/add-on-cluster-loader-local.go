package eksconfig

import (
	"path/filepath"
	"time"
)

// AddOnClusterLoaderLocal defines parameters for EKS cluster
// add-on local Cluster Loader.
// ref. https://github.com/kubernetes/perf-tests
type AddOnClusterLoaderLocal struct {
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

	// RequestsSummary is the cluster loader results.
	RequestsSummary RequestsSummary `json:"requests-summary,omitempty" read-only:"true"`
	// RequestsSummaryJSONPath is the file path to store requests summary results in JSON format.
	RequestsSummaryJSONPath string `json:"requests-summary-json-path" read-only:"true"`
	// RequestsSummaryTablePath is the file path to store requests summary results in table format.
	RequestsSummaryTablePath string `json:"requests-summary-table-path" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnClusterLoaderLocal is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnClusterLoaderLocal = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_CLUSTER_LOADER_LOCAL_"

// IsEnabledAddOnClusterLoaderLocal returns true if "AddOnClusterLoaderLocal" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnClusterLoaderLocal() bool {
	if cfg.AddOnClusterLoaderLocal == nil {
		return false
	}
	if cfg.AddOnClusterLoaderLocal.Enable {
		return true
	}
	cfg.AddOnClusterLoaderLocal = nil
	return false
}

func getDefaultAddOnClusterLoaderLocal() *AddOnClusterLoaderLocal {
	return &AddOnClusterLoaderLocal{
		Enable:   false,
		Duration: time.Minute,
	}
}

func (cfg *Config) validateAddOnClusterLoaderLocal() error {
	if !cfg.IsEnabledAddOnClusterLoaderLocal() {
		return nil
	}

	if cfg.AddOnClusterLoaderLocal.Duration == time.Duration(0) {
		cfg.AddOnClusterLoaderLocal.Duration = time.Minute
	}
	cfg.AddOnClusterLoaderLocal.DurationString = cfg.AddOnClusterLoaderLocal.Duration.String()

	if cfg.AddOnClusterLoaderLocal.RequestsSummaryJSONPath == "" {
		cfg.AddOnClusterLoaderLocal.RequestsSummaryJSONPath = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-cluster-loader-local-request-summary.json")
	}
	if cfg.AddOnClusterLoaderLocal.RequestsSummaryTablePath == "" {
		cfg.AddOnClusterLoaderLocal.RequestsSummaryTablePath = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-cluster-loader-local-request-summary.txt")
	}

	return nil
}
