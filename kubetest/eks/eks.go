// Package eks implements 'kubetest' deployer and publisher interface.
// It uses 'awstester' and 'kubectl' binaries, rather than importing
// EKS packages directly. This is to eliminate the need of dependency
// management, both in upstream and downstream.
package eks

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/awstester/eksconfig"
	"github.com/aws/awstester/eksdeployer"
	"github.com/aws/awstester/internal/eks/ingress/path"
	"github.com/aws/awstester/pkg/wrk"

	"k8s.io/test-infra/kubetest/process"
	kexec "k8s.io/utils/exec"
)

// tester implements EKS deployer using "awstester" binary.
// Satisfies "k8s.io/test-infra/kubetest/main.go" 'deployer' and 'publisher" interfaces.
// Reference https://github.com/kubernetes/test-infra/blob/master/kubetest/main.go.
type tester struct {
	stopc         chan struct{}
	mu            sync.RWMutex
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
	tr.awsTesterPath, err = kexec.New().LookPath("awstester")
	if err != nil {
		return nil, fmt.Errorf("cannot find 'awstester' executable (%v)", err)
	}
	tr.kubectlPath, err = kexec.New().LookPath("kubectl")
	if err != nil {
		return nil, fmt.Errorf("cannot find 'kubectl' executable (%v)", err)
	}
	_, err = kexec.New().LookPath("aws-iam-authenticator")
	if err != nil {
		return nil, fmt.Errorf("cannot find 'aws-iam-authenticator' executable (%v)", err)
	}
	return tr, nil
}

var (
	timeout = 3 * time.Hour
	control = process.NewControl(timeout, time.NewTimer(timeout), time.NewTimer(timeout), true)
)

// Up creates a new EKS cluster.
func (tr *tester) Up() (err error) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	createCmd := exec.Command(
		tr.awsTesterPath,
		"eks",
		"--path="+tr.cfg.ConfigPath,
		"create",
		"cluster",
	)
	errc := make(chan error)
	go func() {
		_, oerr := control.Output(createCmd)
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
	tr.mu.Lock()
	defer tr.mu.Unlock()

	_, err = control.Output(exec.Command(
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
	_, err = control.Output(exec.Command(
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
	_, err = control.Output(exec.Command(
		tr.awsTesterPath,
		"eks",
		"--path="+tr.cfg.ConfigPath,
		"upload",
		"s3",
		localPath,
		s3Path,
	))
	return err
}

// Publish publishes a success file.
func (tr *tester) Publish() error {
	tr.LoadConfig()
	logOutputS3 := tr.cfg.ClusterName + "/" + filepath.Base(tr.cfg.LogOutputToUploadPath)
	_, err := control.Output(exec.Command(
		tr.awsTesterPath,
		"eks",
		"--path="+tr.cfg.ConfigPath,
		"upload",
		"s3",
		tr.cfg.LogOutputToUploadPath,
		logOutputS3,
	))
	return err
}

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

func (tr *tester) TestCorrectness() error {
	if _, err := tr.LoadConfig(); err != nil {
		return err
	}

	ep := "http://" + tr.cfg.ALBIngressController.ELBv2NamespaceToDNSName["default"]
	if tr.cfg.ALBIngressController.TestMode == "ingress-test-server" {
		ep += path.Path
	}

	resp, err := http.Get(ep)
	if err != nil {
		return fmt.Errorf("failed to HTTP Get %q (%v)", ep, err)
	}

	var d []byte
	d, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if !bytes.Equal(bytes.Repeat([]byte("0"), tr.cfg.ALBIngressController.TestResponseSize), d) {
		return fmt.Errorf("HTTP Get %q returned unexpected (%q)", ep, string(d))
	}
	return nil
}

func (tr *tester) TestQPS() error {
	if _, err := tr.LoadConfig(); err != nil {
		return err
	}

	f, err := ioutil.TempFile(os.TempDir(), "ingress-test-client")
	if err != nil {
		return err
	}
	testResultPath := f.Name()
	f.Close()
	os.RemoveAll(testResultPath)

	ep := "http://" + tr.cfg.ALBIngressController.ELBv2NamespaceToDNSName["default"]
	var output []byte
	switch tr.cfg.ALBIngressController.TestMode {
	case "ingress-test-server":
		output, err = control.Output(exec.Command(
			tr.awsTesterPath,
			"eks",
			"ingress",
			"client",
			"--endpoint="+ep,
			"--routes="+fmt.Sprint(tr.cfg.ALBIngressController.TestServerRoutes),
			"--clients="+fmt.Sprint(tr.cfg.ALBIngressController.TestClients),
			"--requests="+fmt.Sprint(tr.cfg.ALBIngressController.TestClientRequests),
			"--result-path="+testResultPath,
		))
		if err != nil {
			return err
		}

	case "nginx":
		// wrk --threads 2 --connections 200 --duration 15s --latency http://127.0.0.1
		args := []string{
			"--threads", "2",
			"--connections", fmt.Sprintf("%d", tr.cfg.ALBIngressController.TestClients),
			"--duration", "15s",
			"--latency",
			ep,
		}
		fmt.Printf("starting wrk command: wrk %s\n", strings.Join(args, " "))
		output, err = control.Output(exec.Command("wrk", args...))
		if err != nil {
			return err
		}
		fmt.Printf("finished wrk command: wrk %s\n", strings.Join(args, " "))
	}

	fmt.Printf("TestQPS Result: %q\n\n%s\n\n", ep, string(output))

	if err = ioutil.WriteFile(
		tr.cfg.ALBIngressController.ScalabilityOutputToUploadPath,
		output,
		0600,
	); err != nil {
		return err
	}
	if tr.cfg.LogAutoUpload {
		err = tr.DumpClusterLogs(
			tr.cfg.ALBIngressController.ScalabilityOutputToUploadPath,
			fmt.Sprintf("%s/alb.scalability.log", tr.cfg.ClusterName),
		)
		if err != nil {
			return err
		}
	}

	// just to load QPS and failures
	rcfg, err := eksconfig.Load(testResultPath)
	if err != nil {
		return err
	}

	if tr.cfg.ALBIngressController.TestMode == "ingress-test-server" {
		tr.cfg.ALBIngressController.TestResultQPS = rcfg.ALBIngressController.TestResultQPS
		tr.cfg.ALBIngressController.TestResultFailures = rcfg.ALBIngressController.TestResultFailures
	} else {
		pv, perr := wrk.Parse(string(output))
		if perr == nil {
			tr.cfg.ALBIngressController.TestResultQPS = pv.RequestsPerSec
			tr.cfg.ALBIngressController.TestResultFailures = pv.ErrorsConnect + pv.ErrorsWrite + pv.ErrorsRead + pv.ErrorsTimeout
		}
	}
	tr.cfg.Sync()

	if rcfg.ALBIngressController.TestResultFailures > tr.cfg.ALBIngressController.TestClientErrorThreshold {
		return fmt.Errorf("expected failures under threshold %d, got %d", tr.cfg.ALBIngressController.TestClientErrorThreshold, rcfg.ALBIngressController.TestResultFailures)
	}
	if tr.cfg.ALBIngressController.TestResultQPS > 0.0 &&
		tr.cfg.ALBIngressController.TestResultQPS < tr.cfg.ALBIngressController.TestExpectQPS {
		return fmt.Errorf("expected QPS %f, got %f",
			tr.cfg.ALBIngressController.TestExpectQPS,
			tr.cfg.ALBIngressController.TestResultQPS,
		)
	}
	return nil
}

func (tr *tester) TestMetrics() error {
	cfg, err := tr.LoadConfig()
	if err != nil {
		return err
	}

	ep := "http://" + cfg.ALBIngressController.ELBv2NamespaceToDNSName["kube-system"] + "/metrics"
	var resp *http.Response
	resp, err = http.Get(ep)
	if err != nil {
		return fmt.Errorf("failed to HTTP Get %q (%v)", ep, err)
	}
	var d []byte
	d, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	err = ioutil.WriteFile(tr.cfg.ALBIngressController.MetricsOutputToUploadPath, d, 0600)
	if err != nil {
		return err
	}
	if tr.cfg.LogAutoUpload {
		err = tr.DumpClusterLogs(
			tr.cfg.ALBIngressController.MetricsOutputToUploadPath,
			fmt.Sprintf("%s/alb.metrics.log", tr.cfg.ClusterName),
		)
		if err != nil {
			return err
		}
	}
	return nil
}
