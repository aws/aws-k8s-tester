package kubeflow

import (
	"os"
	"testing"

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
