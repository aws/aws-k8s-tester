package eks

import (
	"fmt"
	"os/exec"
	"reflect"
	"time"

	"github.com/aws/aws-k8s-tester/eksdeployer"
)

// NewTester creates a new EKS tester.
func NewTester(timeout time.Duration, verbose bool) (eksdeployer.Tester, error) {
	dp, err := NewDeployer(timeout, verbose)
	if err != nil {
		return nil, err
	}
	d, ok := dp.(*deployer)
	if !ok {
		return nil, fmt.Errorf("expected *deployer, got %v", reflect.TypeOf(dp))
	}
	return &tester{deployer: d}, nil
}

type tester struct {
	*deployer
}

func (tr *tester) TestALBCorrectness() (err error) {
	if _, err = tr.LoadConfig(); err != nil {
		return err
	}
	_, err = tr.ctrl.Output(exec.Command(
		tr.awsK8sTesterPath,
		"eks",
		"--path="+tr.cfg.ConfigPath,
		"test", "alb", "correctness",
	))
	return err
}

func (tr *tester) TestALBQPS() (err error) {
	if _, err = tr.LoadConfig(); err != nil {
		return err
	}
	_, err = tr.ctrl.Output(exec.Command(
		tr.awsK8sTesterPath,
		"eks",
		"--path="+tr.cfg.ConfigPath,
		"test", "alb", "qps",
	))
	return err
}

func (tr *tester) TestALBMetrics() (err error) {
	if _, err = tr.LoadConfig(); err != nil {
		return err
	}
	_, err = tr.ctrl.Output(exec.Command(
		tr.awsK8sTesterPath,
		"eks",
		"--path="+tr.cfg.ConfigPath,
		"test", "alb", "metrics",
	))
	return err
}

// UploadToBucketForTests uploads a local file to aws-k8s-tester S3 bucket.
func (tr *tester) UploadToBucketForTests(localPath, s3Path string) (err error) {
	_, err = tr.ctrl.Output(exec.Command(
		tr.awsK8sTesterPath,
		"eks",
		"--path="+tr.cfg.ConfigPath,
		"s3-upload",
		localPath,
		s3Path,
	))
	return err
}
