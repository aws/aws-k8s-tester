/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package eks implements 'kubetest' deployer interface.
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

// tester implements EKS deployer interface using "awstester" binary.
// Satisfies "k8s.io/test-infra/kubetest/main.go" 'deployer' and 'publisher" interfaces.
// Reference https://github.com/kubernetes/test-infra/blob/master/kubetest/main.go.
type tester struct {
	stopc         chan struct{}
	cfg           *eksconfig.Config
	awsTesterPath string
	kubectlPath   string
	ctrl          *process.Control
}

// New creates a new EKS deployer with AWS CLI.
func New(cfg *eksconfig.Config, ctrl *process.Control) (eksdeployer.Interface, error) {
	cfg.Embedded = false
	tr := &tester{
		stopc: make(chan struct{}),
		cfg:   cfg,
		ctrl:  ctrl,
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
	fmt.Println("Up using binary 'awstester'")

	createCmd := exec.Command(
		tr.awsTesterPath,
		"eks",
		"--path="+tr.cfg.ConfigPath,
		"create",
		"cluster",
	)
	errc := make(chan error)
	go func() {
		_, oerr := tr.ctrl.Output(createCmd)
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
	_, err = tr.ctrl.Output(exec.Command(
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
	_, err = tr.ctrl.Output(exec.Command(
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

// DumpClusterLogs dumps all logs to artifact directory.
// Let default kubetest log dumper handle all artifact uploads.
// See https://github.com/kubernetes/test-infra/pull/9811/files#r225776067.
func (tr *tester) DumpClusterLogs(artifactDir, _ string) (err error) {
	tr.LoadConfig()

	_, err = tr.ctrl.Output(exec.Command(
		tr.awsTesterPath,
		"eks",
		"--path="+tr.cfg.ConfigPath,
		"test", "dump-cluster-logs",
		artifactDir,
	))
	return err
}

///////////////////////////////////////////////
// Extra methods for EKS specific operations //
///////////////////////////////////////////////

// UploadToBucketForTests uploads a local file to awstester S3 bucket.
func (tr *tester) UploadToBucketForTests(localPath, s3Path string) (err error) {
	_, err = tr.ctrl.Output(exec.Command(
		tr.awsTesterPath,
		"eks",
		"--path="+tr.cfg.ConfigPath,
		"s3-upload",
		localPath,
		s3Path,
	))
	return err
}

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
	_, err = tr.ctrl.Output(exec.Command(
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
	_, err = tr.ctrl.Output(exec.Command(
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
	_, err = tr.ctrl.Output(exec.Command(
		tr.awsTesterPath,
		"eks",
		"--path="+tr.cfg.ConfigPath,
		"test", "alb", "metrics",
	))
	return err
}
