package eksconfig

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
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

	// ClusterLoaderPath is the clusterloader executable binary path.
	// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2
	ClusterLoaderPath        string `json:"cluster-loader-path"`
	ClusterLoaderDownloadURL string `json:"cluster-loader-download-url"`
	// ClusterLoaderTestConfigPath is the clusterloader2 test configuration file.
	// Set via "--testconfig" flag.
	ClusterLoaderTestConfigPath string `json:"cluster-loader-test-config-path"`
	// ClusterLoaderReportDir is the clusterloader2 test report directory.
	// Set via "--report-dir" flag.
	ClusterLoaderReportDir string `json:"cluster-loader-report-dir"`
	// ClusterLoaderLogsPath is the log file path to stream clusterloader binary runs.
	ClusterLoaderLogsPath string `json:"cluster-loader-logs-path" read-only:"true"`

	// Runs is the number of "clusterloader2" runs back-to-back.
	Runs int `json:"runs"`

	// Nodes is the number of nodes.
	// Set via "--nodes" flag.
	Nodes int `json:"nodes"`

	//
	//
	// below are set via "--testoverrides" flag

	NodesPerNamespace int `json:"nodes-per-namespace"`
	PodsPerNode       int `json:"pods-per-node"`

	BigGroupSize    int `json:"big-group-size"`
	MediumGroupSize int `json:"medium-group-size"`
	SmallGroupSize  int `json:"small-group-size"`

	SmallStatefulSetsPerNamespace  int `json:"small-stateful-sets-per-namespace"`
	MediumStatefulSetsPerNamespace int `json:"medium-stateful-sets-per-namespace"`

	CL2EnablePVS              bool `json:"cl2-enable-pvs`
	PrometheusScrapeKubeProxy bool `json:"prometheus-scrape-kube-proxy`
	EnableSystemPodMetrics    bool `json:"enable-system-pod-metrics`
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
	cfg := &AddOnClusterLoaderLocal{
		Enable: false,

		ClusterLoaderPath:        "/tmp/clusterloader2",
		ClusterLoaderDownloadURL: "https://aws-k8s-tester-public.s3-us-west-2.amazonaws.com/clusterloader2-amd64-linux",

		Runs: 1,

		Nodes: 10,

		NodesPerNamespace: 10,
		PodsPerNode:       10,

		BigGroupSize:    25,
		MediumGroupSize: 10,
		SmallGroupSize:  5,

		SmallStatefulSetsPerNamespace:  0,
		MediumStatefulSetsPerNamespace: 0,

		CL2EnablePVS:              false,
		PrometheusScrapeKubeProxy: false,
		EnableSystemPodMetrics:    false,
	}
	if runtime.GOOS == "darwin" {
		cfg.ClusterLoaderDownloadURL = strings.Replace(cfg.ClusterLoaderDownloadURL, "linux", "darwin", -1)
	}
	return cfg
}

func (cfg *Config) validateAddOnClusterLoaderLocal() error {
	if !cfg.IsEnabledAddOnClusterLoaderLocal() {
		return nil
	}

	if cfg.AddOnClusterLoaderLocal.ClusterLoaderLogsPath == "" {
		cfg.AddOnClusterLoaderLocal.ClusterLoaderLogsPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-cluster-loader-local-logs.log"
	}

	if cfg.AddOnClusterLoaderLocal.ClusterLoaderPath == "" && cfg.AddOnClusterLoaderLocal.ClusterLoaderDownloadURL == "" {
		return errors.New("empty AddOnClusterLoaderLocal.ClusterLoaderPath and ClusterLoaderDownloadURL")
	}
	if cfg.AddOnClusterLoaderLocal.ClusterLoaderTestConfigPath == "" {
		return errors.New("empty AddOnClusterLoaderLocal.ClusterLoaderTestConfigPath")
	}
	if cfg.AddOnClusterLoaderLocal.ClusterLoaderReportDir == "" {
		cfg.AddOnClusterLoaderLocal.ClusterLoaderReportDir = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-cluster-loader-local-report")
	}
	if err := fileutil.IsDirWriteable(cfg.AddOnClusterLoaderLocal.ClusterLoaderReportDir); err != nil {
		return err
	}

	if cfg.AddOnClusterLoaderLocal.Runs == 0 {
		return errors.New("unexpected zero AddOnClusterLoaderLocal.Runs")
	}

	if cfg.AddOnClusterLoaderLocal.Nodes == 0 {
		return errors.New("unexpected zero AddOnClusterLoaderLocal.Nodes")
	}

	if cfg.AddOnClusterLoaderLocal.CL2EnablePVS {
		return fmt.Errorf("unexpected AddOnClusterLoaderLocal.CL2EnablePVS %v; not supported yet", cfg.AddOnClusterLoaderLocal.CL2EnablePVS)
	}
	if cfg.AddOnClusterLoaderLocal.PrometheusScrapeKubeProxy {
		return fmt.Errorf("unexpected AddOnClusterLoaderLocal.PrometheusScrapeKubeProxy %v; not supported yet", cfg.AddOnClusterLoaderLocal.PrometheusScrapeKubeProxy)
	}
	if cfg.AddOnClusterLoaderLocal.EnableSystemPodMetrics {
		return fmt.Errorf("unexpected AddOnClusterLoaderLocal.EnableSystemPodMetrics %v; not supported yet", cfg.AddOnClusterLoaderLocal.EnableSystemPodMetrics)
	}

	return nil
}
