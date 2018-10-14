package eks

import (
	"fmt"
	"sync"
	"time"

	"github.com/aws/awstester/eksconfig"
	"github.com/aws/awstester/eksdeployer"
	"github.com/aws/awstester/internal/eks/alb"
	"github.com/aws/awstester/internal/eks/s3"
	"github.com/aws/awstester/pkg/zaputil"

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

// NewAWSCLIDeployer creates a new EKS deployer with AWS CLI.
func NewAWSCLIDeployer(cfg *eksconfig.Config) (eksdeployer.Interface, error) {
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		return nil, err
	}

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

func (ac *awsCli) DumpClusterLogs(localPath, remotePath string) error {
	return ac.s3Plugin.UploadToBucketForTests(localPath, remotePath)
}

func (ac *awsCli) Publish() error {
	// TODO: follower the pattern implemented in 'embedded'
	return ac.s3Plugin.UploadToBucketForTests(
		ac.cfg.LogOutputToUploadPath,
		ac.cfg.LogOutputToUploadPathBucket,
	)
}

///////////////////////////////////////////////
// Extra methods for EKS specific operations //
///////////////////////////////////////////////

func (ac *awsCli) Stop() {
	panic("TODO")
}

func (ac *awsCli) LoadConfig() (eksconfig.Config, error) {
	panic("TODO")
}

func (ac *awsCli) TestCorrectness() error {
	panic("TODO")
}

func (ac *awsCli) TestQPS() error {
	panic("TODO")
}

func (ac *awsCli) TestMetrics() error {
	panic("TODO")
}
