// Package conformance implements Conformance tests.
// ref. https://github.com/cncf/k8s-conformance/blob/master/instructions.md
// ref. https://github.com/vmware-tanzu/sonobuoy
package conformance

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/httputil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/mholt/archiver/v3"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

// Config defines Conformance configuration.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}

	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

// Tester defines Conformance tester.
type Tester interface {
	// Create installs Conformance.
	Create() error
	// Delete deletes Conformance.
	Delete() error
}

func NewTester(cfg Config) (Tester, error) {
	return &tester{cfg: cfg, donec: make(chan struct{})}, nil
}

type tester struct {
	cfg   Config
	donec chan struct{}
}

func (ts *tester) Create() error {
	if ts.cfg.EKSConfig.AddOnConformance.Created {
		ts.cfg.Logger.Info("skipping create AddOnConformance")
		return nil
	}

	ts.cfg.Logger.Info("starting conformance tests")
	ts.cfg.EKSConfig.AddOnConformance.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()

	defer func() {
		ts.cfg.EKSConfig.AddOnConformance.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnConformance.CreateTookString = ts.cfg.EKSConfig.AddOnConformance.CreateTook.String()
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
	if err := ts.getResultsSonobuoy(); err != nil {
		return err
	}

	return nil
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnConformance.Created {
		ts.cfg.Logger.Info("skipping delete AddOnConformance")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnConformance.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnConformance.DeleteTookString = ts.cfg.EKSConfig.AddOnConformance.DeleteTook.String()
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
	return ts.cfg.EKSConfig.Sync()
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
	fmt.Printf("\n'sonobuoy help' output:\n\n%s\n\n", out)

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(ctx, ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath, "run", "--help").CombinedOutput()
	cancel()
	out = strings.TrimSpace(string(output))
	if err != nil {
		return fmt.Errorf("'sonobuoy run --help' failed (output %q, error %v)", out, err)
	}
	fmt.Printf("\n'sonobuoy run --help' output:\n\n%s\n\n", out)

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(ctx, ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath, "version").CombinedOutput()
	cancel()
	out = strings.TrimSpace(string(output))
	if err != nil {
		return fmt.Errorf("'sonobuoy version' failed (output %q, error %v)", out, err)
	}
	fmt.Printf("\n'sonobuoy version' output:\n\n%s\n\n", out)

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
	fmt.Printf("\n'%s' output:\n\n%s\n\n", cmd, out)

	ts.cfg.Logger.Info("deleted sonobuoy", zap.String("command", cmd))
	return nil
}

func (ts *tester) runSonobuoy() (err error) {
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
	}
	cmd := strings.Join(args, " ")

	ts.cfg.Logger.Info("running sonobuoy", zap.String("command", cmd))

	var output []byte
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute) // do not wait, so just set timeout for launching tests
	output, err = exec.New().CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
	cancel()
	out := strings.TrimSpace(string(output))
	if err != nil {
		// TODO: check error
		ts.cfg.Logger.Warn("failed to run sonobuoy", zap.String("command", cmd), zap.Error(err))
	}
	fmt.Printf("\n'%s' output:\n\n%s\n\n", cmd, out)

	ts.cfg.Logger.Info("ran sonobuoy", zap.String("command", cmd))
	return nil
}

func (ts *tester) checkSonobuoy() (err error) {
	argsLogs := []string{
		ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath,
		"--logtostderr",
		"--alsologtostderr",
		"--v=3",
		"logs",
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnConformance.Namespace,
	}
	cmdLogs := strings.Join(argsLogs, " ")

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
		zap.String("logs-command", cmdLogs),
		zap.String("status-command", cmdStatus),
	)

	donec := time.After(ts.cfg.EKSConfig.AddOnConformance.SonobuoyRunTimeout)
	start, waitDur := time.Now(), ts.cfg.EKSConfig.AddOnConformance.SonobuoyRunTimeout
	for time.Now().Sub(start) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("sonobuoy check stopped")
			return nil
		case <-donec:
			ts.cfg.Logger.Warn("sonobuoy check timeout")
			return fmt.Errorf("sonobuoy run took too long (exceeded %v)", ts.cfg.EKSConfig.AddOnConformance.SonobuoyRunTimeout)
		case <-time.After(time.Minute):
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		output, err := exec.New().CommandContext(ctx, argsLogs[0], argsLogs[1:]...).CombinedOutput()
		cancel()
		out := strings.TrimSpace(string(output))
		if err != nil {
			ts.cfg.Logger.Warn("failed to run sonobuoy logs", zap.String("command", cmdLogs), zap.Error(err))
		}
		fmt.Printf("\n'%s' output:\n\n%s\n\n", cmdLogs, out)

		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		output, err = exec.New().CommandContext(ctx, argsStatus[0], argsStatus[1:]...).CombinedOutput()
		cancel()
		out = strings.TrimSpace(string(output))
		if err != nil {
			ts.cfg.Logger.Warn("failed to run sonobuoy status", zap.String("command", cmdStatus), zap.Error(err))
		}
		fmt.Printf("\n'%s' output:\n\n%s\n\n", cmdStatus, out)

		// ref. https://github.com/vmware-tanzu/sonobuoy/blob/master/cmd/sonobuoy/app/status.go
		if strings.Contains(out, "Sonobuoy has completed. ") || strings.Contains(out, "Sonobuoy plugins have completed. ") {
			break
		}
		if strings.Contains(out, "Sonobuoy has failed. ") || strings.Contains(out, "Sonobuoy is in unknown state") {
			return errors.New("sonobuoy run failed")
		}
	}

	return ts.cfg.EKSConfig.Sync()
}

/*
/tmp/sonobuoy status --kubeconfig={{ .KubeConfigPath }}
/tmp/sonobuoy logs -f --kubeconfig={{ .KubeConfigPath }}
/tmp/sonobuoy results --kubeconfig={{ .KubeConfigPath }}

SONOBUOY_OUTPUT=$(/tmp/sonobuoy retrieve --kubeconfig={{ .KubeConfigPath }})
mkdir -p /tmp/results
tar xzf $SONOBUOY_OUTPUT -C /tmp/results
find /tmp/results
*/

func (ts *tester) getResultsSonobuoy() (err error) {
	argsRetrieve := []string{
		ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath,
		"retrieve",
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnConformance.Namespace,
		ts.cfg.EKSConfig.AddOnConformance.SonobuoyRetrievePath,
	}
	cmdRetrieve := strings.Join(argsRetrieve, " ")

	argsResults := []string{
		ts.cfg.EKSConfig.AddOnConformance.SonobuoyPath,
		"results",
		ts.cfg.EKSConfig.AddOnConformance.SonobuoyRetrievePath,
		"--mode=detailed",
	}
	cmdResults := strings.Join(argsResults, " ")

	ts.cfg.Logger.Info("running sonobuoy",
		zap.String("retrieve-command", cmdRetrieve),
		zap.String("results-command", cmdResults),
	)

	os.RemoveAll(ts.cfg.EKSConfig.AddOnConformance.SonobuoyRetrievePath)
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
		fmt.Printf("\n'%s' output:\n\n%s\n\n", cmdRetrieve, out)
		break
	}

	if !fileutil.Exist(ts.cfg.EKSConfig.AddOnConformance.SonobuoyRetrievePath) {
		return fmt.Errorf("AddOnConformance.SonobuoyRetrievePath does not exist [%q]", ts.cfg.EKSConfig.AddOnConformance.SonobuoyRetrievePath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	output, err := exec.New().CommandContext(ctx, argsResults[0], argsResults[1:]...).CombinedOutput()
	cancel()
	out := strings.TrimSpace(string(output))
	if err != nil {
		ts.cfg.Logger.Warn("failed to run sonobuoy results", zap.String("command", cmdResults), zap.Error(err))
		return
	}
	fmt.Printf("\n'%s' output:\n\n%s\n\n", cmdResults, out)

	return nil
}
