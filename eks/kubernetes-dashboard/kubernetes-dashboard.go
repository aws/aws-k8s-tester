// Package kubernetesdashboard implements Kubernetes dashboard add-on.
package kubernetesdashboard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/httputil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"go.uber.org/zap"
)

// Config defines Dashboard configuration.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}
	Sig    chan os.Signal

	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

// Tester defines Dashboard tester
type Tester interface {
	// Create installs Dashboard.
	Create() error
	// Delete deletes Dashboard.
	Delete() error
}

func NewTester(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg         Config
	proxyCmd    *exec.Cmd
	proxyCancel func()
}

func (ts *tester) Create() error {
	if ts.cfg.EKSConfig.AddOnKubernetesDashboard.Created {
		ts.cfg.Logger.Info("skipping create AddOnKubernetesDashboard")
		return nil
	}

	ts.cfg.EKSConfig.AddOnKubernetesDashboard.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()

	defer func() {
		ts.cfg.EKSConfig.AddOnKubernetesDashboard.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnKubernetesDashboard.CreateTookString = ts.cfg.EKSConfig.AddOnKubernetesDashboard.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := ts.installMetricsServer(); err != nil {
		return err
	}
	if err := ts.installDashboard(); err != nil {
		return err
	}
	if err := ts.installEKSAdmin(); err != nil {
		return err
	}
	// TODO: use ingress
	if err := ts.startProxy(true); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnKubernetesDashboard.Created {
		ts.cfg.Logger.Info("skipping delete AddOnKubernetesDashboard")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnKubernetesDashboard.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnKubernetesDashboard.DeleteTookString = ts.cfg.EKSConfig.AddOnKubernetesDashboard.DeleteTook.String()
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
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) startProxy(dry bool) error {
	args := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"proxy",
	}

	if !dry {
		ts.cfg.Logger.Info("starting Kubernetes Dashboard proxy", zap.String("cmd-path", ts.cfg.EKSConfig.KubectlPath))
		ctx, cancel := context.WithCancel(context.Background())
		ts.proxyCmd = exec.CommandContext(ctx, args[0], args[1:]...)
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
			case sig := <-ts.cfg.Sig:
				return fmt.Errorf("received os signal %v", sig)
			case <-time.After(5 * time.Second):
			}

			out, err := httputil.ReadInsecure(ts.cfg.Logger, os.Stderr, ts.cfg.EKSConfig.AddOnKubernetesDashboard.URL)
			if err != nil {
				ts.cfg.Logger.Warn("failed to read Kubernetes Dashboard proxy; retrying", zap.Error(err))
				time.Sleep(5 * time.Second)
				continue
			}
			httpOutput := string(out)
			fmt.Printf("\nKubernetes Dashboard proxy output:\n%s\n", httpOutput)

			if strings.Contains(httpOutput, `The Kubernetes Authors.`) || true {
				ts.cfg.Logger.Info("read Kubernetes Dashboard proxy; exiting")
				break
			}

			ts.cfg.Logger.Warn("unexpected Kubernetes Dashboard proxy output; retrying")
		}
	}

	fmt.Printf("\nkubectl proxy command:\n%s\n", strings.Join(args, " "))
	fmt.Printf("\nKubernetes Dashboard Token:\n%s\n", ts.cfg.EKSConfig.AddOnKubernetesDashboard.AuthenticationToken)
	fmt.Printf("\nKubernetes Dashboard URL:\n%s\n\n", ts.cfg.EKSConfig.AddOnKubernetesDashboard.URL)

	return ts.cfg.EKSConfig.Sync()
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
