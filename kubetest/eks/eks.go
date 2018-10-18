// Package eks implements 'kubetest' deployer and publisher interface.
// It uses 'awstester' and 'kubectl' binaries, rather than importing
// EKS packages directly. This is to eliminate the need of dependency
// management, both in upstream and downstream.
package eks

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/aws/awstester/eksconfig"
	"github.com/aws/awstester/eksdeployer"

	"k8s.io/test-infra/kubetest/process"
)

// tester implements EKS deployer using "awstester" binary.
// Satisfies "k8s.io/test-infra/kubetest/main.go" 'deployer' and 'publisher" interfaces.
// Reference https://github.com/kubernetes/test-infra/blob/master/kubetest/main.go.
type tester struct {
	stopc         chan struct{}
	cfg           *eksconfig.Config
	awsTesterPath string
	kubectlPath   string
}

// New creates a new EKS deployer with AWS CLI.
func New(cfg *eksconfig.Config) (eksdeployer.Interface, error) {
	cfg.Embedded = false
	tr := &tester{
		stopc: make(chan struct{}),
		cfg:   cfg,
	}

	var err error
	tr.awsTesterPath, err = exec.LookPath("awstester")
	if err != nil {
		return nil, fmt.Errorf("cannot find 'awstester' executable (%v)", err)
	}
	tr.kubectlPath, err = exec.LookPath("kubectl")
	if err != nil {
		return nil, fmt.Errorf("cannot find 'kubectl' executable (%v)", err)
	}
	// TODO(gyuho): replace this kubernetes native Go client
	_, err = exec.LookPath("aws-iam-authenticator")
	if err != nil {
		return nil, fmt.Errorf("cannot find 'aws-iam-authenticator' executable (%v)", err)
	}

	return tr, nil
}

// Up creates a new EKS cluster.
func (tr *tester) Up() (err error) {
	createCmd := exec.Command(
		tr.awsTesterPath,
		"eks",
		"--path="+tr.cfg.ConfigPath,
		"create",
		"cluster",
	)
	errc := make(chan error)
	go func() {
		ctrl := process.NewControl(
			tr.cfg.KubetestControlTimeout,
			time.NewTimer(tr.cfg.KubetestControlTimeout),
			time.NewTimer(tr.cfg.KubetestControlTimeout),
			tr.cfg.KubetestVerbose,
		)
		_, oerr := ctrl.Output(createCmd)
		errc <- oerr
	}()
	select {
	case <-tr.stopc:
		fmt.Fprintln(os.Stderr, "received stop signal, interrupting 'create cluster' command...")
		ierr := createCmd.Process.Signal(syscall.SIGINT)
		err = fmt.Errorf("'create cluster' command interrupted (interrupt error %v)", ierr)
	case err = <-errc:
	}
	return err
}

// Down tears down the existing EKS cluster.
func (tr *tester) Down() (err error) {
	ctrl := process.NewControl(
		tr.cfg.KubetestControlTimeout,
		time.NewTimer(tr.cfg.KubetestControlTimeout),
		time.NewTimer(tr.cfg.KubetestControlTimeout),
		tr.cfg.KubetestVerbose,
	)
	_, err = ctrl.Output(exec.Command(
		tr.awsTesterPath,
		"eks",
		"--path="+tr.cfg.ConfigPath,
		"delete",
		"cluster",
	))
	return err
}

// IsUp returns an error if the cluster is not up and running.
func (tr *tester) IsUp() (err error) {
	ctrl := process.NewControl(
		tr.cfg.KubetestControlTimeout,
		time.NewTimer(tr.cfg.KubetestControlTimeout),
		time.NewTimer(tr.cfg.KubetestControlTimeout),
		tr.cfg.KubetestVerbose,
	)
	_, err = ctrl.Output(exec.Command(
		tr.awsTesterPath,
		"eks",
		"--path="+tr.cfg.ConfigPath,
		"check",
		"cluster",
	))
	if err != nil {
		return err
	}

	tr.cfg, err = eksconfig.Load(tr.cfg.ConfigPath)
	if err != nil {
		return err
	}

	if tr.cfg.ClusterState.Status != "ACTIVE" {
		return fmt.Errorf("cluster %q status is %q",
			tr.cfg.ClusterName,
			tr.cfg.ClusterState.Status,
		)
	}
	return nil
}

// TestSetup checks if EKS testing cluster has been set up or not.
func (tr *tester) TestSetup() error {
	return tr.IsUp()
}

// GetClusterCreated returns EKS cluster creation time and error (if any).
func (tr *tester) GetClusterCreated(v string) (time.Time, error) {
	err := tr.IsUp()
	if err != nil {
		return time.Time{}, err
	}
	tr.cfg, err = eksconfig.Load(tr.cfg.ConfigPath)
	if err != nil {
		return time.Time{}, err
	}
	return tr.cfg.ClusterState.Created, nil
}

// DumpClusterLogs uploads local cluster logs to S3.
func (tr *tester) DumpClusterLogs(localPath, s3Path string) (err error) {
	if !tr.cfg.KubetestEnableDumpClusterLogs {
		return nil
	}

	ctrl := process.NewControl(
		tr.cfg.KubetestControlTimeout,
		time.NewTimer(tr.cfg.KubetestControlTimeout),
		time.NewTimer(tr.cfg.KubetestControlTimeout),
		tr.cfg.KubetestVerbose,
	)
	_, err = ctrl.Output(exec.Command(
		tr.awsTesterPath,
		"eks",
		"--path="+tr.cfg.ConfigPath,
		"s3-upload",
		localPath,
		s3Path,
	))
	return err
}

/*
// Publish publishes a success file.
// Add this when required... See "kubetest/e2e.go".
func (tr *tester) Publish() (err error) {
	tr.LoadConfig()
	logOutputS3 := tr.cfg.ClusterName + "/" + filepath.Base(tr.cfg.LogOutputToUploadPath)

	ctrl := process.NewControl(
		tr.cfg.KubetestControlTimeout,
		time.NewTimer(tr.cfg.KubetestControlTimeout),
		time.NewTimer(tr.cfg.KubetestControlTimeout),
		tr.cfg.KubetestVerbose,
	)
	_, err = ctrl.Output(exec.Command(
		tr.awsTesterPath,
		"eks",
		"--path="+tr.cfg.ConfigPath,
		"s3-upload",
		tr.cfg.LogOutputToUploadPath,
		logOutputS3,
	))
	return err
}
*/

///////////////////////////////////////////////
// Extra methods for EKS specific operations //
///////////////////////////////////////////////

func (tr *tester) Stop() {
	close(tr.stopc)
}

func (tr *tester) LoadConfig() (eksconfig.Config, error) {
	var err error
	tr.cfg, err = eksconfig.Load(tr.cfg.ConfigPath)
	return *tr.cfg, err
}

func (tr *tester) TestALBCorrectness() (err error) {
	if _, err = tr.LoadConfig(); err != nil {
		return err
	}

	ctrl := process.NewControl(
		tr.cfg.KubetestControlTimeout,
		time.NewTimer(tr.cfg.KubetestControlTimeout),
		time.NewTimer(tr.cfg.KubetestControlTimeout),
		tr.cfg.KubetestVerbose,
	)
	_, err = ctrl.Output(exec.Command(
		tr.awsTesterPath,
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

	ctrl := process.NewControl(
		tr.cfg.KubetestControlTimeout,
		time.NewTimer(tr.cfg.KubetestControlTimeout),
		time.NewTimer(tr.cfg.KubetestControlTimeout),
		tr.cfg.KubetestVerbose,
	)
	_, err = ctrl.Output(exec.Command(
		tr.awsTesterPath,
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

	ctrl := process.NewControl(
		tr.cfg.KubetestControlTimeout,
		time.NewTimer(tr.cfg.KubetestControlTimeout),
		time.NewTimer(tr.cfg.KubetestControlTimeout),
		tr.cfg.KubetestVerbose,
	)
	_, err = ctrl.Output(exec.Command(
		tr.awsTesterPath,
		"eks",
		"--path="+tr.cfg.ConfigPath,
		"test", "alb", "metrics",
	))
	return err
}
