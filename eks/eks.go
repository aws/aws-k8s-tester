// Package eks implements EKS cluster operations.
package eks

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/eks/alb"
	"github.com/aws/aws-k8s-tester/eks/fargate"
	"github.com/aws/aws-k8s-tester/eks/gpu"
	"github.com/aws/aws-k8s-tester/eks/irsa"
	"github.com/aws/aws-k8s-tester/eks/jobs"
	"github.com/aws/aws-k8s-tester/eks/mng"
	"github.com/aws/aws-k8s-tester/eks/nlb"
	"github.com/aws/aws-k8s-tester/eks/secrets"
	"github.com/aws/aws-k8s-tester/eksconfig"
	pkgaws "github.com/aws/aws-k8s-tester/pkg/aws"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	awseks "github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/kms/kmsiface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/dustin/go-humanize"
	"github.com/mitchellh/colorstring"
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

	downMu *sync.Mutex

	lg  *zap.Logger
	cfg *eksconfig.Config

	awsSession *session.Session
	iamAPI     iamiface.IAMAPI
	kmsAPI     kmsiface.KMSAPI
	ssmAPI     ssmiface.SSMAPI
	cfnAPI     cloudformationiface.CloudFormationAPI
	ec2API     ec2iface.EC2API
	s3API      s3iface.S3API
	asgAPI     autoscalingiface.AutoScalingAPI
	elbv2API   elbv2iface.ELBV2API

	eksSession *session.Session
	eksAPI     eksiface.EKSAPI

	k8sClientSet *kubernetes.Clientset

	gpuTester           gpu.Tester
	mngTester           mng.Tester
	nlbHelloWorldTester alb.Tester
	alb2048Tester       alb.Tester
	jobPerlTester       jobs.Tester
	jobEchoTester       jobs.Tester
	secretsTester       secrets.Tester
	irsaTester          irsa.Tester
	fargateTester       fargate.Tester
}

// New creates a new EKS tester.
func New(cfg *eksconfig.Config) (*Tester, error) {
	fmt.Println("😎 🙏")
	colorstring.Printf("\n\n\n[light_green]New [default](%q, %q)\n", cfg.ConfigPath, cfg.KubectlCommand())
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
		lg.Warn("failed to ensure executable", zap.Error(err))
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
		lg.Warn("failed to ensure executable", zap.Error(err))
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
			lg.Warn("failed to ensure executable", zap.Error(err))
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
		stopCreationCh:     make(chan struct{}),
		stopCreationChOnce: new(sync.Once),
		interruptSig:       make(chan os.Signal),
		downMu:             new(sync.Mutex),
		lg:                 lg,
		cfg:                cfg,
	}
	signal.Notify(ts.interruptSig, syscall.SIGTERM, syscall.SIGINT)

	defer ts.cfg.Sync()

	awsCfg := &pkgaws.Config{
		Logger:        ts.lg,
		DebugAPICalls: ts.cfg.LogLevel == "debug",
		Region:        ts.cfg.Region,
	}
	var stsOutput *sts.GetCallerIdentityOutput
	ts.awsSession, stsOutput, ts.cfg.Status.AWSCredentialPath, err = pkgaws.New(awsCfg)
	if err != nil {
		return nil, err
	}
	ts.cfg.Status.AWSAccountID = aws.StringValue(stsOutput.Account)
	ts.cfg.Status.AWSUserID = aws.StringValue(stsOutput.UserId)
	ts.cfg.Status.AWSIAMRoleARN = aws.StringValue(stsOutput.Arn)
	ts.cfg.Sync()

	ts.iamAPI = iam.New(ts.awsSession)
	ts.kmsAPI = kms.New(ts.awsSession)
	ts.ssmAPI = ssm.New(ts.awsSession)
	ts.cfnAPI = cloudformation.New(ts.awsSession)

	ts.ec2API = ec2.New(ts.awsSession)
	if _, err := ts.ec2API.DescribeInstances(&ec2.DescribeInstancesInput{MaxResults: aws.Int64(5)}); err != nil {
		return nil, fmt.Errorf("failed to describe instances using EC2 API (%v)", err)
	}
	colorstring.Println("[green]EC2 API available!")

	ts.s3API = s3.New(ts.awsSession)
	ts.asgAPI = autoscaling.New(ts.awsSession)
	ts.elbv2API = elbv2.New(ts.awsSession)

	// create a separate session for EKS (for resolver endpoint)
	ts.eksSession, _, ts.cfg.Status.AWSCredentialPath, err = pkgaws.New(&pkgaws.Config{
		Logger:        ts.lg,
		DebugAPICalls: ts.cfg.LogLevel == "debug",
		Region:        ts.cfg.Region,
		ResolverURL:   ts.cfg.Parameters.ResolverURL,
		SigningName:   ts.cfg.Parameters.SigningName,
	})
	if err != nil {
		return nil, err
	}
	ts.eksAPI = awseks.New(ts.eksSession)

	// check EKS API availability
	lresp, err := ts.eksAPI.ListClusters(&awseks.ListClustersInput{
		MaxResults: aws.Int64(20),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters using EKS API (%v)", err)
	}
	colorstring.Println("[green]EKS API available!")
	ts.lg.Info("listing EKS clusters with limit 20", zap.Int("clusters", len(lresp.Clusters)))
	for _, v := range lresp.Clusters {
		ts.lg.Info("EKS cluster", zap.String("name", aws.StringValue(v)))
	}

	// update k8s client if cluster has already been created
	if err = ts.createK8sClientSet(); err != nil {
		ts.lg.Warn("failed to create k8s client", zap.Error(err))
	}

	if err = ts.createSubTesters(); err != nil {
		return nil, err
	}

	return ts, nil
}

func (ts *Tester) createSubTesters() (err error) {
	if !ts.cfg.IsAddOnManagedNodeGroupsEnabled() {
		return nil
	}

	colorstring.Printf("\n\n\n[light_green]createSubTesters [default](%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())

	ts.lg.Info("creating gpuTester")
	ts.gpuTester, err = gpu.New(gpu.Config{
		Logger:    ts.lg,
		Stopc:     ts.stopCreationCh,
		Sig:       ts.interruptSig,
		EKSConfig: ts.cfg,
		K8SClient: ts,
		Namespace: ts.cfg.Name,
	})
	if err != nil {
		return err
	}

	ts.lg.Info("creating mngTester")
	ts.mngTester, err = mng.New(mng.Config{
		Logger:    ts.lg,
		Stopc:     ts.stopCreationCh,
		Sig:       ts.interruptSig,
		EKSConfig: ts.cfg,
		K8SClient: ts,
		IAMAPI:    ts.iamAPI,
		CFNAPI:    ts.cfnAPI,
		EC2API:    ts.ec2API,
		ASGAPI:    ts.asgAPI,
		EKSAPI:    ts.eksAPI,
	})
	if err != nil {
		return err
	}

	if ts.cfg.IsAddOnNLBHelloWorldEnabled() {
		ts.lg.Info("creating nlbHelloWorldTester")
		ts.nlbHelloWorldTester, err = nlb.New(nlb.Config{
			Logger:    ts.lg,
			Stopc:     ts.stopCreationCh,
			Sig:       ts.interruptSig,
			EKSConfig: ts.cfg,
			K8SClient: ts,
			ELB2API:   ts.elbv2API,
		})
		if err != nil {
			return err
		}
	}

	if ts.cfg.IsAddOnALB2048Enabled() {
		ts.lg.Info("creating alb2048Tester")
		ts.alb2048Tester, err = alb.New(alb.Config{
			Logger:    ts.lg,
			Stopc:     ts.stopCreationCh,
			Sig:       ts.interruptSig,
			CFNAPI:    ts.cfnAPI,
			EKSConfig: ts.cfg,
			K8SClient: ts,
			ELB2API:   ts.elbv2API,
		})
		if err != nil {
			return err
		}
	}

	if ts.cfg.IsAddOnJobPerlEnabled() {
		ts.lg.Info("creating jobPerlTester")
		ts.jobPerlTester, err = jobs.New(jobs.Config{
			Logger:    ts.lg,
			Stopc:     ts.stopCreationCh,
			Sig:       ts.interruptSig,
			EKSConfig: ts.cfg,
			K8SClient: ts,
			JobName:   jobs.JobNamePi,
			Completes: ts.cfg.AddOnJobPerl.Completes,
			Parallels: ts.cfg.AddOnJobPerl.Parallels,
		})
		if err != nil {
			return err
		}
	}

	if ts.cfg.IsAddOnJobEchoEnabled() {
		ts.lg.Info("creating jobEchoTester")
		ts.jobEchoTester, err = jobs.New(jobs.Config{
			Logger:    ts.lg,
			Stopc:     ts.stopCreationCh,
			Sig:       ts.interruptSig,
			EKSConfig: ts.cfg,
			K8SClient: ts,
			JobName:   jobs.JobNameEcho,
			Completes: ts.cfg.AddOnJobEcho.Completes,
			Parallels: ts.cfg.AddOnJobEcho.Parallels,
			EchoSize:  ts.cfg.AddOnJobEcho.Size,
		})
		if err != nil {
			return err
		}
	}

	if ts.cfg.IsAddOnSecretsEnabled() {
		ts.lg.Info("creating secretsTester")
		ts.secretsTester, err = secrets.New(secrets.Config{
			Logger:    ts.lg,
			Stopc:     ts.stopCreationCh,
			Sig:       ts.interruptSig,
			EKSConfig: ts.cfg,
			K8SClient: ts,
		})
		if err != nil {
			return err
		}
	}

	if ts.cfg.IsAddOnIRSAEnabled() {
		ts.lg.Info("creating irsaTester")
		ts.irsaTester, err = irsa.New(irsa.Config{
			Logger:    ts.lg,
			Stopc:     ts.stopCreationCh,
			Sig:       ts.interruptSig,
			EKSConfig: ts.cfg,
			K8SClient: ts,
			CFNAPI:    ts.cfnAPI,
			IAMAPI:    ts.iamAPI,
			S3API:     ts.s3API,
		})
		if err != nil {
			return err
		}
	}

	if ts.cfg.IsAddOnFargateEnabled() {
		ts.lg.Info("creating fargateTester")
		ts.fargateTester, err = fargate.New(fargate.Config{
			Logger:    ts.lg,
			Stopc:     ts.stopCreationCh,
			Sig:       ts.interruptSig,
			EKSConfig: ts.cfg,
			K8SClient: ts,
			IAMAPI:    ts.iamAPI,
			CFNAPI:    ts.cfnAPI,
			EKSAPI:    ts.eksAPI,
		})
		if err != nil {
			return err
		}
	}

	return ts.cfg.Sync()
}

// Up should provision a new cluster for testing
func (ts *Tester) Up() (err error) {
	colorstring.Printf("\n\n\n[light_green]Up [default](%q)\n", ts.cfg.ConfigPath)

	now := time.Now()

	defer func() {
		colorstring.Printf("\n\n\n[light_green]Up.defer start [default](%q, %q)\n\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())

		if err == nil {
			if ts.cfg.Status.Up {
				colorstring.Printf("\n\n[light_green]kubectl [default](%q, %q)\n\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
				fmt.Println(ts.cfg.KubectlCommands())
				colorstring.Printf("\n\n[light_green]SSH [default](%q, %q)\n\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
				fmt.Println(ts.cfg.SSHCommands())

				ts.lg.Info("Up succeeded",
					zap.String("request-started", humanize.RelTime(now, time.Now(), "ago", "from now")),
				)

				colorstring.Printf("\n\n\n[light_green]Up.defer end [default](%q, %q)\n\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
				colorstring.Printf("\n\n😁 😁 [blue]:) [default]Up success\n\n\n")
			} else {
				colorstring.Printf("\n\n😲 😲 [yellow]aborted [default]Up ???\n\n\n")
			}
			return
		}

		if !ts.cfg.OnFailureDelete {
			if ts.cfg.Status.Up {
				colorstring.Printf("\n\n[light_green]kubectl [default](%q, %q)\n\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
				fmt.Println(ts.cfg.KubectlCommands())
				colorstring.Printf("\n\n[light_green]SSH [default](%q, %q)\n\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
				fmt.Println(ts.cfg.SSHCommands())
			}

			ts.lg.Warn("Up failed",
				zap.String("request-started", humanize.RelTime(now, time.Now(), "ago", "from now")),
				zap.Error(err),
			)

			colorstring.Printf("\n\n\n[light_red]Up.defer end [default](%q, %q)\n\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
			colorstring.Printf("\n\n😱 ☹ 😡 [light_red](-_-) [default]Up fail\n\n\n")
			return
		}

		if ts.cfg.Status.Up {
			colorstring.Printf("\n\n[light_green]kubectl [default](%q, %q)\n\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
			fmt.Println(ts.cfg.KubectlCommands())
			colorstring.Printf("\n\n[light_green]SSH [default](%q, %q)\n\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
			fmt.Println(ts.cfg.SSHCommands())
		}

		ts.lg.Warn("Up failed; reverting resource creation",
			zap.String("request-started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			zap.Error(err),
		)
		waitDur := time.Duration(ts.cfg.OnFailureDeleteWaitSeconds) * time.Second
		if waitDur > 0 {
			ts.lg.Info("waiting before clean up", zap.Duration("wait", waitDur))
			select {
			case <-ts.stopCreationCh:
				ts.lg.Info("wait aborted before clean up")
			case <-ts.interruptSig:
				ts.lg.Info("wait aborted before clean up")
			case <-time.After(waitDur):
			}
		}
		derr := ts.down()
		if derr != nil {
			ts.lg.Warn("failed to revert Up", zap.Error(derr))
		} else {
			ts.lg.Warn("reverted Up")
		}

		colorstring.Printf("\n\n\n[light_red]Up.defer end [default](%q, %q)\n\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		colorstring.Printf("\n\n😱 ☹  [light_red](-_-) [default]Up fail\n\n\n")
	}()

	ts.lg.Info("Up started",
		zap.String("name", ts.cfg.Name),
	)
	defer ts.cfg.Sync()

	colorstring.Printf("\n\n\n[light_green]createVPC [default](%q)\n", ts.cfg.ConfigPath)
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.interruptSig,
		ts.createVPC,
	); err != nil {
		return err
	}

	colorstring.Printf("\n\n\n[light_green]createClusterRole [default](%q)\n", ts.cfg.ConfigPath)
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.interruptSig,
		ts.createClusterRole,
	); err != nil {
		return err
	}

	colorstring.Printf("\n\n\n[light_green]createCluster [default](%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.interruptSig,
		ts.createCluster,
	); err != nil {
		return err
	}

	waitDur := time.Minute
	ts.lg.Info("waiting before running health check", zap.Duration("wait", waitDur))
	time.Sleep(waitDur)

	colorstring.Printf("\n\n\n[light_green]checkHealth [default](%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.interruptSig,
		ts.checkHealth,
	); err != nil {
		return err
	}

	if ts.cfg.CommandAfterCreateCluster != "" {
		colorstring.Printf("\n[light_green]runCommand CommandAfterCreateCluster [default](%q)\n", ts.cfg.CommandAfterCreateCluster)
		out, err := runCommand(ts.lg, ts.cfg.CommandAfterCreateCluster)
		if err != nil {
			err = ioutil.WriteFile(ts.cfg.CommandAfterCreateClusterOutputPath, []byte(ts.cfg.CommandAfterCreateCluster+"\n\n# output\n"+string(out)+"\n\n# error\n"+err.Error()), 0600)
			if err != nil {
				return fmt.Errorf("failed to write file %q (%v)", ts.cfg.CommandAfterCreateClusterOutputPath, err)
			}
		} else {
			err = ioutil.WriteFile(ts.cfg.CommandAfterCreateClusterOutputPath, []byte(ts.cfg.CommandAfterCreateCluster+"\n\n# output\n"+string(out)), 0600)
			if err != nil {
				return fmt.Errorf("failed to write file %q (%v)", ts.cfg.CommandAfterCreateClusterOutputPath, err)
			}
		}
		colorstring.Printf("\n[light_gray]runCommand output[default]:\n%s\n", string(out))
	}

	if ts.cfg.IsAddOnManagedNodeGroupsEnabled() {
		colorstring.Printf("\n\n\n[light_green]mngTester.Create [default](%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.interruptSig,
			ts.mngTester.Create,
		); err != nil {
			return err
		}

		needGPU := false
	found:
		for _, mv := range ts.cfg.AddOnManagedNodeGroups.MNGs {
			switch mv.AMIType {
			case awseks.AMITypesAl2X8664Gpu:
				needGPU = true
				break found
			}
		}
		if !ts.cfg.StatusManagedNodeGroups.NvidiaDriverInstalled && needGPU {
			colorstring.Printf("\n\n\n[light_green]gpuTester.InstallNvidiaDriver [default](%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
			if err := catchInterrupt(
				ts.lg,
				ts.stopCreationCh,
				ts.stopCreationChOnce,
				ts.interruptSig,
				ts.gpuTester.InstallNvidiaDriver,
			); err != nil {
				ts.lg.Warn("failed to install Nvidia driver", zap.Error(err))
			}

			colorstring.Printf("\n\n\n[light_green]gpuTester.RunNvidiaSMI [default](%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
			if err := catchInterrupt(
				ts.lg,
				ts.stopCreationCh,
				ts.stopCreationChOnce,
				ts.interruptSig,
				ts.gpuTester.RunNvidiaSMI,
			); err != nil {
				return err
			}
		}

		colorstring.Printf("\n\n\n[light_green]checkHealth [default](%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.interruptSig,
			ts.checkHealth,
		); err != nil {
			return err
		}
		colorstring.Printf("\n\n[light_green]kubectl [default](%q, %q)\n\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		fmt.Println(ts.cfg.KubectlCommands())
		colorstring.Printf("\n\n[light_green]SSH [default](%q, %q)\n\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		fmt.Println(ts.cfg.SSHCommands())

		if ts.cfg.IsAddOnNLBHelloWorldEnabled() {
			colorstring.Printf("\n\n\n[light_green]nlbHelloWorldTester.Create [default](%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnNLBHelloWorld.Namespace)
			if err := catchInterrupt(
				ts.lg,
				ts.stopCreationCh,
				ts.stopCreationChOnce,
				ts.interruptSig,
				ts.nlbHelloWorldTester.Create,
			); err != nil {
				return err
			}
		}

		if ts.cfg.IsAddOnALB2048Enabled() {
			colorstring.Printf("\n\n\n[light_green]alb2048Tester.Create [default](%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnALB2048.Namespace)
			if err := catchInterrupt(
				ts.lg,
				ts.stopCreationCh,
				ts.stopCreationChOnce,
				ts.interruptSig,
				ts.alb2048Tester.Create,
			); err != nil {
				return err
			}
		}

		if ts.cfg.IsAddOnJobPerlEnabled() {
			colorstring.Printf("\n\n\n[light_green]jobPerlTester.Create [default](%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnJobPerl.Namespace)
			if err := catchInterrupt(
				ts.lg,
				ts.stopCreationCh,
				ts.stopCreationChOnce,
				ts.interruptSig,
				ts.jobPerlTester.Create,
			); err != nil {
				return err
			}
		}

		if ts.cfg.IsAddOnJobEchoEnabled() {
			colorstring.Printf("\n\n\n[light_green]jobEchoTester.Create [default](%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnJobEcho.Namespace)
			if err := catchInterrupt(
				ts.lg,
				ts.stopCreationCh,
				ts.stopCreationChOnce,
				ts.interruptSig,
				ts.jobEchoTester.Create,
			); err != nil {
				return err
			}
		}

		if ts.cfg.IsAddOnSecretsEnabled() {
			colorstring.Printf("\n\n\n[light_green]secretsTester.Create [default](%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnSecrets.Namespace)
			if err := catchInterrupt(
				ts.lg,
				ts.stopCreationCh,
				ts.stopCreationChOnce,
				ts.interruptSig,
				ts.secretsTester.Create,
			); err != nil {
				return err
			}
		}

		if ts.cfg.IsAddOnIRSAEnabled() {
			colorstring.Printf("\n\n\n[light_green]irsaTester.Create [default](%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnIRSA.Namespace)
			if err := catchInterrupt(
				ts.lg,
				ts.stopCreationCh,
				ts.stopCreationChOnce,
				ts.interruptSig,
				ts.irsaTester.Create,
			); err != nil {
				return err
			}
		}

		if ts.cfg.IsAddOnFargateEnabled() {
			colorstring.Printf("\n\n\n[light_green]fargateTester.Create [default](%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnFargate.Namespace)
			if err := catchInterrupt(
				ts.lg,
				ts.stopCreationCh,
				ts.stopCreationChOnce,
				ts.interruptSig,
				ts.fargateTester.Create,
			); err != nil {
				return err
			}
		}

		colorstring.Printf("\n\n\n[light_green]mngTester.FetchLogs [default](%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		waitDur := 20 * time.Second
		ts.lg.Info("sleeping before mngTester.FetchLogs", zap.Duration("wait", waitDur))
		time.Sleep(waitDur)
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.interruptSig,
			ts.mngTester.FetchLogs,
		); err != nil {
			return err
		}

		if ts.cfg.IsAddOnSecretsEnabled() {
			colorstring.Printf("\n\n\n[light_green]secretsTester.AggregateResults [default](%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnSecrets.Namespace)
			if err := catchInterrupt(
				ts.lg,
				ts.stopCreationCh,
				ts.stopCreationChOnce,
				ts.interruptSig,
				ts.secretsTester.AggregateResults,
			); err != nil {
				return err
			}
		}

		if ts.cfg.IsAddOnIRSAEnabled() {
			colorstring.Printf("\n\n\n[light_green]irsaTester.AggregateResults [default](%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnIRSA.Namespace)
			if err := catchInterrupt(
				ts.lg,
				ts.stopCreationCh,
				ts.stopCreationChOnce,
				ts.interruptSig,
				ts.irsaTester.AggregateResults,
			); err != nil {
				return err
			}
		}

		colorstring.Printf("\n\n\n[light_green]checkHealth [default](%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.interruptSig,
			ts.checkHealth,
		); err != nil {
			return err
		}

		if ts.cfg.CommandAfterCreateAddOns != "" {
			colorstring.Printf("\n[light_green]runCommand CommandAfterCreateAddOns [default](%q)\n", ts.cfg.CommandAfterCreateAddOns)
			out, err := runCommand(ts.lg, ts.cfg.CommandAfterCreateAddOns)
			if err != nil {
				err = ioutil.WriteFile(ts.cfg.CommandAfterCreateAddOnsOutputPath, []byte(ts.cfg.CommandAfterCreateAddOns+"\n\n# output\n"+string(out)+"\n\n# error\n"+err.Error()), 0600)
				if err != nil {
					return fmt.Errorf("failed to write file %q (%v)", ts.cfg.CommandAfterCreateAddOnsOutputPath, err)
				}
			} else {
				err = ioutil.WriteFile(ts.cfg.CommandAfterCreateAddOnsOutputPath, []byte(ts.cfg.CommandAfterCreateAddOns+"\n\n# output\n"+string(out)), 0600)
				if err != nil {
					return fmt.Errorf("failed to write file %q (%v)", ts.cfg.CommandAfterCreateAddOnsOutputPath, err)
				}
			}
			colorstring.Printf("\n[light_gray]runCommand output[default]:\n%s\n", string(out))
		}
	}

	return ts.cfg.Sync()
}

// Down cancels the cluster creation and destroy the test cluster if any.
func (ts *Tester) Down() error {
	ts.downMu.Lock()
	defer ts.downMu.Unlock()
	return ts.down()
}

func (ts *Tester) down() (err error) {
	colorstring.Printf("\n\n\n[light_green]Down start [default](%q)\n\n", ts.cfg.ConfigPath)

	now := time.Now()
	ts.lg.Warn("starting Down",
		zap.String("name", ts.cfg.Name),
		zap.String("cluster-arn", ts.cfg.Status.ClusterARN),
	)

	defer func() {
		ts.cfg.Sync()
		if err == nil {
			ts.lg.Info("successfully finished Down",
				zap.String("request-started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)

			colorstring.Printf("\n\n\n[light_green]Down.defer end [default](%q)\n\n", ts.cfg.ConfigPath)
			colorstring.Printf("\n\n😁 [blue]:) [default]Down success\n\n\n")

		} else {

			ts.lg.Info("failed Down",
				zap.Error(err),
				zap.String("request-started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)

			colorstring.Printf("\n\n\n[light_red]Down.defer end [default](%q)\n\n", ts.cfg.ConfigPath)
			colorstring.Printf("\n\n😱 ☹  [light_red](-_-) [default]Down fail\n\n\n")
		}
	}()

	var errs []string

	if ts.cfg.IsAddOnManagedNodeGroupsEnabled() && len(ts.cfg.StatusManagedNodeGroups.Nodes) > 0 {
		if ts.cfg.IsAddOnFargateEnabled() && ts.cfg.AddOnFargate.Created {
			colorstring.Printf("\n\n\n[light_green]fargateTester.Delete [default](%q)\n", ts.cfg.ConfigPath)
			if err := ts.fargateTester.Delete(); err != nil {
				ts.lg.Warn("fargateTester.Delete failed", zap.Error(err))
				errs = append(errs, err.Error())
			}
		}

		if ts.cfg.IsAddOnIRSAEnabled() && ts.cfg.AddOnIRSA.Created {
			colorstring.Printf("\n\n\n[light_green]irsaTester.Delete [default](%q)\n", ts.cfg.ConfigPath)
			if err := ts.irsaTester.Delete(); err != nil {
				ts.lg.Warn("irsaTester.Delete failed", zap.Error(err))
				errs = append(errs, err.Error())
			}
		}

		if ts.cfg.IsAddOnSecretsEnabled() && ts.cfg.AddOnSecrets.Created {
			colorstring.Printf("\n\n\n[light_green]secretsTester.Delete [default](%q)\n", ts.cfg.ConfigPath)
			if err := ts.secretsTester.Delete(); err != nil {
				ts.lg.Warn("secretsTester.Delete failed", zap.Error(err))
				errs = append(errs, err.Error())
			}
		}

		if ts.cfg.IsAddOnJobEchoEnabled() && ts.cfg.AddOnJobEcho.Created {
			colorstring.Printf("\n\n\n[light_green]jobEchoTester.Delete [default](%q)\n", ts.cfg.ConfigPath)
			if err := ts.jobEchoTester.Delete(); err != nil {
				ts.lg.Warn("jobEchoTester.Delete failed", zap.Error(err))
				errs = append(errs, err.Error())
			}
		}

		if ts.cfg.IsAddOnJobPerlEnabled() && ts.cfg.AddOnJobPerl.Created {
			colorstring.Printf("\n\n\n[light_green]jobPerlTester.Delete [default](%q)\n", ts.cfg.ConfigPath)
			if err := ts.jobPerlTester.Delete(); err != nil {
				ts.lg.Warn("jobPerlTester.Delete failed", zap.Error(err))
				errs = append(errs, err.Error())
			}
		}

		if ts.cfg.IsAddOnALB2048Enabled() && ts.cfg.AddOnALB2048.Created {
			colorstring.Printf("\n\n\n[light_green]alb2048Tester.Delete [default](%q)\n", ts.cfg.ConfigPath)
			if err := ts.alb2048Tester.Delete(); err != nil {
				ts.lg.Warn("alb2048Tester.Delete failed", zap.Error(err))
				errs = append(errs, err.Error())
			} else {
				waitDur := time.Minute
				ts.lg.Info("sleeping after deleting ALB", zap.Duration("wait", waitDur))
				time.Sleep(waitDur)
			}
		}

		if ts.cfg.IsAddOnNLBHelloWorldEnabled() && ts.cfg.AddOnNLBHelloWorld.Created {
			colorstring.Printf("\n\n\n[light_green]nlbHelloWorldTester.Delete [default](%q)\n", ts.cfg.ConfigPath)
			if err := ts.nlbHelloWorldTester.Delete(); err != nil {
				ts.lg.Warn("nlbHelloWorldTester.Delete failed", zap.Error(err))
				errs = append(errs, err.Error())
			} else {
				waitDur := time.Minute
				ts.lg.Info("sleeping after deleting NLB", zap.Duration("wait", waitDur))
				time.Sleep(waitDur)
			}
		}
	}

	// NOTE(jaypipes): Wait for a bit here because we asked Kubernetes to
	// delete the NLB hello world and ALB2048 Deployment/Service above, and
	// both of these interact with the underlying Kubernetes AWS cloud provider
	// to clean up the cloud load balancer backing the Service of type
	// LoadBalancer. The calls to delete the Service return immediately
	// (successfully) but the cloud load balancer resources may not have been
	// deleted yet, including the ENIs that were associated with the cloud load
	// balancer. When, later, aws-k8s-tester tries deleting the VPC associated
	// with the test cluster, it will run into permissions issues because the
	// IAM role that created the ENIs associated with the ENIs in subnets
	// associated with the cloud load balancers will no longer exist.
	//
	// https://github.com/aws/aws-k8s-tester/issues/70
	// https://github.com/kubernetes/kubernetes/issues/53451
	// https://github.com/kubernetes/enhancements/blob/master/keps/sig-network/20190423-service-lb-finalizer.md
	if ts.cfg.IsAddOnManagedNodeGroupsEnabled() && len(ts.cfg.StatusManagedNodeGroups.Nodes) > 0 &&
		((ts.cfg.IsAddOnALB2048Enabled() && ts.cfg.AddOnALB2048.Created) ||
			(ts.cfg.IsAddOnNLBHelloWorldEnabled() && ts.cfg.AddOnNLBHelloWorld.Created)) {
		waitDur := 2 * time.Minute
		ts.lg.Info("sleeping after deleting LB", zap.Duration("wait", waitDur))
		time.Sleep(waitDur)
	}

	// following need to be run in order to resolve delete dependency
	// DO NOT "len(ts.cfg.StatusManagedNodeGroups.Nodes) > 0"; MNG may have failed to create
	// e.g. cluster must be deleted before VPC delete
	if ts.cfg.IsAddOnManagedNodeGroupsEnabled() {
		colorstring.Printf("\n\n\n[light_green]mngTester.Delete [default](%q)\n", ts.cfg.ConfigPath)
		if err := ts.mngTester.Delete(); err != nil {
			ts.lg.Warn("mngTester.Delete failed", zap.Error(err))
			errs = append(errs, err.Error())
		}

		waitDur := 10 * time.Second
		ts.lg.Info("sleeping before cluster deletion", zap.Duration("wait", waitDur))
		time.Sleep(waitDur)
	}

	colorstring.Printf("\n\n\n[light_green]deleteCluster [default](%q)\n", ts.cfg.ConfigPath)
	if err := ts.deleteCluster(); err != nil {
		ts.lg.Warn("deleteCluster failed", zap.Error(err))
		errs = append(errs, err.Error())
	}

	colorstring.Printf("\n\n\n[light_green]deleteClusterRole [default](%q)\n", ts.cfg.ConfigPath)
	if err := ts.deleteClusterRole(); err != nil {
		ts.lg.Warn("deleteClusterRole failed", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if ts.cfg.Parameters.VPCID == "" { // VPC was created
		waitDur := 30 * time.Second
		ts.lg.Info("sleeping before VPC deletion", zap.Duration("wait", waitDur))
		time.Sleep(waitDur)
	}

	colorstring.Printf("\n\n\n[light_green]deleteVPC [default](%q)\n", ts.cfg.ConfigPath)
	if err := ts.deleteVPC(); err != nil {
		ts.lg.Warn("deleteVPC failed", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}
	return ts.cfg.Sync()
}

// CreateMNG creates/adds EKS "Managed Node Group"s.
// The existing node groups won't be recreated.
func (ts *Tester) CreateMNG() error {
	if !ts.cfg.IsAddOnManagedNodeGroupsEnabled() {
		ts.lg.Warn("mng has not been enabled; skipping creation MNG")
		return nil
	}

	colorstring.Printf("\n\n\n[light_green]mngTester.Create [default](%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.interruptSig,
		ts.mngTester.Create,
	); err != nil {
		return err
	}

	needGPU := false
found:
	for _, mv := range ts.cfg.AddOnManagedNodeGroups.MNGs {
		switch mv.AMIType {
		case awseks.AMITypesAl2X8664Gpu:
			needGPU = true
			break found
		}
	}
	if !ts.cfg.StatusManagedNodeGroups.NvidiaDriverInstalled && needGPU {
		colorstring.Printf("\n\n\n[light_green]gpuTester.InstallNvidiaDriver [default](%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.interruptSig,
			ts.gpuTester.InstallNvidiaDriver,
		); err != nil {
			ts.lg.Warn("failed to install Nvidia driver", zap.Error(err))
		}

		colorstring.Printf("\n\n\n[light_green]gpuTester.RunNvidiaSMI [default](%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.interruptSig,
			ts.gpuTester.RunNvidiaSMI,
		); err != nil {
			return err
		}
	}

	return ts.cfg.Sync()
}

// IsUp should return true if a test cluster is successfully provisioned
func (ts *Tester) IsUp() (up bool, err error) {
	if !ts.cfg.Status.Up {
		return false, nil
	}
	return true, ts.checkHealth()
}

// DumpClusterLogs should export logs from the cluster. It may be called
// multiple times. Options for this should come from New(...)
func (ts *Tester) DumpClusterLogs() error {
	return ts.mngTester.FetchLogs()
}

// DownloadClusterLogs dumps all logs to artifact directory.
// Let default kubetest log dumper handle all artifact uploads.
// See https://github.com/kubernetes/test-infra/pull/9811/files#r225776067.
func (ts *Tester) DownloadClusterLogs(artifactDir, _ string) error {
	return ts.mngTester.DownloadClusterLogs(artifactDir)
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

// KubernetesClientSet returns Kubernetes Go client.
func (ts *Tester) KubernetesClientSet() *kubernetes.Clientset {
	if ts.k8sClientSet == nil {
		if err := ts.createK8sClientSet(); err != nil {
			ts.lg.Warn("failed to create k8s client set", zap.Error(err))
			return nil
		}
	}
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
