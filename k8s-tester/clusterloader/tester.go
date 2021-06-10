// Package clusterloader installs clusterloader.
// Replace https://github.com/aws/aws-k8s-tester/tree/v1.5.9/eks/cluster-loader.
package clusterloader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester/tester"
	"github.com/aws/aws-k8s-tester/utils/file"
	"github.com/dustin/go-humanize"
	"github.com/manifoldco/promptui"
	"github.com/mholt/archiver/v3"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

// TODO: support s3 uploads

// Config defines parameters for Kubernetes clusterloader tests.
type Config struct {
	Enable bool `json:"enable"`
	Prompt bool `json:"-"`

	Stopc     chan struct{} `json:"-"`
	Logger    *zap.Logger   `json:"-"`
	LogWriter io.Writer     `json:"-"`
	Client    client.Client `json:"-"`

	// MinimumNodes is the minimum number of Kubernetes nodes required for installing this addon.
	MinimumNodes int `json:"minimum_nodes"`

	// ClusterloaderPath is the path to download the "clusterloader".
	ClusterloaderPath string `json:"clusterloader_path"`
	// ClusterloaderDownloadURL is the download URL to download "clusterloader" binary from.
	ClusterloaderDownloadURL string `json:"clusterloader_download_url"`

	// Provider is the provider name for "clusterloader2".
	Provider string `json:"provider"`

	// Runs is the number of "clusterloader2" runs back-to-back.
	Runs int `json:"runs"`
	// RunTimeout is the timeout for the total test runs.
	RunTimeout       time.Duration `json:"run_timeout"`
	RunTimeoutString string        `json:"run_timeout_string" read-only:"true"`

	// TestConfigPath is the clusterloader2 test configuration file.
	// Must be located along with other configuration files.
	// For instance, if the clusterloader2 default configuration file is located at
	// ${HOME}/go/src/k8s.io/perf-tests/clusterloader2/testing/load/config.yaml,
	// then run this tester from "${HOME}/go/src/k8s.io/perf-tests/clusterloader2".
	// ref. https://github.com/kubernetes/perf-tests/blob/master/clusterloader2/testing/load/config.yaml
	// Set via "--testconfig" flag.
	TestConfigPath string `json:"test_config_path"`

	// RunFromCluster is set 'true' to override KUBECONFIG set in "Client" field.
	// If "false", instead pass Client.Config().KubeconfigPath to "--kubeconfig" flag.
	// Set via "--run-from-cluster" flag.
	// ref. https://github.com/kubernetes/perf-tests/pull/1295
	RunFromCluster bool `json:"run_from_cluster"`
	// Nodes is the number of nodes.
	// Set via "--nodes" flag.
	Nodes int `json:"nodes"`
	// EnableExecService is set to "true" to allow executing arbitrary commands from a pod running in the cluster.
	// Set via "--enable-exec-service" flag.
	// ref. https://github.com/kubernetes/perf-tests/blob/master/clusterloader2/cmd/clusterloader.go#L120
	EnableExecService bool `json:"enable_exec_service"`

	// TestOverride defines "testoverrides" flag values.
	// Set via "--testoverrides" flag.
	// See https://github.com/kubernetes/perf-tests/tree/master/clusterloader2/testing/overrides for more.
	// ref. https://github.com/kubernetes/perf-tests/pull/1345
	TestOverride *TestOverride `json:"test_override"`

	// TestReportDir is the clusterloader2 test report output directory.
	// Set via "--report-dir" flag.
	TestReportDir string `json:"test_report_dir" read-only:"true"`
	// TestReportDirTarGzPath is the test report .tar.gz file path.
	TestReportDirTarGzPath string `json:"test_report_dir_tar_gz_path" read-only:"true"`
	// TestLogPath is the "clusterloader2" test log file path.
	TestLogPath string `json:"test_log_path" read-only:"true"`
	// PodStartupLatency is the result of clusterloader runs.
	PodStartupLatency PerfData `json:"pod_startup_latency" read-only:"true"`
	// PodStartupLatencyPath is the JSON file path to store pod startup latency.
	PodStartupLatencyPath string `json:"pod_startup_latency_path" read-only:"true"`
}

func (cfg *Config) ValidateAndSetDefaults() error {
	if cfg.ClusterloaderPath == "" {
		cfg.ClusterloaderPath = DefaultClusterloaderPath()
	}
	if cfg.ClusterloaderDownloadURL == "" {
		cfg.ClusterloaderDownloadURL = DefaultClusterloaderDownloadURL()
	}

	if cfg.Runs == 0 {
		return fmt.Errorf("invalid Runs %d", cfg.Runs)
	}
	if cfg.RunTimeout == time.Duration(0) {
		cfg.RunTimeout = DefaultRunTimeout
	}
	cfg.RunTimeoutString = cfg.RunTimeout.String()

	if !file.Exist(cfg.TestConfigPath) {
		return fmt.Errorf("TestConfigPath %q does not exist", cfg.TestConfigPath)
	}

	if cfg.Nodes == 0 {
		cfg.Nodes = cfg.MinimumNodes
	}

	if cfg.TestReportDir == "" {
		cfg.TestReportDir = DefaultTestReportDir()
	}
	if cfg.TestReportDirTarGzPath == "" {
		cfg.TestReportDirTarGzPath = DefaultTestReportDirTarGzPath()
	}
	if !strings.HasSuffix(cfg.TestReportDirTarGzPath, ".tar.gz") {
		return fmt.Errorf("TestReportDirTarGzPath %q requires .tar.gz suffix", cfg.TestReportDirTarGzPath)
	}
	if cfg.TestLogPath == "" {
		cfg.TestLogPath = DefaultTestLogPath()
	}
	if cfg.PodStartupLatencyPath == "" {
		cfg.PodStartupLatencyPath = DefaultPodStartupLatencyPath()
	}

	return nil
}

var (
	unixNano                      = time.Now().UnixNano()
	defaultTestReportDir          = filepath.Join(os.TempDir(), fmt.Sprintf("clusterloader-test-report-dir-%x", unixNano))
	defaultTestReportDirTarGzPath = filepath.Join(os.TempDir(), fmt.Sprintf("clusterloader-test-report-dir-%x.tar.gz", unixNano))
	defaultTestOverridePath       = filepath.Join(defaultTestReportDir, fmt.Sprintf("clusterloader-test-overrides-%x.yaml", unixNano))
	defaultTestLogPath            = filepath.Join(defaultTestReportDir, fmt.Sprintf("clusterloader-test-log-%x.log", unixNano))
	defaultPodStartupLatencyPath  = filepath.Join(defaultTestReportDir, fmt.Sprintf("clusterloader-pod-startup-latency-%x.json", unixNano))
)

func DefaultTestOverridePath() string {
	return defaultTestOverridePath
}

func DefaultTestReportDir() string {
	return defaultTestReportDir
}

func DefaultTestReportDirTarGzPath() string {
	return defaultTestReportDirTarGzPath
}

func DefaultTestLogPath() string {
	return defaultTestLogPath
}

func DefaultPodStartupLatencyPath() string {
	return defaultPodStartupLatencyPath
}

const (
	DefaultMinimumNodes int = 1

	DefaultRuns       = 2
	DefaultRunTimeout = 30 * time.Minute

	DefaultRunFromCluster    = false
	DefaultNodes             = 10
	DefaultEnableExecService = false
)

func NewDefault() *Config {
	return &Config{
		Enable:       false,
		Prompt:       false,
		MinimumNodes: DefaultMinimumNodes,

		ClusterloaderPath:        DefaultClusterloaderPath(),
		ClusterloaderDownloadURL: DefaultClusterloaderDownloadURL(),

		Provider: DefaultProvider,

		Runs:       DefaultRuns,
		RunTimeout: DefaultRunTimeout,

		RunFromCluster:    DefaultRunFromCluster,
		Nodes:             DefaultNodes,
		EnableExecService: DefaultEnableExecService,

		TestOverride: newDefaultTestOverride(),

		TestReportDir:          DefaultTestReportDir(),
		TestReportDirTarGzPath: DefaultTestReportDirTarGzPath(),
		TestLogPath:            DefaultTestLogPath(),
		PodStartupLatencyPath:  DefaultPodStartupLatencyPath(),
	}
}

func New(cfg *Config) k8s_tester.Tester {
	return &tester{
		cfg: cfg,

		donec:          make(chan struct{}),
		donecCloseOnce: new(sync.Once),
	}
}

type tester struct {
	cfg         *Config
	testLogFile *os.File

	donec          chan struct{}
	donecCloseOnce *sync.Once

	rootCtx    context.Context
	rootCancel context.CancelFunc
}

var pkgName = path.Base(reflect.TypeOf(tester{}).PkgPath())

func Env() string {
	return "ADD_ON_" + strings.ToUpper(strings.Replace(pkgName, "-", "_", -1))
}

func EnvTestOverride() string {
	return Env() + "_TEST_OVERRIDE"
}

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Enabled() bool { return ts.cfg.Enable }

func (ts *tester) Apply() (err error) {
	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}

	if nodes, err := client.ListNodes(ts.cfg.Client.KubernetesClient()); len(nodes) < ts.cfg.MinimumNodes || err != nil {
		return fmt.Errorf("failed to validate minimum nodes requirement %d (nodes %v, error %v)", ts.cfg.MinimumNodes, len(nodes), err)
	}

	if err = installClusterloader(ts.cfg.Logger, ts.cfg.ClusterloaderPath, ts.cfg.ClusterloaderDownloadURL); err != nil {
		return err
	}

	if err = os.MkdirAll(ts.cfg.TestReportDir, 0700); err != nil {
		return err
	}
	if err = file.IsDirWriteable(ts.cfg.TestReportDir); err != nil {
		return err
	}
	ts.cfg.Logger.Info("mkdir report dir", zap.String("dir", ts.cfg.TestReportDir))

	if err := ts.cfg.TestOverride.Sync(ts.cfg.Logger); err != nil {
		return err
	}

	ts.testLogFile, err = os.OpenFile(ts.cfg.TestLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer func() {
		ts.testLogFile.Sync()
		ts.testLogFile.Close()
	}()

	checkDonec := ts.streamTestLogs()
	runErr := ts.runCL2s(checkDonec)

	testFinishedCount, err := ts.countTestFinishes()
	if err != nil {
		return err
	}

	podStartupLats, err := ts.appendResultsToTestLogPath()
	if err != nil {
		return err
	}
	if err = ts.collectPodStartupLatency(podStartupLats); err != nil {
		return err
	}

	if err = ts.compressReports(); err != nil {
		return err
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

func (ts *tester) Delete() error {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}

	var errs []string

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	return nil
}

func (ts *tester) runPrompt(action string) (ok bool) {
	if ts.cfg.Prompt {
		msg := fmt.Sprintf("Ready to %q resources, should we continue?", action)
		prompt := promptui.Select{
			Label: msg,
			Items: []string{
				"No, cancel it!",
				fmt.Sprintf("Yes, let's %q!", action),
			},
		}
		idx, answer, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx != 1 {
			fmt.Printf("cancelled %q [index %d, answer %q]\n", action, idx, answer)
			return false
		}
	}
	return true
}

// stream command run outputs for debugging purposes
func (ts *tester) streamTestLogs() (checkDonec chan struct{}) {
	checkDonec = make(chan struct{})
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

			if ts.testLogFile != nil {
				ts.testLogFile.Sync()
			}
			b, lerr := ioutil.ReadFile(ts.cfg.TestLogPath)
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
			fmt.Fprintf(ts.cfg.LogWriter, "\n%q output:\n%s\n\n", ts.cfg.TestLogPath, output)
		}
	}()
	return checkDonec
}

func (ts *tester) getCL2Args() (args []string) {
	args = []string{
		ts.cfg.ClusterloaderPath,
		"--logtostderr",     // log to standard error instead of files (default true)
		"--alsologtostderr", // log to standard error as well as files
		fmt.Sprintf("--enable-exec-service=%v", ts.cfg.EnableExecService),
		"--testconfig=" + ts.cfg.TestConfigPath,
		"--testoverrides=" + ts.cfg.TestOverride.Path,
		"--report-dir=" + ts.cfg.TestReportDir,
		"--nodes=" + fmt.Sprintf("%d", ts.cfg.Nodes),
		"--provider=" + ts.cfg.Provider,
	}
	if ts.cfg.RunFromCluster {
		// ref. https://github.com/kubernetes/perf-tests/pull/1295
		args = append(args, "--run-from-cluster=true")
	} else {
		args = append(args, "--kubeconfig="+ts.cfg.Client.Config().KubeconfigPath)
	}
	return args
}

/*
E0610 03:20:23.917606   16894 simple_test_executor.go:391] Resource cleanup error: [timed out waiting for the condition
timed out waiting for the condition]
I0610 03:20:23.917636   16894 clusterloader.go:227] --------------------------------------------------------------------------------
I0610 03:20:23.917650   16894 clusterloader.go:228] Test Finished
I0610 03:20:23.917666   16894 clusterloader.go:229]   Test: ./testing/load/config.yaml
I0610 03:20:23.917681   16894 clusterloader.go:230]   Status: Success
I0610 03:20:23.917693   16894 clusterloader.go:234] --------------------------------------------------------------------------------

/tmp/kubectl-v1.21.1 --kubeconfig=/tmp/eks-2021060922-sunahvg04tie.kubeconfig.yaml get ns
{
    "lastTransitionTime": "2021-06-10T03:10:28Z",
    "message": "Discovery failed for some groups, 1 failing: unable to retrieve the complete list of server APIs: metrics.k8s.io/v1beta1: the server is currently unable to handle the request",
    "reason": "DiscoveryFailed",
    "status": "True",
    "type": "NamespaceDeletionDiscoveryFailure"
},
*/
func (ts *tester) runCL2(idx int, args []string) (err error) {
	ts.cfg.Logger.Info("running clusterloader2", zap.Int("index", idx), zap.String("command", strings.Join(args, " ")))
	// each clusterloader2 run takes about 2-minute
	// but may stuck with test namespace deletion
	ctx, cancel := context.WithTimeout(ts.rootCtx, 5*time.Minute)
	cmd := exec.New().CommandContext(ctx, args[0], args[1:]...)
	cmd.SetStderr(ts.testLogFile)
	cmd.SetStdout(ts.testLogFile)
	err = cmd.Run()
	cancel()
	return err
}

func (ts *tester) runCL2s(checkDonec chan struct{}) (runErr error) {
	args := ts.getCL2Args()
	now := time.Now()
	errc := make(chan error)
	ts.rootCtx, ts.rootCancel = context.WithTimeout(context.Background(), ts.cfg.RunTimeout)
	go func() {
		for i := 0; i < ts.cfg.Runs; i++ {
			select {
			case <-ts.rootCtx.Done():
				return
			case <-time.After(5 * time.Second):
			}

			rerr := ts.runCL2(i, args)
			if rerr == nil {
				ts.cfg.Logger.Info("completed cluster loader", zap.Int("current-run", i), zap.Int("total-runs", ts.cfg.Runs))
				continue
			}

			ts.cfg.Logger.Warn("checking cluster loader error from log file", zap.Error(rerr))
			b, lerr := ioutil.ReadFile(ts.cfg.TestLogPath)
			if lerr != nil {
				ts.cfg.Logger.Warn("failed to read cluster loader error from logs file", zap.Error(lerr))
				errc <- rerr
				return
			}
			output := strings.TrimSpace(string(b))
			lines := strings.Split(output, "\n")
			linesN := len(lines)
			if linesN > 15 {
				output = strings.Join(lines[linesN-15:], "\n")
			}

			if strings.Contains(output, `Status: Success`) {
				// e.g., "Resource cleanup error: [timed out)"...
				ts.cfg.Logger.Warn("cluster loader command exited but continue for its success status")
				continue
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
	return runErr
}

func (ts *tester) countTestFinishes() (testFinishedCount int, err error) {
	ts.cfg.Logger.Info("counting test finishes", zap.String("test-log-path", ts.cfg.TestLogPath))
	lout, err := ioutil.ReadFile(ts.cfg.TestLogPath)
	if err != nil {
		return 0, err
	}
	logOutput := string(lout)
	testFinishedCount = strings.Count(logOutput, `] Test Finished`)
	return testFinishedCount, nil
}

func (ts *tester) appendResultsToTestLogPath() (podStartupLats []PerfData, err error) {
	// append results in "TestLogPath"
	// "0777" to fix "scp: /var/log/cluster-loader-remote.log: Permission denied"
	logFile, cerr := os.OpenFile(ts.cfg.TestLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0777)
	if cerr != nil {
		return nil, fmt.Errorf("open(%q): %v", ts.cfg.TestLogPath, cerr)
	}
	defer logFile.Close()

	podStartupLats = make([]PerfData, 0)
	cerr = filepath.Walk(ts.cfg.TestReportDir, func(path string, info os.FileInfo, ferr error) error {
		if ferr != nil {
			return ferr
		}
		if info.IsDir() {
			return nil
		}
		ts.cfg.Logger.Info("found report", zap.String("path", path))

		if strings.HasPrefix(filepath.Base(path), "PodStartupLatency_") {
			ts.cfg.Logger.Info("parsing PodStartupLatency", zap.String("path", path))
			p, perr := parsePodStartupLatency(path)
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
	return podStartupLats, cerr
}

func (ts *tester) collectPodStartupLatency(podStartupLats []PerfData) error {
	ts.cfg.PodStartupLatency = mergePodStartupLatency(podStartupLats...)
	podStartupLatData, err := json.Marshal(ts.cfg.PodStartupLatency)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(ts.cfg.PodStartupLatencyPath, podStartupLatData, 0600); err != nil {
		return err
	}
	return nil
}

func (ts *tester) compressReports() error {
	ts.cfg.Logger.Info("tar-gzipping report dir", zap.String("report-dir", ts.cfg.TestReportDir), zap.String("file-path", ts.cfg.TestReportDirTarGzPath))
	if err := os.RemoveAll(ts.cfg.TestReportDirTarGzPath); err != nil {
		ts.cfg.Logger.Warn("failed to remove temp file", zap.Error(err))
		return err
	}

	if err := archiver.Archive([]string{ts.cfg.TestReportDir}, ts.cfg.TestReportDirTarGzPath); err != nil {
		ts.cfg.Logger.Warn("archive failed", zap.Error(err))
		return err
	}
	stat, err := os.Stat(ts.cfg.TestReportDirTarGzPath)
	if err != nil {
		ts.cfg.Logger.Warn("failed to os stat", zap.Error(err))
		return err
	}

	sz := humanize.Bytes(uint64(stat.Size()))
	ts.cfg.Logger.Info("tar-gzipped report dir", zap.String("report-dir", ts.cfg.TestReportDir), zap.String("file-path", ts.cfg.TestReportDirTarGzPath), zap.String("file-size", sz))
	return nil
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

/*
DO NOT FAIL THE TEST JUST BECAUSE IT CANNOT TEAR DOWN EXEC

I0610 01:33:32.780753    7596 clusterloader.go:228] Test Finished
I0610 01:33:32.780758    7596 clusterloader.go:229]   Test: ./testing/load/config.yaml
I0610 01:33:32.780763    7596 clusterloader.go:230]   Status: Success
I0610 01:33:32.780768    7596 clusterloader.go:234] --------------------------------------------------------------------------------

JUnit report was created: /tmp/clusterloader-test-report-dir-168713f4aacf6138/junit.xml
E0610 01:43:32.811103    7596 clusterloader.go:359] Error while tearing down exec service: timed out waiting for the condition
*/

const skipErr = `action gather failed for SchedulingMetrics`
