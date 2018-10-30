package eks

import (
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/eksdeployer"
	"github.com/aws/aws-k8s-tester/internal/eks/alb"
	"github.com/aws/aws-k8s-tester/internal/eks/s3"
	"github.com/aws/aws-k8s-tester/pkg/zaputil"

	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

type awsCli struct {
	stopc chan struct{}

	mu  sync.RWMutex
	lg  *zap.Logger
	cfg *eksconfig.Config

	// for plugins, sub-project implementation
	s3Plugin  s3.Plugin
	albPlugin alb.Plugin

	// TODO: add EBS (with CSI) plugin
	// TODO: add KMS plugin
}

// newTesterAWSCLI creates a new EKS tester with AWS CLI.
func newTesterAWSCLI(cfg *eksconfig.Config) (eksdeployer.Tester, error) {
	_, err := zaputil.New(cfg.LogDebug, cfg.LogOutputs)
	if err != nil {
		return nil, err
	}

	_, err = exec.New().LookPath("kubectl")
	if err != nil {
		return nil, fmt.Errorf("cannot find 'kubectl' executable (%v)", err)
	}
	if _, err = exec.New().LookPath("aws-iam-authenticator"); err != nil {
		return nil, fmt.Errorf("cannot find 'aws-iam-authenticator' executable (%v)", err)
	}

	panic("TODO")
}

func (ac *awsCli) Up() (err error) {
	panic("TODO")
}

func (ac *awsCli) Down() (err error) {
	panic("TODO")
}

func (ac *awsCli) IsUp() (err error) {
	panic("TODO")
}

func (ac *awsCli) TestSetup() (err error) {
	panic("TODO")
}

func (ac *awsCli) GetClusterCreated(v string) (time.Time, error) {
	panic("TODO")
}

// DumpClusterLogs dumps all logs to artifact directory.
// Let default kubetest log dumper handle all artifact uploads.
// See https://github.com/kubernetes/test-infra/pull/9811/files#r225776067.
func (ac *awsCli) DumpClusterLogs(artifactDir, _ string) error {
	panic("TODO")
}

func (ac *awsCli) UploadToBucketForTests(localPath, remotePath string) error {
	return ac.s3Plugin.UploadToBucketForTests(localPath, remotePath)
}

func (ac *awsCli) Stop() {
	panic("TODO")
}

func (ac *awsCli) LoadConfig() (eksconfig.Config, error) {
	panic("TODO")
}

func (ac *awsCli) TestALBCorrectness() error {
	panic("TODO")
}

func (ac *awsCli) TestALBQPS() error {
	panic("TODO")
}

func (ac *awsCli) TestALBMetrics() error {
	panic("TODO")
}
