// Package kubernetesdashboard implements Kubernetes dashboard add-on.
package kubernetesdashboard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"syscall"
	"time"

	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/httputil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"go.uber.org/zap"
)

// Config defines Dashboard configuration.
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg         Config
	proxyCmd    *exec.Cmd
	proxyCancel func()
}

func (ts *tester) Create() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnKubernetesDashboard() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnKubernetesDashboard.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnKubernetesDashboard.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnKubernetesDashboard.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err := ts.installDashboard(); err != nil {
		return err
	}
	if err := ts.installEKSAdmin(); err != nil {
		return err
	}
	// TODO: use ingress
	if err := ts.startProxy(false); err != nil {
		return err
	}

	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnKubernetesDashboard() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnKubernetesDashboard.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnKubernetesDashboard.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.stopProxy(); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnKubernetesDashboard.Created = false
	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) startProxy(enable bool) error {
	proxyArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"proxy",
	}
	proxyCmd := strings.Join(proxyArgs, " ")

	if enable {
		ts.cfg.Logger.Info("starting Kubernetes Dashboard proxy", zap.String("cmd-path", ts.cfg.EKSConfig.KubectlPath))
		ctx, cancel := context.WithCancel(context.Background())
		ts.proxyCmd = exec.CommandContext(ctx, proxyArgs[0], proxyArgs[1:]...)
		ts.proxyCmd.Stderr = os.Stderr
		ts.proxyCmd.Stdout = os.Stdout
		ts.proxyCancel = cancel
		if err := ts.proxyCmd.Start(); err != nil {
			ts.cfg.Logger.Warn("failed to start kubectl proxy command", zap.Error(err))
			ts.proxyCancel()
			if ts.proxyCmd.Process != nil {
				ts.proxyCmd.Process.Kill()
			}
			return err
		}
		ts.cfg.EKSConfig.AddOnKubernetesDashboard.KubectlProxyPID = ts.proxyCmd.Process.Pid
		ts.cfg.Logger.Info("started Kubernetes Dashboard proxy", zap.Int("pid", ts.cfg.EKSConfig.AddOnKubernetesDashboard.KubectlProxyPID))

		waitDur := time.Minute
		retryStart := time.Now()
		for time.Now().Sub(retryStart) < waitDur {
			select {
			case <-ts.cfg.Stopc:
				return errors.New("Kubernetes Dashboard proxy creation aborted")
			case <-time.After(5 * time.Second):
			}

			out, err := httputil.ReadInsecure(ts.cfg.Logger, ioutil.Discard, ts.cfg.EKSConfig.AddOnKubernetesDashboard.URL)
			if err != nil {
				ts.cfg.Logger.Warn("failed to read Kubernetes Dashboard proxy; retrying", zap.Error(err))
				time.Sleep(5 * time.Second)
				continue
			}
			httpOutput := string(out)
			fmt.Fprintf(ts.cfg.LogWriter, "\nKubernetes Dashboard proxy output:\n%s\n", httpOutput)

			if strings.Contains(httpOutput, `The Kubernetes Authors`) {
				ts.cfg.Logger.Info("read Kubernetes Dashboard proxy; exiting")
				break
			}

			ts.cfg.Logger.Warn("unexpected Kubernetes Dashboard proxy output; retrying")
		}
	}

	fmt.Fprintf(ts.cfg.LogWriter, "\nkubectl proxy command:\n%s\n", proxyCmd)
	fmt.Fprintf(ts.cfg.LogWriter, "\nKubernetes Dashboard Token:\n%s\n", ts.cfg.EKSConfig.AddOnKubernetesDashboard.AuthenticationToken)
	fmt.Fprintf(ts.cfg.LogWriter, "\nKubernetes Dashboard URL:\n%s\n\n", ts.cfg.EKSConfig.AddOnKubernetesDashboard.URL)

	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) stopProxy() error {
	if ts.proxyCmd == nil || ts.cfg.EKSConfig.AddOnKubernetesDashboard.KubectlProxyPID == 0 {
		return nil
	}

	ts.cfg.Logger.Info("stopping Kubernetes Dashboard proxy")

	if ts.proxyCancel != nil {
		ts.proxyCancel()
	}

	if ts.proxyCmd != nil && ts.proxyCmd.Process != nil {
		err := ts.proxyCmd.Process.Kill()
		if err != nil {
			ts.cfg.Logger.Warn("proxyCmd.Process.Kill failed", zap.Error(err))
		} else {
			ts.cfg.Logger.Info("ran proxyCmd.Process.Kill")
		}
	}

	if ts.cfg.EKSConfig.AddOnKubernetesDashboard.KubectlProxyPID != 0 {
		err := syscall.Kill(-ts.cfg.EKSConfig.AddOnKubernetesDashboard.KubectlProxyPID, syscall.SIGKILL)
		if err != nil {
			ts.cfg.Logger.Warn("syscall.Kill failed", zap.Error(err))
		} else {
			ts.cfg.Logger.Info("ran syscall.Kill")
		}
	}

	ts.cfg.Logger.Info("stopped Kubernetes Dashboard proxy")

	return nil
}
