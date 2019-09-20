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
// It uses 'aws-k8s-tester' and 'kubectl' binaries, rather than importing internal packages.
// All underlying implementation and external dependencies are compiled into one binary.
package eks

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	osexec "os/exec"
	"path/filepath"
	"reflect"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/ekstester"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"golang.org/x/oauth2"
	"k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/test-infra/kubetest/process"
	"k8s.io/test-infra/kubetest/util"
)

// deployer implements EKS deployer interface using "aws-k8s-tester" binary.
// Satisfies "k8s.io/test-infra/kubetest/main.go" 'deployer' and 'publisher" interfaces.
// Reference https://github.com/kubernetes/test-infra/blob/master/kubetest/main.go.
type deployer struct {
	stopc chan struct{}
	cfg   *eksconfig.Config
	ctrl  *process.Control
}

// NewDeployer creates a new EKS deployer.
func NewDeployer(timeout time.Duration, verbose bool) (ekstester.Deployer, error) {
	cfg := eksconfig.NewDefault()
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

	dp := &deployer{
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

// Up creates a new EKS cluster.
func (dp *deployer) Up() (err error) {
	// "create cluster" command outputs cluster information
	// in the configuration file (e.g. VPC ID, ALB DNS names, etc.)
	// this needs be reloaded for other deployer method calls
	createCmd := osexec.Command(
		dp.cfg.AWSK8sTesterPath,
		"eks",
		"--path="+dp.cfg.ConfigPath,
		"create",
		"cluster",
	)
	errc := make(chan error)
	go func() {
		_, oerr := dp.ctrl.Output(createCmd)
		errc <- oerr
	}()
	select {
	case <-dp.stopc:
		fmt.Fprintln(os.Stderr, "received stop signal, interrupting 'create cluster' command...")
		ierr := createCmd.Process.Signal(syscall.SIGINT)
		err = fmt.Errorf("'create cluster' command interrupted (interrupt error %v)", ierr)
	case err = <-errc:
	}
	return err
}

// Down tears down the existing EKS cluster.
func (dp *deployer) Down() (err error) {
	// reload configuration from disk to read the latest configuration
	if _, err = dp.LoadConfig(); err != nil {
		return err
	}
	_, err = dp.ctrl.Output(osexec.Command(
		dp.cfg.AWSK8sTesterPath,
		"eks",
		"--path="+dp.cfg.ConfigPath,
		"delete",
		"cluster",
	))
	return err
}

// IsUp returns an error if the cluster is not up and running.
func (dp *deployer) IsUp() (err error) {
	// reload configuration from disk to read the latest configuration
	if _, err = dp.LoadConfig(); err != nil {
		return err
	}
	_, err = dp.ctrl.Output(osexec.Command(
		dp.cfg.AWSK8sTesterPath,
		"eks",
		"--path="+dp.cfg.ConfigPath,
		"check",
		"cluster",
	))
	if err != nil {
		return err
	}
	if _, err = dp.LoadConfig(); err != nil {
		return err
	}
	if dp.cfg.ClusterState.Status != "ACTIVE" {
		return fmt.Errorf("cluster %q status is %q",
			dp.cfg.ClusterName,
			dp.cfg.ClusterState.Status,
		)
	}
	return nil
}

// TestSetup checks if EKS testing cluster has been set up or not.
func (dp *deployer) TestSetup() error {
	return dp.IsUp()
}

// GetClusterCreated returns EKS cluster creation time and error (if any).
func (dp *deployer) GetClusterCreated(v string) (time.Time, error) {
	err := dp.IsUp()
	if err != nil {
		return time.Time{}, err
	}
	return dp.cfg.ClusterState.Created, nil
}

func (dp *deployer) GetWorkerNodeLogs() (err error) {
	// reload configuration from disk to read the latest configuration
	if _, err = dp.LoadConfig(); err != nil {
		return err
	}
	_, err = dp.ctrl.Output(osexec.Command(
		dp.cfg.AWSK8sTesterPath,
		"eks",
		"--path="+dp.cfg.ConfigPath,
		"test",
		"get-worker-node-logs",
	))
	return err
}

// DumpClusterLogs dumps all logs to artifact directory.
// Let default kubetest log dumper handle all artifact uploads.
// See https://github.com/kubernetes/test-infra/pull/9811/files#r225776067.
func (dp *deployer) DumpClusterLogs(artifactDir, _ string) (err error) {
	// reload configuration from disk to read the latest configuration
	if _, err = dp.LoadConfig(); err != nil {
		return err
	}
	_, err = dp.ctrl.Output(osexec.Command(
		dp.cfg.AWSK8sTesterPath,
		"eks",
		"--path="+dp.cfg.ConfigPath,
		"test",
		"get-worker-node-logs",
	))
	if err != nil {
		return err
	}
	_, err = dp.ctrl.Output(osexec.Command(
		dp.cfg.AWSK8sTesterPath,
		"eks",
		"--path="+dp.cfg.ConfigPath,
		"test", "dump-cluster-logs",
		artifactDir,
	))
	return err
}

// KubectlCommand returns "kubectl" command object for API reachability tests.
func (dp *deployer) KubectlCommand() (*osexec.Cmd, error) {
	// reload configuration from disk to read the latest configuration
	if _, err := dp.LoadConfig(); err != nil {
		return nil, err
	}
	return osexec.Command(dp.cfg.KubectlPath, "--kubeconfig="+dp.cfg.KubeConfigPath), nil
}

const authProviderName = "eks"

// KubernetesClientSet returns Kubernetes Go client.
func (dp *deployer) KubernetesClientSet() *kubernetes.Clientset {
	// reload configuration from disk to read the latest configuration
	if _, err := dp.LoadConfig(); err != nil {
		panic(err)
	}

	restCfg := &restclient.Config{
		Host: dp.cfg.ClusterState.Endpoint,
		TLSClientConfig: restclient.TLSClientConfig{
			CAData: []byte(dp.cfg.ClusterState.CADecoded),
		},
		AuthProvider: &clientcmdapi.AuthProviderConfig{
			Name: authProviderName,
			Config: map[string]string{
				// TODO: use this to support temporary credentials
				// "aws-credentials-path": md.awsCredsPath,

				"region":       dp.cfg.AWSRegion,
				"cluster-name": dp.cfg.ClusterName,
			},
		},
	}

	k8sClientSet, err := clientset.NewForConfig(restCfg)
	if err != nil {
		panic(err)
	}
	return k8sClientSet
}

func init() {
	restclient.RegisterAuthProviderPlugin(authProviderName, newAuthProvider)
}

func newAuthProvider(_ string, config map[string]string, _ restclient.AuthProviderConfigPersister) (restclient.AuthProvider, error) {
	// TODO: use this to support temporary credentials
	// awsCredentialsPath := config["aws-credentials-path"]

	awsRegion, ok := config["region"]
	if !ok {
		return nil, fmt.Errorf("'clientcmdapi.AuthProviderConfig' does not include 'region' key %+v", config)
	}
	clusterName, ok := config["cluster-name"]
	if !ok {
		return nil, fmt.Errorf("'clientcmdapi.AuthProviderConfig' does not include 'cluster-name' key %+v", config)
	}

	sess := session.Must(session.NewSession(aws.NewConfig().WithRegion(awsRegion)))
	return &eksAuthProvider{ts: newTokenSource(sess, clusterName)}, nil
}

type eksAuthProvider struct {
	ts oauth2.TokenSource
}

func (p *eksAuthProvider) WrapTransport(rt http.RoundTripper) http.RoundTripper {
	return &oauth2.Transport{
		Source: p.ts,
		Base:   rt,
	}
}

func (p *eksAuthProvider) Login() error {
	return nil
}

func newTokenSource(sess *session.Session, clusterName string) oauth2.TokenSource {
	return &eksTokenSource{sess: sess, clusterName: clusterName}
}

type eksTokenSource struct {
	sess        *session.Session
	clusterName string
}

// Reference
// https://github.com/kubernetes-sigs/aws-iam-authenticator/blob/master/README.md#api-authorization-from-outside-a-cluster
// https://github.com/kubernetes-sigs/aws-iam-authenticator/blob/master/pkg/token/token.go
const (
	v1Prefix        = "k8s-aws-v1."
	clusterIDHeader = "x-k8s-aws-id"
)

func (s *eksTokenSource) Token() (*oauth2.Token, error) {
	stsAPI := sts.New(s.sess)
	request, _ := stsAPI.GetCallerIdentityRequest(&sts.GetCallerIdentityInput{})
	request.HTTPRequest.Header.Add(clusterIDHeader, s.clusterName)

	payload, err := request.Presign(60)
	if err != nil {
		return nil, err
	}
	token := v1Prefix + base64.RawURLEncoding.EncodeToString([]byte(payload))
	tokenExpiration := time.Now().Local().Add(14 * time.Minute)
	return &oauth2.Token{
		AccessToken: token,
		TokenType:   "Bearer",
		Expiry:      tokenExpiration,
	}, nil
}

// Stop stops ongoing operations.
// This is useful for local development.
// For example, one may run "Up" but have to cancel ongoing "Up"
// operation. Then, it can just send syscall.SIGINT to trigger "Stop".
func (dp *deployer) Stop() {
	close(dp.stopc)
}

// LoadConfig reloads configuration from disk to read the latest
// cluster configuration and its states.
func (dp *deployer) LoadConfig() (eksconfig.Config, error) {
	var err error
	dp.cfg, err = eksconfig.Load(dp.cfg.ConfigPath)
	return *dp.cfg, err
}

// NewTester creates a new EKS tester.
func NewTester(timeout time.Duration, verbose bool) (ekstester.Tester, error) {
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

// UploadToBucketForTests uploads a local file to aws-k8s-tester S3 bucket.
func (tr *tester) UploadToBucketForTests(localPath, s3Path string) (err error) {
	_, err = tr.ctrl.Output(osexec.Command(
		tr.cfg.AWSK8sTesterPath,
		"eks",
		"--path="+tr.cfg.ConfigPath,
		"s3-upload",
		localPath,
		s3Path,
	))
	return err
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
