package clusterloader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/utils/file"
	utils_http "github.com/aws/aws-k8s-tester/utils/http"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

var (
	defaultClusterloaderVersion = "v1.6.1"
	defaultClusterloaderPath    = fmt.Sprintf("/tmp/clusterloader2-%s", defaultClusterloaderVersion)
	// ref. https://github.com/aws/aws-k8s-tester/releases
	// e.g. https://github.com/aws/aws-k8s-tester/releases/download/v1.6.1/clusterloader2-darwin-arm64
	defaultClusterloaderDownloadURL = fmt.Sprintf(
		"https://github.com/aws/aws-k8s-tester/releases/download/%s/clusterloader2-%s-%s",
		defaultClusterloaderVersion,
		runtime.GOOS,
		runtime.GOARCH,
	)
)

const DefaultProvider = "eks"

func DefaultClusterloaderPath() string {
	return defaultClusterloaderPath
}

func DefaultClusterloaderDownloadURL() string {
	return defaultClusterloaderDownloadURL
}

func installClusterloader(lg *zap.Logger, clusterloaderPath string, clusterloaderDownloadURL string) (err error) {
	lg.Info("mkdir", zap.String("clusterloader-path-dir", filepath.Dir(clusterloaderPath)))
	if err = os.MkdirAll(filepath.Dir(clusterloaderPath), 0700); err != nil {
		lg.Warn("could not create", zap.String("dir", filepath.Dir(clusterloaderPath)), zap.Error(err))
		return err
	}
	if !file.Exist(clusterloaderPath) {
		if clusterloaderDownloadURL == "" {
			lg.Warn("clusterloader path does not exist, clusterloader download URL empty", zap.String("clusterloader-path", clusterloaderPath))
			return fmt.Errorf("clusterloader path %q does not exist and empty clusterloader download URL", clusterloaderPath)
		}
		clusterloaderPath, _ = filepath.Abs(clusterloaderPath)
		lg.Info("downloading clusterloader", zap.String("clusterloader-path", clusterloaderPath))
		if err := utils_http.Download(lg, os.Stderr, clusterloaderDownloadURL, clusterloaderPath); err != nil {
			lg.Warn("failed to download clusterloader", zap.Error(err))
			return err
		}
	} else {
		lg.Info("skipping clusterloader download; already exist", zap.String("clusterloader-path", clusterloaderPath))
	}
	if err = file.EnsureExecutable(clusterloaderPath); err != nil {
		// file may be already executable while the process does not own the file/directory
		// ref. https://github.com/aws/aws-k8s-tester/issues/66
		lg.Warn("failed to ensure executable", zap.Error(err))
		err = nil
	}

	var output []byte
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(ctx, clusterloaderPath, "--help").CombinedOutput()
	cancel()
	out := strings.TrimSpace(string(output))
	if err != nil {
		lg.Warn("clusterloader2 --help failed", zap.Error(err))
	}
	if !strings.Contains(out, "--alsologtostderr") {
		if err == nil {
			err = fmt.Errorf("%s --help failed", clusterloaderPath)
		} else {
			err = fmt.Errorf("%v; %s --help failed", err, clusterloaderPath)
		}
	} else {
		err = nil
	}

	fmt.Fprintf(os.Stderr, "\n'%s --help' output:\n\n%s\n\n", clusterloaderPath, out)
	return err
}

type TestOverride struct {
	// Path is the test override YAML file path.
	Path string `json:"path" read-only:"true"`

	NodesPerNamespace int `json:"nodes_per_namespace"`
	PodsPerNode       int `json:"pods_per_node"`

	BigGroupSize    int `json:"big_group_size"`
	MediumGroupSize int `json:"medium_group_size"`
	SmallGroupSize  int `json:"small_group_size"`

	SmallStatefulSetsPerNamespace  int `json:"small_stateful_sets_per_namespace"`
	MediumStatefulSetsPerNamespace int `json:"medium_stateful_sets_per_namespace"`

	CL2UseHostNetworkPods           bool `json:"cl2_use_host_network_pods"`
	CL2LoadTestThroughput           int  `json:"cl2_load_test_throughput"`
	CL2EnablePVS                    bool `json:"cl2_enable_pvs"`
	CL2SchedulerThroughputThreshold int  `json:"cl2_scheduler_throughput_threshold"`
	PrometheusScrapeKubeProxy       bool `json:"prometheus_scrape_kube_proxy"`
	EnableSystemPodMetrics          bool `json:"enable_system_pod_metrics"`
}

const (
	DefaultNodesPerNamespace = 10
	DefaultPodsPerNode       = 10

	DefaultBigGroupSize    = 25
	DefaultMediumGroupSize = 10
	DefaultSmallGroupSize  = 5

	DefaultSmallStatefulSetsPerNamespace  = 0
	DefaultMediumStatefulSetsPerNamespace = 0

	DefaultCL2UseHostNetworkPods           = false
	DefaultCL2LoadTestThroughput           = 20
	DefaultCL2EnablePVS                    = false
	DefaultCL2SchedulerThroughputThreshold = 100
	DefaultPrometheusScrapeKubeProxy       = false
	DefaultEnableSystemPodMetrics          = false
)

func newDefaultTestOverride() *TestOverride {
	return &TestOverride{
		Path: DefaultTestOverridePath(),

		NodesPerNamespace: DefaultNodesPerNamespace,
		PodsPerNode:       DefaultPodsPerNode,

		BigGroupSize:    DefaultBigGroupSize,
		MediumGroupSize: DefaultMediumGroupSize,
		SmallGroupSize:  DefaultSmallGroupSize,

		SmallStatefulSetsPerNamespace:  DefaultSmallStatefulSetsPerNamespace,
		MediumStatefulSetsPerNamespace: DefaultMediumStatefulSetsPerNamespace,

		CL2UseHostNetworkPods:           DefaultCL2UseHostNetworkPods,
		CL2LoadTestThroughput:           DefaultCL2LoadTestThroughput,
		CL2EnablePVS:                    DefaultCL2EnablePVS,
		CL2SchedulerThroughputThreshold: DefaultCL2SchedulerThroughputThreshold,
		PrometheusScrapeKubeProxy:       DefaultPrometheusScrapeKubeProxy,
		EnableSystemPodMetrics:          DefaultEnableSystemPodMetrics,
	}
}

func (to *TestOverride) Sync(lg *zap.Logger) error {
	if to.Path == "" {
		to.Path = DefaultTestOverridePath()
	}

	lg.Info("writing test override file", zap.String("path", to.Path))
	buf := bytes.NewBuffer(nil)
	tpl := template.Must(template.New("templateTestOverrides").Parse(templateTestOverrides))
	if err := tpl.Execute(buf, to); err != nil {
		return err
	}

	os.RemoveAll(to.Path)
	f, err := os.Create(to.Path)
	if err != nil {
		return err
	}
	_, err = f.Write(buf.Bytes())
	f.Close()
	if err != nil {
		return err
	}

	lg.Info("wrote test override file", zap.String("path", to.Path))
	return nil
}

// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2/testing/load
// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2/testing/overrides
// ref. https://github.com/kubernetes/perf-tests/pull/1345
const templateTestOverrides = `NODES_PER_NAMESPACE: {{ .NodesPerNamespace }}
PODS_PER_NODE: {{ .PodsPerNode }}
BIG_GROUP_SIZE: {{ .BigGroupSize }}
MEDIUM_GROUP_SIZE: {{ .MediumGroupSize }}
SMALL_GROUP_SIZE: {{ .SmallGroupSize }}
SMALL_STATEFUL_SETS_PER_NAMESPACE: {{ .SmallStatefulSetsPerNamespace }}
MEDIUM_STATEFUL_SETS_PER_NAMESPACE: {{ .MediumStatefulSetsPerNamespace }}
CL2_USE_HOST_NETWORK_PODS: {{ .CL2UseHostNetworkPods }}
CL2_LOAD_TEST_THROUGHPUT: {{ .CL2LoadTestThroughput }}
CL2_ENABLE_PVS: {{ .CL2EnablePVS }}
CL2_SCHEDULER_THROUGHPUT_THRESHOLD: {{ .CL2SchedulerThroughputThreshold }}
PROMETHEUS_SCRAPE_KUBE_PROXY: {{ .PrometheusScrapeKubeProxy }}
ENABLE_SYSTEM_POD_METRICS: {{ .EnableSystemPodMetrics }}
`

func parsePodStartupLatency(fpath string) (perfData PerfData, err error) {
	rf, err := os.OpenFile(fpath, os.O_RDONLY, 0444)
	if err != nil {
		return PerfData{}, fmt.Errorf("failed to open %q (%v)", fpath, err)
	}
	defer rf.Close()
	err = json.NewDecoder(rf).Decode(&perfData)
	return perfData, err
}

func mergePodStartupLatency(datas ...PerfData) (perfData PerfData) {
	if len(datas) == 0 {
		return perfData
	}
	if len(datas) == 1 {
		return datas[0]
	}

	perfData.Labels = make(map[string]string)
	labelToUnit := make(map[string]string)
	labelToData := make(map[string]map[string]float64)

	for _, d := range datas {
		perfData.Version = d.Version
		for k, v := range d.Labels {
			perfData.Labels[k] = v
		}
		for _, cur := range d.DataItems {
			b, err := json.Marshal(cur.Labels)
			if err != nil {
				panic(err)
			}
			key := string(b)

			labelToUnit[key] = cur.Unit
			prev, ok := labelToData[key]
			if ok {
				for k, v := range prev {
					// average
					cur.Data[k] += v
					cur.Data[k] /= 2.0
				}
			}
			labelToData[key] = cur.Data
		}
	}

	for key, data := range labelToData {
		unit := labelToUnit[key]
		var labels map[string]string
		if err := json.Unmarshal([]byte(key), &labels); err != nil {
			panic(err)
		}
		perfData.DataItems = append(perfData.DataItems, DataItem{
			Data:   data,
			Labels: labels,
			Unit:   unit,
		})
	}
	return perfData
}

// Copy from:
// https://pkg.go.dev/k8s.io/perf-tests/clusterloader2/pkg/measurement/util#PerfData
type PerfData struct {
	// Version is the version of the metrics. The metrics consumer could use the version
	// to detect metrics version change and decide what version to support.
	Version   string     `json:"version"`
	DataItems []DataItem `json:"dataItems"`
	// Labels is the labels of the dataset.
	Labels map[string]string `json:"labels,omitempty"`
}

// Copy from:
// https://pkg.go.dev/k8s.io/perf-tests/clusterloader2/pkg/measurement/util#DataItem
type DataItem struct {
	// Data is a map from bucket to real data point (e.g. "Perc90" -> 23.5). Notice
	// that all data items with the same label combination should have the same buckets.
	Data map[string]float64 `json:"data"`
	// Unit is the data unit. Notice that all data items with the same label combination
	// should have the same unit.
	Unit string `json:"unit"`
	// Labels is the labels of the data item.
	Labels map[string]string `json:"labels,omitempty"`
}
