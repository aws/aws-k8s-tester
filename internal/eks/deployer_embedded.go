package eks

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/awstester/eksconfig"
	"github.com/aws/awstester/eksdeployer"
	"github.com/aws/awstester/internal/eks/alb"
	"github.com/aws/awstester/internal/eks/alb/ingress/client"
	"github.com/aws/awstester/internal/eks/alb/ingress/path"
	"github.com/aws/awstester/internal/eks/s3"
	"github.com/aws/awstester/pkg/awsapi"
	"github.com/aws/awstester/pkg/fileutil"
	"github.com/aws/awstester/pkg/httputil"
	"github.com/aws/awstester/pkg/wrk"
	"github.com/aws/awstester/pkg/zaputil"

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
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	awss3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

type embedded struct {
	stopc chan struct{}

	mu  sync.RWMutex
	lg  *zap.Logger
	cfg *eksconfig.Config

	// TODO: move this "kubectl" to AWS CLI deployer
	// and instead use "k8s.io/client-go" with STS token
	kubectl     exec.Interface
	kubectlPath string

	ss  *session.Session
	im  iamiface.IAMAPI
	sts stsiface.STSAPI
	cf  cloudformationiface.CloudFormationAPI
	asg autoscalingiface.AutoScalingAPI
	eks eksiface.EKSAPI
	ec2 ec2iface.EC2API

	ec2InstancesMu *sync.RWMutex
	ec2Instances   []*ec2.Instance

	s3Plugin s3.Plugin

	// for plugins, sub-project implementation
	albPlugin alb.Plugin

	// TODO: add EBS (with CSI) plugin
	// TODO: add KMS plugin
}

// NewEKSDeployer creates a new EKS deployer.
func NewEKSDeployer(cfg *eksconfig.Config) (eksdeployer.Interface, error) {
	cfg.Embedded = true
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	lg, err := zaputil.New(cfg.LogDebug, cfg.LogOutputs)
	if err != nil {
		return nil, err
	}

	md := &embedded{
		stopc:          make(chan struct{}),
		lg:             lg,
		cfg:            cfg,
		kubectl:        exec.New(),
		ec2InstancesMu: &sync.RWMutex{},
	}
	md.kubectlPath, err = md.kubectl.LookPath("kubectl")
	if err != nil {
		return nil, fmt.Errorf("cannot find 'kubectl' executable (%v)", err)
	}
	if _, err = exec.New().LookPath("aws-iam-authenticator"); err != nil {
		return nil, fmt.Errorf("cannot find 'aws-iam-authenticator' executable (%v)", err)
	}

	awsCfg := &awsapi.Config{
		Logger:         md.lg,
		DebugAPICalls:  cfg.LogDebug,
		Region:         cfg.AWSRegion,
		CustomEndpoint: cfg.AWSCustomEndpoint,
	}
	md.ss, err = awsapi.New(awsCfg)
	if err != nil {
		return nil, err
	}

	md.im = iam.New(md.ss)
	md.sts = sts.New(md.ss)
	md.cf = cloudformation.New(md.ss)
	md.asg = autoscaling.New(md.ss)
	md.eks = awseks.New(md.ss)
	md.ec2 = ec2.New(md.ss)
	md.s3Plugin = s3.NewEmbedded(md.lg, md.cfg, awss3.New(md.ss))

	output, oerr := md.sts.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if oerr != nil {
		return nil, oerr
	}
	md.cfg.AWSAccountID = *output.Account

	if cfg.ALBIngressController.Enable {
		md.albPlugin, err = alb.NewEmbedded(md.stopc, lg, md.cfg, md.im, md.ec2, elbv2.New(md.ss), md.s3Plugin)
		if err != nil {
			return nil, err
		}

		// TODO
		// build binary

		// TODO
		// construct container image name
		// TODO: use git sha to tag image?
		// name := fmt.Sprintf("%s/alb:%s", md.cont.GetRegistry(), md.cfg.ClusterName)

		// TODO
		// push container image to repository

		// TODO
		// once complete, remove all Go build and container build commands
		// and replace them with container image name
		// in order to indicate to next run that image has already been built
		// and to just reuse the image rather than building and pushing again

		// for now, assume previous commands or Prow jobs already built
		// corresponding binary and container image to current branch
	}

	// to connect to an existing cluster
	op, err := md.im.GetRole(&iam.GetRoleInput{
		RoleName: aws.String(md.cfg.ClusterState.ServiceRoleWithPolicyName),
	})
	if err == nil {
		md.cfg.ClusterState.ServiceRoleWithPolicyARN = *op.Role.Arn
		lg.Info("found existing service role ARN", zap.String("arn", md.cfg.ClusterState.ServiceRoleWithPolicyARN))
	}

	// to connect to an existing cluster
	do, err := md.cf.DescribeStacks(&cloudformation.DescribeStacksInput{
		StackName: aws.String(md.cfg.ClusterState.CFStackVPCName),
	})
	if err == nil && len(do.Stacks) == 1 {
		for _, op := range do.Stacks[0].Outputs {
			if *op.OutputKey == "VpcId" {
				md.cfg.ClusterState.CFStackVPCID = *op.OutputValue
				lg.Info("found existing VPC stack VPC ID", zap.String("id", md.cfg.ClusterState.CFStackVPCID))
				continue
			}
			if *op.OutputKey == "SubnetIds" {
				vv := *op.OutputValue
				md.cfg.ClusterState.CFStackVPCSubnetIDs = strings.Split(vv, ",")
				lg.Info("found existing VPC stack Subnet IDs", zap.Strings("ids", md.cfg.ClusterState.CFStackVPCSubnetIDs))
				continue
			}
			if *op.OutputKey == "SecurityGroups" {
				md.cfg.ClusterState.CFStackVPCSecurityGroupID = *op.OutputValue
				lg.Info("found existing VPC stack security group", zap.String("id", md.cfg.ClusterState.CFStackVPCSecurityGroupID))
			}
		}
	}

	// to connect to an existing cluster
	co, err := md.eks.DescribeCluster(&awseks.DescribeClusterInput{
		Name: aws.String(md.cfg.ClusterName),
	})
	if err == nil {
		md.cfg.ClusterState.Status = *co.Cluster.Status
		md.cfg.ClusterState.Created = *co.Cluster.CreatedAt
		md.cfg.ClusterState.PlatformVersion = *co.Cluster.PlatformVersion
		if md.cfg.ClusterState.Status == "ACTIVE" {
			// cluster is already created
			// if up command failed, separate down command would need
			// fetch cluster information with cluster name
			md.cfg.ClusterState.Endpoint = *co.Cluster.Endpoint
			md.cfg.ClusterState.CA = *co.Cluster.CertificateAuthority.Data
			if err = writeKubeConfig(
				md.cfg.ClusterState.Endpoint,
				md.cfg.ClusterState.CA,
				md.cfg.ClusterName,
				md.cfg.KubeConfigPath,
			); err != nil {
				return nil, err
			}
			md.lg.Info(
				"overwrote KUBECONFIG from an existing cluster",
				zap.String("KUBECONFIG", md.cfg.KubeConfigPath),
				zap.String("cluster-name", md.cfg.ClusterName),
			)
		}
	}

	lg.Info(
		"created EKS deployer",
		zap.String("cluster-name", cfg.ClusterName),
		zap.String("awstester-eks-config-path", cfg.ConfigPath),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return md, md.cfg.Sync()
}

// Up creates an EKS cluster for 'kubetest'.
// If it fails at any point of operation, it rolls back everything.
// And expect to create a cluster from scratch with a new name.
//
// TODO: if custom endpoint is specified,
// either create a new cluster from scratch or
// use the existing cluster
func (md *embedded) Up() (err error) {
	md.mu.Lock()
	defer md.mu.Unlock()

	if md.cfg.ClusterState.Status == "ACTIVE" {
		return fmt.Errorf("%q is already %q", md.cfg.ClusterName, md.cfg.ClusterState.Status)
	}
	if md.cfg.LogAccess {
		if err = md.s3Plugin.CreateBucketForAccessLogs(); err != nil {
			return err
		}
	}

	defer func() {
		if err != nil {
			md.lg.Warn("failed Up", zap.Error(err))
			if md.cfg.ALBIngressController.Enable && md.cfg.ALBIngressController.Created {
				if err = md.albPlugin.DeleteIngressObjects(); err != nil {
					md.lg.Warn("failed to delete ALB Ingress Controller ELBv2", zap.Error(err))
				}
				if derr := md.albPlugin.DeleteSecurityGroup(); derr != nil {
					md.lg.Warn("failed to delete ALB Ingress Controller security group", zap.Error(derr))
				}
			}
			if derr := md.deleteWorkerNode(); derr != nil {
				md.lg.Warn("failed to delete node group stack", zap.Error(derr))
			}
			if derr := md.deleteKeyPair(); derr != nil {
				md.lg.Warn("failed to delete key pair", zap.Error(derr))
			}
			if derr := md.deleteCluster(false); derr != nil {
				md.lg.Warn("failed to delete cluster", zap.Error(derr))
			}
			if derr := md.deleteVPC(); derr != nil {
				md.lg.Warn("failed to delete VPC stack", zap.Error(derr))
			}
			if derr := md.detachPolicyForAWSServiceRoleForAmazonEKS(); derr != nil {
				md.lg.Warn("failed to delete service role policy", zap.Error(derr))
			}
			if derr := md.deleteAWSServiceRoleForAmazonEKS(); derr != nil {
				md.lg.Warn("failed to delete service role", zap.Error(derr))
			}
			md.lg.Warn("reverted Up", zap.Error(err))
		}
	}()

	now := time.Now().UTC()

	md.lg.Info("Up",
		zap.String("cluster-name", md.cfg.ClusterName),
		zap.String("custom-endpoint", md.cfg.AWSCustomEndpoint),
		zap.String("KUBECONFIG", md.cfg.KubeConfigPath),
	)
	defer md.cfg.Sync()

	if err = catchStopc(md.lg, md.stopc, md.createAWSServiceRoleForAmazonEKS); err != nil {
		return err
	}
	if err = catchStopc(md.lg, md.stopc, md.attachPolicyForAWSServiceRoleForAmazonEKS); err != nil {
		return err
	}
	if err = catchStopc(md.lg, md.stopc, md.createVPC); err != nil {
		return err
	}
	if err = catchStopc(md.lg, md.stopc, md.createCluster); err != nil {
		return err
	}
	if err = catchStopc(md.lg, md.stopc, md.upgradeCNI); err != nil {
		return err
	}
	if err = catchStopc(md.lg, md.stopc, md.createKeyPair); err != nil {
		return err
	}
	if err = catchStopc(md.lg, md.stopc, md.createWorkerNode); err != nil {
		return err
	}

	if md.cfg.AWSCredentialToMountPath != "" {
		if err = md.mountAWSCredential(); err != nil {
			return err
		}
	}

	md.cfg.Sync()
	md.cfg.SetClusterUpTook(time.Now().UTC().Sub(now))

	if md.cfg.ALBIngressController.Enable {
		albStart := time.Now().UTC()

		if err = catchStopc(md.lg, md.stopc, md.albPlugin.DeployBackend); err != nil {
			return err
		}
		if err = catchStopc(md.lg, md.stopc, md.albPlugin.CreateRBAC); err != nil {
			return err
		}
		if err = catchStopc(md.lg, md.stopc, md.albPlugin.DeployIngressController); err != nil {
			return err
		}
		if err = catchStopc(md.lg, md.stopc, md.albPlugin.CreateSecurityGroup); err != nil {
			return err
		}
		if err = catchStopc(md.lg, md.stopc, md.albPlugin.CreateIngressObjects); err != nil {
			return err
		}
		md.cfg.ALBIngressController.Created = true

		md.cfg.Sync()
		md.cfg.SetIngressUpTook(time.Now().UTC().Sub(albStart))
	}

	md.lg.Info("Up finished",
		zap.String("cluster-name", md.cfg.ClusterName),
		zap.String("custom-endpoint", md.cfg.AWSCustomEndpoint),
		zap.String("KUBECONFIG", md.cfg.KubeConfigPath),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)

	if err = md.cfg.Sync(); err != nil {
		return err
	}

	if md.cfg.LogAutoUpload {
		if err = md.upload(); err != nil {
			md.lg.Warn("failed to upload", zap.Error(err))
		}
		if err = md.uploadWorkerNode(); err != nil {
			md.lg.Warn("failed to upload worker node logs", zap.Error(err))
		}
		if err = md.uploadALB(); err != nil {
			md.lg.Warn("failed to upload ALB", zap.Error(err))
		}
	}
	return nil
}

func catchStopc(lg *zap.Logger, stopc chan struct{}, run func() error) (err error) {
	errc := make(chan error)
	go func() {
		errc <- run()
	}()
	select {
	case <-stopc:
		lg.Info("interrupting")
		gerr := <-errc
		lg.Info("interrupted", zap.Error(gerr))
		err = fmt.Errorf("interrupted (run function returned %v)", gerr)
	case err = <-errc:
	}
	return err
}

// Down terminates and deletes the EKS cluster.
func (md *embedded) Down() (err error) {
	md.mu.Lock()
	defer md.mu.Unlock()

	if md.cfg.ClusterState.Status == "DELETING" ||
		md.cfg.ClusterState.Status == "FAILED" {
		return fmt.Errorf("cluster %q status is already %q",
			md.cfg.ClusterName,
			md.cfg.ClusterState.Status,
		)
	}

	now := time.Now().UTC()

	var errs []string
	md.lg.Info("Down", zap.String("cluster-name", md.cfg.ClusterName))
	if err = md.uploadWorkerNode(); err != nil {
		md.lg.Warn("failed to upload worker node logs", zap.Error(err))
	}
	if md.cfg.ALBIngressController.Enable && md.cfg.ALBIngressController.Created {
		if err = md.albPlugin.DeleteIngressObjects(); err != nil {
			md.lg.Warn("failed to delete ALB Ingress Controller ELBv2", zap.Error(err))
			errs = append(errs, err.Error())
		}
		// fail without deleting worker node group
		// since worker node EC2 instance has dependency on this security group
		// e.g. DependencyViolation: resource sg-01a2f9aef81a857f6 has a dependent object
		// so try again after deleting node group stack
		if err = md.albPlugin.DeleteSecurityGroup(); err != nil {
			md.lg.Warn("tried to delete ALB Ingress Controller security group", zap.Error(err))
		}
	}
	if err = md.deleteWorkerNode(); err != nil {
		md.lg.Warn("failed to delete node group stack", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if md.cfg.ALBIngressController.Enable && md.cfg.ALBIngressController.Created {
		if err = md.albPlugin.DeleteSecurityGroup(); err != nil {
			md.lg.Warn("failed to delete ALB Ingress Controller security group", zap.Error(err))
			errs = append(errs, err.Error())
		}
		md.cfg.ALBIngressController.Created = false
	}
	if err = md.deleteKeyPair(); err != nil {
		md.lg.Warn("failed to delete key pair", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if err = md.deleteCluster(true); err != nil {
		md.lg.Warn("failed to delete cluster", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if err = md.deleteVPC(); err != nil {
		md.lg.Warn("failed to delete VPC stack", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if err = md.detachPolicyForAWSServiceRoleForAmazonEKS(); err != nil {
		md.lg.Warn("failed to delete service role policy", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if err = md.deleteAWSServiceRoleForAmazonEKS(); err != nil {
		md.lg.Warn("failed to delete service role", zap.Error(err))
		errs = append(errs, err.Error())
	}

	md.lg.Info("Down finished",
		zap.String("cluster-name", md.cfg.ClusterName),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)

	if err = md.cfg.Sync(); err != nil {
		return err
	}
	if md.cfg.LogAutoUpload {
		if err = md.upload(); err != nil {
			md.lg.Warn("failed to upload", zap.Error(err))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}
	return nil
}

// IsUp returns an error if the cluster is not up and running.
func (md *embedded) IsUp() (err error) {
	if md.cfg.ClusterName == "" {
		return errors.New("cannot check empty cluster")
	}
	var do *awseks.DescribeClusterOutput
	do, err = md.eks.DescribeCluster(&awseks.DescribeClusterInput{
		Name: aws.String(md.cfg.ClusterName),
	})
	if err != nil {
		return err
	}
	md.cfg.ClusterState.Status = *do.Cluster.Status
	md.cfg.ClusterState.Created = *do.Cluster.CreatedAt
	md.cfg.ClusterState.PlatformVersion = *do.Cluster.PlatformVersion
	if md.cfg.ClusterState.Status == "ACTIVE" {
		return md.cfg.Sync()
	}
	return fmt.Errorf("cluster %q status is %q",
		md.cfg.ClusterName,
		md.cfg.ClusterState.Status,
	)
}

// TestSetup checks if EKS testing cluster has been set up or not.
func (md *embedded) TestSetup() (err error) {
	return md.IsUp()
}

// GetClusterCreated returns EKS cluster creation time and error (if any).
func (md *embedded) GetClusterCreated(v string) (time.Time, error) {
	err := md.IsUp()
	if err != nil {
		return time.Time{}, err
	}
	return md.cfg.ClusterState.Created, nil
}

// DumpClusterLogs dumps all logs to artifact directory.
// Let default kubetest log dumper handle all artifact uploads.
// See https://github.com/kubernetes/test-infra/pull/9811/files#r225776067.
func (md *embedded) DumpClusterLogs(artifactDir, _ string) (err error) {
	err = md.downloadWorkerNodeLogs()
	if err != nil {
		return err
	}
	for fpath, p := range md.cfg.GetWorkerNodeLogs() {
		if err = fileutil.Copy(fpath, filepath.Join(artifactDir, p)); err != nil {
			return err
		}
	}
	if err = fileutil.Copy(md.cfg.ConfigPath, filepath.Join(artifactDir, md.cfg.ConfigPathBucket)); err != nil {
		return err
	}
	return fileutil.Copy(md.cfg.LogOutputToUploadPath, filepath.Join(artifactDir, md.cfg.LogOutputToUploadPathBucket))
}

func (md *embedded) UploadToBucketForTests(localPath, remotePath string) error {
	return md.s3Plugin.UploadToBucketForTests(localPath, remotePath)
}

///////////////////////////////////////////////
// Extra methods for EKS specific operations //
///////////////////////////////////////////////

func (md *embedded) Stop() { close(md.stopc) }

// LoadConfig returns the current configuration and its states.
func (md *embedded) LoadConfig() (eksconfig.Config, error) {
	return *md.cfg, nil
}

func (md *embedded) TestALBCorrectness() error {
	ep := "http://" + md.cfg.ALBIngressController.ELBv2NamespaceToDNSName["default"]
	if md.cfg.ALBIngressController.TestMode == "ingress-test-server" {
		ep += path.Path
	}
	if !httputil.CheckGet(
		md.lg,
		ep,
		strings.Repeat("0", md.cfg.ALBIngressController.TestResponseSize),
		30,
		5*time.Second,
		md.stopc) {
		return fmt.Errorf("failed to HTTP Get %q", ep)
	}
	return md.albPlugin.TestAWSResources()
}

func (md *embedded) TestALBQPS() error {
	ep := "http://" + md.cfg.ALBIngressController.ELBv2NamespaceToDNSName["default"]

	var rs client.TestResult
	var rbytes []byte
	switch md.cfg.ALBIngressController.TestMode {
	case "ingress-test-server":
		cli, err := client.New(
			md.lg,
			ep,
			md.cfg.ALBIngressController.TestServerRoutes,
			md.cfg.ALBIngressController.TestClients,
			md.cfg.ALBIngressController.TestClientRequests,
		)
		if err != nil {
			return err
		}
		rs = cli.Run()
		rbytes = []byte(rs.Result)

	case "nginx":
		// wrk --threads 2 --connections 200 --duration 15s --latency http://127.0.0.1
		args := []string{
			"--threads", "2",
			"--connections", fmt.Sprintf("%d", md.cfg.ALBIngressController.TestClients),
			"--duration", "15s",
			"--latency",
			ep,
		}
		md.lg.Info("starting wrk", zap.String("command", strings.Join(args, " ")))
		cmd := exec.New().CommandContext(context.Background(), "wrk", args...)
		var err error
		rbytes, err = cmd.CombinedOutput()
		if err != nil {
			return err
		}
		md.lg.Info("finished wrk", zap.String("command", strings.Join(args, " ")))
	}

	fmt.Printf("TestALBQPS Result: %q\n\n%s\n\n", ep, string(rbytes))

	if err := ioutil.WriteFile(
		md.cfg.ALBIngressController.ScalabilityOutputToUploadPath,
		rbytes,
		0600,
	); err != nil {
		return err
	}

	if md.cfg.LogAutoUpload {
		if err := md.uploadALB(); err != nil {
			md.lg.Warn("failed to upload ALB", zap.Error(err))
		}
	}

	if md.cfg.ALBIngressController.TestMode == "ingress-test-server" {
		md.cfg.ALBIngressController.TestResultQPS = rs.QPS
		md.cfg.ALBIngressController.TestResultFailures = rs.Failure
	} else {
		pv, perr := wrk.Parse(string(rbytes))
		if perr != nil {
			md.lg.Warn("failed to parse 'wrk' command output", zap.String("output", string(rbytes)), zap.Error(perr))
		}
		md.cfg.ALBIngressController.TestResultQPS = pv.RequestsPerSec
		md.cfg.ALBIngressController.TestResultFailures = pv.ErrorsConnect + pv.ErrorsWrite + pv.ErrorsRead + pv.ErrorsTimeout
	}
	md.cfg.Sync()

	if int64(len(rs.Errors)) > md.cfg.ALBIngressController.TestClientErrorThreshold {
		return fmt.Errorf("expected errors under threshold %d, got %v", md.cfg.ALBIngressController.TestClientErrorThreshold, rs.Errors)
	}
	if md.cfg.ALBIngressController.TestResultFailures > md.cfg.ALBIngressController.TestClientErrorThreshold {
		return fmt.Errorf("expected failures under threshold %d, got %d", md.cfg.ALBIngressController.TestClientErrorThreshold, md.cfg.ALBIngressController.TestResultFailures)
	}
	if md.cfg.ALBIngressController.TestExpectQPS > 0.0 &&
		md.cfg.ALBIngressController.TestResultQPS < md.cfg.ALBIngressController.TestExpectQPS {
		return fmt.Errorf("expected QPS %f, got %f", md.cfg.ALBIngressController.TestExpectQPS, md.cfg.ALBIngressController.TestResultQPS)
	}
	return nil
}

func (md *embedded) TestALBMetrics() error {
	ep := "http://" + md.cfg.ALBIngressController.ELBv2NamespaceToDNSName["kube-system"] + "/metrics"

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

	err = ioutil.WriteFile(md.cfg.ALBIngressController.MetricsOutputToUploadPath, d, 0600)
	if err != nil {
		return err
	}
	if md.cfg.LogAutoUpload {
		if err = md.uploadALB(); err != nil {
			md.lg.Warn("failed to upload ALB", zap.Error(err))
		}
	}
	return nil
}

// SECURITY NOTE: MAKE SURE PRIVATE KEY NEVER GETS UPLOADED TO CLOUD STORAGE AND DLETE AFTER USE!!!
func (md *embedded) upload() (err error) {
	err = md.s3Plugin.UploadToBucketForTests(
		md.cfg.ConfigPath,
		md.cfg.ConfigPathBucket,
	)
	if err != nil {
		return err
	}
	return md.s3Plugin.UploadToBucketForTests(
		md.cfg.LogOutputToUploadPath,
		md.cfg.LogOutputToUploadPathBucket,
	)
}

// TODO: parallelize for >100 nodes?
func (md *embedded) uploadWorkerNode() (err error) {
	if !md.cfg.EnableNodeSSH {
		return nil
	}
	err = md.downloadWorkerNodeLogs()
	if err != nil {
		return err
	}
	for fpath, s3Path := range md.cfg.GetWorkerNodeLogs() {
		err = md.s3Plugin.UploadToBucketForTests(fpath, s3Path)
		if err != nil {
			md.lg.Warn(
				"failed to upload",
				zap.String("file-path", fpath),
				zap.Error(err),
			)
			time.Sleep(2 * time.Second)
			continue
		}

		md.lg.Info("uploaded", zap.String("s3-path", s3Path))
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

func (md *embedded) uploadALB() (err error) {
	err = md.s3Plugin.UploadToBucketForTests(
		md.cfg.ALBIngressController.IngressTestServerDeploymentServiceSpecPath,
		md.cfg.ALBIngressController.IngressTestServerDeploymentServiceSpecPathBucket,
	)
	if err != nil {
		return err
	}
	err = md.s3Plugin.UploadToBucketForTests(
		md.cfg.ALBIngressController.IngressControllerSpecPath,
		md.cfg.ALBIngressController.IngressControllerSpecPathBucket,
	)
	if err != nil {
		return err
	}
	err = md.s3Plugin.UploadToBucketForTests(
		md.cfg.ALBIngressController.IngressObjectSpecPath,
		md.cfg.ALBIngressController.IngressObjectSpecPathBucket,
	)
	if err != nil {
		return err
	}
	if md.cfg.ALBIngressController.TestScalability {
		err = md.s3Plugin.UploadToBucketForTests(
			md.cfg.ALBIngressController.ScalabilityOutputToUploadPath,
			md.cfg.ALBIngressController.ScalabilityOutputToUploadPathBucket,
		)
		if err != nil {
			return err
		}
	}
	return md.s3Plugin.UploadToBucketForTests(
		md.cfg.ALBIngressController.MetricsOutputToUploadPath,
		md.cfg.ALBIngressController.MetricsOutputToUploadPathBucket,
	)
}
