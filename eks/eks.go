// Package eks implements EKS cluster operations.
// It implements "k8s.io/test-infra/kubetest2/pkg/types.Deployer" and
// "k8s.io/test-infra/kubetest2/pkg/types.Options".
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Deployer
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
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

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/eks/alb"
	"github.com/aws/aws-k8s-tester/eks/appmesh"
	"github.com/aws/aws-k8s-tester/eks/configmaps"
	"github.com/aws/aws-k8s-tester/eks/cronjobs"
	"github.com/aws/aws-k8s-tester/eks/csrs"
	"github.com/aws/aws-k8s-tester/eks/fargate"
	"github.com/aws/aws-k8s-tester/eks/gpu"
	"github.com/aws/aws-k8s-tester/eks/irsa"
	jobsecho "github.com/aws/aws-k8s-tester/eks/jobs-echo"
	jobspi "github.com/aws/aws-k8s-tester/eks/jobs-pi"
	"github.com/aws/aws-k8s-tester/eks/mng"
	"github.com/aws/aws-k8s-tester/eks/ng"
	"github.com/aws/aws-k8s-tester/eks/nlb"
	"github.com/aws/aws-k8s-tester/eks/secrets"
	"github.com/aws/aws-k8s-tester/eksconfig"
	pkgaws "github.com/aws/aws-k8s-tester/pkg/aws"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	k8sclient "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-k8s-tester/version"
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
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/exec"
)

// Tester implements "kubetest2" Deployer.
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc
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

	ngTester            ng.Tester
	mngTester           mng.Tester
	gpuTester           gpu.Tester
	nlbHelloWorldTester alb.Tester
	alb2048Tester       alb.Tester
	jobsPiTester        jobspi.Tester
	jobsEchoTester      jobsecho.Tester
	cronJobsTester      cronjobs.Tester
	csrsTester          csrs.Tester
	configMapsTester    configmaps.Tester
	secretsTester       secrets.Tester
	irsaTester          irsa.Tester
	fargateTester       fargate.Tester
	appMeshTester       appmesh.Tester
}

// New returns a new EKS kubetest2 Deployer.
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Deployer
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func New(cfg *eksconfig.Config) (*Tester, error) {
	fmt.Println("😎 🙏")
	fmt.Println(version.Version())
	fmt.Printf("\n*********************************\n")
	fmt.Printf("New %q\n", cfg.ConfigPath)
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
		err = nil
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
	if !fileutil.Exist(cfg.KubectlPath) {
		lg.Info("downloading kubectl", zap.String("kubectl-path", cfg.KubectlPath))
		f, err := os.Create(cfg.KubectlPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create %q (%v)", cfg.KubectlPath, err)
		}
		cfg.KubectlPath = f.Name()
		cfg.KubectlPath, _ = filepath.Abs(cfg.KubectlPath)
		if err := httpDownloadFile(lg, cfg.KubectlDownloadURL, f); err != nil {
			f.Close()
			return nil, err
		}
		if err := f.Close(); err != nil {
			return nil, fmt.Errorf("failed to close kubectl %v", err)
		}
	} else {
		lg.Info("skipping kubectl download; already exist", zap.String("kubectl-path", cfg.KubectlPath))
	}
	if err := fileutil.EnsureExecutable(cfg.KubectlPath); err != nil {
		// file may be already executable while the process does not own the file/directory
		// ref. https://github.com/aws/aws-k8s-tester/issues/66
		lg.Warn("failed to ensure executable", zap.Error(err))
		err = nil
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
		if !fileutil.Exist(cfg.AWSIAMAuthenticatorPath) {
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
				f.Close()
				return nil, err
			}
			if err := f.Close(); err != nil {
				return nil, fmt.Errorf("failed to close aws-iam-authenticator %v", err)
			}
		} else {
			lg.Info("skipping aws-iam-authenticator download; already exist", zap.String("aws-iam-authenticator-path", cfg.AWSIAMAuthenticatorPath))
		}
		if err := fileutil.EnsureExecutable(cfg.AWSIAMAuthenticatorPath); err != nil {
			// file may be already executable while the process does not own the file/directory
			// ref. https://github.com/aws/aws-k8s-tester/issues/66
			lg.Warn("failed to ensure executable", zap.Error(err))
			err = nil
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
	fmt.Println("EC2 API available!")

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
	fmt.Println("EKS API available!")
	ts.lg.Info("listing EKS clusters with limit 20", zap.Int("clusters", len(lresp.Clusters)))
	for _, v := range lresp.Clusters {
		ts.lg.Info("EKS cluster", zap.String("name", aws.StringValue(v)))
	}

	// update k8s client if cluster has already been created
	kcfg := k8sclient.EKSConfig{
		Region:         ts.cfg.Region,
		ClusterName:    ts.cfg.Name,
		ClientQPS:      ts.cfg.ClientQPS,
		ClientBurst:    ts.cfg.ClientBurst,
		KubeConfigPath: ts.cfg.KubeConfigPath,
	}
	if ts.cfg.Status != nil {
		kcfg.ClusterAPIServerEndpoint = ts.cfg.Status.ClusterAPIServerEndpoint
		kcfg.ClusterCADecoded = ts.cfg.Status.ClusterCADecoded
	}
	ts.k8sClientSet, err = k8sclient.NewEKS(ts.lg, kcfg)
	if err != nil {
		ts.lg.Warn("failed to create k8s client", zap.Error(err))
	}

	if err = ts.createSubTesters(); err != nil {
		return nil, err
	}

	return ts, nil
}

func (ts *Tester) createSubTesters() (err error) {
	fmt.Printf("\n*********************************\n")
	fmt.Printf("createSubTesters (%q)\n", ts.cfg.ConfigPath)

	ts.lg.Info("creating ngTester")
	ts.ngTester, err = ng.New(ng.Config{
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
		SSMAPI:    ts.ssmAPI,
		S3API:     ts.s3API,
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
		S3API:     ts.s3API,
	})
	if err != nil {
		return err
	}

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

	if ts.cfg.IsEnabledAddOnNLBHelloWorld() {
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

	if ts.cfg.IsEnabledAddOnALB2048() {
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

	if ts.cfg.IsEnabledAddOnJobsPi() {
		ts.lg.Info("creating jobsPiTester")
		ts.jobsPiTester, err = jobspi.New(jobspi.Config{
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

	if ts.cfg.IsEnabledAddOnJobsEcho() {
		ts.lg.Info("creating jobsEchoTester")
		ts.jobsEchoTester, err = jobsecho.New(jobsecho.Config{
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

	if ts.cfg.IsEnabledAddOnCronJobs() {
		ts.lg.Info("creating cronJobsTester")
		ts.cronJobsTester, err = cronjobs.New(cronjobs.Config{
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

	if ts.cfg.IsEnabledAddOnCSRs() {
		ts.lg.Info("creating csrsTester")
		ts.csrsTester, err = csrs.New(csrs.Config{
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

	if ts.cfg.IsEnabledAddOnConfigMaps() {
		ts.lg.Info("creating configMapsTester")
		ts.configMapsTester, err = configmaps.New(configmaps.Config{
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

	if ts.cfg.IsEnabledAddOnConfigMaps() {
		ts.lg.Info("creating configMapsTester")
		ts.configMapsTester, err = configmaps.New(configmaps.Config{
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

	if ts.cfg.IsEnabledAddOnSecrets() {
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

	if ts.cfg.IsEnabledAddOnIRSA() {
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

	if ts.cfg.IsEnabledAddOnFargate() {
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

	if ts.cfg.IsEnabledAddOnAppMesh() {
		ts.lg.Info("creating appMeshTester")
		ts.appMeshTester, err = appmesh.NewTester(appmesh.Config{
			Logger:    ts.lg,
			Stopc:     ts.stopCreationCh,
			Sig:       ts.interruptSig,
			EKSConfig: ts.cfg,
			K8SClient: ts,
			CFNAPI:    ts.cfnAPI,
		})
	}

	return ts.cfg.Sync()
}

// Up should provision a new cluster for testing.
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Deployer
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func (ts *Tester) Up() (err error) {
	fmt.Printf("\n*********************************\n")
	fmt.Printf("Up start (%q)\n", ts.cfg.ConfigPath)

	now := time.Now()

	defer func() {
		fmt.Printf("\n*********************************\n")
		fmt.Printf("Up.defer start (%q, %q)\n\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())

		if serr := ts.uploadToS3(); serr != nil {
			ts.lg.Warn("failed to upload artifacts to S3", zap.Error(serr))
		}

		if err == nil {
			if ts.cfg.Status.Up {
				fmt.Printf("\n*********************************\n")
				ts.lg.Sugar().Infof("SSH (%s)", ts.cfg.ConfigPath)
				fmt.Println(ts.cfg.SSHCommands())

				fmt.Printf("\n*********************************\n")
				ts.lg.Sugar().Infof("kubectl (%s)", ts.cfg.ConfigPath)
				fmt.Println(ts.cfg.KubectlCommands())

				ts.lg.Info("Up succeeded",
					zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
				)

				fmt.Printf("\n*********************************\n")
				ts.lg.Sugar().Infof("Up.defer end (%s, %s)", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
				fmt.Printf("\n\n😁 😁 :) Up success\n\n\n")
			} else {
				fmt.Printf("\n\n😲 😲 aborted Up ???\n\n\n")
			}
			return
		}

		if !ts.cfg.OnFailureDelete {
			if ts.cfg.Status.Up {
				fmt.Printf("\n*********************************\n")
				ts.lg.Sugar().Infof("SSH (%s)", ts.cfg.ConfigPath)
				fmt.Println(ts.cfg.SSHCommands())

				fmt.Printf("\n*********************************\n")
				ts.lg.Sugar().Infof("kubectl (%s)", ts.cfg.ConfigPath)
				fmt.Println(ts.cfg.KubectlCommands())
			}

			ts.lg.Warn("Up failed",
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
				zap.Error(err),
			)

			fmt.Printf("\n*********************************\n")
			ts.lg.Sugar().Infof("Up.defer end (%s, %s)", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
			fmt.Printf("\n\n😱 ☹ 😡 (-_-) Up fail\n\n\n")
			return
		}

		if ts.cfg.Status.Up {
			fmt.Printf("\n*********************************\n")
			ts.lg.Sugar().Infof("SSH (%s)", ts.cfg.ConfigPath)
			fmt.Println(ts.cfg.SSHCommands())

			fmt.Printf("\n*********************************\n")
			ts.lg.Sugar().Infof("kubectl (%s)", ts.cfg.ConfigPath)
			fmt.Println(ts.cfg.KubectlCommands())
		}

		fmt.Printf("\n*********************************\n")
		fmt.Printf("😱 ☹ 😡 (-_-) Up fail\n")
		ts.lg.Warn("Up failed; reverting resource creation",
			zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
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

		fmt.Printf("\n*********************************\n")
		ts.lg.Sugar().Infof("Up.defer end (%s, %s)", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		fmt.Printf("\n\n😱 ☹ 😡 (-_-) Up fail\n\n\n")
	}()

	ts.lg.Info("Up started",
		zap.String("version", version.Version()),
		zap.String("name", ts.cfg.Name),
	)
	defer ts.cfg.Sync()

	fmt.Printf("\n*********************************\n")
	fmt.Printf("createS3 (%q)\n", ts.cfg.ConfigPath)
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.interruptSig,
		ts.createS3,
	); err != nil {
		return err
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("createEncryption (%q)\n", ts.cfg.ConfigPath)
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.interruptSig,
		ts.createEncryption,
	); err != nil {
		return err
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("createKeyPair (%q)\n", ts.cfg.ConfigPath)
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.interruptSig,
		ts.createKeyPair,
	); err != nil {
		return err
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("createClusterRole (%q)\n", ts.cfg.ConfigPath)
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.interruptSig,
		ts.createClusterRole,
	); err != nil {
		return err
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("createVPC (%q)\n", ts.cfg.ConfigPath)
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.interruptSig,
		ts.createVPC,
	); err != nil {
		return err
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("createCluster (%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
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

	fmt.Printf("\n*********************************\n")
	fmt.Printf("checkHealth (%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
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
		fmt.Printf("\n*********************************\n")
		fmt.Printf("\nrunCommand CommandAfterCreateCluster (%q)\n", ts.cfg.CommandAfterCreateCluster)
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
		fmt.Printf("\nrunCommand output:\n\n%s\n", string(out))
	}

	if ts.cfg.IsEnabledAddOnNodeGroups() {
		if ts.ngTester == nil {
			return errors.New("ts.ngTester == nil when AddOnNodeGroups.Enable == true")
		}
		// create NG first, so MNG configmap update can be called afterwards
		fmt.Printf("\n*********************************\n")
		fmt.Printf("ngTester.Create (%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.interruptSig,
			ts.ngTester.Create,
		); err != nil {
			return err
		}
	}

	if ts.cfg.IsEnabledAddOnManagedNodeGroups() {
		if ts.ngTester == nil {
			return errors.New("ts.mngTester == nil when AddOnManagedNodeGroups.Enable == true")
		}
		fmt.Printf("\n*********************************\n")
		fmt.Printf("mngTester.Create (%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.interruptSig,
			ts.mngTester.Create,
		); err != nil {
			return err
		}
	}

	needGPU := false
	if ts.cfg.IsEnabledAddOnNodeGroups() {
	gpuFound1:
		for _, mv := range ts.cfg.AddOnNodeGroups.ASGs {
			switch mv.AMIType {
			case ec2config.AMITypeAL2X8664GPU:
				needGPU = true
				break gpuFound1
			}
		}
	}
	if !needGPU && ts.cfg.IsEnabledAddOnManagedNodeGroups() {
	gpuFound2:
		for _, mv := range ts.cfg.AddOnManagedNodeGroups.MNGs {
			switch mv.AMIType {
			case awseks.AMITypesAl2X8664Gpu:
				needGPU = true
				break gpuFound2
			}
		}
	}
	if needGPU {
		fmt.Printf("\n*********************************\n")
		fmt.Printf("gpuTester.InstallNvidiaDriver (%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.interruptSig,
			ts.gpuTester.InstallNvidiaDriver,
		); err != nil {
			ts.lg.Warn("failed to install Nvidia driver", zap.Error(err))
		}

		fmt.Printf("\n*********************************\n")
		fmt.Printf("gpuTester.RunNvidiaSMI (%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
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

	fmt.Printf("\n*********************************\n")
	fmt.Printf("checkHealth (%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.interruptSig,
		ts.checkHealth,
	); err != nil {
		return err
	}

	fmt.Printf("\n*********************************\n")
	ts.lg.Sugar().Infof("SSH (%s)", ts.cfg.ConfigPath)
	fmt.Println(ts.cfg.SSHCommands())

	fmt.Printf("\n*********************************\n")
	ts.lg.Sugar().Infof("kubectl (%s)", ts.cfg.ConfigPath)
	fmt.Println(ts.cfg.KubectlCommands())

	if ts.cfg.IsEnabledAddOnNLBHelloWorld() {
		if ts.nlbHelloWorldTester == nil {
			return errors.New("ts.nlbHelloWorldTester == nil when AddOnNLBHelloWorld.Enable == true")
		}
		fmt.Printf("\n*********************************\n")
		fmt.Printf("nlbHelloWorldTester.Create (%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnNLBHelloWorld.Namespace)
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

	if ts.cfg.IsEnabledAddOnALB2048() {
		if ts.alb2048Tester == nil {
			return errors.New("ts.alb2048Tester == nil when AddOnALB2048.Enable == true")
		}
		fmt.Printf("\n*********************************\n")
		fmt.Printf("alb2048Tester.Create (%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnALB2048.Namespace)
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

	if ts.cfg.IsEnabledAddOnJobsPi() {
		if ts.jobsPiTester == nil {
			return errors.New("ts.jobsPiTester == nil when AddOnJobsPi.Enable == true")
		}
		fmt.Printf("\n*********************************\n")
		fmt.Printf("jobsPiTester.Create (%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnJobsPi.Namespace)
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.interruptSig,
			ts.jobsPiTester.Create,
		); err != nil {
			return err
		}
	}

	if ts.cfg.IsEnabledAddOnJobsEcho() {
		if ts.jobsEchoTester == nil {
			return errors.New("ts.jobsEchoTester == nil when AddOnJobsEcho.Enable == true")
		}
		fmt.Printf("\n*********************************\n")
		fmt.Printf("jobsEchoTester.Create (%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnJobsEcho.Namespace)
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.interruptSig,
			ts.jobsEchoTester.Create,
		); err != nil {
			return err
		}
	}

	if ts.cfg.IsEnabledAddOnCronJobs() {
		if ts.cronJobsTester == nil {
			return errors.New("ts.cronJobsTester == nil when AddOnCronJobs.Enable == true")
		}
		fmt.Printf("\n*********************************\n")
		fmt.Printf("cronJobsTester.Create (%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnCronJobs.Namespace)
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.interruptSig,
			ts.cronJobsTester.Create,
		); err != nil {
			return err
		}
	}

	if ts.cfg.IsEnabledAddOnCSRs() {
		if ts.csrsTester == nil {
			return errors.New("ts.csrsTester == nil when AddOnCSRs.Enable == true")
		}
		fmt.Printf("\n*********************************\n")
		fmt.Printf("csrsTester.Create (%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnCSRs.Namespace)
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.interruptSig,
			ts.csrsTester.Create,
		); err != nil {
			return err
		}
	}

	if ts.cfg.IsEnabledAddOnConfigMaps() {
		if ts.configMapsTester == nil {
			return errors.New("ts.configMapsTester == nil when AddOnConfigMaps.Enable == true")
		}
		fmt.Printf("\n*********************************\n")
		fmt.Printf("configMapsTester.Create (%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnConfigMaps.Namespace)
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.interruptSig,
			ts.configMapsTester.Create,
		); err != nil {
			return err
		}
	}

	if ts.cfg.IsEnabledAddOnSecrets() {
		if ts.secretsTester == nil {
			return errors.New("ts.secretsTester == nil when AddOnSecrets.Enable == true")
		}
		fmt.Printf("\n*********************************\n")
		fmt.Printf("secretsTester.Create (%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnSecrets.Namespace)
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

	if ts.cfg.IsEnabledAddOnIRSA() {
		if ts.irsaTester == nil {
			return errors.New("ts.irsaTester == nil when AddOnIRSA.Enable == true")
		}
		fmt.Printf("\n*********************************\n")
		fmt.Printf("irsaTester.Create (%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnIRSA.Namespace)
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

	if ts.cfg.IsEnabledAddOnFargate() {
		if ts.fargateTester == nil {
			return errors.New("ts.fargateTester == nil when AddOnFargate.Enable == true")
		}
		fmt.Printf("\n*********************************\n")
		fmt.Printf("fargateTester.Create (%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnFargate.Namespace)
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

	if ts.cfg.IsEnabledAddOnAppMesh() {
		if ts.appMeshTester == nil {
			return errors.New("ts.appMeshTester == nil when AddOnAppMesh.Enable == true")
		}
		fmt.Printf("\n*********************************\n")
		fmt.Printf("appMeshTester.Create (%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnAppMesh.Namespace)
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.interruptSig,
			ts.appMeshTester.Create,
		); err != nil {
			return err
		}
	}

	if ts.cfg.IsEnabledAddOnNodeGroups() && ts.cfg.AddOnNodeGroups.FetchLogs {
		if ts.ngTester == nil {
			return errors.New("ts.ngTester == nil when AddOnNodeGroups.Enable == true")
		}

		fmt.Printf("\n*********************************\n")
		fmt.Printf("ngTester.FetchLogs (%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())

		waitDur := 20 * time.Second
		ts.lg.Info("sleeping before ngTester.FetchLogs", zap.Duration("wait", waitDur))
		time.Sleep(waitDur)

		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.interruptSig,
			ts.ngTester.FetchLogs,
		); err != nil {
			return err
		}
	}

	if ts.cfg.IsEnabledAddOnManagedNodeGroups() && ts.cfg.AddOnManagedNodeGroups.FetchLogs {
		if ts.mngTester == nil {
			return errors.New("ts.mngTester == nil when AddOnManagedNodeGroups.Enable == true")
		}

		fmt.Printf("\n*********************************\n")
		fmt.Printf("mngTester.FetchLogs (%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())

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
	}

	if (ts.cfg.IsEnabledAddOnNodeGroups() && ts.cfg.AddOnNodeGroups.FetchLogs) ||
		(ts.cfg.IsEnabledAddOnManagedNodeGroups() && ts.cfg.AddOnManagedNodeGroups.FetchLogs) {

		if ts.cfg.IsEnabledAddOnSecrets() {
			fmt.Printf("\n*********************************\n")
			fmt.Printf("secretsTester.AggregateResults (%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnSecrets.Namespace)
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

		if ts.cfg.IsEnabledAddOnIRSA() {
			fmt.Printf("\n*********************************\n")
			fmt.Printf("irsaTester.AggregateResults (%q, \"%s --namespace=%s get all\")\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand(), ts.cfg.AddOnIRSA.Namespace)
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
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("checkHealth (%q, %q)\n", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
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
		fmt.Printf("\nrunCommand CommandAfterCreateAddOns (%q)\n", ts.cfg.CommandAfterCreateAddOns)
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
		fmt.Printf("\nrunCommand output:\n\n%s\n", string(out))
	}

	return ts.cfg.Sync()
}

// Down cancels the cluster creation and destroy the test cluster if any.
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Deployer
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func (ts *Tester) Down() error {
	ts.downMu.Lock()
	defer ts.downMu.Unlock()
	return ts.down()
}

func (ts *Tester) down() (err error) {
	fmt.Printf("\n*********************************\n")
	fmt.Printf("Down start (%q)\n\n", ts.cfg.ConfigPath)

	now := time.Now()
	ts.lg.Warn("starting Down",
		zap.String("name", ts.cfg.Name),
		zap.String("cluster-arn", ts.cfg.Status.ClusterARN),
	)
	if serr := ts.uploadToS3(); serr != nil {
		ts.lg.Warn("failed to upload artifacts to S3", zap.Error(serr))
	}

	defer func() {
		ts.cfg.Sync()
		if err == nil {
			fmt.Printf("\n*********************************\n")
			fmt.Printf("Down.defer end (%q)\n\n", ts.cfg.ConfigPath)
			fmt.Printf("\n\n😁 😁 :) Down success\n\n\n")

			ts.lg.Info("successfully finished Down",
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)

		} else {
			fmt.Printf("\n*********************************\n")
			fmt.Printf("Down.defer end (%q)\n\n", ts.cfg.ConfigPath)
			fmt.Printf("\n\n😱 ☹ 😡 (-_-) Down fail\n\n\n")

			ts.lg.Info("failed Down",
				zap.Error(err),
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)
		}
	}()

	var errs []string

	fmt.Printf("\n*********************************\n")
	fmt.Printf("deleteKeyPair (%q)\n", ts.cfg.ConfigPath)
	if err := ts.deleteKeyPair(); err != nil {
		ts.lg.Warn("failed to delete key pair", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if ts.cfg.IsEnabledAddOnNodeGroups() || ts.cfg.IsEnabledAddOnManagedNodeGroups() {
		if ts.cfg.IsEnabledAddOnFargate() && ts.cfg.AddOnFargate.Created {
			fmt.Printf("\n*********************************\n")
			fmt.Printf("fargateTester.Delete (%q)\n", ts.cfg.ConfigPath)
			if err := ts.fargateTester.Delete(); err != nil {
				ts.lg.Warn("fargateTester.Delete failed", zap.Error(err))
				errs = append(errs, err.Error())
			}
		}

		if ts.cfg.IsEnabledAddOnIRSA() && ts.cfg.AddOnIRSA.Created {
			fmt.Printf("\n*********************************\n")
			fmt.Printf("irsaTester.Delete (%q)\n", ts.cfg.ConfigPath)
			if err := ts.irsaTester.Delete(); err != nil {
				ts.lg.Warn("irsaTester.Delete failed", zap.Error(err))
				errs = append(errs, err.Error())
			}
		}

		if ts.cfg.IsEnabledAddOnSecrets() && ts.cfg.AddOnSecrets.Created {
			fmt.Printf("\n*********************************\n")
			fmt.Printf("secretsTester.Delete (%q)\n", ts.cfg.ConfigPath)
			if err := ts.secretsTester.Delete(); err != nil {
				ts.lg.Warn("secretsTester.Delete failed", zap.Error(err))
				errs = append(errs, err.Error())
			}
		}

		if ts.cfg.IsEnabledAddOnConfigMaps() && ts.cfg.AddOnConfigMaps.Created {
			fmt.Printf("\n*********************************\n")
			fmt.Printf("configMapsTester.Delete (%q)\n", ts.cfg.ConfigPath)
			if err := ts.configMapsTester.Delete(); err != nil {
				ts.lg.Warn("configMapsTester.Delete failed", zap.Error(err))
				errs = append(errs, err.Error())
			}
		}

		if ts.cfg.IsEnabledAddOnCSRs() && ts.cfg.AddOnCSRs.Created {
			fmt.Printf("\n*********************************\n")
			fmt.Printf("csrsTester.Delete (%q)\n", ts.cfg.ConfigPath)
			if err := ts.csrsTester.Delete(); err != nil {
				ts.lg.Warn("csrsTester.Delete failed", zap.Error(err))
				errs = append(errs, err.Error())
			}
		}

		if ts.cfg.IsEnabledAddOnCronJobs() && ts.cfg.AddOnCronJobs.Created {
			fmt.Printf("\n*********************************\n")
			fmt.Printf("cronJobsTester.Delete (%q)\n", ts.cfg.ConfigPath)
			if err := ts.cronJobsTester.Delete(); err != nil {
				ts.lg.Warn("cronJobsTester.Delete failed", zap.Error(err))
				errs = append(errs, err.Error())
			}
		}

		if ts.cfg.IsEnabledAddOnJobsEcho() && ts.cfg.AddOnJobsEcho.Created {
			fmt.Printf("\n*********************************\n")
			fmt.Printf("jobsEchoTester.Delete (%q)\n", ts.cfg.ConfigPath)
			if err := ts.jobsEchoTester.Delete(); err != nil {
				ts.lg.Warn("jobsEchoTester.Delete failed", zap.Error(err))
				errs = append(errs, err.Error())
			}
		}

		if ts.cfg.IsEnabledAddOnJobsPi() && ts.cfg.AddOnJobsPi.Created {
			fmt.Printf("\n*********************************\n")
			fmt.Printf("jobsPiTester.Delete (%q)\n", ts.cfg.ConfigPath)
			if err := ts.jobsPiTester.Delete(); err != nil {
				ts.lg.Warn("jobsPiTester.Delete failed", zap.Error(err))
				errs = append(errs, err.Error())
			}
		}

		if ts.cfg.IsEnabledAddOnALB2048() && ts.cfg.AddOnALB2048.Created {
			fmt.Printf("\n*********************************\n")
			fmt.Printf("alb2048Tester.Delete (%q)\n", ts.cfg.ConfigPath)
			if err := ts.alb2048Tester.Delete(); err != nil {
				ts.lg.Warn("alb2048Tester.Delete failed", zap.Error(err))
				errs = append(errs, err.Error())
			} else {
				waitDur := time.Minute
				ts.lg.Info("sleeping after deleting ALB", zap.Duration("wait", waitDur))
				time.Sleep(waitDur)
			}
		}

		if ts.cfg.IsEnabledAddOnNLBHelloWorld() && ts.cfg.AddOnNLBHelloWorld.Created {
			fmt.Printf("\n*********************************\n")
			fmt.Printf("nlbHelloWorldTester.Delete (%q)\n", ts.cfg.ConfigPath)
			if err := ts.nlbHelloWorldTester.Delete(); err != nil {
				ts.lg.Warn("nlbHelloWorldTester.Delete failed", zap.Error(err))
				errs = append(errs, err.Error())
			} else {
				waitDur := time.Minute
				ts.lg.Info("sleeping after deleting NLB", zap.Duration("wait", waitDur))
				time.Sleep(waitDur)
			}
		}
		if ts.cfg.IsEnabledAddOnAppMesh() && ts.cfg.AddOnAppMesh.Created {
			fmt.Printf("\n*********************************\n")
			fmt.Printf("appMeshTester.Delete (%q)\n", ts.cfg.ConfigPath)
			if err := ts.appMeshTester.Delete(); err != nil {
				ts.lg.Warn("appMeshTester.Delete failed", zap.Error(err))
				errs = append(errs, err.Error())
			} else {
				waitDur := time.Minute
				ts.lg.Info("sleeping after deleting appMesh", zap.Duration("wait", waitDur))
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
	if (ts.cfg.IsEnabledAddOnNodeGroups() || ts.cfg.IsEnabledAddOnManagedNodeGroups()) &&
		((ts.cfg.IsEnabledAddOnALB2048() && ts.cfg.AddOnALB2048.Created) ||
			(ts.cfg.IsEnabledAddOnNLBHelloWorld() && ts.cfg.AddOnNLBHelloWorld.Created)) {
		waitDur := 2 * time.Minute
		ts.lg.Info("sleeping after deleting LB", zap.Duration("wait", waitDur))
		time.Sleep(waitDur)
	}

	// following need to be run in order to resolve delete dependency
	// e.g. cluster must be deleted before VPC delete
	if ts.cfg.IsEnabledAddOnManagedNodeGroups() {
		fmt.Printf("\n*********************************\n")
		fmt.Printf("mngTester.Delete (%q)\n", ts.cfg.ConfigPath)
		if err := ts.mngTester.Delete(); err != nil {
			ts.lg.Warn("mngTester.Delete failed", zap.Error(err))
			errs = append(errs, err.Error())
		}

		waitDur := 10 * time.Second
		ts.lg.Info("sleeping before cluster deletion", zap.Duration("wait", waitDur))
		time.Sleep(waitDur)
	}

	if ts.cfg.IsEnabledAddOnNodeGroups() {
		fmt.Printf("\n*********************************\n")
		fmt.Printf("ngTester.Delete (%q)\n", ts.cfg.ConfigPath)
		if err := ts.ngTester.Delete(); err != nil {
			ts.lg.Warn("ngTester.Delete failed", zap.Error(err))
			errs = append(errs, err.Error())
		}

		waitDur := 10 * time.Second
		ts.lg.Info("sleeping before cluster deletion", zap.Duration("wait", waitDur))
		time.Sleep(waitDur)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("deleteCluster (%q)\n", ts.cfg.ConfigPath)
	if err := ts.deleteCluster(); err != nil {
		ts.lg.Warn("deleteCluster failed", zap.Error(err))
		errs = append(errs, err.Error())
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("deleteEncryption (%q)\n", ts.cfg.ConfigPath)
	if err := ts.deleteEncryption(); err != nil {
		ts.lg.Warn("deleteEncryption failed", zap.Error(err))
		errs = append(errs, err.Error())
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("deleteClusterRole (%q)\n", ts.cfg.ConfigPath)
	if err := ts.deleteClusterRole(); err != nil {
		ts.lg.Warn("deleteClusterRole failed", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if ts.cfg.Parameters.VPCCreate { // VPC was created
		waitDur := 30 * time.Second
		ts.lg.Info("sleeping before VPC deletion", zap.Duration("wait", waitDur))
		time.Sleep(waitDur)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("deleteVPC (%q)\n", ts.cfg.ConfigPath)
	if err := ts.deleteVPC(); err != nil {
		ts.lg.Warn("deleteVPC failed", zap.Error(err))
		errs = append(errs, err.Error())
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("deleteS3 (%q)\n", ts.cfg.ConfigPath)
	if err := ts.deleteS3(); err != nil {
		ts.lg.Warn("deleteS3 failed", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}
	return ts.cfg.Sync()
}

// IsUp should return true if a test cluster is successfully provisioned.
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Deployer
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func (ts *Tester) IsUp() (up bool, err error) {
	if ts.cfg == nil {
		return false, nil
	}
	if ts.cfg.Status == nil {
		return false, nil
	}
	if !ts.cfg.Status.Up {
		return false, nil
	}
	return true, ts.checkHealth()
}

// DumpClusterLogs should export logs from the cluster. It may be called
// multiple times. Options for this should come from New(...)
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Deployer
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func (ts *Tester) DumpClusterLogs() error {
	if err := ts.ngTester.FetchLogs(); err != nil {
		return err
	}
	return ts.mngTester.FetchLogs()
}

// DownloadClusterLogs dumps all logs to artifact directory.
// Let default kubetest log dumper handle all artifact uploads.
// See https://github.com/kubernetes/test-infra/pull/9811/files#r225776067.
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Deployer
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func (ts *Tester) DownloadClusterLogs(artifactDir, _ string) error {
	return ts.mngTester.DownloadClusterLogs(artifactDir)
}

// Build should build kubernetes and package it in whatever format
// the deployer consumes.
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Deployer
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func (ts *Tester) Build() error {
	// no-op
	return nil
}

// LoadConfig reloads configuration from disk to read the latest
// cluster configuration and its states.
// It's either reloaded from disk or returned from embedded EKS deployer.
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Deployer
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func (ts *Tester) LoadConfig() (eksconfig.Config, error) {
	return *ts.cfg, nil
}

// KubernetesClientSet returns Kubernetes Go client.
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Deployer
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func (ts *Tester) KubernetesClientSet() *kubernetes.Clientset {
	if ts.k8sClientSet == nil {
		kcfg := k8sclient.EKSConfig{
			Region:         ts.cfg.Region,
			ClusterName:    ts.cfg.Name,
			ClientQPS:      ts.cfg.ClientQPS,
			ClientBurst:    ts.cfg.ClientBurst,
			KubeConfigPath: ts.cfg.KubeConfigPath,
		}
		if ts.cfg.Status != nil {
			kcfg.ClusterAPIServerEndpoint = ts.cfg.Status.ClusterAPIServerEndpoint
			kcfg.ClusterCADecoded = ts.cfg.Status.ClusterCADecoded
		}
		var err error
		ts.k8sClientSet, err = k8sclient.NewEKS(ts.lg, kcfg)
		if err != nil {
			ts.lg.Warn("failed to create k8s client set", zap.Error(err))
			return nil
		}
	}
	return ts.k8sClientSet
}

// Kubeconfig returns a path to a kubeconfig file for the cluster.
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Deployer
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func (ts *Tester) Kubeconfig() (string, error) {
	if ts.cfg == nil {
		return "", errors.New("empty tester object")
	}
	return ts.cfg.KubeConfigPath, nil
}

// Provider returns the kubernetes provider for legacy deployers.
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Deployer
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func (ts *Tester) Provider() string {
	return "eks"
}

// HelpRequested true, help text will be shown to the user after instancing
// the deployer and tester.
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func (ts *Tester) HelpRequested() bool {
	return false
}

// ShouldBuild true, kubetest2 will be calling deployer.Build.
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func (ts *Tester) ShouldBuild() bool {
	return false
}

// ShouldUp true, kubetest2 will be calling deployer.Up.
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func (ts *Tester) ShouldUp() bool {
	if ts.cfg == nil {
		return false
	}
	return !ts.cfg.Status.Up
}

// ShouldDown true, kubetest2 will be calling deployer.Down.
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func (ts *Tester) ShouldDown() bool {
	if ts.cfg == nil {
		return false
	}
	return ts.cfg.Status.Up
}

// ShouldTest true, kubetest2 will be calling tester.Test.
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func (ts *Tester) ShouldTest() bool {
	if ts.cfg == nil {
		return false
	}
	return ts.cfg.Status.Up
}

// ArtifactsDir returns the path to the directory where artifacts should be written
// (including metadata files like junit_runner.xml).
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func (ts *Tester) ArtifactsDir() string {
	if ts.cfg == nil {
		return ""
	}
	if ts.cfg.IsEnabledAddOnManagedNodeGroups() {
		return ts.cfg.AddOnManagedNodeGroups.LogsDir
	}
	if ts.cfg.IsEnabledAddOnNodeGroups() {
		return ts.cfg.AddOnNodeGroups.LogsDir
	}
	return ""
}
