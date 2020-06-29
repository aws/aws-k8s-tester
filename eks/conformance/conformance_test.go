package conformance

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"go.uber.org/zap"
)

func Test_downloadInstallSonobuoy(t *testing.T) {
	tt := &tester{cfg: Config{
		Logger:    zap.NewExample(),
		EKSConfig: eksconfig.NewDefault(),
	}}
	os.RemoveAll(tt.cfg.EKSConfig.AddOnConformance.SonobuoyPath)
	if err := tt.downloadInstallSonobuoy(); err != nil {
		t.Fatal(err)
	}
}

func Test_results(t *testing.T) {
	s, err := exec.LookPath("sonobuoy")
	if err != nil {
		t.Skip(err)
	}

	if err = readResults(zap.NewExample(), os.Stderr, s, "test-data/test.tar.gz"); err != nil {
		t.Fatal(err)
	}

	os.RemoveAll("test-data/output")
	defer os.RemoveAll("test-data/output")
	logPath, xmlPath, err := untarResults(zap.NewExample(), "test-data/test.tar.gz", "test-data/output")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(logPath)
	fmt.Println(xmlPath)
}
