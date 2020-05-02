package conformance

import (
	"os"
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
