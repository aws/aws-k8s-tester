// Package conformance a simple pi Pod with Job.
// Replace https://github.com/aws/aws-k8s-tester/tree/v1.5.9/eks/conformance.
package conformance

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester/tester"
	"github.com/aws/aws-k8s-tester/utils/file"
	"github.com/aws/aws-k8s-tester/utils/rand"
	utils_time "github.com/aws/aws-k8s-tester/utils/time"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// TODO: support e2e test binary runs without sonobuoy dependency

// Config defines parameters for Kubernetes conformance tests.
// ref. https://github.com/cncf/k8s-conformance/blob/master/instructions.md
// ref. https://github.com/vmware-tanzu/sonobuoy
type Config struct {
	Enable bool `json:"enable"`
	Prompt bool `json:"-"`

	Stopc     chan struct{} `json:"-"`
	Logger    *zap.Logger   `json:"-"`
	LogWriter io.Writer     `json:"-"`
	Client    client.Client `json:"-"`

	// MinimumNodes is the minimum number of Kubernetes nodes required for installing this addon.
	MinimumNodes int `json:"minimum_nodes"`
	// Namespace to create test resources.
	Namespace string `json:"namespace"`

	KubeconfigPath string `json:"kubeconfig_path"`

	// SonobuoyPath is the path to download the "sonobuoy".
	SonobuoyPath string `json:"sonobuoy_path"`
	// SonobuoyDownloadURL is the download URL to download "sonobuoy" binary from.
	// ref. https://github.com/vmware-tanzu/sonobuoy/releases
	SonobuoyDownloadURL string `json:"sonobuoy_download_url"`

	SonobuoyRunTimeout          time.Duration `json:"sonobuoy_run_timeout"`
	SonobuoyRunTimeoutString    string        `json:"sonobuoy_run_timeout_string" read-only:"true"`
	SonobuoyDeleteTimeout       time.Duration `json:"sonobuoy_delete_timeout"`
	SonobuoyDeleteTimeoutString string        `json:"sonobuoy_delete_timeout_string" read-only:"true"`

	// SonobuoyRunMode is the "mode" flag value for "sonobuoy run" command.
	// Valid modes are 'non-disruptive-conformance', 'quick', 'certified-conformance'.
	// The default is 'certified-conformance'.
	// ref. https://github.com/vmware-tanzu/sonobuoy
	SonobuoyRunMode                 string `json:"sonobuoy_run_mode"`
	SonobuoyRunE2EFocus             string `json:"sonobuoy_run_e2e_focus"`
	SonobuoyRunE2ESkip              string `json:"sonobuoy_run_e2e_skip"`
	SonobuoyRunKubeConformanceImage string `json:"sonobuoy_run_kube_conformance_image"`
	// SonobuoyRunE2ERepoConfig File path to e2e registry config.
	// ref. https://sonobuoy.io/docs/master/airgap/
	SonobuoyRunE2ERepoConfig string `json:"sonobuoy_run_e2e_repo_config"`
	// SonobuoyRunImage is the sonobuoy run image override for the sonobuoy worker.
	SonobuoyRunImage string `json:"sonobuoy_run_image"`
	// SonobuoyRunSystemdLogsImage is the image for systemd-logs plugin image.
	SonobuoyRunSystemdLogsImage string `json:"sonobuoy_run_systemd_logs_image"`

	// SonobuoyResultsTarGzPath is the sonobuoy results tar.gz file path after downloaded from the sonobuoy Pod.
	SonobuoyResultsTarGzPath string `json:"sonobuoy_results_tar_gz_path"`
	// SonobuoyResultsE2ELogPath is the sonobuoy results log file path after downloaded from the sonobuoy Pod.
	SonobuoyResultsE2ELogPath string `json:"sonobuoy_results_e2e_log_path"`
	// SonobuoyResultsJunitXMLPath is the sonobuoy results junit XML file path after downloaded from the sonobuoy Pod.
	SonobuoyResultsJunitXMLPath string `json:"sonobuoy_results_junit_xml_path"`
	// SonobuoyResultsOutputDir is the sonobuoy results output path after untar.
	SonobuoyResultsOutputDir string `json:"sonobuoy_results_output_dir"`
}

func (cfg *Config) ValidateAndSetDefaults() error {
	if cfg.SonobuoyPath == "" {
		cfg.SonobuoyPath = DefaultSonobuoyPath()
	}
	if cfg.SonobuoyDownloadURL == "" {
		cfg.SonobuoyDownloadURL = DefaultSonobuoyDownloadURL()
	}

	if cfg.SonobuoyRunTimeout == time.Duration(0) {
		cfg.SonobuoyRunTimeout = DefaultSonobuoyRunTimeout
	}
	cfg.SonobuoyRunTimeoutString = cfg.SonobuoyRunTimeout.String()
	if cfg.SonobuoyDeleteTimeout == time.Duration(0) {
		cfg.SonobuoyDeleteTimeout = DefaultSonobuoyDeleteTimeout
	}
	cfg.SonobuoyDeleteTimeoutString = cfg.SonobuoyDeleteTimeout.String()

	if cfg.SonobuoyRunMode == "" {
		cfg.SonobuoyRunMode = DefaultSonobuoyRunMode
	}
	switch cfg.SonobuoyRunMode {
	case DefaultSonobuoyRunMode:
	case "non-disruptive-conformance":
	case "quick":
	default:
		return fmt.Errorf("unknown SonobuoyRunMode %q", cfg.SonobuoyRunMode)
	}
	if cfg.SonobuoyRunKubeConformanceImage == "" {
		cfg.SonobuoyRunKubeConformanceImage = DefaultSonobuoyRunKubeConformanceImage
	}

	if !strings.HasSuffix(cfg.SonobuoyResultsTarGzPath, ".tar.gz") {
		return fmt.Errorf("SonobuoyResultsTarGzPath %q missing .tar.gz", cfg.SonobuoyResultsTarGzPath)
	}
	if !strings.HasSuffix(cfg.SonobuoyResultsE2ELogPath, ".e2e.log") {
		return fmt.Errorf("SonobuoyResultsE2ELogPath %q missing .log", cfg.SonobuoyResultsE2ELogPath)
	}
	if !strings.HasSuffix(cfg.SonobuoyResultsJunitXMLPath, ".xml") {
		return fmt.Errorf("SonobuoyResultsJunitXMLPath %q missing .xml", cfg.SonobuoyResultsJunitXMLPath)
	}

	return nil
}

const (
	DefaultMinimumNodes                    int = 1
	DefaultSonobuoyRunTimeout                  = 5 * time.Hour
	DefaultSonobuoyDeleteTimeout               = 5 * time.Minute
	DefaultSonobuoyRunMode                     = "certified-conformance"
	DefaultSonobuoyRunKubeConformanceImage     = "k8s.gcr.io/conformance:v1.21.0"
)

func NewDefault() *Config {
	return &Config{
		Enable:       false,
		Prompt:       false,
		MinimumNodes: DefaultMinimumNodes,
		Namespace:    pkgName + "-" + rand.String(10) + "-" + utils_time.GetTS(10),

		KubeconfigPath: "",

		SonobuoyPath:        DefaultSonobuoyPath(),
		SonobuoyDownloadURL: DefaultSonobuoyDownloadURL(),

		SonobuoyRunTimeout:          DefaultSonobuoyRunTimeout,
		SonobuoyRunTimeoutString:    DefaultSonobuoyRunTimeout.String(),
		SonobuoyDeleteTimeout:       DefaultSonobuoyDeleteTimeout,
		SonobuoyDeleteTimeoutString: DefaultSonobuoyDeleteTimeout.String(),

		SonobuoyRunMode:                 DefaultSonobuoyRunMode,
		SonobuoyRunE2EFocus:             "",
		SonobuoyRunE2ESkip:              "",
		SonobuoyRunKubeConformanceImage: DefaultSonobuoyRunKubeConformanceImage,
		SonobuoyRunE2ERepoConfig:        "",
		SonobuoyRunImage:                "",
		SonobuoyRunSystemdLogsImage:     "",

		SonobuoyResultsTarGzPath:    file.GetTempFilePath("sonobuoy_results") + ".tar.gz",
		SonobuoyResultsE2ELogPath:   file.GetTempFilePath("sonobuoy_results") + ".e2e.log",
		SonobuoyResultsJunitXMLPath: file.GetTempFilePath("sonobuoy_results") + ".xml",
		SonobuoyResultsOutputDir:    file.MkDir("", "sonobuoy-output"),
	}
}

func New(cfg *Config) k8s_tester.Tester {
	return &tester{
		cfg: cfg,
	}
}

type tester struct {
	cfg *Config
}

var pkgName = path.Base(reflect.TypeOf(tester{}).PkgPath())

func Env() string {
	return "ADD_ON_" + strings.ToUpper(strings.Replace(pkgName, "-", "_", -1))
}

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Enabled() bool { return ts.cfg.Enable }

func (ts *tester) Apply() error {
	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}

	if nodes, err := client.ListNodes(ts.cfg.Client.KubernetesClient()); len(nodes) < ts.cfg.MinimumNodes || err != nil {
		return fmt.Errorf("failed to validate minimum nodes requirement %d (nodes %v, error %v)", ts.cfg.MinimumNodes, len(nodes), err)
	}

	if err := client.CreateNamespace(ts.cfg.Logger, ts.cfg.Client.KubernetesClient(), ts.cfg.Namespace); err != nil {
		return err
	}

	if err := installSonobuoy(ts.cfg.Logger, ts.cfg.SonobuoyPath, ts.cfg.SonobuoyDownloadURL); err != nil {
		return err
	}
	if err := ts.deleteSonobuoy(); err != nil {
		return err
	}
	if err := ts.runSonobuoy(); err != nil {
		return err
	}
	if err := ts.checkSonobuoy(); err != nil {
		return err
	}
	if err := ts.checkResults(); err != nil {
		return err
	}

	return nil
}

func (ts *tester) Delete() error {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}

	var errs []string

	if err := installSonobuoy(ts.cfg.Logger, ts.cfg.SonobuoyPath, ts.cfg.SonobuoyDownloadURL); err != nil {
		return err
	}
	if err := ts.deleteSonobuoy(); err != nil {
		return err
	}

	if err := client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
		ts.cfg.Namespace,
		client.DefaultNamespaceDeletionInterval,
		client.DefaultNamespaceDeletionTimeout,
		client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	return nil
}

func (ts *tester) runPrompt(action string) (ok bool) {
	if ts.cfg.Prompt {
		msg := fmt.Sprintf("Ready to %q resources for the namespace %q, should we continue?", action, ts.cfg.Namespace)
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

func (ts *tester) deleteSonobuoy() (err error) {
	args := []string{
		ts.cfg.SonobuoyPath,
		"--logtostderr",
		"--alsologtostderr",
		"--v=3",
		"delete",
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.Namespace,
		"--wait",
	}
	cmd := strings.Join(args, " ")

	ts.cfg.Logger.Info("deleting sonobuoy",
		zap.String("command", cmd),
		zap.String("timeout", ts.cfg.SonobuoyDeleteTimeoutString),
	)

	var output []byte
	ctx, cancel := context.WithTimeout(context.Background(), ts.cfg.SonobuoyDeleteTimeout)
	output, err = exec.New().CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
	cancel()
	out := strings.TrimSpace(string(output))
	if err != nil {
		// TODO: check error
		ts.cfg.Logger.Warn("failed to delete sonobuoy", zap.String("command", cmd), zap.Error(err))
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\n'%s' output:\n\n%s\n\n", cmd, out)

	ts.cfg.Logger.Info("deleted sonobuoy", zap.String("command", cmd))
	return nil
}

func (ts *tester) runSonobuoy() (err error) {
	timeoutSeconds := int64(ts.cfg.SonobuoyRunTimeout.Seconds())
	args := []string{
		ts.cfg.SonobuoyPath,
		"--logtostderr",
		"--alsologtostderr",
		"--v=3",
		"run",
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.Namespace,
		"--mode=" + ts.cfg.SonobuoyRunMode,
		"--kube-conformance-image=" + ts.cfg.SonobuoyRunKubeConformanceImage,
		"--show-default-podspec=true",
		fmt.Sprintf("--timeout=%d", timeoutSeconds), // default "10800", 3-hour
	}
	if ts.cfg.SonobuoyRunE2ERepoConfig != "" {
		args = append(args, "--e2e-repo-config="+ts.cfg.SonobuoyRunE2ERepoConfig)
	}
	if ts.cfg.SonobuoyRunImage != "" {
		args = append(args, "--sonobuoy-image="+ts.cfg.SonobuoyRunImage)
	}
	if ts.cfg.SonobuoyRunSystemdLogsImage != "" {
		args = append(args, "--systemd-logs-image="+ts.cfg.SonobuoyRunSystemdLogsImage)
	}
	if ts.cfg.SonobuoyRunE2EFocus != "" {
		args = append(args, "--e2e-focus="+ts.cfg.SonobuoyRunE2EFocus)
	}
	if ts.cfg.SonobuoyRunE2ESkip != "" {
		args = append(args, "--e2e-skip="+ts.cfg.SonobuoyRunE2ESkip)
	}
	cmd := strings.Join(args, " ")

	ts.cfg.Logger.Info("running sonobuoy",
		zap.Duration("timeout", ts.cfg.SonobuoyRunTimeout),
		zap.Int64("timeout-seconds", timeoutSeconds),
		zap.String("mode", ts.cfg.SonobuoyRunMode),
		zap.String("command", cmd),
	)

	var output []byte
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute) // do not wait, so just set timeout for launching tests
	output, err = exec.New().CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
	cancel()
	out := strings.TrimSpace(string(output))
	if err != nil {
		// TODO: check error
		ts.cfg.Logger.Warn("failed to run sonobuoy", zap.String("command", cmd), zap.Error(err))
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\n'%s' output:\n\n%s\n\n", cmd, out)

	ts.cfg.Logger.Info("ran sonobuoy", zap.String("mode", ts.cfg.SonobuoyRunMode), zap.String("command", cmd))
	return nil
}

func (ts *tester) checkSonobuoy() (err error) {
	ts.cfg.Logger.Info("checking pod/sonobuoy-e2e-job")
	sonobuoyE2EJobPod := ""
	retryStart := time.Now()
	for time.Since(retryStart) < 10*time.Minute {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("sonobuoy check stopped")
			return nil
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("sonobuoy check timeout")
			return fmt.Errorf("sonobuoy run took too long (exceeded %v)", ts.cfg.SonobuoyRunTimeout)
		case <-time.After(10 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		var pods *v1.PodList
		pods, err = ts.cfg.Client.KubernetesClient().
			CoreV1().
			Pods(ts.cfg.Namespace).
			List(ctx, meta_v1.ListOptions{})
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to list pods", zap.Error(err))
			continue
		}

		for _, pv := range pods.Items {
			ts.cfg.Logger.Info("found pod", zap.String("name", pv.GetName()))
			if strings.HasPrefix(pv.GetName(), "sonobuoy-e2e-job-") {
				sonobuoyE2EJobPod = pv.GetName()
				break
			}
		}
		if sonobuoyE2EJobPod != "" {
			break
		}
	}
	if sonobuoyE2EJobPod == "" {
		return fmt.Errorf("failed to find pod/sonobuoy-e2e-job in %q", ts.cfg.Namespace)
	}
	ts.cfg.Logger.Info("found pod/sonobuoy-e2e-job", zap.String("name", sonobuoyE2EJobPod))

	argsLogsSonobuoy := []string{
		ts.cfg.SonobuoyPath,
		"--logtostderr",
		"--alsologtostderr",
		"--v=3",
		"logs",
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.Namespace,
	}
	cmdLogsSonobuoy := strings.Join(argsLogsSonobuoy, " ")

	argsLogsPod := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.Namespace,
		"logs",
		fmt.Sprintf("pod/%s", sonobuoyE2EJobPod),
		"e2e",
		"--tail=30",
	}
	cmdLogsPod := strings.Join(argsLogsPod, " ")

	argsStatus := []string{
		ts.cfg.SonobuoyPath,
		"--logtostderr",
		"--alsologtostderr",
		"--v=3",
		"status",
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.Namespace,
		"--show-all",
	}
	cmdStatus := strings.Join(argsStatus, " ")

	ts.cfg.Logger.Info("running sonobuoy",
		zap.String("logs-command-sonobuoy", cmdLogsSonobuoy),
		zap.String("logs-command-pod", cmdLogsPod),
		zap.String("status-command", cmdStatus),
	)

	deadline := time.Now().Add(ts.cfg.SonobuoyRunTimeout)
	donec := time.After(ts.cfg.SonobuoyRunTimeout)
	start, waitDur := time.Now(), ts.cfg.SonobuoyRunTimeout

	interval := 15 * time.Minute
	cnt := 0
	for time.Since(start) < waitDur {
		cnt++
		ts.cfg.Logger.Info(
			"waiting for sonobuoy run",
			zap.Duration("interval", interval),
			zap.String("time-left", deadline.Sub(time.Now()).String()),
			zap.Duration("timeout", ts.cfg.SonobuoyRunTimeout),
		)
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("sonobuoy check stopped")
			return nil
		case <-donec:
			ts.cfg.Logger.Warn("sonobuoy check timeout")
			return fmt.Errorf("sonobuoy run took too long (exceeded %v)", ts.cfg.SonobuoyRunTimeout)
		case <-time.After(interval):
		}

		argsLogs, cmdLogs := argsLogsSonobuoy, cmdLogsSonobuoy
		if cnt%2 == 0 {
			argsLogs, cmdLogs = argsLogsPod, cmdLogsPod
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		output, err := exec.New().CommandContext(ctx, argsLogs[0], argsLogs[1:]...).CombinedOutput()
		cancel()
		out := strings.TrimSpace(string(output))
		if err != nil {
			ts.cfg.Logger.Warn("failed to fetch sonobuoy logs", zap.String("command", cmdLogs), zap.Error(err))
		}
		lines := strings.Split(out, "\n")
		linesN := len(lines)
		if linesN > 30 { // tail 30 lines
			out = strings.Join(lines[linesN-30:], "\n")
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n'%s' output (total lines %d, last 30 lines):\n\n%s\n\n", cmdLogs, linesN, out)

		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		output, err = exec.New().CommandContext(ctx, argsStatus[0], argsStatus[1:]...).CombinedOutput()
		cancel()
		out = strings.TrimSpace(string(output))
		if err != nil {
			ts.cfg.Logger.Warn("failed to run sonobuoy status", zap.String("command", cmdStatus), zap.Error(err))
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n'%s' output:\n\n%s\n\n", cmdStatus, out)

		// ref. https://github.com/vmware-tanzu/sonobuoy/blob/master/cmd/sonobuoy/app/status.go
		if strings.Contains(out, "Sonobuoy has completed. ") ||
			strings.Contains(out, "Sonobuoy plugins have completed. ") {
			break
		}
		if strings.Contains(out, "Sonobuoy has failed. ") ||
			strings.Contains(out, "Sonobuoy is in unknown state") {
			return errors.New("sonobuoy run failed")
		}

		interval = time.Duration(float64(interval) * 0.7)
		if interval < 2*time.Minute {
			interval = 2 * time.Minute
		}
	}

	return nil
}

func (ts *tester) checkResults() (err error) {
	argsRetrieve := []string{
		ts.cfg.SonobuoyPath,
		"retrieve",
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.Namespace,
		os.TempDir(),
	}
	cmdRetrieve := strings.Join(argsRetrieve, " ")

	ts.cfg.Logger.Info("running sonobuoy", zap.String("retrieve-command", cmdRetrieve))

	os.RemoveAll(ts.cfg.SonobuoyResultsTarGzPath)
	start, waitDur := time.Now(), 3*time.Minute
	for time.Since(start) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("sonobuoy retrieve stopped")
			return nil
		default:
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		output, err := exec.New().CommandContext(ctx, argsRetrieve[0], argsRetrieve[1:]...).CombinedOutput()
		cancel()
		out := strings.TrimSpace(string(output))
		if err != nil {
			ts.cfg.Logger.Warn("failed to run sonobuoy retrieve", zap.String("command", cmdRetrieve), zap.Error(err))
			time.Sleep(10 * time.Second)
			continue
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n'%s' output:\n\n%s\n\n", cmdRetrieve, out)

		if err = file.Copy(out, ts.cfg.SonobuoyResultsTarGzPath); err != nil {
			ts.cfg.Logger.Warn("failed to copy sonobuoy retrieve results", zap.Error(err))
			return err
		}

		ts.cfg.Logger.Info("retrieved sonobuoy results", zap.String("path", ts.cfg.SonobuoyResultsTarGzPath))
		break
	}

	err = readResults(
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.SonobuoyPath,
		ts.cfg.SonobuoyResultsTarGzPath,
	)
	if err != nil {
		ts.cfg.Logger.Warn("read results failed", zap.Error(err))
	}

	logPath, xmlPath, terr := untarResults(
		ts.cfg.Logger,
		ts.cfg.SonobuoyResultsTarGzPath,
		ts.cfg.SonobuoyResultsOutputDir,
	)
	if terr != nil {
		ts.cfg.Logger.Warn("failed to untar results", zap.Error(terr))
		if err == nil {
			err = terr
		} else {
			err = fmt.Errorf("read results error [%v], untar error [%v]", err, terr)
		}
	}
	if err != nil {
		return err
	}
	if err = file.Copy(logPath, ts.cfg.SonobuoyResultsE2ELogPath); err != nil {
		return err
	}
	if err = file.Copy(xmlPath, ts.cfg.SonobuoyResultsJunitXMLPath); err != nil {
		return err
	}

	return nil
}
