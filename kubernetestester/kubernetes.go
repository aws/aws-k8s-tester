package kubernetestester

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	osexec "os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/kubernetesconfig"
	"k8s.io/test-infra/kubetest/process"
	"k8s.io/test-infra/kubetest/util"
)

type tester struct {
	stopc chan struct{}
	cfg   *kubernetesconfig.Config
	ctrl  *process.Control
}

// LoadConfig reloads configuration from disk to read the latest
// cluster configuration and its states.
func (tt *tester) LoadConfig() (kubernetesconfig.Config, error) {
	var err error
	tt.cfg, err = kubernetesconfig.Load(tt.cfg.ConfigPath)
	return *tt.cfg, err
}

// NewTester creates a new Kubernetes tester.
func NewTester(timeout time.Duration, verbose bool) (Tester, error) {
	cfg := kubernetesconfig.NewDefault()
	err := cfg.UpdateFromEnvs()
	if err != nil {
		return nil, err
	}
	var f *os.File
	f, err = ioutil.TempFile(os.TempDir(), "aws-k8s-tester-config")
	if err != nil {
		return nil, err
	}
	cfg.ConfigPath = f.Name()
	if err = f.Close(); err != nil {
		return nil, fmt.Errorf("failed to close aws-k8s-tester-config file %v", err)
	}
	if err = cfg.Sync(); err != nil {
		return nil, err
	}

	dp := &tester{
		stopc: make(chan struct{}),
		cfg:   cfg,
		ctrl: process.NewControl(
			timeout,
			time.NewTimer(timeout),
			time.NewTimer(timeout),
			verbose,
		),
	}

	if err = os.RemoveAll(cfg.AWSK8sTesterPath); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(cfg.AWSK8sTesterPath), 0700); err != nil {
		return nil, err
	}
	f, err = os.Create(cfg.AWSK8sTesterPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create %q (%v)", cfg.AWSK8sTesterPath, err)
	}
	cfg.AWSK8sTesterPath = f.Name()
	if err = httpRead(cfg.AWSK8sTesterDownloadURL, f); err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, fmt.Errorf("failed to close aws-k8s-tester file %v", err)
	}
	if err = util.EnsureExecutable(cfg.AWSK8sTesterPath); err != nil {
		return nil, err
	}
	return dp, nil
}

func (tt *tester) Create() (err error) {
	// "create cluster" command outputs cluster information
	// in the configuration file (e.g. VPC ID, ALB DNS names, etc.)
	// this needs be reloaded for other deployer method calls
	createCmd := osexec.Command(
		tt.cfg.AWSK8sTesterPath,
		"kubernetes",
		"--path="+tt.cfg.ConfigPath,
		"create",
		"cluster",
	)
	errc := make(chan error)
	go func() {
		_, oerr := tt.ctrl.Output(createCmd)
		errc <- oerr
	}()
	select {
	case <-tt.stopc:
		fmt.Fprintln(os.Stderr, "received stop signal, interrupting 'create cluster' command...")
		ierr := createCmd.Process.Signal(syscall.SIGINT)
		err = fmt.Errorf("'create cluster' command interrupted (interrupt error %v)", ierr)
	case err = <-errc:
	}
	return err
}

func (tt *tester) Terminate() (err error) {
	// reload configuration from disk to read the latest configuration
	if _, err = tt.LoadConfig(); err != nil {
		return err
	}
	_, err = tt.ctrl.Output(osexec.Command(
		tt.cfg.AWSK8sTesterPath,
		"kubernetes",
		"--path="+tt.cfg.ConfigPath,
		"delete",
		"cluster",
	))
	return err
}

func (tt *tester) KubeConfig() (p string, err error) {
	// reload configuration from disk to read the latest configuration
	if _, err = tt.LoadConfig(); err != nil {
		return "", err
	}
	return tt.cfg.KubeConfigPath, nil
}

var httpTransport *http.Transport

func init() {
	httpTransport = new(http.Transport)
	httpTransport.RegisterProtocol("file", http.NewFileTransport(http.Dir("/")))
}

// curl -L [URL] | writer
func httpRead(u string, wr io.Writer) error {
	log.Printf("curl %s", u)
	cli := &http.Client{Transport: httpTransport}
	r, err := cli.Get(u)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode >= 400 {
		return fmt.Errorf("%v returned %d", u, r.StatusCode)
	}
	_, err = io.Copy(wr, r.Body)
	return err
}
