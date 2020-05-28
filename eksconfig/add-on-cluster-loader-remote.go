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
	cfg := &AddOnClusterLoaderRemote{
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

func (cfg *Config) validateAddOnClusterLoaderRemote() error {
	if !cfg.IsEnabledAddOnClusterLoaderRemote() {
		return nil
	}

	if cfg.AddOnClusterLoaderRemote.Namespace == "" {
		cfg.AddOnClusterLoaderRemote.Namespace = cfg.Name + "-cluster-loader-remote"
	}

	if cfg.AddOnClusterLoaderRemote.ClusterLoaderPath == "" && cfg.AddOnClusterLoaderRemote.ClusterLoaderDownloadURL == "" {
		return errors.New("empty AddOnClusterLoaderRemote.ClusterLoaderPath and ClusterLoaderDownloadURL")
	}
	if cfg.AddOnClusterLoaderRemote.ClusterLoaderTestConfigPath == "" {
		return errors.New("empty AddOnClusterLoaderRemote.ClusterLoaderTestConfigPath")
	}
	if cfg.AddOnClusterLoaderRemote.ClusterLoaderReportDir == "" {
		cfg.AddOnClusterLoaderRemote.ClusterLoaderReportDir = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-cluster-loader-remote-report")
	}
	if err := fileutil.IsDirWriteable(cfg.AddOnClusterLoaderRemote.ClusterLoaderReportDir); err != nil {
		return err
	}

	if cfg.AddOnClusterLoaderRemote.Runs == 0 {
		return errors.New("unexpected zero AddOnClusterLoaderRemote.Runs")
	}

	if cfg.AddOnClusterLoaderRemote.Nodes == 0 {
		return errors.New("unexpected zero AddOnClusterLoaderRemote.Nodes")
	}

	if cfg.AddOnClusterLoaderRemote.CL2EnablePVS {
		return fmt.Errorf("unexpected AddOnClusterLoaderRemote.CL2EnablePVS %v; not supported yet", cfg.AddOnClusterLoaderRemote.CL2EnablePVS)
	}
	if cfg.AddOnClusterLoaderRemote.PrometheusScrapeKubeProxy {
		return fmt.Errorf("unexpected AddOnClusterLoaderRemote.PrometheusScrapeKubeProxy %v; not supported yet", cfg.AddOnClusterLoaderRemote.PrometheusScrapeKubeProxy)
	}
	if cfg.AddOnClusterLoaderRemote.EnableSystemPodMetrics {
		return fmt.Errorf("unexpected AddOnClusterLoaderRemote.EnableSystemPodMetrics %v; not supported yet", cfg.AddOnClusterLoaderRemote.EnableSystemPodMetrics)
	}

	return nil
}
