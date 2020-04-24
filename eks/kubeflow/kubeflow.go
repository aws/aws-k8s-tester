// Package kubeflow implements kubeflow add-on.
// ref. https://www.kubeflow.org/docs/aws/deploy/install-kubeflow/
package kubeflow

import (
	"context"
	"errors"
	"fmt"
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

// Config defines Kubeflow configuration.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}
	Sig    chan os.Signal

	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

// Tester defines Kubeflow tester
type Tester interface {
	// Create installs Kubeflow.
	Create() error
	// Delete deletes Kubeflow.
	Delete() error
}

func NewTester(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg Config
}

func (ts *tester) Create() error {
	if ts.cfg.EKSConfig.AddOnKubeflow.Created {
		ts.cfg.Logger.Info("skipping create AddOnKubeflow")
		return nil
	}

	ts.cfg.EKSConfig.AddOnKubeflow.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()

	defer func() {
		ts.cfg.EKSConfig.AddOnKubeflow.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnKubeflow.CreateTookString = ts.cfg.EKSConfig.AddOnKubeflow.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := ts.downloadInstallKfctl(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnKubeflow.Created {
		ts.cfg.Logger.Info("skipping delete AddOnKubeflow")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnKubeflow.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnKubeflow.DeleteTookString = ts.cfg.EKSConfig.AddOnKubeflow.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnKubeflow.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) downloadInstallKfctl() (err error) {
	ts.cfg.Logger.Info("mkdir", zap.String("kfctl-path-dir", filepath.Dir(ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath)))
	if err = os.MkdirAll(filepath.Dir(ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath), 0700); err != nil {
		return fmt.Errorf("could not create %q (%v)", filepath.Dir(ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath), err)
	}

	if !fileutil.Exist(ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath) {
		tarPath := filepath.Join(os.TempDir(), fmt.Sprintf("kfctl-%x.tar.gz", time.Now().UnixNano()))
		if err = httputil.Download(ts.cfg.Logger, os.Stderr, ts.cfg.EKSConfig.AddOnKubeflow.KfctlDownloadURL, tarPath); err != nil {
			return err
		}
		tmpPath := filepath.Join(os.TempDir(), "kfctl")
		os.RemoveAll(tmpPath)
		if err = archiver.Unarchive(tarPath, os.TempDir()); err != nil {
			return fmt.Errorf("failed to decompress kfctl tar file %v", err)
		}
		if err = fileutil.Copy(tmpPath, ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath); err != nil {
			return fmt.Errorf("failed to copy file %v", err)
		}
	} else {
		ts.cfg.Logger.Info("skipping kfctl download; already exist", zap.String("kfctl-path", ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath))
	}

	if err = fileutil.EnsureExecutable(ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath); err != nil {
		// file may be already executable while the process does not own the file/directory
		// ref. https://github.com/aws/aws-k8s-tester/issues/66
		ts.cfg.Logger.Warn("failed to ensure executable", zap.Error(err))
		err = nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	var output []byte
	output, err = exec.New().CommandContext(ctx, ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath, "version").CombinedOutput()
	cancel()
	out := strings.TrimSpace(string(output))
	if err != nil {
		return fmt.Errorf("'kfctl version' failed (output %q, error %v)", out, err)
	}
	ts.cfg.Logger.Info(
		"kfctl version",
		zap.String("kfctl-path", ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath),
		zap.String("kfctl-version", out),
	)

	return nil
}
