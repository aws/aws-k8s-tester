// Package kubeflow implements kubeflow add-on.
// ref. https://www.kubeflow.org/docs/aws/deploy/install-kubeflow/
package kubeflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
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
		ts.cfg.Logger.Info("downloading kfctl", zap.String("kfctl-path", ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath))
		var f *os.File
		f, err = os.Create(ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath)
		if err != nil {
			return fmt.Errorf("failed to create %q (%v)", ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath, err)
		}
		ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath = f.Name()
		ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath, _ = filepath.Abs(ts.cfg.EKSConfig.AddOnKubeflow.KfctlPath)
		if err := httpDownloadFile(ts.cfg.Logger, ts.cfg.EKSConfig.AddOnKubeflow.KfctlDownloadURL, f); err != nil {
			f.Close()
			return err
		}
		if err = f.Close(); err != nil {
			return fmt.Errorf("failed to close kfctl %v", err)
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
	out := string(output)
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

var httpFileTransport *http.Transport

func init() {
	httpFileTransport = new(http.Transport)
	httpFileTransport.RegisterProtocol("file", http.NewFileTransport(http.Dir("/")))
}

// curl -L [URL] | writer
func httpDownloadFile(lg *zap.Logger, u string, wr io.Writer) error {
	lg.Info("downloading", zap.String("url", u))
	cli := &http.Client{Transport: httpFileTransport}
	r, err := cli.Get(u)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode >= 400 {
		return fmt.Errorf("%q returned %d", u, r.StatusCode)
	}

	_, err = io.Copy(wr, r.Body)
	if err != nil {
		lg.Warn("failed to download", zap.String("url", u), zap.Error(err))
	} else {
		if f, ok := wr.(*os.File); ok {
			lg.Info("downloaded",
				zap.String("url", u),
				zap.String("file-path", f.Name()),
			)
		} else {
			lg.Info("downloaded",
				zap.String("url", u),
				zap.String("value-of", reflect.ValueOf(wr).String()),
			)
		}
	}
	return err
}
