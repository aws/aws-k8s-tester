package eksconfig

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	measurement_util "k8s.io/perf-tests/clusterloader2/pkg/measurement/util"
)

/*
Note: make sure all other test config is copied in the "same" directory as "--testconfig" (in local)

"/var/log/cluster-loader-remote.log" output:
I0529 18:59:08.745755      27 simple_test_executor.go:162] Step "Scaling and updating objects" ended
W0529 18:59:08.745762      27 simple_test_executor.go:165] Got errors during step execution: [reading template (job.yaml) for identifier error: reading error: open /job.yaml: no such file or directory
reading template (statefulset.yaml) for identifier error: reading error: open /statefulset.yaml: no such file or directory
reading template (daemonset.yaml) for identifier error: reading error: open /daemonset.yaml: no such file or directory
reading template (deployment.yaml) for identifier error: reading error: open /deployment.yaml: no such file or directory
reading template (statefulset.yaml) for identifier error: reading error: open /statefulset.yaml: no such file or directory
reading template (deployment.yaml) for identifier error: reading error: open /deployment.yaml: no such file or directory
reading template (deployment.yaml) for identifier error: reading error: open /deployment.yaml: no such file or directory
reading template (job.yaml) for identifier error: reading error: open /job.yaml: no such file or directory
reading template (job.yaml) for identifier error: reading error: open /job.yaml: no such file or directory]
I0529 18:59:08.745802      27 simple_test_executor.go:135] Step "Waiting for objects to become scaled" started
*/

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

	// S3Dir is the S3 directory to store all test results.
	// It is under the bucket "eksconfig.Config.S3BucketName".
	S3Dir string `json:"s3-dir"`

	// ClusterLoaderPath is the clusterloader executable binary path.
	// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2
	ClusterLoaderPath        string `json:"cluster-loader-path"`
	ClusterLoaderDownloadURL string `json:"cluster-loader-download-url"`
	// TestConfigPath is the clusterloader2 test configuration file.
	// Set via "--testconfig" flag.
	TestConfigPath string `json:"test-config-path"`

	// ReportDir is the clusterloader2 test report directory.
	// Set via "--report-dir" flag.
	ReportDir string `json:"report-dir"`

	// ReportTarGzPath is the .tar.gz file path for report directory.
	ReportTarGzPath  string `json:"report-tar-gz-path" read-only:"true"`
	ReportTarGzS3Key string `json:"report-tar-gz-s3-key" read-only:"true"`
	// LogPath is the log file path to stream clusterloader binary runs.
	LogPath  string `json:"log-path" read-only:"true"`
	LogS3Key string `json:"log-s3-key" read-only:"true"`
	// PodStartupLatencyPath is the JSON file path to store pod startup latency.
	PodStartupLatencyPath  string `json:"pod-startup-latency-path" read-only:"true"`
	PodStartupLatencyS3Key string `json:"pod-startup-latency-s3-key" read-only:"true"`

	// Runs is the number of "clusterloader2" runs back-to-back.
	Runs int `json:"runs"`
	// Timeout is the timeout for the total test runs.
	Timeout time.Duration `json:"timeout"`

	// Nodes is the number of nodes.
	// Set via "--nodes" flag.
	Nodes int `json:"nodes"`

	//
	//
	// below are set via "--testoverrides" flag
	// see https://github.com/kubernetes/perf-tests/tree/master/clusterloader2/testing/overrides for more.

	NodesPerNamespace int `json:"nodes-per-namespace"`
	PodsPerNode       int `json:"pods-per-node"`

	BigGroupSize    int `json:"big-group-size"`
	MediumGroupSize int `json:"medium-group-size"`
	SmallGroupSize  int `json:"small-group-size"`

	SmallStatefulSetsPerNamespace  int `json:"small-stateful-sets-per-namespace"`
	MediumStatefulSetsPerNamespace int `json:"medium-stateful-sets-per-namespace"`

	CL2LoadTestThroughput     int  `json:"cl2-load-test-throughput"`
	CL2EnablePVS              bool `json:"cl2-enable-pvs"`
	PrometheusScrapeKubeProxy bool `json:"prometheus-scrape-kube-proxy"`
	EnableSystemPodMetrics    bool `json:"enable-system-pod-metrics"`

	PodStartupLatency measurement_util.PerfData `json:"pod-startup-latency" read-only:"true"`
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
		ClusterLoaderDownloadURL: "https://github.com/aws/aws-k8s-tester/releases/download/v1.3.9/clusterloader2-linux-amd64",

		Runs:    2,
		Timeout: 30 * time.Minute,

		Nodes: 10,

		NodesPerNamespace: 10,
		PodsPerNode:       10,

		BigGroupSize:    25,
		MediumGroupSize: 10,
		SmallGroupSize:  5,

		SmallStatefulSetsPerNamespace:  0,
		MediumStatefulSetsPerNamespace: 0,

		// ref. https://github.com/kubernetes/perf-tests/blob/master/clusterloader2/testing/load/kubemark/throughput_override.yaml
		CL2LoadTestThroughput:     20,
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

	if cfg.AddOnClusterLoaderLocal.S3Dir == "" {
		cfg.AddOnClusterLoaderLocal.S3Dir = path.Join(cfg.Name, "add-on-cluster-loader-local")
	}

	if cfg.AddOnClusterLoaderLocal.ReportTarGzPath == "" {
		cfg.AddOnClusterLoaderLocal.ReportTarGzPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-cluster-loader-local.tar.gz"
	}
	if cfg.AddOnClusterLoaderLocal.ReportTarGzS3Key == "" {
		cfg.AddOnClusterLoaderLocal.ReportTarGzS3Key = path.Join(
			cfg.AddOnClusterLoaderLocal.S3Dir,
			filepath.Base(cfg.AddOnClusterLoaderLocal.ReportTarGzPath),
		)
	}
	if cfg.AddOnClusterLoaderLocal.LogPath == "" {
		cfg.AddOnClusterLoaderLocal.LogPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-cluster-loader-local.log"
	}
	if cfg.AddOnClusterLoaderLocal.LogS3Key == "" {
		cfg.AddOnClusterLoaderLocal.LogS3Key = path.Join(
			cfg.AddOnClusterLoaderLocal.S3Dir,
			filepath.Base(cfg.AddOnClusterLoaderLocal.LogPath),
		)
	}
	if cfg.AddOnClusterLoaderLocal.PodStartupLatencyPath == "" {
		cfg.AddOnClusterLoaderLocal.PodStartupLatencyPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-cluster-loader-remote.log"
	}
	if cfg.AddOnClusterLoaderLocal.PodStartupLatencyS3Key == "" {
		cfg.AddOnClusterLoaderLocal.PodStartupLatencyS3Key = path.Join(
			cfg.AddOnClusterLoaderLocal.S3Dir,
			filepath.Base(cfg.AddOnClusterLoaderLocal.PodStartupLatencyPath),
		)
	}

	if cfg.AddOnClusterLoaderLocal.ClusterLoaderPath == "" && cfg.AddOnClusterLoaderLocal.ClusterLoaderDownloadURL == "" {
		return errors.New("empty AddOnClusterLoaderLocal.ClusterLoaderPath and ClusterLoaderDownloadURL")
	}
	if cfg.AddOnClusterLoaderLocal.TestConfigPath == "" {
		return errors.New("empty AddOnClusterLoaderLocal.TestConfigPath")
	}
	if cfg.AddOnClusterLoaderLocal.ReportDir == "" {
		cfg.AddOnClusterLoaderLocal.ReportDir = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-cluster-loader-local-report")
	}
	if err := fileutil.IsDirWriteable(cfg.AddOnClusterLoaderLocal.ReportDir); err != nil {
		return err
	}

	if cfg.AddOnClusterLoaderLocal.Runs == 0 {
		return errors.New("unexpected zero AddOnClusterLoaderLocal.Runs")
	}
	if cfg.AddOnClusterLoaderLocal.Timeout == 0 {
		return errors.New("unexpected zero AddOnClusterLoaderLocal.Timeout")
	}

	if cfg.AddOnClusterLoaderLocal.Nodes == 0 {
		return errors.New("unexpected zero AddOnClusterLoaderLocal.Nodes")
	}

	if cfg.AddOnClusterLoaderLocal.CL2LoadTestThroughput == 0 {
		// ref. https://github.com/kubernetes/perf-tests/blob/master/clusterloader2/testing/load/kubemark/throughput_override.yaml
		cfg.AddOnClusterLoaderLocal.CL2LoadTestThroughput = 20
	}
	if cfg.AddOnClusterLoaderLocal.PrometheusScrapeKubeProxy {
		return fmt.Errorf("unexpected AddOnClusterLoaderLocal.PrometheusScrapeKubeProxy %v; not supported yet", cfg.AddOnClusterLoaderLocal.PrometheusScrapeKubeProxy)
	}
	if cfg.AddOnClusterLoaderLocal.EnableSystemPodMetrics {
		return fmt.Errorf("unexpected AddOnClusterLoaderLocal.EnableSystemPodMetrics %v; not supported yet", cfg.AddOnClusterLoaderLocal.EnableSystemPodMetrics)
	}

	return nil
}
