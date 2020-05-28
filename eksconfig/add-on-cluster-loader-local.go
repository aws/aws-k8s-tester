package eksconfig

import (
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnClusterLoaderLocal defines parameters for EKS cluster
// add-on cluster loader local.
// It generates loads from the local host machine.
// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2
type AddOnClusterLoaderLocal struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	NodesPerNamespace              int  `json:"nodes-per-namespace"`
	PodsPerNamespace               int  `json:"pods-per-namespace"`
	BigGroupSize                   int  `json:"big-group-size"`
	MediumGroupSize                int  `json:"medium-group-size"`
	SmallGroupSize                 int  `json:"small-group-size"`
	SmallStatefulSetsPerNamespace  int  `json:"small-stateful-sets-per-namespace"`
	MediumStatefulSetsPerNamespace int  `json:"medium-stateful-sets-per-namespace"`
	CL2EnablePVS                   bool `json:"cl2-enable-pvs`
	PrometheusScrapeKubeProxy      bool `json:"prometheus-scrape-kube-proxy`
	EnableSystemPodMetrics         bool `json:"enable-system-pod-metrics`
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
		Enable:            false,
		NodesPerNamespace: 10,
	}
}

func (cfg *Config) validateAddOnClusterLoaderLocal() error {
	if !cfg.IsEnabledAddOnClusterLoaderLocal() {
		return nil
	}

	if cfg.AddOnClusterLoaderLocal.Namespace == "" {
		cfg.AddOnClusterLoaderLocal.Namespace = cfg.Name + "-cluster-loader-local"
	}

	return nil
}
