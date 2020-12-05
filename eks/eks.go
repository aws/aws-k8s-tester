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
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	alb_2048 "github.com/aws/aws-k8s-tester/eks/alb-2048"
	app_mesh "github.com/aws/aws-k8s-tester/eks/app-mesh"
	"github.com/aws/aws-k8s-tester/eks/cluster"
	"github.com/aws/aws-k8s-tester/eks/cluster-loader/clusterloader2"
	cluster_loader_local "github.com/aws/aws-k8s-tester/eks/cluster-loader/local"
	cluster_loader_remote "github.com/aws/aws-k8s-tester/eks/cluster-loader/remote"
	cluster_version_upgrade "github.com/aws/aws-k8s-tester/eks/cluster/version-upgrade"
	"github.com/aws/aws-k8s-tester/eks/clusterautoscaler"
	cni_vpc "github.com/aws/aws-k8s-tester/eks/cni-vpc"
	config_maps_local "github.com/aws/aws-k8s-tester/eks/configmaps/local"
	config_maps_remote "github.com/aws/aws-k8s-tester/eks/configmaps/remote"
	"github.com/aws/aws-k8s-tester/eks/conformance"
	cron_jobs "github.com/aws/aws-k8s-tester/eks/cron-jobs"
	csi_ebs "github.com/aws/aws-k8s-tester/eks/csi-ebs"
	csrs_local "github.com/aws/aws-k8s-tester/eks/csrs/local"
	csrs_remote "github.com/aws/aws-k8s-tester/eks/csrs/remote"
	cuda_vector_add "github.com/aws/aws-k8s-tester/eks/cuda-vector-add"
	cw_agent "github.com/aws/aws-k8s-tester/eks/cw-agent"
	"github.com/aws/aws-k8s-tester/eks/fargate"
	"github.com/aws/aws-k8s-tester/eks/fluentd"
	"github.com/aws/aws-k8s-tester/eks/gpu"
	hollow_nodes_local "github.com/aws/aws-k8s-tester/eks/hollow-nodes/local"
	hollow_nodes_remote "github.com/aws/aws-k8s-tester/eks/hollow-nodes/remote"
	"github.com/aws/aws-k8s-tester/eks/irsa"
	irsa_fargate "github.com/aws/aws-k8s-tester/eks/irsa-fargate"
	jobs_echo "github.com/aws/aws-k8s-tester/eks/jobs-echo"
	jobs_pi "github.com/aws/aws-k8s-tester/eks/jobs-pi"
	jupyter_hub "github.com/aws/aws-k8s-tester/eks/jupyter-hub"
	"github.com/aws/aws-k8s-tester/eks/kubeflow"
	kubernetes_dashboard "github.com/aws/aws-k8s-tester/eks/kubernetes-dashboard"
	metrics_server "github.com/aws/aws-k8s-tester/eks/metrics-server"
	"github.com/aws/aws-k8s-tester/eks/mng"
	"github.com/aws/aws-k8s-tester/eks/ng"
	nlb_guestbook "github.com/aws/aws-k8s-tester/eks/nlb-guestbook"
	nlb_hello_world "github.com/aws/aws-k8s-tester/eks/nlb-hello-world"
	"github.com/aws/aws-k8s-tester/eks/overprovisioning"
	php_apache "github.com/aws/aws-k8s-tester/eks/php-apache"
	prometheus_grafana "github.com/aws/aws-k8s-tester/eks/prometheus-grafana"
	secrets_local "github.com/aws/aws-k8s-tester/eks/secrets/local"
	secrets_remote "github.com/aws/aws-k8s-tester/eks/secrets/remote"
	stresser_local "github.com/aws/aws-k8s-tester/eks/stresser/local"
	stresser_remote "github.com/aws/aws-k8s-tester/eks/stresser/remote"
	"github.com/aws/aws-k8s-tester/eks/tester"
	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eks/wordpress"
	"github.com/aws/aws-k8s-tester/eksconfig"
	pkg_aws "github.com/aws/aws-k8s-tester/pkg/aws"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/httputil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-k8s-tester/pkg/terminal"
	"github.com/aws/aws-k8s-tester/pkg/user"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	aws_eks "github.com/aws/aws-sdk-go/service/eks"
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
	"k8s.io/klog"
	"k8s.io/utils/exec"
)

// Tester implements "kubetest2" Deployer.
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc
type Tester struct {
	color func(string) string

	stopCreationCh     chan struct{}
	stopCreationChOnce *sync.Once

	osSig chan os.Signal

	downMu *sync.Mutex

	lg        *zap.Logger
	logWriter io.Writer
	logFile   *os.File

	cfg *eksconfig.Config

	awsSession *session.Session

	iamAPI   iamiface.IAMAPI
	kmsAPI   kmsiface.KMSAPI
	ssmAPI   ssmiface.SSMAPI
	cfnAPI   cloudformationiface.CloudFormationAPI
	ec2API   ec2iface.EC2API
	s3API    s3iface.S3API
	cwAPI    cloudwatchiface.CloudWatchAPI
	asgAPI   autoscalingiface.AutoScalingAPI
	elbv2API elbv2iface.ELBV2API

	ecrAPISameRegion ecriface.ECRAPI

	// used for EKS + EKS MNG API calls
	eksSession *session.Session
	eksAPI     eksiface.EKSAPI

	s3Uploaded bool

	clusterTester cluster.Tester
	k8sClient     k8s_client.EKS

	// only create/install, no need delete
	cniTester eks_tester.Tester

	ngTester  ng.Tester
	mngTester mng.Tester
	gpuTester gpu.Tester

	// TODO, Shift to "Addon" api for ordered installation
	testers []eks_tester.Tester

	// Addons constructs a dependency ordering of Addons
	addons [][]tester.Addon
}

// New returns a new EKS kubetest2 Deployer.
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Deployer
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func New(cfg *eksconfig.Config) (ts *Tester, err error) {
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		return nil, err
	}

	lg, logWriter, logFile, err := logutil.NewWithStderrWriter(cfg.LogLevel, cfg.LogOutputs)
	if err != nil {
		return nil, err
	}
	_ = zap.ReplaceGlobals(lg)
	lg.Info("set up log writer and file", zap.Strings("outputs", cfg.LogOutputs))

	isColor := cfg.LogColor
	co, cerr := terminal.IsColor()
	if cerr == nil {
		lg.Info("requested output in color", zap.String("output", co), zap.Error(cerr))
		colorstring.Printf("\n\n[light_green]HELLO COLOR\n\n")
		isColor = true
	} else if !cfg.LogColorOverride {
		lg.Warn("requested output in color but not supported; overriding", zap.String("output", co), zap.Error(cerr))
		isColor = false
	} else {
		lg.Info("requested output color", zap.Bool("is-color", isColor), zap.String("output", co), zap.Error(cerr))
	}
	cfg.LogColor = isColor
	cfg.Sync()

	colorize := cfg.Colorize

	fmt.Fprintf(logWriter, colorize("\n\n\n[yellow]*********************************\n"))
	fmt.Fprintln(logWriter, "😎 🙏 🚶 ✔️ 👍")
	fmt.Fprintf(logWriter, colorize("[light_green]New %q [default](%q)\n\n"), cfg.ConfigPath, version.Version())

	if err = fileutil.EnsureExecutable(cfg.AWSCLIPath); err != nil {
		// file may be already executable while the process does not own the file/directory
		// ref. https://github.com/aws/aws-k8s-tester/issues/66
		lg.Warn("failed to ensure executable", zap.Error(err))
		err = nil
	}

	var vo []byte

	// aws --version
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	vo, err = exec.New().CommandContext(ctx, cfg.AWSCLIPath, "--version").CombinedOutput()
	cancel()
	if err != nil {
		return nil, fmt.Errorf("'aws --version' failed (output %q, error %v); required for 'aws eks update-kubeconfig'", string(vo), err)
	}
	lg.Info(
		"aws version",
		zap.String("aws-cli-path", cfg.AWSCLIPath),
		zap.String("aws-version", string(vo)),
	)

	lg.Info("mkdir", zap.String("kubectl-path-dir", filepath.Dir(cfg.KubectlPath)))
	if err = os.MkdirAll(filepath.Dir(cfg.KubectlPath), 0700); err != nil {
		return nil, fmt.Errorf("could not create %q (%v)", filepath.Dir(cfg.KubectlPath), err)
	}
	if !fileutil.Exist(cfg.KubectlPath) {
		if cfg.KubectlDownloadURL == "" {
			return nil, fmt.Errorf("%q does not exist but no download URL", cfg.KubectlPath)
		}
		cfg.KubectlPath, _ = filepath.Abs(cfg.KubectlPath)
		lg.Info("downloading kubectl", zap.String("kubectl-path", cfg.KubectlPath))
		if err = httputil.Download(lg, os.Stderr, cfg.KubectlDownloadURL, cfg.KubectlPath); err != nil {
			return nil, err
		}
	} else {
		lg.Info("skipping kubectl download; already exist", zap.String("kubectl-path", cfg.KubectlPath))
	}
	if err = fileutil.EnsureExecutable(cfg.KubectlPath); err != nil {
		// file may be already executable while the process does not own the file/directory
		// ref. https://github.com/aws/aws-k8s-tester/issues/66
		lg.Warn("failed to ensure executable", zap.Error(err))
		err = nil
	}
	// kubectl version --client=true
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	vo, err = exec.New().CommandContext(ctx, cfg.KubectlPath, "version", "--client=true").CombinedOutput()
	cancel()
	if err != nil {
		return nil, fmt.Errorf("'kubectl version' failed (output %q, error %v)", string(vo), err)
	}
	lg.Info(
		"kubectl version",
		zap.String("kubectl-path", cfg.KubectlPath),
		zap.String("kubectl-version", string(vo)),
	)

	if cfg.AWSIAMAuthenticatorPath != "" && cfg.AWSIAMAuthenticatorDownloadURL != "" {
		lg.Info("mkdir", zap.String("aws-iam-authenticator-path-dir", filepath.Dir(cfg.AWSIAMAuthenticatorPath)))
		if err = os.MkdirAll(filepath.Dir(cfg.AWSIAMAuthenticatorPath), 0700); err != nil {
			return nil, fmt.Errorf("could not create %q (%v)", filepath.Dir(cfg.AWSIAMAuthenticatorPath), err)
		}
		if !fileutil.Exist(cfg.AWSIAMAuthenticatorPath) {
			cfg.AWSIAMAuthenticatorPath, _ = filepath.Abs(cfg.AWSIAMAuthenticatorPath)
			lg.Info("downloading aws-iam-authenticator", zap.String("aws-iam-authenticator-path", cfg.AWSIAMAuthenticatorPath))
			if err = os.RemoveAll(cfg.AWSIAMAuthenticatorPath); err != nil {
				return nil, err
			}
			if err = httputil.Download(lg, os.Stderr, cfg.AWSIAMAuthenticatorDownloadURL, cfg.AWSIAMAuthenticatorPath); err != nil {
				return nil, err
			}
		} else {
			lg.Info("skipping aws-iam-authenticator download; already exist", zap.String("aws-iam-authenticator-path", cfg.AWSIAMAuthenticatorPath))
		}
		if err = fileutil.EnsureExecutable(cfg.AWSIAMAuthenticatorPath); err != nil {
			// file may be already executable while the process does not own the file/directory
			// ref. https://github.com/aws/aws-k8s-tester/issues/66
			lg.Warn("failed to ensure executable", zap.Error(err))
			err = nil
		}
		// aws-iam-authenticator version
		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		vo, err = exec.New().CommandContext(ctx, cfg.AWSIAMAuthenticatorPath, "version").CombinedOutput()
		cancel()
		if err != nil {
			return nil, fmt.Errorf("'aws-iam-authenticator version' failed (output %q, error %v)", string(vo), err)
		}
		lg.Info(
			"aws-iam-authenticator version",
			zap.String("aws-iam-authenticator-path", cfg.AWSIAMAuthenticatorPath),
			zap.String("aws-iam-authenticator-version", string(vo)),
		)
	}

	ts = &Tester{
		color:              colorize,
		stopCreationCh:     make(chan struct{}),
		stopCreationChOnce: new(sync.Once),
		osSig:              make(chan os.Signal),
		downMu:             new(sync.Mutex),
		lg:                 lg,
		logWriter:          logWriter,
		logFile:            logFile,
		cfg:                cfg,
	}
	signal.Notify(ts.osSig, syscall.SIGTERM, syscall.SIGINT)

	defer ts.cfg.Sync()

	awsCfg := &pkg_aws.Config{
		Logger:        ts.lg,
		DebugAPICalls: ts.cfg.LogLevel == "debug",
		Partition:     ts.cfg.Partition,
		Region:        ts.cfg.Region,
	}
	var stsOutput *sts.GetCallerIdentityOutput
	ts.awsSession, stsOutput, ts.cfg.Status.AWSCredentialPath, err = pkg_aws.New(awsCfg)
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
	if _, err = ts.ec2API.DescribeInstances(&ec2.DescribeInstancesInput{MaxResults: aws.Int64(5)}); err != nil {
		return nil, fmt.Errorf("failed to describe instances using EC2 API (%v)", err)
	}
	fmt.Fprintln(ts.logWriter, "EC2 API available!")

	ts.s3API = s3.New(ts.awsSession)
	ts.cwAPI = cloudwatch.New(ts.awsSession)
	ts.asgAPI = autoscaling.New(ts.awsSession)
	ts.elbv2API = elbv2.New(ts.awsSession)

	ts.ecrAPISameRegion = ecr.New(ts.awsSession, aws.NewConfig().WithRegion(ts.cfg.Region))

	ts.lg.Info("checking ECR API availability; listing repositories")
	var ecrResp *ecr.DescribeRepositoriesOutput
	ecrResp, err = ts.ecrAPISameRegion.DescribeRepositories(&ecr.DescribeRepositoriesInput{
		MaxResults: aws.Int64(20),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe repositories using ECR API (%v)", err)
	}
	ts.lg.Info("listed repositories with limit 20", zap.Int("repositories", len(ecrResp.Repositories)))
	for _, v := range ecrResp.Repositories {
		ts.lg.Info("EKS repository", zap.String("repository-uri", aws.StringValue(v.RepositoryUri)))
	}

	// create a separate session for EKS (for resolver endpoint)
	ts.eksSession, _, ts.cfg.Status.AWSCredentialPath, err = pkg_aws.New(&pkg_aws.Config{
		Logger:        ts.lg,
		DebugAPICalls: ts.cfg.LogLevel == "debug",
		Partition:     ts.cfg.Partition,
		Region:        ts.cfg.Region,
		ResolverURL:   ts.cfg.Parameters.ResolverURL,
		SigningName:   ts.cfg.Parameters.SigningName,
	})
	if err != nil {
		return nil, err
	}
	ts.eksAPI = aws_eks.New(ts.eksSession)

	ts.lg.Info("checking EKS API availability; listing clusters")
	var eksListResp *aws_eks.ListClustersOutput
	eksListResp, err = ts.eksAPI.ListClusters(&aws_eks.ListClustersInput{
		MaxResults: aws.Int64(20),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters using EKS API (%v)", err)
	}
	ts.lg.Info("listed clusters with limit 20", zap.Int("clusters", len(eksListResp.Clusters)))
	for _, v := range eksListResp.Clusters {
		ts.lg.Info("EKS cluster", zap.String("name", aws.StringValue(v)))
	}

	// update k8s client if cluster has already been created
	ts.lg.Info("creating k8s client from previous states if any")
	kcfg := &k8s_client.EKSConfig{
		Logger:                             ts.lg,
		Region:                             ts.cfg.Region,
		ClusterName:                        ts.cfg.Name,
		KubeConfigPath:                     ts.cfg.KubeConfigPath,
		KubectlPath:                        ts.cfg.KubectlPath,
		ServerVersion:                      ts.cfg.Parameters.Version,
		EncryptionEnabled:                  ts.cfg.Parameters.EncryptionCMKARN != "",
		S3API:                              ts.s3API,
		S3BucketName:                       ts.cfg.S3BucketName,
		S3MetricsRawOutputDirKubeAPIServer: path.Join(ts.cfg.Name, "metrics-kube-apiserver"),
		MetricsRawOutputDirKubeAPIServer:   filepath.Join(filepath.Dir(ts.cfg.ConfigPath), ts.cfg.Name+"-metrics-kube-apiserver"),
		Clients:                            ts.cfg.Clients,
		ClientQPS:                          ts.cfg.ClientQPS,
		ClientBurst:                        ts.cfg.ClientBurst,
		ClientTimeout:                      ts.cfg.ClientTimeout,
	}
	if ts.cfg.IsEnabledAddOnClusterVersionUpgrade() {
		kcfg.UpgradeServerVersion = ts.cfg.AddOnClusterVersionUpgrade.Version
	}
	if ts.cfg.Status != nil {
		kcfg.ClusterAPIServerEndpoint = ts.cfg.Status.ClusterAPIServerEndpoint
		kcfg.ClusterCADecoded = ts.cfg.Status.ClusterCADecoded
	}
	// in case cluster has already been created
	ts.k8sClient, err = k8s_client.NewEKS(kcfg)
	if err != nil {
		ts.lg.Warn("failed to create k8s client from previous states", zap.Error(err))
	} else {
		ts.lg.Info("created k8s client from previous states")
		// call here, because "createCluster" won't be called
		// if loaded from previous states
		// e.g. delete
		if err = ts.createTesters(); err != nil {
			return nil, err
		}
	}

	return ts, nil
}

func (ts *Tester) LogWriter() io.Writer {
	return ts.logWriter
}

func (ts *Tester) createTesters() (err error) {
	fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_green]createTesters [default](%q)\n"), ts.cfg.ConfigPath)

	ts.clusterTester = cluster.New(cluster.Config{
		Logger:    ts.lg,
		LogWriter: ts.logWriter,
		Stopc:     ts.stopCreationCh,
		EKSConfig: ts.cfg,
		S3API:     ts.s3API,
		IAMAPI:    ts.iamAPI,
		KMSAPI:    ts.kmsAPI,
		CFNAPI:    ts.cfnAPI,
		EC2API:    ts.ec2API,
		EKSAPI:    ts.eksAPI,
		ELBV2API:  ts.elbv2API,
	})

	ts.cniTester = cni_vpc.New(cni_vpc.Config{
		Logger:    ts.lg,
		LogWriter: ts.logWriter,
		Stopc:     ts.stopCreationCh,
		EKSConfig: ts.cfg,
		K8SClient: ts.k8sClient,
		ECRAPI:    ecr.New(ts.awsSession, aws.NewConfig().WithRegion(ts.cfg.GetAddOnCNIVPCRepositoryRegion())),
	})

	ts.ngTester = ng.New(ng.Config{
		Logger:    ts.lg,
		LogWriter: ts.logWriter,
		Stopc:     ts.stopCreationCh,
		EKSConfig: ts.cfg,
		K8SClient: ts.k8sClient,
		S3API:     ts.s3API,
		IAMAPI:    ts.iamAPI,
		CFNAPI:    ts.cfnAPI,
		EC2API:    ts.ec2API,
		ASGAPI:    ts.asgAPI,
		EKSAPI:    ts.eksAPI,
		SSMAPI:    ts.ssmAPI,
	})
	ts.mngTester = mng.New(mng.Config{
		Logger:    ts.lg,
		LogWriter: ts.logWriter,
		Stopc:     ts.stopCreationCh,
		EKSConfig: ts.cfg,
		K8SClient: ts.k8sClient,
		S3API:     ts.s3API,
		IAMAPI:    ts.iamAPI,
		CFNAPI:    ts.cfnAPI,
		EC2API:    ts.ec2API,
		ASGAPI:    ts.asgAPI,
		EKSAPI:    ts.eksAPI,
	})
	ts.gpuTester = gpu.New(gpu.Config{
		Logger:    ts.lg,
		LogWriter: ts.logWriter,
		Stopc:     ts.stopCreationCh,
		EKSConfig: ts.cfg,
		K8SClient: ts.k8sClient,
	})

	// Groups of installable addons. Addons are installed in groups, where each group installs all components in parallel
	ts.addons = [][]tester.Addon{{
		&clusterautoscaler.ClusterAutoscaler{
			Config:    ts.cfg,
			K8sClient: ts.k8sClient,
		},
		&overprovisioning.Overprovisioning{
			Config:    ts.cfg,
			K8sClient: ts.k8sClient,
		},
		&metrics_server.MetricsServer{
			Config:    ts.cfg,
			K8sClient: ts.k8sClient,
		},
		&clusterloader2.ClusterLoader{
			Config:    ts.cfg,
			K8sClient: ts.k8sClient,
		},
	}}

	ts.testers = []eks_tester.Tester{
		cw_agent.New(cw_agent.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
		}),
		fluentd.New(fluentd.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			ECRAPI:    ecr.New(ts.awsSession, aws.NewConfig().WithRegion(ts.cfg.GetAddOnFluentdRepositoryBusyboxRegion())),
		}),
		metrics_server.New(metrics_server.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
		}),
		conformance.New(conformance.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			S3API:     ts.s3API,
		}),
		app_mesh.New(app_mesh.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			S3API:     ts.s3API,
			CFNAPI:    ts.cfnAPI,
		}),
		csi_ebs.New(csi_ebs.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
		}),
		kubernetes_dashboard.New(kubernetes_dashboard.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
		}),
		prometheus_grafana.New(prometheus_grafana.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			ELB2API:   ts.elbv2API,
		}),
		php_apache.New(php_apache.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			ECRAPI:    ecr.New(ts.awsSession, aws.NewConfig().WithRegion(ts.cfg.GetAddOnPHPApacheRepositoryRegion())),
		}),
		nlb_hello_world.New(nlb_hello_world.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			ELB2API:   ts.elbv2API,
		}),
		nlb_guestbook.New(nlb_guestbook.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			ELB2API:   ts.elbv2API,
		}),
		alb_2048.New(alb_2048.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			CFNAPI:    ts.cfnAPI,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			ELB2API:   ts.elbv2API,
		}),
		jobs_pi.New(jobs_pi.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
		}),
		jobs_echo.New(jobs_echo.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			ECRAPI:    ecr.New(ts.awsSession, aws.NewConfig().WithRegion(ts.cfg.GetAddOnJobsEchoRepositoryBusyboxRegion())),
		}),
		cron_jobs.New(cron_jobs.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			ECRAPI:    ecr.New(ts.awsSession, aws.NewConfig().WithRegion(ts.cfg.GetAddOnCronJobsRepositoryBusyboxRegion())),
		}),
		csrs_local.New(csrs_local.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			S3API:     ts.s3API,
			CWAPI:     ts.cwAPI,
		}),
		csrs_remote.New(csrs_remote.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			S3API:     ts.s3API,
			CWAPI:     ts.cwAPI,
			ECRAPI:    ecr.New(ts.awsSession, aws.NewConfig().WithRegion(ts.cfg.GetAddOnCSRsRemoteRepositoryRegion())),
		}),
		config_maps_local.New(config_maps_local.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			S3API:     ts.s3API,
			CWAPI:     ts.cwAPI,
		}),
		config_maps_remote.New(config_maps_remote.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			S3API:     ts.s3API,
			CWAPI:     ts.cwAPI,
			ECRAPI:    ts.ecrAPISameRegion,
		}),
		secrets_local.New(secrets_local.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			S3API:     ts.s3API,
			CWAPI:     ts.cwAPI,
		}),
		secrets_remote.New(secrets_remote.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			S3API:     ts.s3API,
			CWAPI:     ts.cwAPI,
			ECRAPI:    ecr.New(ts.awsSession, aws.NewConfig().WithRegion(ts.cfg.GetAddOnSecretsRemoteRepositoryRegion())),
		}),
		fargate.New(fargate.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			S3API:     ts.s3API,
			IAMAPI:    ts.iamAPI,
			CFNAPI:    ts.cfnAPI,
			EKSAPI:    ts.eksAPI,
			ECRAPI:    ecr.New(ts.awsSession, aws.NewConfig().WithRegion(ts.cfg.GetAddOnFargateRepositoryRegion())),
		}),
		irsa.New(irsa.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			S3API:     ts.s3API,
			CFNAPI:    ts.cfnAPI,
			IAMAPI:    ts.iamAPI,
			ECRAPI:    ecr.New(ts.awsSession, aws.NewConfig().WithRegion(ts.cfg.GetAddOnIRSARepositoryRegion())),
		}),
		irsa_fargate.New(irsa_fargate.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			S3API:     ts.s3API,
			IAMAPI:    ts.iamAPI,
			CFNAPI:    ts.cfnAPI,
			EKSAPI:    ts.eksAPI,
			ECRAPI:    ecr.New(ts.awsSession, aws.NewConfig().WithRegion(ts.cfg.GetAddOnIRSAFargateRepositoryRegion())),
		}),
		wordpress.New(wordpress.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			ELB2API:   ts.elbv2API,
		}),
		jupyter_hub.New(jupyter_hub.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			ELB2API:   ts.elbv2API,
		}),
		kubeflow.New(kubeflow.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
		}),
		cuda_vector_add.New(cuda_vector_add.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
		}),
		cluster_loader_local.New(cluster_loader_local.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			S3API:     ts.s3API,
			CWAPI:     ts.cwAPI,
		}),
		cluster_loader_remote.New(cluster_loader_remote.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			S3API:     ts.s3API,
			CWAPI:     ts.cwAPI,
			ECRAPI:    ecr.New(ts.awsSession, aws.NewConfig().WithRegion(ts.cfg.GetAddOnClusterLoaderRemoteRepositoryRegion())),
		}),
		hollow_nodes_local.New(hollow_nodes_local.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
		}),
		hollow_nodes_remote.New(hollow_nodes_remote.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			ECRAPI:    ecr.New(ts.awsSession, aws.NewConfig().WithRegion(ts.cfg.GetAddOnHollowNodesRemoteRepositoryRegion())),
		}),
		stresser_local.New(stresser_local.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			S3API:     ts.s3API,
			CWAPI:     ts.cwAPI,
		}),
		stresser_remote.New(stresser_remote.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			S3API:     ts.s3API,
			CWAPI:     ts.cwAPI,
			ECRAPI:    ecr.New(ts.awsSession, aws.NewConfig().WithRegion(ts.cfg.GetAddOnStresserRemoteRepositoryRegion())),
		}),
		cluster_version_upgrade.New(cluster_version_upgrade.Config{
			Logger:    ts.lg,
			LogWriter: ts.logWriter,
			Stopc:     ts.stopCreationCh,
			EKSConfig: ts.cfg,
			K8SClient: ts.k8sClient,
			EKSAPI:    ts.eksAPI,
		}),
	}
	if serr := ts.cfg.Sync(); serr != nil {
		fmt.Fprintf(ts.logWriter, ts.color("[light_magenta]cfg.Sync failed [default]%v\n"), serr)
	}

	return nil
}

// Up should provision a new cluster for testing.
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Deployer
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func (ts *Tester) Up() (err error) {
	fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_green]UP START [default](%q, %q)\n"), ts.cfg.ConfigPath, user.Get())

	now := time.Now()

	defer func() {
		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_green]UP DEFER START [default](%q)\n"), ts.cfg.ConfigPath)
		fmt.Fprintf(ts.logWriter, "\n\n# to delete cluster\naws-k8s-tester eks delete cluster --path %s\n\n", ts.cfg.ConfigPath)
		ts.logFile.Sync()

		if serr := ts.uploadToS3(); serr != nil {
			ts.lg.Warn("failed to upload artifacts to S3", zap.Error(serr))
		} else {
			ts.s3Uploaded = true
		}

		if err == nil {
			if ts.cfg.Status.Up {
				if ts.cfg.TotalNodes < 10 {
					fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
					fmt.Fprintf(ts.logWriter, ts.color("[light_green]SSH [default](%q)\n"), ts.cfg.ConfigPath)
					fmt.Fprintln(ts.logWriter, ts.cfg.SSHCommands())
				}
				fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
				fmt.Fprintf(ts.logWriter, ts.color("[light_green]kubectl [default](%q)\n"), ts.cfg.ConfigPath)
				fmt.Fprintln(ts.logWriter, ts.cfg.KubectlCommands())

				ts.lg.Info("Up succeeded",
					zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
				)

				ts.lg.Sugar().Infof("Up.defer end (%s, %s)", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
				fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
				fmt.Fprintf(ts.logWriter, ts.color("\n\n💯 😁 👍 :) [light_green]UP SUCCESS\n\n\n"))

			} else {
				fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
				fmt.Fprintf(ts.logWriter, ts.color("\n\n😲 😲 😲  [light_magenta]UP ABORTED ???\n\n\n"))

			}
			fmt.Fprintf(ts.logWriter, "\n\n# to delete cluster\naws-k8s-tester eks delete cluster --path %s\n\n", ts.cfg.ConfigPath)
			ts.logFile.Sync()
			return
		}

		if !ts.cfg.OnFailureDelete {
			if ts.cfg.Status.Up {
				if ts.cfg.TotalNodes < 10 {
					fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
					fmt.Fprintf(ts.logWriter, ts.color("[light_green]SSH [default](%q)\n"), ts.cfg.ConfigPath)
					fmt.Fprintln(ts.logWriter, ts.cfg.SSHCommands())
				}
				fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
				fmt.Fprintf(ts.logWriter, ts.color("[light_green]kubectl [default](%q)\n"), ts.cfg.ConfigPath)
				fmt.Fprintln(ts.logWriter, ts.cfg.KubectlCommands())
			}

			ts.lg.Warn("Up failed",
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
				zap.Error(err),
			)
			fmt.Fprintf(ts.logWriter, ts.color("\n\n\n[light_magenta]UP FAIL ERROR:\n\n[default]%v\n\n\n"), err)
			fmt.Fprintf(ts.logWriter, "\n\n# to delete cluster\naws-k8s-tester eks delete cluster --path %s\n\n", ts.cfg.ConfigPath)

			ts.lg.Sugar().Infof("Up.defer end (%s, %s)", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
			fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
			fmt.Fprintf(ts.logWriter, ts.color("\n\n🔥 💀 👽 😱 😡 ⛈   (-_-) [light_magenta]UP FAIL\n\n\n"))
			fmt.Fprintf(ts.logWriter, "\n\n# to delete cluster\naws-k8s-tester eks delete cluster --path %s\n\n", ts.cfg.ConfigPath)
			ts.logFile.Sync()
			return
		}

		if ts.cfg.Status.Up {
			if ts.cfg.TotalNodes < 10 {
				fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
				fmt.Fprintf(ts.logWriter, ts.color("[light_green]SSH [default](%q)\n"), ts.cfg.ConfigPath)
				fmt.Fprintln(ts.logWriter, ts.cfg.SSHCommands())
			}
			fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
			fmt.Fprintf(ts.logWriter, ts.color("[light_green]kubectl [default](%q)\n"), ts.cfg.ConfigPath)
			fmt.Fprintln(ts.logWriter, ts.cfg.KubectlCommands())
		}
		fmt.Fprintf(ts.logWriter, ts.color("\n\n\n[light_magenta]UP FAIL ERROR:\n\n[default]%v\n\n\n"), err)
		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("🔥 💀 👽 😱 😡 ⛈   (-_-) [light_magenta]UP FAIL\n"))
		fmt.Fprintf(ts.logWriter, "\n\n# to delete cluster\naws-k8s-tester eks delete cluster --path %s\n\n", ts.cfg.ConfigPath)

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
			case <-ts.osSig:
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
		fmt.Fprintf(ts.logWriter, ts.color("\n\n\n[light_magenta]UP FAIL ERROR:\n\n[default]%v\n\n\n"), err)
		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("\n\n🔥 💀 👽 😱 😡 ⛈   (-_-) [light_magenta]UP FAIL\n\n\n"))
		fmt.Fprintf(ts.logWriter, "\n\n# to delete cluster\naws-k8s-tester eks delete cluster --path %s\n\n", ts.cfg.ConfigPath)

		ts.lg.Sugar().Infof("Up.defer end (%s, %s)", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		ts.logFile.Sync()
	}()

	ts.lg.Info("starting Up",
		zap.String("version", version.Version()),
		zap.String("user", user.Get()),
		zap.String("name", ts.cfg.Name),
	)
	defer ts.cfg.Sync()

	fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_green]createS3 [default](%q)\n"), ts.cfg.ConfigPath)
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.osSig,
		ts.createS3,
		"createS3",
	); err != nil {
		return err
	}

	fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_green]createKeyPair [default](%q)\n"), ts.cfg.ConfigPath)
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.osSig,
		ts.createKeyPair,
		"createKeyPair",
	); err != nil {
		return err
	}

	fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_green]createCluster [default](%q, %q)\n"), ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.osSig,
		ts.clusterTester.Create,
		ts.clusterTester.Name(),
	); err != nil {
		return err
	}
	ts.k8sClient = ts.clusterTester.Client()
	if err := ts.createTesters(); err != nil {
		return err
	}

	if ts.cfg.CommandAfterCreateCluster != "" {
		if err := ts.cfg.EvaluateCommandRefs(); err != nil {
			return err
		}

		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_green]runCommand.CommandAfterCreateCluster [default](%q)\n"), ts.cfg.CommandAfterCreateCluster)
		out, err := runCommand(ts.lg, ts.cfg.CommandAfterCreateCluster, ts.cfg.CommandAfterCreateClusterTimeout)
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
		fmt.Fprintf(ts.logWriter, "\nrunCommand output:\n\n%s\n", string(out))
	}
	if serr := ts.uploadToS3(); serr != nil {
		ts.lg.Warn("failed to upload artifacts to S3", zap.Error(serr))
	}

	if ts.cfg.IsEnabledAddOnCNIVPC() {
		if ts.cniTester == nil {
			return errors.New("ts.cniTester == nil when AddOnCNIVPC.Enable == true")
		}

		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_green]cniTester.Create [default](%q, %q)\n"), ts.cfg.ConfigPath, ts.cfg.KubectlCommand()+" --namespace kube-system")
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.osSig,
			ts.cniTester.Create,
			ts.cniTester.Name(),
		); err != nil {
			return err
		}
	}

	if ts.cfg.IsEnabledAddOnNodeGroups() {
		if ts.ngTester == nil {
			return errors.New("ts.ngTester == nil when AddOnNodeGroups.Enable == true")
		}

		// create NG first, so MNG configmap update can be called afterwards
		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_green]ngTester.Create [default](%q, %q)\n"), ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.osSig,
			ts.ngTester.Create,
			ts.ngTester.Name(),
		); err != nil {
			return err
		}
	}

	if ts.cfg.IsEnabledAddOnManagedNodeGroups() {
		if ts.mngTester == nil {
			return errors.New("ts.mngTester == nil when AddOnManagedNodeGroups.Enable == true")
		}

		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_green]mngTester.Create [default](%q, %q)\n"), ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.osSig,
			ts.mngTester.Create,
			ts.mngTester.Name(),
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
			case aws_eks.AMITypesAl2X8664Gpu:
				needGPU = true
				break gpuFound2
			}
		}
	}
	if needGPU {
		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_green]gpuTester.InstallNvidiaDriver [default](%q, %q)\n"), ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.osSig,
			ts.gpuTester.InstallNvidiaDriver,
			ts.gpuTester.Name(),
		); err != nil {
			ts.lg.Warn("failed to install nvidia driver", zap.Error(err))
			return err
		}

		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_green]gpuTester.CreateNvidiaSMI [default](%q, %q)\n"), ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.osSig,
			ts.gpuTester.CreateNvidiaSMI,
			ts.gpuTester.Name(),
		); err != nil {
			ts.lg.Warn("failed to create nvidia-smi", zap.Error(err))
			return err
		}
	}

	fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_green]%q.CheckHealth [default](%q, %q)\n"), ts.clusterTester.Name(), ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
	if ts.k8sClient == nil {
		// TODO: investigate why "ts.k8sClient == nil"
		ts.lg.Warn("[TODO] unexpected nil k8s client after cluster creation")
	}
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.osSig,
		ts.clusterTester.CheckHealth,
		ts.clusterTester.Name(),
	); err != nil {
		return err
	}
	if serr := ts.uploadToS3(); serr != nil {
		ts.lg.Warn("failed to upload artifacts to S3", zap.Error(serr))
	}

	for idx, cur := range ts.testers {
		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_green]testers[%02d].Create [cyan]%q [default](%q, %q)\n"), idx, cur.Name(), ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.osSig,
			cur.Create,
			cur.Name(),
		)

		if idx%10 == 0 {
			fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
			fmt.Fprintf(ts.logWriter, ts.color("[light_green]testers[%02d] [cyan]%q.CheckHealth [default](%q, %q)\n"), idx, ts.clusterTester.Name(), ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
			if ts.k8sClient == nil {
				// TODO: investigate why "ts.k8sClient == nil"
				ts.lg.Warn("[TODO] unexpected nil k8s client after cluster creation")
			}
			if err := catchInterrupt(
				ts.lg,
				ts.stopCreationCh,
				ts.stopCreationChOnce,
				ts.osSig,
				ts.clusterTester.CheckHealth,
				ts.clusterTester.Name(),
			); err != nil {
				return err
			}

			fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
			fmt.Fprintf(ts.logWriter, ts.color("[light_green]testers[%02d] uploadToS3 [cyan]%q [default](%q, %q)\n"), idx, cur.Name(), ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
			if serr := ts.uploadToS3(); serr != nil {
				ts.lg.Warn("failed to upload artifacts to S3", zap.Error(serr))
			}
		}

		if err != nil {
			return err
		}
	}

	if ts.cfg.IsEnabledAddOnNodeGroups() && ts.cfg.AddOnNodeGroups.Created && ts.cfg.AddOnNodeGroups.FetchLogs {
		if ts.ngTester == nil {
			return errors.New("ts.ngTester == nil when AddOnNodeGroups.Enable == true")
		}

		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_green]ngTester.FetchLogs [default](%q, %q)\n"), ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		waitDur := 15 * time.Second
		ts.lg.Info("sleeping before ngTester.FetchLogs", zap.Duration("wait", waitDur))
		time.Sleep(waitDur)

		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.osSig,
			ts.ngTester.FetchLogs,
			ts.ngTester.Name(),
		); err != nil {
			return err
		}
	}

	if ts.cfg.IsEnabledAddOnManagedNodeGroups() && ts.cfg.AddOnManagedNodeGroups.Created && ts.cfg.AddOnManagedNodeGroups.FetchLogs {
		if ts.mngTester == nil {
			return errors.New("ts.mngTester == nil when AddOnManagedNodeGroups.Enable == true")
		}

		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_green]mngTester.FetchLogs [default](%q, %q)\n"), ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		waitDur := 15 * time.Second
		ts.lg.Info("sleeping before mngTester.FetchLogs", zap.Duration("wait", waitDur))
		time.Sleep(waitDur)
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.osSig,
			ts.mngTester.FetchLogs,
			ts.mngTester.Name(),
		); err != nil {
			return err
		}
	}

	fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_green]%q.CheckHealth [default](%q, %q)\n"), ts.clusterTester.Name(), ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
	if ts.k8sClient == nil {
		// TODO: investigate why "ts.k8sClient == nil"
		ts.lg.Warn("[TODO] unexpected nil k8s client after cluster creation")
	}
	if err := catchInterrupt(
		ts.lg,
		ts.stopCreationCh,
		ts.stopCreationChOnce,
		ts.osSig,
		ts.clusterTester.CheckHealth,
		ts.clusterTester.Name(),
	); err != nil {
		return err
	}

	if ts.cfg.CommandAfterCreateAddOns != "" {
		if err := ts.cfg.EvaluateCommandRefs(); err != nil {
			return err
		}

		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_green]runCommand.CommandAfterCreateAddOns [default](%q)\n"), ts.cfg.CommandAfterCreateAddOns)
		out, err := runCommand(ts.lg, ts.cfg.CommandAfterCreateAddOns, ts.cfg.CommandAfterCreateAddOnsTimeout)
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
		fmt.Fprintf(ts.logWriter, "\nrunCommand output:\n\n%s\n", string(out))
	}

	logFetchAgain := false
	if ts.cfg.IsEnabledAddOnManagedNodeGroups() && ts.cfg.AddOnManagedNodeGroups.Created {
		if ts.mngTester == nil {
			return errors.New("ts.mngTester == nil when AddOnManagedNodeGroups.Enable == true")
		}
	scaleFound:
		for _, cur := range ts.cfg.AddOnManagedNodeGroups.MNGs {
			for _, up := range cur.ScaleUpdates {
				if up.Enable {
					logFetchAgain = true
					break scaleFound
				}
			}
		}
		if !logFetchAgain {
			for _, cur := range ts.cfg.AddOnManagedNodeGroups.MNGs {
				if cur.VersionUpgrade != nil && cur.VersionUpgrade.Enable {
					logFetchAgain = true
					break
				}
			}
		}

		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_green]mngTester.Scale [default](%q, logFetchAgain %v)\n"), ts.cfg.ConfigPath, logFetchAgain)
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.osSig,
			ts.mngTester.Scale,
			ts.mngTester.Name(),
		); err != nil {
			return err
		}

		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_green]mngTester.UpgradeVersion [default](%q, logFetchAgain %v)\n"), ts.cfg.ConfigPath, logFetchAgain)
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.osSig,
			ts.mngTester.UpgradeVersion,
			ts.mngTester.Name(),
		); err != nil {
			return err
		}
	}

	if logFetchAgain && ts.cfg.IsEnabledAddOnManagedNodeGroups() && ts.cfg.AddOnManagedNodeGroups.Created && ts.cfg.AddOnManagedNodeGroups.FetchLogs {
		if ts.mngTester == nil {
			return errors.New("ts.mngTester == nil when AddOnManagedNodeGroups.Enable == true")
		}

		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_green]mngTester.FetchLogs after upgrade [default](%q, %q)\n"), ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		waitDur := 15 * time.Second
		ts.lg.Info("sleeping before mngTester.FetchLogs", zap.Duration("wait", waitDur))
		time.Sleep(waitDur)
		if err := catchInterrupt(
			ts.lg,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.osSig,
			ts.mngTester.FetchLogs,
			ts.mngTester.Name(),
		); err != nil {
			return err
		}
	}

	if ts.cfg.CommandAfterCreateAddOns != "" {
		if err := ts.cfg.EvaluateCommandRefs(); err != nil {
			return err
		}

		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_green]runCommand.CommandAfterCreateAddOns [default](%q)\n"), ts.cfg.CommandAfterCreateAddOns)
		out, err := runCommand(ts.lg, ts.cfg.CommandAfterCreateAddOns, ts.cfg.CommandAfterCreateAddOnsTimeout)
		if err != nil {
			err = ioutil.WriteFile(ts.cfg.CommandAfterCreateAddOnsOutputPath, []byte(ts.cfg.CommandAfterCreateAddOns+"\n\n# output\n"+string(out)+"\n\n# error\n"+err.Error()), 0600)
			if err != nil {
				ts.lg.Warn("failed to write CommandAfterCreateAddOnsOutputPath", zap.Error(err))
			}
		} else {
			err = ioutil.WriteFile(ts.cfg.CommandAfterCreateAddOnsOutputPath, []byte(ts.cfg.CommandAfterCreateAddOns+"\n\n# output\n"+string(out)), 0600)
			if err != nil {
				ts.lg.Warn("failed to write CommandAfterCreateAddOnsOutputPath", zap.Error(err))
			}
		}
		fmt.Fprintf(ts.logWriter, "\nrunCommand output:\n\n%s\n", string(out))
	}

	// Generic installation of ordered addons. Add your addon to ts.addons
	for _, order := range ts.addons {
		if err := ts.runAsync(order, func(a tester.Addon) error {
			zap.S().Infof("Applying addon %s", reflect.TypeOf(a))
			return a.Apply()
		}); err != nil {
			return fmt.Errorf("while applying addons, %w", err)
		}
		ts.cfg.Sync()
	}
	if serr := ts.cfg.Sync(); serr != nil {
		fmt.Fprintf(ts.logWriter, ts.color("[light_magenta]cfg.Sync failed [default]%v\n"), serr)
	}

	return nil
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
	fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_blue]DOWN START [default](%q, %q)\n"), ts.cfg.ConfigPath, ts.cfg.KubectlCommand())

	now := time.Now()
	ts.lg.Warn("starting Down",
		zap.String("user", user.Get()),
		zap.String("name", ts.cfg.Name),
		zap.String("cluster-arn", ts.cfg.Status.ClusterARN),
	)
	if !ts.s3Uploaded {
		if serr := ts.uploadToS3(); serr != nil {
			ts.lg.Warn("failed to upload artifacts to S3", zap.Error(serr))
		}
	}

	defer func() {
		ts.logFile.Sync()
		ts.cfg.Sync()

		if err == nil {
			fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
			fmt.Fprintf(ts.logWriter, ts.color("[light_blue]DOWN DEFER START [default](%q)\n"), ts.cfg.ConfigPath)
			fmt.Fprintf(ts.logWriter, ts.color("\n\n💯 😁 👍 :) [light_blue]DOWN SUCCESS\n\n\n"))

			ts.lg.Info("successfully finished Down",
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)

		} else {
			fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
			fmt.Fprintf(ts.logWriter, ts.color("[light_blue]DOWN DEFER START [default](%q)\n"), ts.cfg.ConfigPath)
			fmt.Fprintf(ts.logWriter, ts.color("🔥 💀 👽 😱 😡 ⛈   (-_-) [light_magenta]DOWN FAIL\n"))

			ts.lg.Info("failed Down",
				zap.Error(err),
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)
		}
	}()

	var errs []string

	if ts.cfg.SkipDeleteClusterAndNodes {
		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_yellow]SKIP [light_blue]deleteKeyPair [default](SkipDeleteClusterAndNodes 'true', %q)\n"), ts.cfg.ConfigPath)
	} else {
		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_blue]deleteKeyPair [default](%q)\n"), ts.cfg.ConfigPath)
		if err := ts.deleteKeyPair(); err != nil {
			ts.lg.Warn("failed to delete key pair", zap.Error(err))
			errs = append(errs, err.Error())
		}
	}

	testersN := len(ts.testers)
	for idx := range ts.testers {
		idx = testersN - idx - 1
		cur := ts.testers[idx]
		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_blue]testers[%02d].Delete [cyan]%q [default](%q, %q)\n"), idx, cur.Name(), ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		if err := cur.Delete(); err != nil {
			ts.lg.Warn("failed tester.Delete", zap.Error(err))
			errs = append(errs, err.Error())
		}
	}

	if ts.cfg.SkipDeleteClusterAndNodes {
		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_yellow]SKIP [light_blue]cluster/nodes.Delete [default](SkipDeleteClusterAndNodes 'true', %q)\n"), ts.cfg.ConfigPath)
	} else {
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
		if ts.cfg.IsEnabledAddOnManagedNodeGroups() && ts.mngTester != nil {
			fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
			fmt.Fprintf(ts.logWriter, ts.color("[light_blue]mngTester.Delete [default](%q)\n"), ts.cfg.ConfigPath)
			if err := ts.mngTester.Delete(); err != nil {
				ts.lg.Warn("failed mngTester.Delete", zap.Error(err))
				errs = append(errs, err.Error())
			}

			waitDur := 10 * time.Second
			ts.lg.Info("sleeping before cluster deletion", zap.Duration("wait", waitDur))
			time.Sleep(waitDur)
		}

		if ts.cfg.IsEnabledAddOnNodeGroups() && ts.ngTester != nil {
			fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
			fmt.Fprintf(ts.logWriter, ts.color("[light_blue]ngTester.Delete [default](%q)\n"), ts.cfg.ConfigPath)
			if err := ts.ngTester.Delete(); err != nil {
				ts.lg.Warn("failed ngTester.Delete", zap.Error(err))
				errs = append(errs, err.Error())
			}

			waitDur := 10 * time.Second
			ts.lg.Info("sleeping before cluster deletion", zap.Duration("wait", waitDur))
			time.Sleep(waitDur)
		}

		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_blue]clusterTester.Delete [default](%q)\n"), ts.cfg.ConfigPath)
		if err := ts.clusterTester.Delete(); err != nil {
			ts.lg.Warn("failed clusterTester.Delete", zap.Error(err))
			errs = append(errs, err.Error())
		}

		fmt.Fprintf(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_blue]deleteS3 [default](%q)\n"), ts.cfg.ConfigPath)
		if err := ts.deleteS3(); err != nil {
			ts.lg.Warn("failed deleteS3", zap.Error(err))
			errs = append(errs, err.Error())
		}
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
	return true, ts.clusterTester.CheckHealth()
}

// DumpClusterLogs should export logs from the cluster. It may be called
// multiple times. Options for this should come from New(...)
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Deployer
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func (ts *Tester) DumpClusterLogs() error {
	if ts.cfg.IsEnabledAddOnNodeGroups() {
		if err := ts.ngTester.FetchLogs(); err != nil {
			return err
		}
	}
	if ts.cfg.IsEnabledAddOnManagedNodeGroups() {
		return ts.mngTester.FetchLogs()
	}
	return nil
}

// DownloadClusterLogs dumps all logs to artifact directory.
// Let default kubetest log dumper handle all artifact uploads.
// See https://github.com/kubernetes/test-infra/pull/9811/files#r225776067.
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Deployer
// ref. https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options
func (ts *Tester) DownloadClusterLogs(artifactDir, _ string) error {
	if ts.cfg.IsEnabledAddOnNodeGroups() {
		if err := ts.mngTester.DownloadClusterLogs(artifactDir); err != nil {
			return err
		}
	}
	if ts.cfg.IsEnabledAddOnManagedNodeGroups() {
		return ts.ngTester.DownloadClusterLogs(artifactDir)
	}
	return nil
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
	return ts.k8sClient.KubernetesClientSet()
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

func catchInterrupt(lg *zap.Logger, stopc chan struct{}, stopcCloseOnce *sync.Once, osSigCh chan os.Signal, run func() error, name string) (err error) {
	errc := make(chan error)
	go func() {
		errc <- run()
	}()

	select {
	case _, ok := <-stopc:
		rerr := <-errc
		lg.Info("interrupted; stopc received, errc received", zap.Error(rerr))
		err = fmt.Errorf("stopc returned, stopc open %v, run function returned %v (%q)", ok, rerr, name)

	case osSig := <-osSigCh:
		stopcCloseOnce.Do(func() { close(stopc) })
		rerr := <-errc
		lg.Info("OS signal received, errc received", zap.String("signal", osSig.String()), zap.Error(rerr))
		err = fmt.Errorf("received os signal %v, closed stopc, run function returned %v (%q)", osSig, rerr, name)

	case err = <-errc:
		if err != nil {
			err = fmt.Errorf("run function returned %v (%q)", err, name)
		}
	}
	return err
}

// runAsync asynchronously executes a function over a slice of addons.
// If any function errors, the function will return with the error after all addons have executed
func (ts *Tester) runAsync(addons []tester.Addon, execute func(a tester.Addon) error) error {
	errors := make(chan error)
	done := make(chan bool)
	var wg sync.WaitGroup

	// Fire off addon functions
	for _, addon := range addons {
		if !addon.IsEnabled() {
			klog.Infof("Skipping disabled addon %s", reflect.TypeOf(addon))
			continue
		}
		wg.Add(1)
		// Take a copy for the goroutine since addon will be mutated before it executes
		a := addon
		go func() {
			defer wg.Done()
			if err := execute(a); err != nil {
				errors <- err
			}
		}()
	}

	// Wait for all routines to exit and signal done
	go func() {
		wg.Wait()
		close(done)
	}()

	// Wait for done or an error to occur
	select {
	case <-done:
		break
	case err := <-errors:
		close(errors)
		return err
	}
	return nil
}
