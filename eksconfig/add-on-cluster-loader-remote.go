package eksconfig

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

/*
Note: make sure all other test config is copied in the "same" directory as "--testconfig" (in Dockerfile)

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

// AddOnClusterLoaderRemote defines parameters for EKS cluster
// add-on cluster loader remote.
// It generates loads from the remote host machine.
// ref. https://github.com/kubernetes/perf-tests/pull/1295
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

	// RepositoryAccountID is the account ID for tester ECR image.
	// e.g. "aws/aws-k8s-tester" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/aws/aws-k8s-tester"
	RepositoryAccountID string `json:"repository-account-id,omitempty"`
	// RepositoryName is the repositoryName for tester ECR image.
	// e.g. "aws/aws-k8s-tester" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/aws/aws-k8s-tester"
	RepositoryName string `json:"repository-name,omitempty"`
	// RepositoryImageTag is the image tag for tester ECR image.
	// e.g. "latest" for image URI "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/aws/aws-k8s-tester:latest"
	RepositoryImageTag string `json:"repository-image-tag,omitempty"`

	// DeploymentReplicas is the number of "clusterloader2" Pods to run.
	// For now, only 1 is supported.
	DeploymentReplicas int32 `json:"deployment-replicas" read-only:"true"`

	// ClusterLoaderPath is the clusterloader executable binary path.
	// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2
	ClusterLoaderPath        string `json:"cluster-loader-path"`
	ClusterLoaderDownloadURL string `json:"cluster-loader-download-url"`

	// ReportTarGzPath is the .tar.gz file path for report directory.
	// This is the local path after downloaded from remote nodes.
	ReportTarGzPath string `json:"report-tar-gz-path" read-only:"true"`
	// LogPath is the log file path to stream clusterloader binary runs.
	LogPath string `json:"log-path" read-only:"true"`

	// Runs is the number of "clusterloader2" runs back-to-back.
	Runs int `json:"runs"`

	// Nodes is the number of nodes.
	// Set via "--nodes" flag.
	Nodes int `json:"nodes"`
	// Timeout is the timeout for the total test runs.
	Timeout time.Duration `json:"timeout"`

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
		ClusterLoaderDownloadURL: "https://github.com/aws/aws-k8s-tester/releases/download/v1.2.6/clusterloader2-linux-amd64",

		Runs:               1,
		DeploymentReplicas: 1,
		Timeout:            30 * time.Minute,

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

func (cfg *Config) validateAddOnClusterLoaderRemote() error {
	if !cfg.IsEnabledAddOnClusterLoaderRemote() {
		return nil
	}

	if cfg.AddOnClusterLoaderRemote.Namespace == "" {
		cfg.AddOnClusterLoaderRemote.Namespace = cfg.Name + "-cluster-loader-remote"
	}

	if cfg.AddOnClusterLoaderRemote.RepositoryAccountID == "" {
		return errors.New("AddOnClusterLoaderRemote.RepositoryAccountID empty")
	}
	if cfg.AddOnClusterLoaderRemote.RepositoryName == "" {
		return errors.New("AddOnClusterLoaderRemote.RepositoryName empty")
	}
	if cfg.AddOnClusterLoaderRemote.RepositoryImageTag == "" {
		return errors.New("AddOnClusterLoaderRemote.RepositoryImageTag empty")
	}

	if cfg.AddOnClusterLoaderRemote.ReportTarGzPath == "" {
		cfg.AddOnClusterLoaderRemote.ReportTarGzPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-cluster-loader-remote.tar.gz"
	}
	if cfg.AddOnClusterLoaderRemote.LogPath == "" {
		cfg.AddOnClusterLoaderRemote.LogPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-cluster-loader-remote.log"
	}

	if cfg.AddOnClusterLoaderRemote.DeploymentReplicas != 1 {
		return fmt.Errorf("unexpected AddOnClusterLoaderRemote.DeploymentReplicas %d", cfg.AddOnClusterLoaderRemote.DeploymentReplicas)
	}

	if cfg.AddOnClusterLoaderRemote.ClusterLoaderPath == "" && cfg.AddOnClusterLoaderRemote.ClusterLoaderDownloadURL == "" {
		return errors.New("empty AddOnClusterLoaderRemote.ClusterLoaderPath and ClusterLoaderDownloadURL")
	}

	if cfg.AddOnClusterLoaderRemote.Runs == 0 {
		return errors.New("unexpected zero AddOnClusterLoaderRemote.Runs")
	}
	if cfg.AddOnClusterLoaderRemote.Timeout == 0 {
		return errors.New("unexpected zero AddOnClusterLoaderRemote.Timeout")
	}

	if cfg.AddOnClusterLoaderRemote.Nodes == 0 {
		return errors.New("unexpected zero AddOnClusterLoaderRemote.Nodes")
	}

	if cfg.AddOnClusterLoaderLocal.CL2LoadTestThroughput == 0 {
		// ref. https://github.com/kubernetes/perf-tests/blob/master/clusterloader2/testing/load/kubemark/throughput_override.yaml
		cfg.AddOnClusterLoaderLocal.CL2LoadTestThroughput = 20
	}
	if cfg.AddOnClusterLoaderRemote.PrometheusScrapeKubeProxy {
		return fmt.Errorf("unexpected AddOnClusterLoaderRemote.PrometheusScrapeKubeProxy %v; not supported yet", cfg.AddOnClusterLoaderRemote.PrometheusScrapeKubeProxy)
	}
	if cfg.AddOnClusterLoaderRemote.EnableSystemPodMetrics {
		return fmt.Errorf("unexpected AddOnClusterLoaderRemote.EnableSystemPodMetrics %v; not supported yet", cfg.AddOnClusterLoaderRemote.EnableSystemPodMetrics)
	}

	return nil
}
