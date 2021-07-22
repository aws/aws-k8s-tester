package conformance

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/utils/file"
	utils_http "github.com/aws/aws-k8s-tester/utils/http"
	archive "github.com/mholt/archiver/v3"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

var (
	// ref. https://github.com/vmware-tanzu/sonobuoy/releases
	defaultSonobuoyVersion = "0.52.0"
	defaultSonobuoyPath    = fmt.Sprintf("/tmp/sonobuoy-%s", defaultSonobuoyVersion)
	// ref. https://github.com/vmware-tanzu/sonobuoy/releases
	// e.g., https://github.com/vmware-tanzu/sonobuoy/releases/download/v0.50.0/sonobuoy_0.50.0_linux_arm64.tar.gz
	defaultSonobuoyDownloadURL = fmt.Sprintf(
		"https://github.com/vmware-tanzu/sonobuoy/releases/download/v%s/sonobuoy_%s_%s_%s.tar.gz",
		defaultSonobuoyVersion,
		defaultSonobuoyVersion,
		runtime.GOOS,
		runtime.GOARCH,
	)
)

func DefaultSonobuoyPath() string {
	return defaultSonobuoyPath
}

func DefaultSonobuoyDownloadURL() string {
	// darwin, arm64 not available
	// ref. https://github.com/vmware-tanzu/sonobuoy/releases
	if strings.Contains(defaultSonobuoyDownloadURL, "darwin_arm64") {
		defaultSonobuoyDownloadURL = strings.ReplaceAll(defaultSonobuoyDownloadURL, "darwin_arm64", "darwin_amd64")
	}
	return defaultSonobuoyDownloadURL
}

func installSonobuoy(lg *zap.Logger, sonobuoyPath string, sonobuoyDownloadURL string) (err error) {
	lg.Info("mkdir", zap.String("sonobuoy-path-dir", filepath.Dir(sonobuoyPath)))
	if err = os.MkdirAll(filepath.Dir(sonobuoyPath), 0700); err != nil {
		lg.Warn("could not create", zap.String("dir", filepath.Dir(sonobuoyPath)), zap.Error(err))
		return err
	}
	if !file.Exist(sonobuoyPath) {
		if sonobuoyDownloadURL == "" {
			lg.Warn("sonobuoy path does not exist, sonobuoy download URL empty", zap.String("sonobuoy-path", sonobuoyPath))
			return fmt.Errorf("sonobuoy path %q does not exist and empty sonobuoy download URL", sonobuoyPath)
		}
		sonobuoyPath, _ = filepath.Abs(sonobuoyPath)
		sonobuoyTarGzPath := filepath.Join(os.TempDir(), fmt.Sprintf("sonobuoy-%x.tar.gz", time.Now().UnixNano()))
		defer os.RemoveAll(sonobuoyTarGzPath)
		lg.Info("downloading sonobuoy", zap.String("sonobuoy-path", sonobuoyPath), zap.String("tar-gz-path", sonobuoyTarGzPath))
		if err := utils_http.Download(lg, os.Stderr, sonobuoyDownloadURL, sonobuoyTarGzPath); err != nil {
			lg.Warn("failed to download sonobuoy", zap.Error(err))
			return err
		}
		tmpDir, err := ioutil.TempDir(os.TempDir(), "sonobuoy")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)
		if err = archive.Unarchive(sonobuoyTarGzPath, tmpDir); err != nil {
			return fmt.Errorf("failed to decompress sonobuoy tar file %v", err)
		}
		if err = file.Copy(filepath.Join(tmpDir, "sonobuoy"), sonobuoyPath); err != nil {
			return fmt.Errorf("failed to copy file %v", err)
		}
	} else {
		lg.Info("skipping sonobuoy download; already exist", zap.String("sonobuoy-path", sonobuoyPath))
	}
	if err = file.EnsureExecutable(sonobuoyPath); err != nil {
		// file may be already executable while the process does not own the file/directory
		// ref. https://github.com/aws/aws-k8s-tester/issues/66
		lg.Warn("failed to ensure executable", zap.Error(err))
		err = nil
	}

	var output []byte
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(ctx, sonobuoyPath, "help").CombinedOutput()
	cancel()
	out := strings.TrimSpace(string(output))
	if err != nil {
		return fmt.Errorf("'%s help' failed (output %q, error %v)", sonobuoyPath, out, err)
	}
	fmt.Fprintf(os.Stderr, "\n'%s help' output:\n\n%s\n\n", sonobuoyPath, out)

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(ctx, sonobuoyPath, "run", "--help").CombinedOutput()
	cancel()
	out = strings.TrimSpace(string(output))
	if err != nil {
		return fmt.Errorf("'%s run --help' failed (output %q, error %v)", sonobuoyPath, out, err)
	}
	fmt.Fprintf(os.Stderr, "\n'%s run --help' output:\n\n%s\n\n", sonobuoyPath, out)

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(ctx, sonobuoyPath, "version").CombinedOutput()
	cancel()
	out = strings.TrimSpace(string(output))
	if err != nil {
		return fmt.Errorf("'%s version' failed (output %q, error %v)", sonobuoyPath, out, err)
	}
	fmt.Fprintf(os.Stderr, "\n'%s version' output:\n\n%s\n\n", sonobuoyPath, out)

	return nil
}

func readResults(lg *zap.Logger, logWriter io.Writer, sonobuoyPath string, tarGzPath string) error {
	if !file.Exist(tarGzPath) {
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
	if !file.Exist(tarGzPath) {
		return "", "", fmt.Errorf("sonobuoy result tar.gz file does not exist [%q]", tarGzPath)
	}

	err = archive.Unarchive(tarGzPath, outputDir)
	if err != nil {
		return "", "", fmt.Errorf("failed to decompress sonobuoy results tar file %v", err)
	}
	lg.Info("untar success", zap.String("tar-gz-path", tarGzPath), zap.String("output-directory", outputDir))

	logPath = filepath.Join(outputDir, "plugins", "e2e", "results", "global", "e2e.log")
	if !file.Exist(logPath) {
		return "", "", fmt.Errorf("result dir %q does not have e2e.log %q", outputDir, logPath)
	}
	xmlPath = filepath.Join(outputDir, "plugins", "e2e", "results", "global", "junit_01.xml")
	if !file.Exist(xmlPath) {
		return "", "", fmt.Errorf("result dir %q does not have junit_01.xml %q", outputDir, xmlPath)
	}
	return logPath, xmlPath, nil
}
