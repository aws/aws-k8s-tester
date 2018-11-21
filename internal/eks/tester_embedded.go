package eks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/ekstester"
	"github.com/aws/aws-k8s-tester/internal/eks/alb"
	"github.com/aws/aws-k8s-tester/internal/eks/alb/ingress/client"
	"github.com/aws/aws-k8s-tester/internal/eks/alb/ingress/path"
	"github.com/aws/aws-k8s-tester/internal/eks/s3"
	"github.com/aws/aws-k8s-tester/pkg/awsapi"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/httputil"
	"github.com/aws/aws-k8s-tester/pkg/wrk"
	"github.com/aws/aws-k8s-tester/pkg/zaputil"

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
	"k8s.io/test-infra/kubetest/util"
	"k8s.io/utils/exec"
)

type embedded struct {
	stopc chan struct{}

	mu  sync.RWMutex
	lg  *zap.Logger
	cfg *eksconfig.Config

	// TODO: move this "kubectl" to AWS CLI deployer
	// and instead use "k8s.io/client-go" with STS token
	kubectlPath             string
	awsIAMAuthenticatorPath string

	ss  *session.Session
	im  iamiface.IAMAPI
	sts stsiface.STSAPI
	cf  cloudformationiface.CloudFormationAPI
	asg autoscalingiface.AutoScalingAPI
	eks eksiface.EKSAPI
	ec2 ec2iface.EC2API

	ec2InstancesLogMu *sync.RWMutex

	s3Plugin s3.Plugin

	// for plugins, sub-project implementation
	albPlugin alb.Plugin

	// TODO: add EBS (with CSI) plugin
	// TODO: add KMS plugin
}

// newTesterEmbedded creates a new embedded AWS tester.
func newTesterEmbedded(cfg *eksconfig.Config) (ekstester.Tester, error) {
	now := time.Now().UTC()

	lg, err := zaputil.New(cfg.LogDebug, cfg.LogOutputs)
	if err != nil {
		return nil, err
	}

	md := &embedded{
		stopc:             make(chan struct{}),
		lg:                lg,
		cfg:               cfg,
		ec2InstancesLogMu: &sync.RWMutex{},
	}

	md.kubectlPath, _ = exec.New().LookPath("kubectl")

	if cfg.KubectlDownloadURL != "" { // overwrite
		if runtime.GOOS == "darwin" {
			cfg.KubectlDownloadURL = strings.Replace(cfg.KubectlDownloadURL, "linux", "darwin", -1)
		}
		var f *os.File
		f, err = ioutil.TempFile(os.TempDir(), "kubectl")
		if err != nil {
			return nil, fmt.Errorf("failed to create %q (%v)", md.kubectlPath, err)
		}

		md.kubectlPath = f.Name()
		md.kubectlPath, _ = filepath.Abs(md.kubectlPath)
		if err = httpRead(md.lg, cfg.KubectlDownloadURL, f); err != nil {
			return nil, err
		}
		f.Close()
		if err = util.EnsureExecutable(md.kubectlPath); err != nil {
			return nil, err
		}

		err = fileutil.Copy(md.kubectlPath, "/usr/local/bin/kubectl")
		if err != nil {
			md.lg.Warn("failed to copy",
				zap.String("kubectl", md.kubectlPath),
				zap.Error(err),
			)
		}
	}
	lg.Info("setting KUBECTL_PATH environmental variable for kubetest", zap.Strings("envs", os.Environ()))
	os.Setenv("KUBECTL_PATH", md.kubectlPath)
	os.Setenv("kubectl", md.kubectlPath)
	lg.Info("set KUBECTL_PATH environmental variable for kubetest", zap.Strings("envs", os.Environ()))

	if cfg.AWSIAMAuthenticatorDownloadURL != "" { // overwrite
		if runtime.GOOS == "darwin" {
			cfg.AWSIAMAuthenticatorDownloadURL = strings.Replace(cfg.AWSIAMAuthenticatorDownloadURL, "linux", "darwin", -1)
		}
		var f *os.File
		f, err = ioutil.TempFile(os.TempDir(), "aws-iam-authenticator")
		if err != nil {
			return nil, fmt.Errorf("failed to create %q (%v)", md.awsIAMAuthenticatorPath, err)
		}
		md.awsIAMAuthenticatorPath = f.Name()
		md.awsIAMAuthenticatorPath, _ = filepath.Abs(md.awsIAMAuthenticatorPath)
		if err = httpRead(md.lg, cfg.AWSIAMAuthenticatorDownloadURL, f); err != nil {
			return nil, err
		}
		f.Close()
		if err = util.EnsureExecutable(md.awsIAMAuthenticatorPath); err != nil {
			return nil, err
		}
		err = fileutil.Copy(md.awsIAMAuthenticatorPath, "/usr/local/bin/aws-iam-authenticator")
		if err != nil {
			md.lg.Warn("failed to copy",
				zap.String("aws-iam-authenticator", md.awsIAMAuthenticatorPath),
				zap.Error(err),
			)
		}
	}

	md.lg.Info(
		"checking kubectl and aws-iam-authenticator",
		zap.String("kubectl", md.kubectlPath),
		zap.String("kubectl-download-url", md.cfg.KubectlDownloadURL),
		zap.String("aws-iam-authenticator", md.awsIAMAuthenticatorPath),
		zap.String("aws-iam-authenticator-download-url", md.cfg.AWSIAMAuthenticatorDownloadURL),
	)

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

	output, oerr := md.sts.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if oerr != nil {
		return nil, oerr
	}
	md.cfg.AWSAccountID = *output.Account

	// up to 63 characters
	// https://docs.aws.amazon.com/awscloudtrail/latest/userguide/cloudtrail-s3-bucket-naming-requirements.html
	md.cfg.Tag += "-" + strings.ToLower(*output.UserId)
	h, _ := os.Hostname()
	if len(h) > 5 {
		h = strings.ToLower(h)
		h = strings.Replace(h, ".", "", -1)
		h = strings.Replace(h, "-", "", -1)
		h = strings.Replace(h, "_", "", -1)
		md.cfg.Tag += h
	}
	if len(md.cfg.Tag) > 42 {
		md.cfg.Tag = md.cfg.Tag[:42]
	}
	md.cfg.LogOutputToUploadPathURL = genS3URL(md.cfg.AWSRegion, md.cfg.Tag, md.cfg.LogOutputToUploadPathBucket)
	md.cfg.ConfigPathURL = genS3URL(md.cfg.AWSRegion, md.cfg.Tag, md.cfg.ConfigPathBucket)
	md.cfg.KubeConfigPathURL = genS3URL(md.cfg.AWSRegion, md.cfg.Tag, md.cfg.KubeConfigPathBucket)
	if md.cfg.ALBIngressController != nil {
		md.cfg.ALBIngressController.IngressTestServerDeploymentServiceSpecPathURL = genS3URL(md.cfg.AWSRegion, md.cfg.Tag, md.cfg.ALBIngressController.IngressTestServerDeploymentServiceSpecPathBucket)
		md.cfg.ALBIngressController.IngressControllerSpecPathURL = genS3URL(md.cfg.AWSRegion, md.cfg.Tag, md.cfg.ALBIngressController.IngressControllerSpecPathBucket)
		md.cfg.ALBIngressController.IngressObjectSpecPathURL = genS3URL(md.cfg.AWSRegion, md.cfg.Tag, md.cfg.ALBIngressController.IngressObjectSpecPathBucket)
		md.cfg.ALBIngressController.ScalabilityOutputToUploadPathURL = genS3URL(md.cfg.AWSRegion, md.cfg.Tag, md.cfg.ALBIngressController.ScalabilityOutputToUploadPathBucket)
		md.cfg.ALBIngressController.MetricsOutputToUploadPathURL = genS3URL(md.cfg.AWSRegion, md.cfg.Tag, md.cfg.ALBIngressController.MetricsOutputToUploadPathBucket)
	}
	md.s3Plugin = s3.NewEmbedded(md.lg, md.cfg, awss3.New(md.ss))

	if cfg.ALBIngressController.Enable {
		md.albPlugin = alb.NewEmbedded(md.stopc, lg, md.cfg, md.kubectlPath, md.im, md.ec2, elbv2.New(md.ss), md.s3Plugin)
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
				md.cfg.VPCID = *op.OutputValue
				lg.Info("found existing VPC stack VPC ID", zap.String("id", md.cfg.VPCID))
				continue
			}
			if *op.OutputKey == "SubnetIds" {
				vv := *op.OutputValue
				md.cfg.SubnetIDs = strings.Split(vv, ",")
				lg.Info("found existing VPC stack Subnet IDs", zap.Strings("ids", md.cfg.SubnetIDs))
				continue
			}
			if *op.OutputKey == "SecurityGroups" {
				md.cfg.SecurityGroupID = *op.OutputValue
				lg.Info("found existing VPC stack security group", zap.String("id", md.cfg.SecurityGroupID))
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
		md.cfg.PlatformVersion = *co.Cluster.PlatformVersion
		if md.cfg.ClusterState.Status == "ACTIVE" {
			// cluster is already created
			// if up command failed, separate down command would need
			// fetch cluster information with cluster name
			md.cfg.ClusterState.Endpoint = *co.Cluster.Endpoint
			md.cfg.ClusterState.CA = *co.Cluster.CertificateAuthority.Data
			if err = writeKUBECONFIG(
				md.lg,
				md.awsIAMAuthenticatorPath,
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

			kvOut, kvOutErr := exec.New().CommandContext(
				context.Background(),
				md.kubectlPath,
				"--kubeconfig="+md.cfg.KubeConfigPath,
				"version",
			).CombinedOutput()
			md.lg.Info(
				"checking kubectl after cluster creation",
				zap.String("kubectl", md.kubectlPath),
				zap.String("kubectl-download-url", md.cfg.KubectlDownloadURL),
				zap.String("kubectl-version", string(kvOut)),
				zap.String("kubectl-version-err", fmt.Sprintf("%v", kvOutErr)),
			)
		}
	}

	lg.Info(
		"created EKS deployer",
		zap.String("cluster-name", cfg.ClusterName),
		zap.String("aws-k8s-tester-eksconfig-path", cfg.ConfigPath),
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

	if md.cfg.WaitBeforeDown > 0 {
		md.lg.Info("waiting before cluster tear down", zap.Duration("wait", md.cfg.WaitBeforeDown))
		select {
		case <-time.After(md.cfg.WaitBeforeDown):
			// TODO: handle interrupt syscall
		}
		md.lg.Info("waited before cluster tear down", zap.Duration("wait", md.cfg.WaitBeforeDown))
	}

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
		if err = md.createAWSCredentialSecret(); err != nil {
			return err
		}
	}

	md.cfg.Sync()
	md.cfg.SetClusterUpTook(time.Now().UTC().Sub(now))

	if md.cfg.UploadWorkerNodeLogs {
		if err = md.uploadWorkerNodeLogs(); err != nil {
			md.lg.Warn("failed to upload worker node logs", zap.Error(err))
		}
	}

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

	if md.cfg.UploadTesterLogs {
		if err = md.uploadTesterLogs(); err != nil {
			md.lg.Warn("failed to upload", zap.Error(err))
		}
	}
	if md.cfg.ALBIngressController.Enable && md.cfg.ALBIngressController.UploadTesterLogs {
		if err = md.uploadALBTesterLogs(); err != nil {
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

	if md.cfg.UploadWorkerNodeLogs {
		md.lg.Info(
			"uploading worker node logs before shutdown",
			zap.String("cluster-name", md.cfg.ClusterName),
		)
		if err = md.uploadWorkerNodeLogs(); err != nil {
			md.lg.Warn("failed to upload worker node logs", zap.Error(err))
		}
	}

	md.lg.Info("Down", zap.String("cluster-name", md.cfg.ClusterName))
	var errs []string
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
	if md.cfg.UploadTesterLogs {
		if err = md.uploadTesterLogs(); err != nil {
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
	md.cfg.PlatformVersion = *do.Cluster.PlatformVersion
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
	err = md.GetWorkerNodeLogs()
	if err != nil {
		return err
	}

	md.ec2InstancesLogMu.RLock()
	defer md.ec2InstancesLogMu.RUnlock()

	for fpath, p := range md.cfg.ClusterState.WorkerNodeLogs {
		if err = fileutil.Copy(fpath, filepath.Join(artifactDir, p)); err != nil {
			return err
		}
	}

	if err = fileutil.Copy(
		md.cfg.ConfigPath,
		filepath.Join(artifactDir, md.cfg.ConfigPathBucket),
	); err != nil {
		return err
	}
	return fileutil.Copy(
		md.cfg.LogOutputToUploadPath,
		filepath.Join(artifactDir, md.cfg.LogOutputToUploadPathBucket),
	)
}

func (md *embedded) UploadToBucketForTests(localPath, remotePath string) error {
	return md.s3Plugin.UploadToBucketForTests(localPath, remotePath)
}

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
			"--duration", fmt.Sprintf("%s", time.Duration(md.cfg.ALBIngressController.TestScalabilityMinutes)*time.Minute),
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

	if md.cfg.ALBIngressController.UploadTesterLogs {
		if err := md.uploadALBTesterLogs(); err != nil {
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
	if md.cfg.ALBIngressController.UploadTesterLogs {
		if err = md.uploadALBTesterLogs(); err != nil {
			md.lg.Warn("failed to upload ALB", zap.Error(err))
		}
	}
	return nil
}

// SECURITY NOTE: MAKE SURE PRIVATE KEY NEVER GETS UPLOADED TO CLOUD STORAGE AND DLETE AFTER USE!!!
func (md *embedded) uploadTesterLogs() (err error) {
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
func (md *embedded) uploadWorkerNodeLogs() (err error) {
	if !md.cfg.EnableWorkerNodeSSH {
		return nil
	}
	err = md.GetWorkerNodeLogs()
	if err != nil {
		return err
	}

	md.ec2InstancesLogMu.RLock()
	defer md.ec2InstancesLogMu.RUnlock()

	for fpath, s3Path := range md.cfg.ClusterState.WorkerNodeLogs {
		err = md.s3Plugin.UploadToBucketForTests(fpath, s3Path)
		if err != nil {
			md.lg.Warn(
				"failed to upload",
				zap.String("file-path", fpath),
				zap.Error(err),
			)
			time.Sleep(3 * time.Second)
			continue
		}
		md.lg.Info("uploaded", zap.String("s3-path", s3Path))
		time.Sleep(30 * time.Millisecond)
	}
	return nil
}

func (md *embedded) uploadALBTesterLogs() (err error) {
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
	if md.cfg.ALBIngressController.TestMetrics {
		return md.s3Plugin.UploadToBucketForTests(
			md.cfg.ALBIngressController.MetricsOutputToUploadPath,
			md.cfg.ALBIngressController.MetricsOutputToUploadPathBucket,
		)
	}
	return nil
}

// genS3URL returns S3 URL path.
// e.g. https://s3-us-west-2.amazonaws.com/aws-k8s-tester-20180925/hello-world
func genS3URL(region, bucket, s3Path string) string {
	return fmt.Sprintf("https://s3-%s.amazonaws.com/%s/%s", region, bucket, s3Path)
}

var httpTransport *http.Transport

func init() {
	httpTransport = new(http.Transport)
	httpTransport.RegisterProtocol("file", http.NewFileTransport(http.Dir("/")))
}

// curl -L [URL] | writer
func httpRead(lg *zap.Logger, u string, wr io.Writer) error {
	lg.Info("downloading", zap.String("url", u))
	cli := &http.Client{Transport: httpTransport}
	r, err := cli.Get(u)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode >= 400 {
		return fmt.Errorf("%q returned %d", u, r.StatusCode)
	}
	_, err = io.Copy(wr, r.Body)
	lg.Info("downloaded", zap.String("url", u), zap.Error(err))
	return err
}
