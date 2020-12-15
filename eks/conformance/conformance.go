// Package conformance implements Conformance tests.
// ref. https://github.com/cncf/k8s-conformance/blob/master/instructions.md
// ref. https://github.com/vmware-tanzu/sonobuoy
package conformance

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	aws_s3 "github.com/aws/aws-k8s-tester/pkg/aws/s3"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/httputil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/mholt/archiver/v3"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines Conformance configuration.
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	S3API     s3iface.S3API
}

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
	return &tester{cfg: cfg, donec: make(chan struct{})}
}

type tester struct {
	cfg   Config
	donec chan struct{}
}

func (ts *tester) Create() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnConformance() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnConformance.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnConformance.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnConformance.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err := ts.downloadInstallSonobuoy(); err != nil {
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
	if !ts.cfg.EKSConfig.IsEnabledAddOnConformance() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnConformance.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnConformance.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteSonobuoy(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete sonobuoy %q", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnConformance.Created = false
	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) downloadInstallSonobuoy() (err error) {
	ts.cfg.Logger.Info("mkdir", zap.String("sonobuoy-path-dir", filepath.Dir(ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath)))
	if err = os.MkdirAll(filepath.Dir(ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath), 0700); err != nil {
		return fmt.Errorf("could not create %q (%v)", filepath.Dir(ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath), err)
	}

	if !fileutil.Exist(ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath) {
		tarPath := filepath.Join(os.TempDir(), fmt.Sprintf("sonobuoy-%x.tar.gz", time.Now().UnixNano()))
		if err = httputil.Download(ts.cfg.Logger, os.Stderr, ts.cfg.EKSConfig.AddOnConformance.SonobuoyDownloadURL, tarPath); err != nil {
			return err
		}
		tmpDir, err := ioutil.TempDir(os.TempDir(), "sonobuoy")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)
		if err = archiver.Unarchive(tarPath, tmpDir); err != nil {
			return fmt.Errorf("failed to decompress sonobuoy tar file %v", err)
		}
		if err = fileutil.Copy(filepath.Join(tmpDir, "sonobuoy"), ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath); err != nil {
			return fmt.Errorf("failed to copy file %v", err)
		}
	} else {
		ts.cfg.Logger.Info("skipping sonobuoy download; already exist", zap.String("sonobuoy-path", ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath))
	}

	if err = fileutil.EnsureExecutable(ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath); err != nil {
		// file may be already executable while the process does not own the file/directory
		// ref. https://github.com/aws/aws-k8s-tester/issues/66
		ts.cfg.Logger.Warn("failed to ensure executable", zap.Error(err))
		err = nil
	}

	var output []byte
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(ctx, ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath, "help").CombinedOutput()
	cancel()
	out := string(output)
	if err != nil {
		return fmt.Errorf("'sonobuoy help' failed (output %q, error %v)", out, err)
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\n'sonobuoy help' output:\n\n%s\n\n", out)

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(ctx, ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath, "run", "--help").CombinedOutput()
	cancel()
	out = strings.TrimSpace(string(output))
	if err != nil {
		return fmt.Errorf("'sonobuoy run --help' failed (output %q, error %v)", out, err)
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\n'sonobuoy run --help' output:\n\n%s\n\n", out)

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(ctx, ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath, "version").CombinedOutput()
	cancel()
	out = strings.TrimSpace(string(output))
	if err != nil {
		return fmt.Errorf("'sonobuoy version' failed (output %q, error %v)", out, err)
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\n'sonobuoy version' output:\n\n%s\n\n", out)

	ts.cfg.Logger.Info(
		"sonobuoy version",
		zap.String("sonobuoy-path", ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath),
		zap.String("sonobuoy-version", out),
	)
	return nil
}

func (ts *tester) deleteSonobuoy() (err error) {
	args := []string{
		ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath,
		"--logtostderr",
		"--alsologtostderr",
		"--v=3",
		"delete",
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnConformance.Namespace,
		"--wait",
	}
	cmd := strings.Join(args, " ")

	ts.cfg.Logger.Info("deleting sonobuoy",
		zap.String("command", cmd),
		zap.String("timeout", ts.cfg.EKSConfig.AddOnConformance.SonobuoyDeleteTimeoutString),
	)

	var output []byte
	ctx, cancel := context.WithTimeout(context.Background(), ts.cfg.EKSConfig.AddOnConformance.SonobuoyDeleteTimeout)
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
	timeoutSeconds := int64(ts.cfg.EKSConfig.AddOnConformance.SonobuoyRunTimeout.Seconds())
	args := []string{
		ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath,
		"--logtostderr",
		"--alsologtostderr",
		"--v=3",
		"run",
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnConformance.Namespace,
		"--mode=" + ts.cfg.EKSConfig.AddOnConformance.SonobuoyRunMode,
		"--kube-conformance-image=" + ts.cfg.EKSConfig.AddOnConformance.SonobuoyRunKubeConformanceImage,
		"--show-default-podspec=true",
		fmt.Sprintf("--timeout=%d", timeoutSeconds), // default "10800", 3-hour
	}
	if ts.cfg.EKSConfig.AddOnConformance.SonobuoyImage != "" {
		args = append(args, "--sonobuoy-image="+ts.cfg.EKSConfig.AddOnConformance.SonobuoyImage)
	}
	if ts.cfg.EKSConfig.AddOnConformance.SystemdLogsImage != "" {
		args = append(args, "--systemd-logs-image="+ts.cfg.EKSConfig.AddOnConformance.SystemdLogsImage)
	}
	if ts.cfg.EKSConfig.AddOnConformance.SonobuoyE2eRepoConfig != "" {
		args = append(args, "--e2e-repo-config" + ts.cfg.EKSConfig.AddOnConformance.SonobuoyE2eRepoConfig)
	}
	cmd := strings.Join(args, " ")

	ts.cfg.Logger.Info("running sonobuoy",
		zap.Duration("timeout", ts.cfg.EKSConfig.AddOnConformance.SonobuoyRunTimeout),
		zap.Int64("timeout-seconds", timeoutSeconds),
		zap.String("mode", ts.cfg.EKSConfig.AddOnConformance.SonobuoyRunMode),
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

	ts.cfg.Logger.Info("ran sonobuoy", zap.String("mode", ts.cfg.EKSConfig.AddOnConformance.SonobuoyRunMode), zap.String("command", cmd))
	return nil
}

func (ts *tester) checkSonobuoy() (err error) {
	ts.cfg.Logger.Info("checking pod/sonobuoy-e2e-job")
	sonobuoyE2EJobPod := ""
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < 10*time.Minute {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("sonobuoy check stopped")
			return nil
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("sonobuoy check timeout")
			return fmt.Errorf("sonobuoy run took too long (exceeded %v)", ts.cfg.EKSConfig.AddOnConformance.SonobuoyRunTimeout)
		case <-time.After(10 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		var pods *v1.PodList
		pods, err = ts.cfg.K8SClient.
			KubernetesClientSet().
			CoreV1().
			Pods(ts.cfg.EKSConfig.AddOnConformance.Namespace).
			List(ctx, metav1.ListOptions{})
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
		return fmt.Errorf("failed to find pod/sonobuoy-e2e-job in %q", ts.cfg.EKSConfig.AddOnConformance.Namespace)
	}
	ts.cfg.Logger.Info("found pod/sonobuoy-e2e-job", zap.String("name", sonobuoyE2EJobPod))

	argsLogsSonobuoy := []string{
		ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath,
		"--logtostderr",
		"--alsologtostderr",
		"--v=3",
		"logs",
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnConformance.Namespace,
	}
	cmdLogsSonobuoy := strings.Join(argsLogsSonobuoy, " ")

	argsLogsPod := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnConformance.Namespace,
		"logs",
		fmt.Sprintf("pod/%s", sonobuoyE2EJobPod),
		"e2e",
		"--tail=30",
	}
	cmdLogsPod := strings.Join(argsLogsPod, " ")

	argsStatus := []string{
		ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath,
		"--logtostderr",
		"--alsologtostderr",
		"--v=3",
		"status",
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnConformance.Namespace,
		"--show-all",
	}
	cmdStatus := strings.Join(argsStatus, " ")

	ts.cfg.Logger.Info("running sonobuoy",
		zap.String("logs-command-sonobuoy", cmdLogsSonobuoy),
		zap.String("logs-command-pod", cmdLogsPod),
		zap.String("status-command", cmdStatus),
	)

	deadline := time.Now().Add(ts.cfg.EKSConfig.AddOnConformance.SonobuoyRunTimeout)
	donec := time.After(ts.cfg.EKSConfig.AddOnConformance.SonobuoyRunTimeout)
	start, waitDur := time.Now(), ts.cfg.EKSConfig.AddOnConformance.SonobuoyRunTimeout

	interval := 15 * time.Minute
	cnt := 0
	for time.Now().Sub(start) < waitDur {
		cnt++
		ts.cfg.Logger.Info(
			"waiting for sonobuoy run",
			zap.Duration("interval", interval),
			zap.String("time-left", deadline.Sub(time.Now()).String()),
			zap.Duration("timeout", ts.cfg.EKSConfig.AddOnConformance.SonobuoyRunTimeout),
		)
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("sonobuoy check stopped")
			return nil
		case <-donec:
			ts.cfg.Logger.Warn("sonobuoy check timeout")
			return fmt.Errorf("sonobuoy run took too long (exceeded %v)", ts.cfg.EKSConfig.AddOnConformance.SonobuoyRunTimeout)
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

	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) checkResults() (err error) {
	argsRetrieve := []string{
		ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath,
		"retrieve",
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnConformance.Namespace,
		os.TempDir(),
	}
	cmdRetrieve := strings.Join(argsRetrieve, " ")

	ts.cfg.Logger.Info("running sonobuoy", zap.String("retrieve-command", cmdRetrieve))

	os.RemoveAll(ts.cfg.EKSConfig.AddOnConformance.SonobuoyResultTarGzPath)
	start, waitDur := time.Now(), 3*time.Minute
	for time.Now().Sub(start) < waitDur {
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

		if err = fileutil.Copy(out, ts.cfg.EKSConfig.AddOnConformance.SonobuoyResultTarGzPath); err != nil {
			ts.cfg.Logger.Warn("failed to copy sonobuoy retrieve results", zap.Error(err))
			return err
		}

		ts.cfg.Logger.Info("retrieved sonobuoy results", zap.String("path", ts.cfg.EKSConfig.AddOnConformance.SonobuoyResultTarGzPath))
		break
	}

	err = readResults(
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath,
		ts.cfg.EKSConfig.AddOnConformance.SonobuoyResultTarGzPath,
	)
	if err != nil {
		ts.cfg.Logger.Warn("read results failed", zap.Error(err))
	}

	logPath, xmlPath, terr := untarResults(
		ts.cfg.Logger,
		ts.cfg.EKSConfig.AddOnConformance.SonobuoyResultTarGzPath,
		ts.cfg.EKSConfig.AddOnConformance.SonobuoyResultDir,
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
	if err = fileutil.Copy(logPath, ts.cfg.EKSConfig.AddOnConformance.SonobuoyResultE2eLogPath); err != nil {
		return err
	}
	if err = fileutil.Copy(xmlPath, ts.cfg.EKSConfig.AddOnConformance.SonobuoyResultJunitXMLPath); err != nil {
		return err
	}

	if err = aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		ts.cfg.EKSConfig.AddOnConformance.SonobuoyResultTarGzS3Key,
		ts.cfg.EKSConfig.AddOnConformance.SonobuoyResultTarGzPath,
	); err != nil {
		return err
	}
	if err = aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		ts.cfg.EKSConfig.AddOnConformance.SonobuoyResultE2eLogS3Key,
		ts.cfg.EKSConfig.AddOnConformance.SonobuoyResultE2eLogPath,
	); err != nil {
		return err
	}
	if err = aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		ts.cfg.EKSConfig.AddOnConformance.SonobuoyResultJunitXMLS3Key,
		ts.cfg.EKSConfig.AddOnConformance.SonobuoyResultJunitXMLPath,
	); err != nil {
		return err
	}

	return nil
}

func readResults(lg *zap.Logger, logWriter io.Writer, sonobuoyPath string, tarGzPath string) error {
	if !fileutil.Exist(tarGzPath) {
		return fmt.Errorf("AddOnConformance.SonobuoyResultTarGzPath does not exist [%q]", tarGzPath)
	}

	args := []string{sonobuoyPath, "results", tarGzPath}
	cmd := strings.Join(args, " ")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	output, err := exec.New().CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
	cancel()
	out := strings.TrimSpace(string(output))
	if err != nil {
		lg.Warn("failed to run sonobuoy results", zap.String("command", cmd), zap.Error(err))
		return err
	}
	fmt.Fprintf(logWriter, "\n'%s' output:\n\n%s\n\n", cmd, out)

	if !strings.Contains(out, "Plugin: e2e\nStatus: passed") {
		return errors.New("sonobuoy tests failed (expected 'Status: passed')")
	}

	lg.Info("sonobuoy results passed", zap.String("path", tarGzPath))
	return nil
}

func untarResults(lg *zap.Logger, tarGzPath string, outputDir string) (logPath string, xmlPath string, err error) {
	if !fileutil.Exist(tarGzPath) {
		return "", "", fmt.Errorf("AddOnConformance.SonobuoyResultTarGzPath does not exist [%q]", tarGzPath)
	}

	err = archiver.Unarchive(tarGzPath, outputDir)
	if err != nil {
		return "", "", fmt.Errorf("failed to decompress sonobuoy results tar file %v", err)
	}
	lg.Info("untar success", zap.String("tar-gz-path", tarGzPath), zap.String("output-directory", outputDir))

	logPath = filepath.Join(outputDir, "plugins", "e2e", "results", "global", "e2e.log")
	if !fileutil.Exist(logPath) {
		return "", "", fmt.Errorf("result dir %q does not have e2e.log %q", outputDir, logPath)
	}
	xmlPath = filepath.Join(outputDir, "plugins", "e2e", "results", "global", "junit_01.xml")
	if !fileutil.Exist(xmlPath) {
		return "", "", fmt.Errorf("result dir %q does not have junit_01.xml %q", outputDir, xmlPath)
	}
	return logPath, xmlPath, nil
}
