// Package clusterloader implements cluster loader.
// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2
// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2/testing/overrides
package clusterloader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	aws_s3 "github.com/aws/aws-k8s-tester/pkg/aws/s3"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/httputil"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/dustin/go-humanize"
	"github.com/mholt/archiver/v3"
	"go.uber.org/zap"
	measurement_util "k8s.io/perf-tests/clusterloader2/pkg/measurement/util"
	"k8s.io/utils/exec"
)

// Config configures cluster loader.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}

	S3API        s3iface.S3API
	S3BucketName string

	// KubeConfigPath is the kubeconfig path.
	// Optional. If empty, uses in-cluster client configuration.
	KubeConfigPath string

	// ClusterLoaderPath is the clusterloader executable binary path.
	// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2
	ClusterLoaderPath        string
	ClusterLoaderDownloadURL string
	// TestConfigPath is the clusterloader2 test configuration file.
	// Set via "--testconfig" flag.
	TestConfigPath string
	// ReportDir is the clusterloader2 test report directory.
	// Set via "--report-dir" flag.
	ReportDir string
	// ReportTarGzPath is the .tar.gz file path for report directory.
	ReportTarGzPath  string
	ReportTarGzS3Key string
	// LogPath is the log file path to stream clusterloader binary runs.
	LogPath  string
	LogS3Key string
	// PodStartupLatencyPath is the combined PodStartupLatency output path.
	PodStartupLatencyPath  string
	PodStartupLatencyS3Key string

	// Runs is the number of "clusterloader2" runs back-to-back.
	Runs    int
	Timeout time.Duration

	// Nodes is the number of nodes.
	// Set via "--nodes" flag.
	Nodes int

	//
	//
	// below are set via "--testoverrides" flag

	NodesPerNamespace int
	PodsPerNode       int

	BigGroupSize    int
	MediumGroupSize int
	SmallGroupSize  int

	SmallStatefulSetsPerNamespace  int
	MediumStatefulSetsPerNamespace int

	CL2UseHostNetworkPods     bool
	CL2LoadTestThroughput     int
	CL2EnablePVS              bool
	PrometheusScrapeKubeProxy bool
	EnableSystemPodMetrics    bool
}

// Loader defines cluster loader operations.
type Loader interface {
	// Start runs the cluster loader and waits for its completion.
	Start() error
	Stop()
}

type loader struct {
	cfg            Config
	donec          chan struct{}
	donecCloseOnce *sync.Once

	rootCtx           context.Context
	rootCancel        context.CancelFunc
	testOverridesPath string
	testLogsFile      *os.File
}

func New(cfg Config) Loader {
	return &loader{
		cfg:               cfg,
		donec:             make(chan struct{}),
		donecCloseOnce:    new(sync.Once),
		testOverridesPath: "",
	}
}

/*
DO NOT FAIL THE TEST JUST BECAUSE IT CANNOT GET METRICS

I0620 10:48:09.278149     256 simple_test_executor.go:384] Resources cleanup time: 15.009539312s
E0620 10:48:09.278189     256 clusterloader.go:219] --------------------------------------------------------------------------------
E0620 10:48:09.278193     256 clusterloader.go:220] Test Finished
E0620 10:48:09.278196     256 clusterloader.go:221]   Test: /clusterloader2-test-config.yaml
E0620 10:48:09.278199     256 clusterloader.go:222]   Status: Fail
E0620 10:48:09.278202     256 clusterloader.go:224]   Errors: [measurement call TestMetrics - TestMetrics error: [action start failed for SchedulingMetrics measurement: unexpected error (code: 0) in ssh connection to master: &errors.errorString{s:"error getting signer for provider : 'GetSigner(...) not implemented for '"}]
measurement call TestMetrics - TestMetrics error: [action gather failed for SchedulingMetrics measurement: unexpected error (code: 0) in ssh connection to master: &errors.errorString{s:"error getting signer for provider : 'GetSigner(...) not implemented for '"}]]
E0620 10:48:09.278206     256 clusterloader.go:226] --------------------------------------------------------------------------------

JUnit report was created: /data/eks-2020062010-exclusiver66-cluster-loader-local-report/junit.xml
F0620 10:48:09.278371     256 clusterloader.go:329] 1 tests have failed!


E0621 01:15:53.003734     415 test_metrics.go:226] TestMetrics: [action gather failed for SchedulingMetrics measurement: unexpected error (code: 0) in ssh connection to master: &errors.errorString{s:"error getting signer for provider : 'GetSigner(...) not implemented for '"}]
I0621 01:15:53.003760     415 simple_test_executor.go:162] Step "Collecting measurements" ended
W0621 01:15:53.003766     415 simple_test_executor.go:165] Got errors during step execution: [measurement call TestMetrics - TestMetrics error: [action gather failed for SchedulingMetrics measurement: unexpected error (code: 0) in ssh connection to master: &errors.errorString{s:"error getting signer for provider : 'GetSigner(...) not implemented for '"}]]
I0621 01:15:53.003789     415 simple_test_executor.go:72] Waiting for the chaos monkey subroutine to end...
I0621 01:15:53.003795     415 simple_test_executor.go:74] Chaos monkey ended.
I0621 01:15:53.007928     415 simple_test_executor.go:94]
{"level":"info","ts":"2020-06-21T01:16:20.231-0700","caller":"cluster-loader/cluster-loader.go:201","msg":"checked cluster loader command output from logs file","total-lines":153}
I0621 01:15:53.007938     415 probes.go:131] Probe DnsLookupLatency wasn't started, skipping the Dispose() step
I0621 01:15:53.007977     415 probes.go:131] Probe InClusterNetworkLatency wasn't started, skipping the Dispose() step
*/

const skipErr = `action gather failed for SchedulingMetrics`

// Start runs the cluster loader and waits for its completion.
func (ts *loader) Start() (err error) {
	ts.cfg.Logger.Info("starting cluster loader")

	if !fileutil.Exist(ts.cfg.TestConfigPath) {
		ts.cfg.Logger.Warn("clusterloader test config file does not exist", zap.String("path", ts.cfg.TestConfigPath))
		return fmt.Errorf("%q not found", ts.cfg.TestConfigPath)
	}

	if err = os.MkdirAll(ts.cfg.ReportDir, 0700); err != nil {
		return err
	}
	if err = fileutil.IsDirWriteable(ts.cfg.ReportDir); err != nil {
		return err
	}

	if err = ts.downloadClusterLoader(); err != nil {
		return err
	}
	if err = ts.writeTestOverrides(); err != nil {
		return err
	}

	args := []string{
		ts.cfg.ClusterLoaderPath,
		"--alsologtostderr",
		"--testconfig=" + ts.cfg.TestConfigPath,
		"--testoverrides=" + ts.testOverridesPath,
		"--report-dir=" + ts.cfg.ReportDir,
		"--nodes=" + fmt.Sprintf("%d", ts.cfg.Nodes),
		"--provider=eks",
	}
	if ts.cfg.KubeConfigPath == "" {
		// ref. https://github.com/kubernetes/perf-tests/pull/1295
		args = append(args, "--run-from-cluster=true")
	} else {
		args = append(args, "--kubeconfig="+ts.cfg.KubeConfigPath)
	}

	ts.testLogsFile, err = os.OpenFile(ts.cfg.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer func() {
		ts.testLogsFile.Sync()
		ts.testLogsFile.Close()
	}()
	// stream command run outputs for debugging purposes
	checkDonec := make(chan struct{})
	go func() {
		defer func() {
			close(checkDonec)
		}()
		for {
			select {
			case <-ts.cfg.Stopc:
				ts.cfg.Logger.Info("exiting cluster loader command output checks")
				return
			case <-ts.rootCtx.Done():
				ts.cfg.Logger.Info("exiting cluster loader command output checks")
				return
			case <-time.After(10 * time.Second):
			}

			if ts.testLogsFile != nil {
				ts.testLogsFile.Sync()
			}
			ts.cfg.Logger.Info("checking cluster loader command output from logs file")
			b, lerr := ioutil.ReadFile(ts.cfg.LogPath)
			if lerr != nil {
				ts.cfg.Logger.Warn("failed to read cluster loader command output from logs file", zap.Error(lerr))
				continue
			}
			output := strings.TrimSpace(string(b))
			lines := strings.Split(output, "\n")
			linesN := len(lines)

			ts.cfg.Logger.Info("checked cluster loader command output from logs file", zap.Int("total-lines", linesN))
			if linesN > 15 {
				output = strings.Join(lines[linesN-15:], "\n")
			}
			fmt.Printf("\n%q output:\n%s\n\n", ts.cfg.LogPath, output)
		}
	}()

	now := time.Now()
	errc := make(chan error)
	var runErr error
	ts.rootCtx, ts.rootCancel = context.WithTimeout(context.Background(), ts.cfg.Timeout)
	go func() {
		for i := 0; i < ts.cfg.Runs; i++ {
			select {
			case <-ts.rootCtx.Done():
				return
			case <-time.After(5 * time.Second):
			}

			rerr := ts.run(i, args)
			if rerr == nil {
				ts.cfg.Logger.Info("completed cluster loader", zap.Int("current-run", i), zap.Int("total-runs", ts.cfg.Runs))
				continue
			}

			ts.cfg.Logger.Warn("checking cluster loader error from log file", zap.Error(rerr))
			b, lerr := ioutil.ReadFile(ts.cfg.LogPath)
			if lerr != nil {
				ts.cfg.Logger.Warn("failed to read cluster loader command output from logs file", zap.Error(lerr))
				errc <- rerr
				return
			}
			output := strings.TrimSpace(string(b))
			lines := strings.Split(output, "\n")
			linesN := len(lines)
			if linesN > 15 {
				output = strings.Join(lines[linesN-15:], "\n")
			}

			if strings.Contains(output, skipErr) {
				ts.cfg.Logger.Warn("cluster loader failed but continue", zap.String("skip-error-message", skipErr))
				continue
			}

			errc <- rerr
			return
		}
		errc <- nil
	}()
	select {
	case <-ts.donec:
		ts.cfg.Logger.Info("done cluster loader")
	case <-ts.cfg.Stopc:
		ts.cfg.Logger.Info("stopping cluster loader")
	case <-ts.rootCtx.Done():
		ts.cfg.Logger.Info("timed out cluster loader")
	case runErr = <-errc:
		if runErr == nil {
			ts.cfg.Logger.Info("successfully ran cluster loader",
				zap.String("took", time.Since(now).String()),
				zap.Int("total-runs", ts.cfg.Runs),
			)
		} else {
			ts.cfg.Logger.Warn("failed to run cluster loader",
				zap.String("took", time.Since(now).String()),
				zap.Int("total-runs", ts.cfg.Runs),
				zap.Error(runErr),
			)
		}
	}
	ts.rootCancel()
	select {
	case <-checkDonec:
		ts.cfg.Logger.Info("confirmed exit cluster loader command output checks")
	case <-time.After(3 * time.Minute):
		ts.cfg.Logger.Warn("took too long to confirm exit cluster loader command output checks")
	}
	if runErr != nil {
		ts.cfg.Logger.Warn("failed to run cluster loader", zap.Error(runErr))
	} else {
		ts.cfg.Logger.Info("successfully ran cluster loader")
	}

	lout, lerr := ioutil.ReadFile(ts.cfg.LogPath)
	if lerr != nil {
		ts.cfg.Logger.Warn("failed to read cluster loader log output", zap.Error(lerr))
		return lerr
	}
	logOutput := string(lout)
	testFinishedCount := strings.Count(logOutput, `] Test Finished`)

	// append results in "LogPath"
	// "0777" to fix "scp: /var/log/cluster-loader-remote.log: Permission denied"
	logFile, cerr := os.OpenFile(ts.cfg.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0777)
	if cerr != nil {
		return fmt.Errorf("open(%q): %v", ts.cfg.LogPath, cerr)
	}
	defer logFile.Close()

	podStartupLats := make([]measurement_util.PerfData, 0)
	cerr = filepath.Walk(ts.cfg.ReportDir, func(path string, info os.FileInfo, ferr error) error {
		if ferr != nil {
			return ferr
		}
		if info.IsDir() {
			return nil
		}
		ts.cfg.Logger.Info("found report", zap.String("path", path))

		if strings.HasPrefix(filepath.Base(path), "PodStartupLatency_") {
			ts.cfg.Logger.Info("parsing PodStartupLatency", zap.String("path", path))
			p, perr := ParsePodStartupLatency(path)
			if perr != nil {
				ts.cfg.Logger.Warn("failed to parse PodStartupLatency", zap.String("path", path))
				return perr
			}
			ts.cfg.Logger.Info("parsed PodStartupLatency", zap.String("path", path))
			podStartupLats = append(podStartupLats, p)
		}

		if _, werr := logFile.WriteString(fmt.Sprintf("\n\n\nreport output from %q:\n\n", path)); werr != nil {
			ts.cfg.Logger.Warn("failed to write report to log file", zap.Error(werr))
			return nil
		}

		b, lerr := ioutil.ReadFile(path)
		if lerr != nil {
			ts.cfg.Logger.Warn("failed to read cluster loader command output from logs file", zap.Error(lerr))
			if _, werr := logFile.WriteString(fmt.Sprintf("failed to write %v", lerr)); werr != nil {
				ts.cfg.Logger.Warn("failed to write report to log file", zap.Error(werr))
				return nil
			}
		} else {
			if _, werr := logFile.Write(b); werr != nil {
				ts.cfg.Logger.Warn("failed to write report to log file", zap.Error(werr))
				return nil
			}
		}
		return nil
	})
	if cerr != nil {
		return cerr
	}
	podStartupLat := MergePodStartupLatency(podStartupLats...)
	podStartupLatData, derr := json.Marshal(podStartupLat)
	if derr != nil {
		ts.cfg.Logger.Warn("failed to marshal PodStartupLatency", zap.Error(derr))
		return derr
	}
	if cerr = ioutil.WriteFile(ts.cfg.PodStartupLatencyPath, podStartupLatData, 0600); cerr != nil {
		ts.cfg.Logger.Warn("failed to write PodStartupLatency", zap.Error(cerr))
		return cerr
	}
	if serr := aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.S3BucketName,
		ts.cfg.PodStartupLatencyS3Key,
		ts.cfg.PodStartupLatencyPath,
	); serr != nil {
		return serr
	}

	ts.cfg.Logger.Info("gzipping report dir", zap.String("report-dir", ts.cfg.ReportDir), zap.String("file-path", ts.cfg.ReportTarGzPath))
	if cerr = os.RemoveAll(ts.cfg.ReportTarGzPath); cerr != nil {
		ts.cfg.Logger.Warn("failed to remove temp file", zap.Error(cerr))
		return cerr
	}
	if cerr = archiver.Archive([]string{ts.cfg.ReportDir}, ts.cfg.ReportTarGzPath); cerr != nil {
		ts.cfg.Logger.Warn("archive failed", zap.Error(cerr))
		return cerr
	}
	stat, cerr := os.Stat(ts.cfg.ReportTarGzPath)
	if cerr != nil {
		ts.cfg.Logger.Warn("failed to os stat", zap.Error(cerr))
		return cerr
	}
	sz := humanize.Bytes(uint64(stat.Size()))
	ts.cfg.Logger.Info("gzipped report dir", zap.String("report-dir", ts.cfg.ReportDir), zap.String("file-path", ts.cfg.ReportTarGzPath), zap.String("file-size", sz))
	if serr := aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.S3BucketName,
		ts.cfg.ReportTarGzS3Key,
		ts.cfg.ReportTarGzPath,
	); serr != nil {
		return serr
	}
	if serr := aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.S3BucketName,
		ts.cfg.LogS3Key,
		ts.cfg.LogPath,
	); serr != nil {
		return serr
	}

	if testFinishedCount == ts.cfg.Runs {
		ts.cfg.Logger.Info("completed expected test runs; overriding error",
			zap.Int("finished-count", testFinishedCount),
			zap.Int("expected-runs", ts.cfg.Runs),
			zap.Error(runErr),
		)
		runErr = nil
	} else {
		ts.cfg.Logger.Warn("failed to complete expected test runs",
			zap.Int("finished-count", testFinishedCount),
			zap.Int("expected-runs", ts.cfg.Runs),
			zap.Error(runErr),
		)
		completeErr := fmt.Errorf("failed to complete expected test runs [expected %d, completed %d]", ts.cfg.Runs, testFinishedCount)
		if runErr == nil {
			runErr = completeErr
		} else {
			runErr = fmt.Errorf("%v (run error: %v)", completeErr, runErr)
		}
	}
	return runErr
}

func (ld *loader) Stop() {
	ld.cfg.Logger.Info("stopping and waiting for cluster loader")
	ld.donecCloseOnce.Do(func() {
		close(ld.donec)
	})
	ld.cfg.Logger.Info("stopped and waited for cluster loader")
}

func (ld *loader) downloadClusterLoader() (err error) {
	ld.cfg.Logger.Info("mkdir", zap.String("clusterloader-path-dir", filepath.Dir(ld.cfg.ClusterLoaderPath)))
	if err = os.MkdirAll(filepath.Dir(ld.cfg.ClusterLoaderPath), 0700); err != nil {
		return fmt.Errorf("could not create %q (%v)", filepath.Dir(ld.cfg.ClusterLoaderPath), err)
	}
	if !fileutil.Exist(ld.cfg.ClusterLoaderPath) {
		if ld.cfg.ClusterLoaderDownloadURL == "" {
			return fmt.Errorf("%q does not exist but no download URL", ld.cfg.ClusterLoaderPath)
		}
		ld.cfg.ClusterLoaderPath, _ = filepath.Abs(ld.cfg.ClusterLoaderPath)
		ld.cfg.Logger.Info("downloading clusterloader", zap.String("clusterloader-path", ld.cfg.ClusterLoaderPath))
		if err = httputil.Download(ld.cfg.Logger, os.Stderr, ld.cfg.ClusterLoaderDownloadURL, ld.cfg.ClusterLoaderPath); err != nil {
			return err
		}
	} else {
		ld.cfg.Logger.Info("skipping clusterloader download; already exist", zap.String("clusterloader-path", ld.cfg.ClusterLoaderPath))
	}
	if err = fileutil.EnsureExecutable(ld.cfg.ClusterLoaderPath); err != nil {
		// file may be already executable while the process does not own the file/directory
		// ref. https://github.com/aws/aws-k8s-tester/issues/66
		ld.cfg.Logger.Warn("failed to ensure executable", zap.Error(err))
		err = nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, herr := exec.New().CommandContext(
		ctx,
		ld.cfg.ClusterLoaderPath,
		"--help",
	).CombinedOutput()
	cancel()
	out := strings.TrimSpace(string(output))
	fmt.Printf("'%s --help' output:\n\n%s\n(error: %v)\n\n", ld.cfg.ClusterLoaderPath, out, herr)
	if !strings.Contains(out, "--alsologtostderr") {
		if err == nil {
			err = fmt.Errorf("%s --help failed", ld.cfg.ClusterLoaderPath)
		} else {
			err = fmt.Errorf("%v; %s --help failed", err, ld.cfg.ClusterLoaderPath)
		}
	}

	return err
}

func (ld *loader) writeTestOverrides() (err error) {
	buf := bytes.NewBuffer(nil)
	tpl := template.Must(template.New("TemplateTestOverrides").Parse(TemplateTestOverrides))
	if err := tpl.Execute(buf, ld.cfg); err != nil {
		return err
	}

	fmt.Printf("test overrides configuration:\n\n%s\n\n", buf.String())

	ld.testOverridesPath, err = fileutil.WriteTempFile(buf.Bytes())
	if err != nil {
		ld.cfg.Logger.Warn("failed to write", zap.Error(err))
		return err
	}

	ld.cfg.Logger.Info("wrote test overrides file", zap.String("path", ld.testOverridesPath))
	return nil
}

// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2/testing/load
// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2/testing/overrides
// ref. https://github.com/kubernetes/perf-tests/pull/1345
const TemplateTestOverrides = `NODES_PER_NAMESPACE: {{ .NodesPerNamespace }}
PODS_PER_NODE: {{ .PodsPerNode }}
BIG_GROUP_SIZE: {{ .BigGroupSize }}
MEDIUM_GROUP_SIZE: {{ .MediumGroupSize }}
SMALL_GROUP_SIZE: {{ .SmallGroupSize }}
SMALL_STATEFUL_SETS_PER_NAMESPACE: {{ .SmallStatefulSetsPerNamespace }}
MEDIUM_STATEFUL_SETS_PER_NAMESPACE: {{ .MediumStatefulSetsPerNamespace }}
CL2_USE_HOST_NETWORK_PODS: {{ .CL2UseHostNetworkPods }}
CL2_LOAD_TEST_THROUGHPUT: {{ .CL2LoadTestThroughput }}
CL2_ENABLE_PVS: {{ .CL2EnablePVS }}
PROMETHEUS_SCRAPE_KUBE_PROXY: {{ .PrometheusScrapeKubeProxy }}
ENABLE_SYSTEM_POD_METRICS: {{ .EnableSystemPodMetrics }}
`

// takes about 2-minute
func (ld *loader) run(idx int, args []string) (err error) {
	ld.cfg.Logger.Info("running cluster loader", zap.Int("index", idx), zap.String("command", strings.Join(args, " ")))
	ctx, cancel := context.WithTimeout(ld.rootCtx, 20*time.Minute)
	cmd := exec.New().CommandContext(ctx, args[0], args[1:]...)
	cmd.SetStderr(ld.testLogsFile)
	cmd.SetStdout(ld.testLogsFile)
	err = cmd.Run()
	cancel()
	return err
}
