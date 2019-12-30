// Package eks implements EKS cluster operations.
package eks

import (
	"context"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/eks/alb"
	"github.com/aws/aws-k8s-tester/eks/jobs"
	"github.com/aws/aws-k8s-tester/eks/nlb"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/awsapi"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	awseks "github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/elb/elbiface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/exec"
)

// Tester implements "kubetest2" Deployer.
// ref. https://github.com/kubernetes/test-infra/blob/master/kubetest2/pkg/types/types.go
type Tester struct {
	stopCreationCh     chan struct{}
	stopCreationChOnce *sync.Once

	interruptSig chan os.Signal

	lg  *zap.Logger
	cfg *eksconfig.Config

	awsSession *session.Session
	iamAPI     iamiface.IAMAPI
	ssmAPI     ssmiface.SSMAPI
	cfnAPI     cloudformationiface.CloudFormationAPI
	ec2API     ec2iface.EC2API
	asgAPI     autoscalingiface.AutoScalingAPI
	elbAPI     elbiface.ELBAPI

	eksSession *session.Session
	eksAPI     eksiface.EKSAPI

	downMu                      *sync.Mutex
	fetchLogsManagedNodeGroupMu *sync.RWMutex

	k8sClientSet *kubernetes.Clientset

	jobPiTester         jobs.Tester
	jobEchoTester       jobs.Tester
	nlbHelloWorldTester alb.Tester
	alb2048Tester       alb.Tester
}

// New creates a new EKS tester.
func New(cfg *eksconfig.Config) (*Tester, error) {
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		return nil, err
	}

	lcfg := logutil.AddOutputPaths(logutil.DefaultZapLoggerConfig, cfg.LogOutputs, cfg.LogOutputs)
	lcfg.Level = zap.NewAtomicLevelAt(logutil.ConvertToZapLevel(cfg.LogLevel))
	lg, err := lcfg.Build()
	if err != nil {
		return nil, err
	}

	if err = fileutil.EnsureExecutable(cfg.AWSCLIPath); err != nil {
		// file may be already executable while the process does not own the file/directory
		// ref. https://github.com/aws/aws-k8s-tester/issues/66
		lg.Error("failed to ensure executable", zap.Error(err))
	}

	// aws --version
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	vo, verr := exec.New().CommandContext(
		ctx,
		cfg.AWSCLIPath,
		"--version",
	).CombinedOutput()
	cancel()
	if verr != nil {
		return nil, fmt.Errorf("'aws --version' failed (output %q, error %v)", string(vo), verr)
	}
	lg.Info(
		"aws version",
		zap.String("aws-cli-path", cfg.AWSCLIPath),
		zap.String("aws-version", string(vo)),
	)

	lg.Info("mkdir", zap.String("kubectl-path-dir", filepath.Dir(cfg.KubectlPath)))
	if err := os.MkdirAll(filepath.Dir(cfg.KubectlPath), 0700); err != nil {
		return nil, fmt.Errorf("could not create %q (%v)", filepath.Dir(cfg.KubectlPath), err)
	}
	lg.Info("downloading kubectl", zap.String("kubectl-path", cfg.KubectlPath))
	if err := os.RemoveAll(cfg.KubectlPath); err != nil {
		return nil, err
	}
	f, err := os.Create(cfg.KubectlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create %q (%v)", cfg.KubectlPath, err)
	}
	cfg.KubectlPath = f.Name()
	cfg.KubectlPath, _ = filepath.Abs(cfg.KubectlPath)
	if err := httpDownloadFile(lg, cfg.KubectlDownloadURL, f); err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("failed to close kubectl %v", err)
	}
	if err := fileutil.EnsureExecutable(cfg.KubectlPath); err != nil {
		// file may be already executable while the process does not own the file/directory
		// ref. https://github.com/aws/aws-k8s-tester/issues/66
		lg.Error("failed to ensure executable", zap.Error(err))
	}
	// kubectl version --client=true
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	vo, verr = exec.New().CommandContext(
		ctx,
		cfg.KubectlPath,
		"version",
		"--client=true",
	).CombinedOutput()
	cancel()
	if verr != nil {
		return nil, fmt.Errorf("'kubectl version' failed (output %q, error %v)", string(vo), verr)
	}
	lg.Info(
		"kubectl version",
		zap.String("kubectl-path", cfg.KubectlPath),
		zap.String("kubectl-version", string(vo)),
	)

	if cfg.AWSIAMAuthenticatorPath != "" && cfg.AWSIAMAuthenticatorDownloadURL != "" {
		lg.Info("mkdir", zap.String("aws-iam-authenticator-path-dir", filepath.Dir(cfg.AWSIAMAuthenticatorPath)))
		if err := os.MkdirAll(filepath.Dir(cfg.AWSIAMAuthenticatorPath), 0700); err != nil {
			return nil, fmt.Errorf("could not create %q (%v)", filepath.Dir(cfg.AWSIAMAuthenticatorPath), err)
		}
		lg.Info("downloading aws-iam-authenticator", zap.String("aws-iam-authenticator-path", cfg.AWSIAMAuthenticatorPath))
		if err := os.RemoveAll(cfg.AWSIAMAuthenticatorPath); err != nil {
			return nil, err
		}
		f, err := os.Create(cfg.AWSIAMAuthenticatorPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create %q (%v)", cfg.AWSIAMAuthenticatorPath, err)
		}
		cfg.AWSIAMAuthenticatorPath = f.Name()
		cfg.AWSIAMAuthenticatorPath, _ = filepath.Abs(cfg.AWSIAMAuthenticatorPath)
		if err := httpDownloadFile(lg, cfg.AWSIAMAuthenticatorDownloadURL, f); err != nil {
			return nil, err
		}
		if err := f.Close(); err != nil {
			return nil, fmt.Errorf("failed to close aws-iam-authenticator %v", err)
		}
		if err := fileutil.EnsureExecutable(cfg.AWSIAMAuthenticatorPath); err != nil {
			// file may be already executable while the process does not own the file/directory
			// ref. https://github.com/aws/aws-k8s-tester/issues/66
			lg.Error("failed to ensure executable", zap.Error(err))
		}
		// aws-iam-authenticator version
		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		vo, verr = exec.New().CommandContext(
			ctx,
			cfg.AWSIAMAuthenticatorPath,
			"version",
		).CombinedOutput()
		cancel()
		if verr != nil {
			return nil, fmt.Errorf("'aws-iam-authenticator version' failed (output %q, error %v)", string(vo), verr)
		}
		lg.Info(
			"aws-iam-authenticator version",
			zap.String("aws-iam-authenticator-path", cfg.AWSIAMAuthenticatorPath),
			zap.String("aws-iam-authenticator-version", string(vo)),
		)
	}

	ts := &Tester{
		stopCreationCh:              make(chan struct{}),
		stopCreationChOnce:          new(sync.Once),
		interruptSig:                make(chan os.Signal),
		lg:                          lg,
		cfg:                         cfg,
		downMu:                      new(sync.Mutex),
		fetchLogsManagedNodeGroupMu: new(sync.RWMutex),
	}
	signal.Notify(ts.interruptSig, syscall.SIGTERM, syscall.SIGINT)

	defer ts.cfg.Sync()

	awsCfg := &awsapi.Config{
		Logger:        ts.lg,
		DebugAPICalls: ts.cfg.LogLevel == "debug",
		Region:        ts.cfg.Region,
	}
	var stsOutput *sts.GetCallerIdentityOutput
	ts.awsSession, stsOutput, _, err = awsapi.New(awsCfg)
	if err != nil {
		return nil, err
	}
	ts.cfg.Status.AWSAccountID = *stsOutput.Account

	ts.iamAPI = iam.New(ts.awsSession)
	ts.ssmAPI = ssm.New(ts.awsSession)
	ts.cfnAPI = cloudformation.New(ts.awsSession)
	ts.ec2API = ec2.New(ts.awsSession)
	ts.asgAPI = autoscaling.New(ts.awsSession)
	ts.elbAPI = elb.New(ts.awsSession)

	// create a separate session for EKS (for resolver endpoint)
	ts.eksSession, _, ts.cfg.Status.AWSCredentialPath, err = awsapi.New(&awsapi.Config{
		Logger:        ts.lg,
		DebugAPICalls: ts.cfg.LogLevel == "debug",
		Region:        ts.cfg.Region,
		ResolverURL:   ts.cfg.Parameters.ClusterResolverURL,
		SigningName:   ts.cfg.Parameters.ClusterSigningName,
	})
	if err != nil {
		return nil, err
	}
	ts.eksAPI = awseks.New(ts.eksSession)

	// reuse existing role
	if ts.cfg.Parameters.ClusterRoleARN != "" {
		ts.lg.Info("reuse existing IAM role", zap.String("cluster-role-arn", ts.cfg.Parameters.ClusterRoleARN))
		ts.cfg.Status.ClusterRoleARN = ts.cfg.Parameters.ClusterRoleARN
	}

	return ts, nil
}

// Up should provision a new cluster for testing
func (ts *Tester) Up() (err error) {
	now := time.Now().UTC()

	defer func() {
		if err == nil {
			ts.lg.Info("Up completed",
				zap.String("config-path", ts.cfg.ConfigPath),
				zap.String("KUBECONFIG", ts.cfg.KubeConfigPath),
				zap.String("cluster-arn", ts.cfg.Status.ClusterARN),
				zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
			)
			return
		}

		ts.lg.Error("failed Up, reverting resource creation",
			zap.String("config-path", ts.cfg.ConfigPath),
			zap.String("KUBECONFIG", ts.cfg.KubeConfigPath),
			zap.String("cluster-arn", ts.cfg.Status.ClusterARN),
			zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
			zap.Error(err),
		)
		derr := ts.down()
		if derr != nil {
			ts.lg.Error("failed to revert Up", zap.Error(derr))
		} else {
			ts.lg.Warn("reverted Up")
		}
	}()

	ts.lg.Info("Up started",
		zap.String("name", ts.cfg.Name),
		zap.String("resolver-url", ts.cfg.Parameters.ClusterResolverURL),
		zap.String("config-path", ts.cfg.ConfigPath),
		zap.String("KUBECONFIG", ts.cfg.KubeConfigPath),
	)
	defer ts.cfg.Sync()

	if err := catchInterrupt(ts.lg, ts.stopCreationCh, ts.stopCreationChOnce, ts.interruptSig, ts.createClusterRole); err != nil {
		return err
	}

	if err := catchInterrupt(ts.lg, ts.stopCreationCh, ts.stopCreationChOnce, ts.interruptSig, ts.createVPC); err != nil {
		return err
	}

	if err := catchInterrupt(ts.lg, ts.stopCreationCh, ts.stopCreationChOnce, ts.interruptSig, ts.createCluster); err != nil {
		return err
	}

	waitDur := time.Minute
	ts.lg.Info("waiting before running health check", zap.Duration("wait", waitDur))
	time.Sleep(waitDur)

	if err := catchInterrupt(ts.lg, ts.stopCreationCh, ts.stopCreationChOnce, ts.interruptSig, ts.runHealthCheck); err != nil {
		return err
	}
	if err := catchInterrupt(ts.lg, ts.stopCreationCh, ts.stopCreationChOnce, ts.interruptSig, ts.createSecretAWSCredential); err != nil {
		return err
	}
	if err := catchInterrupt(ts.lg, ts.stopCreationCh, ts.stopCreationChOnce, ts.interruptSig, ts.createNamespace); err != nil {
		return err
	}

	if ts.cfg.Parameters.ManagedNodeGroupCreate {
		if err := catchInterrupt(ts.lg, ts.stopCreationCh, ts.stopCreationChOnce, ts.interruptSig, ts.createKeyPair); err != nil {
			return err
		}
		if err := catchInterrupt(ts.lg, ts.stopCreationCh, ts.stopCreationChOnce, ts.interruptSig, ts.createManagedNodeGroupRole); err != nil {
			return err
		}
		if err := catchInterrupt(ts.lg, ts.stopCreationCh, ts.stopCreationChOnce, ts.interruptSig, ts.createManagedNodeGroup); err != nil {
			return err
		}
		if err := catchInterrupt(ts.lg, ts.stopCreationCh, ts.stopCreationChOnce, ts.interruptSig, ts.openPortsManagedNodeGroup); err != nil {
			return err
		}
		if err := catchInterrupt(ts.lg, ts.stopCreationCh, ts.stopCreationChOnce, ts.interruptSig, ts.runHealthCheck); err != nil {
			return err
		}
		if ts.cfg.AddOnNLBHelloWorld.Enable {
			ts.nlbHelloWorldTester, err = nlb.New(nlb.Config{
				Logger:    ts.lg,
				Stopc:     ts.stopCreationCh,
				Sig:       ts.interruptSig,
				K8SClient: ts.k8sClientSet,
				EKSConfig: ts.cfg,
			})
			if err := catchInterrupt(ts.lg, ts.stopCreationCh, ts.stopCreationChOnce, ts.interruptSig, ts.nlbHelloWorldTester.Create); err != nil {
				return err
			}
		}
		if ts.cfg.AddOnALB2048.Enable {
			ts.alb2048Tester, err = alb.New(alb.Config{
				Logger:            ts.lg,
				Stopc:             ts.stopCreationCh,
				Sig:               ts.interruptSig,
				CloudFormationAPI: ts.cfnAPI,
				K8SClient:         ts.k8sClientSet,
				EKSConfig:         ts.cfg,
			})
			if err := catchInterrupt(ts.lg, ts.stopCreationCh, ts.stopCreationChOnce, ts.interruptSig, ts.alb2048Tester.Create); err != nil {
				return err
			}
		}
		if ts.cfg.AddOnJobPerl.Enable {
			ts.jobPiTester, err = jobs.New(jobs.Config{
				Logger:    ts.lg,
				Stopc:     ts.stopCreationCh,
				Sig:       ts.interruptSig,
				K8SClient: ts.k8sClientSet,
				Namespace: ts.cfg.Name,
				JobName:   jobs.JobNamePi,
				Completes: ts.cfg.AddOnJobPerl.Completes,
				Parallels: ts.cfg.AddOnJobPerl.Parallels,
			})
			if err != nil {
				return err
			}
			if err := catchInterrupt(ts.lg, ts.stopCreationCh, ts.stopCreationChOnce, ts.interruptSig, ts.jobPiTester.Create); err != nil {
				return err
			}
		}
		if ts.cfg.AddOnJobEcho.Enable {
			ts.jobEchoTester, err = jobs.New(jobs.Config{
				Logger:    ts.lg,
				Stopc:     ts.stopCreationCh,
				Sig:       ts.interruptSig,
				K8SClient: ts.k8sClientSet,
				Namespace: ts.cfg.Name,
				JobName:   jobs.JobNameEcho,
				Completes: ts.cfg.AddOnJobEcho.Completes,
				Parallels: ts.cfg.AddOnJobEcho.Parallels,
				EchoSize:  ts.cfg.AddOnJobEcho.Size,
			})
			if err != nil {
				return err
			}
			if err := catchInterrupt(ts.lg, ts.stopCreationCh, ts.stopCreationChOnce, ts.interruptSig, ts.jobEchoTester.Create); err != nil {
				return err
			}
		}
		if err := catchInterrupt(ts.lg, ts.stopCreationCh, ts.stopCreationChOnce, ts.interruptSig, ts.FetchLogsManagedNodeGroup); err != nil {
			return err
		}
	}

	println()
	if err := catchInterrupt(ts.lg, ts.stopCreationCh, ts.stopCreationChOnce, ts.interruptSig, ts.runHealthCheck); err != nil {
		return err
	}
	println()
	fmt.Println(ts.cfg.KubectlCommands())
	println()
	fmt.Println(ts.cfg.SSHCommands())
	println()

	return ts.cfg.Sync()
}

func (ts *Tester) down() (err error) {
	now := time.Now().UTC()
	ts.lg.Warn("starting Down",
		zap.String("name", ts.cfg.Name),
		zap.String("config-path", ts.cfg.ConfigPath),
		zap.String("cluster-arn", ts.cfg.Status.ClusterARN),
	)
	defer func() {
		ts.cfg.Sync()
		if err == nil {
			ts.lg.Info("successfully finished Down",
				zap.String("name", ts.cfg.Name),
				zap.String("config-path", ts.cfg.ConfigPath),
				zap.String("cluster-arn", ts.cfg.Status.ClusterARN),
				zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
			)
		}
	}()

	// paralleize deletes
	ch := make(chan errorData)
	waits := 0

	waits++
	go func() {
		ch <- errorData{name: "cluster role", err: ts.deleteClusterRole()}
	}()

	if ts.cfg.Parameters.ManagedNodeGroupCreate {
		waits++
		go func() {
			ch <- errorData{name: "EC2 key pair", err: ts.deleteKeyPair()}
		}()
		if ts.cfg.AddOnJobEcho.Enable && ts.jobEchoTester != nil {
			waits++
			go func() {
				ch <- errorData{name: "Job echo", err: ts.jobEchoTester.Delete()}
			}()
		}
		if ts.cfg.AddOnJobPerl.Enable && ts.jobPiTester != nil {
			waits++
			go func() {
				ch <- errorData{name: "Job Pi", err: ts.jobPiTester.Delete()}
			}()
		}
		if ts.cfg.AddOnALB2048.Enable && ts.alb2048Tester != nil {
			waits++
			go func() {
				ch <- errorData{name: "ALB", err: ts.alb2048Tester.Delete()}
			}()
		}
		if ts.cfg.AddOnNLBHelloWorld.Enable && ts.nlbHelloWorldTester != nil {
			waits++
			go func() {
				ch <- errorData{name: "NLB", err: ts.nlbHelloWorldTester.Delete()}
			}()
		}
	}

	var errs []string
	for i := 0; i < waits; i++ {
		if d := <-ch; d.err != nil {
			ts.lg.Warn("failed to delete",
				zap.String("name", d.name),
				zap.Int("waited-goroutines", i+1),
				zap.Error(d.err),
			)
			errs = append(errs, d.err.Error())
		} else {
			ts.lg.Info("waited delete goroutine with no error", zap.String("name", d.name), zap.Int("waited-goroutines", i+1))
		}
	}

	// following need to be run in order to resolve delete dependency
	// e.g. cluster must be deleted before VPC delete
	if err := ts.deleteNamespace(); err != nil {
		ts.lg.Warn("failed to delete namespace", zap.String("namespace", ts.cfg.Name), zap.Error(err))
		errs = append(errs, err.Error())
	}

	if ts.cfg.Parameters.ManagedNodeGroupCreate {
		if err := ts.deleteManagedNodeGroup(); err != nil {
			ts.lg.Warn("failed to delete managed node group", zap.Error(err))
			errs = append(errs, err.Error())
		}
		waitDur := 5 * time.Second
		ts.lg.Info("sleeping before node group role deletion", zap.Duration("wait", waitDur))
		time.Sleep(waitDur)

		// must be run after deleting node group
		// otherwise, "Cannot delete entity, must remove roles from instance profile first. (Service: AmazonIdentityManagement; Status Code: 409; Error Code: DeleteConflict; Request ID: 197f795b-1003-4386-81cc-44a926c42be7)"
		if err := ts.deleteManagedNodeGroupRole(); err != nil {
			ts.lg.Warn("failed to delete managed node group role", zap.Error(err))
			errs = append(errs, err.Error())
		}

		waitDur = 10 * time.Second
		ts.lg.Info("sleeping before cluster deletion", zap.Duration("wait", waitDur))
		time.Sleep(waitDur)
	}

	if err := ts.deleteCluster(); err != nil {
		ts.lg.Warn("failed to delete cluster", zap.Error(err))
		errs = append(errs, err.Error())
	}

	waitDur := 30 * time.Second
	ts.lg.Info("sleeping before VPC deletion", zap.Duration("wait", waitDur))
	time.Sleep(waitDur)

	if err := ts.deleteVPC(); err != nil {
		ts.lg.Warn("failed to delete VPC", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}
	return ts.cfg.Sync()
}

type errorData struct {
	name string
	err  error
}

// Down cancels the cluster creation and destroy the test cluster if any.
func (ts *Tester) Down() error {
	ts.downMu.Lock()
	defer ts.downMu.Unlock()
	return ts.down()
}

// IsUp should return true if a test cluster is successfully provisioned
func (ts *Tester) IsUp() (up bool, err error) {
	if !ts.cfg.Status.Up {
		return false, nil
	}
	return true, ts.healthCheck()
}

// DumpClusterLogs should export logs from the cluster. It may be called
// multiple times. Options for this should come from New(...)
func (ts *Tester) DumpClusterLogs() error {
	return ts.FetchLogsManagedNodeGroup()
}

// DownloadClusterLogs dumps all logs to artifact directory.
// Let default kubetest log dumper handle all artifact uploads.
// See https://github.com/kubernetes/test-infra/pull/9811/files#r225776067.
func (ts *Tester) DownloadClusterLogs(artifactDir, _ string) error {
	err := ts.FetchLogsManagedNodeGroup()
	if err != nil {
		return err
	}

	ts.fetchLogsManagedNodeGroupMu.RLock()
	defer ts.fetchLogsManagedNodeGroupMu.RUnlock()

	for _, fpaths := range ts.cfg.Status.ManagedNodeGroupsLogs {
		for _, fpath := range fpaths {
			newPath := filepath.Join(artifactDir, filepath.Base(fpath))
			if err := fileutil.Copy(fpath, newPath); err != nil {
				return err
			}
		}
	}

	return fileutil.Copy(
		ts.cfg.ConfigPath,
		filepath.Join(artifactDir, filepath.Base(ts.cfg.ConfigPath)),
	)
}

// Build should build kubernetes and package it in whatever format
// the deployer consumes
func (ts *Tester) Build() error {
	// no-op
	return nil
}

// LoadConfig reloads configuration from disk to read the latest
// cluster configuration and its states.
// It's either reloaded from disk or returned from embedded EKS deployer.
func (ts *Tester) LoadConfig() (eksconfig.Config, error) {
	return *ts.cfg, nil
}

// KubectlCommand returns "kubectl" command object for API reachability tests.
func (ts *Tester) KubectlCommand() (*osexec.Cmd, error) {
	return osexec.Command(ts.cfg.KubectlPath, "--kubeconfig="+ts.cfg.KubeConfigPath), nil
}

// KubernetesClientSet returns Kubernetes Go client.
func (ts *Tester) KubernetesClientSet() *kubernetes.Clientset {
	return ts.k8sClientSet
}

// Kubeconfig returns a path to a kubeconfig file for the cluster.
func (ts *Tester) Kubeconfig() (string, error) {
	return ts.cfg.KubeConfigPath, nil
}

// Provider returns the kubernetes provider for legacy deployers.
func (ts *Tester) Provider() string {
	return "eks"
}
