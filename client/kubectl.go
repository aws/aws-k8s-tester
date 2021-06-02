package client

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/aws/aws-k8s-tester/utils/file"
	utils_http "github.com/aws/aws-k8s-tester/utils/http"
	"go.uber.org/zap"
)

// ref. https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/

var (
	// curl -L -s https://dl.k8s.io/release/stable.txt
	defaultKubectlVersion = "v1.21.1"
	defaultKubectlPath    = fmt.Sprintf("/tmp/kubectl-%s", defaultKubectlVersion)
	// e.g., https://dl.k8s.io/release/v1.21.0/bin/linux/amd64/kubectl
	defaultKubectlDownloadURL = fmt.Sprintf(
		"https://dl.k8s.io/release/%s/bin/%s/%s/kubectl",
		defaultKubectlVersion,
		runtime.GOOS,
		runtime.GOARCH,
	)
)

func init() {
	lg := zap.NewExample()
	// curl -L -s https://dl.k8s.io/release/stable.txt
	// ref. https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/
	bb, err := utils_http.ReadInsecure(lg, os.Stderr, "https://dl.k8s.io/release/stable.txt")
	if err == nil {
		sv := strings.TrimSpace(string(bb))
		lg.Info("successfully fetched the latest Kubernetes release version", zap.String("version", sv))
		defaultKubectlVersion = sv
	} else {
		lg.Warn("failed to fetch latest Kubernetes release version; reverting to default version",
			zap.String("default-kubectl-version", defaultKubectlVersion),
			zap.Error(err),
		)
	}
}

func DefaultKubectlPath() string {
	return defaultKubectlPath
}

func DefaultKubectlDownloadURL() string {
	return defaultKubectlDownloadURL
}

func installKubectl(lg *zap.Logger, kubectlPath string, kubectlDownloadURL string) (err error) {
	lg.Info("mkdir", zap.String("kubectl-path-dir", filepath.Dir(kubectlPath)))
	if err = os.MkdirAll(filepath.Dir(kubectlPath), 0700); err != nil {
		lg.Warn("could not create", zap.String("dir", filepath.Dir(kubectlPath)), zap.Error(err))
		return err
	}
	if !file.Exist(kubectlPath) {
		if kubectlDownloadURL == "" {
			lg.Warn("kubectl path does not exist, kubectl download URL empty", zap.String("kubectl-path", kubectlPath))
			return fmt.Errorf("kubectl path %q does not exist and empty kubectl download URL", kubectlPath)
		}
		kubectlPath, _ = filepath.Abs(kubectlPath)
		lg.Info("downloading kubectl", zap.String("kubectl-path", kubectlPath))
		if err := utils_http.Download(lg, os.Stderr, kubectlDownloadURL, kubectlPath); err != nil {
			lg.Warn("failed to download kubectl", zap.Error(err))
			return err
		}
	} else {
		lg.Info("skipping kubectl download; already exist", zap.String("kubectl-path", kubectlPath))
	}
	if err = file.EnsureExecutable(kubectlPath); err != nil {
		// file may be already executable while the process does not own the file/directory
		// ref. https://github.com/aws/aws-k8s-tester/issues/66
		lg.Warn("failed to ensure executable", zap.Error(err))
		err = nil
	}
	return err
}
