package kubeflow

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"go.uber.org/zap"
)

func Test_downloadInstallKfctl(t *testing.T) {
	tt := &tester{cfg: Config{
		Logger:    zap.NewExample(),
		EKSConfig: eksconfig.NewDefault(),
	}}
	os.RemoveAll(tt.cfg.EKSConfig.AddOnKubeflow.KfctlPath)
	if err := tt.downloadInstallKfctl(); err != nil {
		t.Fatal(err)
	}
}

func Test_writeKfctlConfig(t *testing.T) {
	tt := &tester{cfg: Config{
		Logger:    zap.NewExample(),
		EKSConfig: eksconfig.NewDefault(),
	}}
	f, err := ioutil.TempFile(os.TempDir(), fmt.Sprintf("%X", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	tt.cfg.EKSConfig.AddOnKubeflow.KfctlConfigPath = f.Name()
	f.Close()
	os.RemoveAll(tt.cfg.EKSConfig.AddOnKubeflow.KfctlConfigPath)
	if err := tt.writeKfctlConfig(); err != nil {
		t.Fatal(err)
	}
}
