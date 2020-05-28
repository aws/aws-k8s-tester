package eksconfig

import (
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnClusterLoaderRemote defines parameters for EKS cluster
// add-on cluster loader remote.
// It generates loads from the remote host machine.
// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2
type AddOnClusterLoaderRemote struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`
}

// EnvironmentVariablePrefixAddOnClusterLoaderRemote is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnClusterLoaderRemote = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_CLUSTER_LOADER_REMOTE_"

// IsEnabledAddOnClusterLoaderRemote returns true if "AddOnClusterLoaderRemote" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnClusterLoaderRemote() bool {
	if cfg.AddOnClusterLoaderRemote == nil {
		return false
	}
	if cfg.AddOnClusterLoaderRemote.Enable {
		return true
	}
	cfg.AddOnClusterLoaderRemote = nil
	return false
}

func getDefaultAddOnClusterLoaderRemote() *AddOnClusterLoaderRemote {
	return &AddOnClusterLoaderRemote{
		Enable: false,
	}
}

func (cfg *Config) validateAddOnClusterLoaderRemote() error {
	if !cfg.IsEnabledAddOnClusterLoaderRemote() {
		return nil
	}

	if cfg.AddOnClusterLoaderRemote.Namespace == "" {
		cfg.AddOnClusterLoaderRemote.Namespace = cfg.Name + "-cluster-loader-remote"
	}

	return nil
}
