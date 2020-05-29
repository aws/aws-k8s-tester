// Package clusterloader implements cluster loader.
// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2
package clusterloader

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/httputil"
	"github.com/dustin/go-humanize"
	"github.com/mholt/archiver/v3"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

// Config configures cluster loader.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}

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
	ReportTarGzPath string
	// LogPath is the log file path to stream clusterloader binary runs.
	LogPath string

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

	CL2EnablePVS              bool
	PrometheusScrapeKubeProxy bool
	EnableSystemPodMetrics    bool
}

// Loader defines cluster loader operations.
type Loader interface {
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

func (ld *loader) Start() (err error) {
	ld.cfg.Logger.Info("starting cluster loader")

	if !fileutil.Exist(ld.cfg.TestConfigPath) {
		ld.cfg.Logger.Warn("clusterloader test config file does not exist", zap.String("path", ld.cfg.TestConfigPath))
		return fmt.Errorf("%q not found", ld.cfg.TestConfigPath)
	}

	if err = os.MkdirAll(ld.cfg.ReportDir, 0700); err != nil {
		return err
	}
	if err = fileutil.IsDirWriteable(ld.cfg.ReportDir); err != nil {
		return err
	}

	if err = ld.downloadClusterLoader(); err != nil {
		return err
	}
	if err = ld.writeTestOverrides(); err != nil {
		return err
	}

	args := []string{
		ld.cfg.ClusterLoaderPath,
		"--alsologtostderr",
		"--testconfig=" + ld.cfg.TestConfigPath,
		"--testoverrides=" + ld.testOverridesPath,
		"--report-dir=" + ld.cfg.ReportDir,
		"--nodes=" + fmt.Sprintf("%d", ld.cfg.Nodes),
	}
	if ld.cfg.KubeConfigPath != "" {
		args = append(args, "--kubeconfig="+ld.cfg.KubeConfigPath)
	}

	ld.testLogsFile, err = os.OpenFile(ld.cfg.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer func() {
		ld.testLogsFile.Sync()
		ld.testLogsFile.Close()
	}()
	// stream command run outputs for debugging purposes
	go func() {
		for {
			select {
			case <-ld.cfg.Stopc:
				ld.cfg.Logger.Info("exiting cluster loader command output checks")
				return
			case <-ld.rootCtx.Done():
				ld.cfg.Logger.Info("exiting cluster loader command output checks")
				return
			case <-time.After(10 * time.Second):
			}

			if ld.testLogsFile != nil {
				ld.testLogsFile.Sync()
			}
			ld.cfg.Logger.Info("checking cluster loader command output from logs file")
			b, lerr := ioutil.ReadFile(ld.cfg.LogPath)
			if err != nil {
				ld.cfg.Logger.Warn("failed to read cluster loader command output from logs file", zap.Error(lerr))
				continue
			}
			output := strings.TrimSpace(string(b))
			lines := strings.Split(output, "\n")
			linesN := len(lines)

			ld.cfg.Logger.Info("checked cluster loader command output from logs file", zap.Int("total-lines", linesN))
			if linesN > 15 {
				output = strings.Join(lines[linesN-15:], "\n")
			}
			fmt.Printf("\n%q output:\n%s\n\n", ld.cfg.LogPath, output)
		}
	}()

	errc := make(chan error)
	ld.rootCtx, ld.rootCancel = context.WithTimeout(context.Background(), ld.cfg.Timeout)
	go func() {
		for i := 0; i < ld.cfg.Runs; i++ {
			select {
			case <-ld.rootCtx.Done():
				return
			default:
			}
			if err := ld.run(i, args); err != nil {
				errc <- err
				return
			}
		}
		errc <- nil
	}()
	select {
	case <-ld.donec:
		ld.cfg.Logger.Info("done cluster loader")
	case <-ld.cfg.Stopc:
		ld.cfg.Logger.Info("stopping cluster loader")
	case <-ld.rootCtx.Done():
		ld.cfg.Logger.Info("timed out cluster loader")
	case err = <-errc:
		if err == nil {
			ld.cfg.Logger.Info("completed cluster loader")
		} else {
			ld.cfg.Logger.Warn("failed cluster loader", zap.Error(err))
		}
	}
	ld.rootCancel()

	ld.cfg.Logger.Info("gzipping report dir", zap.String("report-dir", ld.cfg.ReportDir), zap.String("file-path", ld.cfg.ReportTarGzPath))
	err = os.RemoveAll(ld.cfg.ReportTarGzPath)
	if err != nil {
		ld.cfg.Logger.Warn("failed to remove temp file", zap.Error(err))
		return err
	}
	err = archiver.Archive([]string{ld.cfg.ReportDir}, ld.cfg.ReportTarGzPath)
	if err != nil {
		ld.cfg.Logger.Warn("archive failed", zap.Error(err))
		return err
	}
	stat, err := os.Stat(ld.cfg.ReportTarGzPath)
	if err != nil {
		ld.cfg.Logger.Warn("failed to os stat", zap.Error(err))
		return err
	}
	sz := humanize.Bytes(uint64(stat.Size()))
	ld.cfg.Logger.Info("gzipped report dir", zap.String("report-dir", ld.cfg.ReportDir), zap.String("file-path", ld.cfg.ReportTarGzPath), zap.String("file-size", sz))

	return nil
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

const TemplateTestOverrides = `NODES_PER_NAMESPACE: {{ .NodesPerNamespace }}
PODS_PER_NODE: {{ .PodsPerNode }}
BIG_GROUP_SIZE: {{ .BigGroupSize }}
MEDIUM_GROUP_SIZE: {{ .MediumGroupSize }}
SMALL_GROUP_SIZE: {{ .SmallGroupSize }}
SMALL_STATEFUL_SETS_PER_NAMESPACE: {{ .SmallStatefulSetsPerNamespace }}
MEDIUM_STATEFUL_SETS_PER_NAMESPACE: {{ .MediumStatefulSetsPerNamespace }}
CL2_ENABLE_PVS: {{ .CL2EnablePVS }}
PROMETHEUS_SCRAPE_KUBE_PROXY: {{ .PrometheusScrapeKubeProxy }}
ENABLE_SYSTEM_POD_METRICS: {{ .EnableSystemPodMetrics }}
`

func (ld *loader) run(idx int, args []string) (err error) {
	ld.cfg.Logger.Info("running cluster loader", zap.Int("index", idx), zap.String("command", strings.Join(args, " ")))
	ctx, cancel := context.WithTimeout(ld.rootCtx, 20*time.Minute)
	cmd := exec.New().CommandContext(ctx, args[0], args[1:]...)
	cmd.SetStderr(ld.testLogsFile)
	cmd.SetStdout(ld.testLogsFile)
	err = cmd.Run()
	cancel()
	if err != nil {
		ld.cfg.Logger.Warn("failed to run cluster loader", zap.Error(err))
	} else {
		ld.cfg.Logger.Info("successfykkt run cluster loader")
	}
	return err
}
