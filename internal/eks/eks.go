package eks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	osexec "os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/ekstester"
	"github.com/aws/aws-k8s-tester/internal/eks/s3"
	"github.com/aws/aws-k8s-tester/pkg/awsapi"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
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
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	awss3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
	"k8s.io/test-infra/kubetest/util"
	"k8s.io/utils/exec"
)

// NewTester returns a new EKS tester.
func NewTester(cfg *eksconfig.Config) (ekstester.Tester, error) {
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		return nil, err
	}
	return newTesterEmbedded(cfg)
}

type embedded struct {
	stopc chan struct{}

	mu  sync.RWMutex
	lg  *zap.Logger
	cfg *eksconfig.Config

	eksSession *session.Session
	eks        eksiface.EKSAPI

	ss  *session.Session
	im  iamiface.IAMAPI
	sts stsiface.STSAPI
	cf  cloudformationiface.CloudFormationAPI
	asg autoscalingiface.AutoScalingAPI
	ec2 ec2iface.EC2API

	ec2InstancesLogMu *sync.RWMutex

	s3Plugin s3.Plugin

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

	if md.cfg.KubectlPath == "" {
		md.cfg.KubectlPath, _ = exec.New().LookPath("kubectl")
	} else {
		if err = os.MkdirAll(filepath.Dir(md.cfg.KubectlPath), 0700); err != nil {
			return nil, fmt.Errorf("could not create %q (%v)", filepath.Dir(md.cfg.KubectlPath), err)
		}
	}
	if md.cfg.KubectlDownloadURL != "" { // overwrite
		if runtime.GOOS == "darwin" {
			md.cfg.KubectlDownloadURL = strings.Replace(md.cfg.KubectlDownloadURL, "linux", "darwin", -1)
		}
		if err = os.RemoveAll(md.cfg.KubectlPath); err != nil {
			return nil, err
		}
		if err = os.MkdirAll(filepath.Dir(cfg.KubectlPath), 0700); err != nil {
			return nil, err
		}
		var f *os.File
		f, err = os.Create(md.cfg.KubectlPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create %q (%v)", md.cfg.KubectlPath, err)
		}
		md.cfg.KubectlPath = f.Name()
		md.cfg.KubectlPath, _ = filepath.Abs(md.cfg.KubectlPath)
		if err = httpRead(md.lg, md.cfg.KubectlDownloadURL, f); err != nil {
			return nil, err
		}
		if err = f.Close(); err != nil {
			return nil, fmt.Errorf("failed to close kubectl %v", err)
		}
		if err = util.EnsureExecutable(md.cfg.KubectlPath); err != nil {
			return nil, err
		}
	}

	if md.cfg.AWSIAMAuthenticatorPath == "" {
		md.cfg.AWSIAMAuthenticatorPath, _ = exec.New().LookPath("aws-iam-authenticator")
	} else {
		if err = os.MkdirAll(filepath.Dir(md.cfg.AWSIAMAuthenticatorPath), 0700); err != nil {
			return nil, fmt.Errorf("could not create %q (%v)", filepath.Dir(md.cfg.AWSIAMAuthenticatorPath), err)
		}
	}
	if md.cfg.AWSIAMAuthenticatorDownloadURL != "" { // overwrite
		if runtime.GOOS == "darwin" {
			md.cfg.AWSIAMAuthenticatorDownloadURL = strings.Replace(md.cfg.AWSIAMAuthenticatorDownloadURL, "linux", "darwin", -1)
		}
		if err = os.RemoveAll(md.cfg.AWSIAMAuthenticatorPath); err != nil {
			return nil, err
		}
		if err = os.MkdirAll(filepath.Dir(cfg.AWSIAMAuthenticatorPath), 0700); err != nil {
			return nil, err
		}
		var f *os.File
		f, err = os.Create(md.cfg.AWSIAMAuthenticatorPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create %q (%v)", md.cfg.AWSIAMAuthenticatorPath, err)
		}
		md.cfg.AWSIAMAuthenticatorPath = f.Name()
		md.cfg.AWSIAMAuthenticatorPath, _ = filepath.Abs(md.cfg.AWSIAMAuthenticatorPath)
		if err = httpRead(md.lg, md.cfg.AWSIAMAuthenticatorDownloadURL, f); err != nil {
			return nil, err
		}
		if err = f.Close(); err != nil {
			return nil, err
		}
		if err = util.EnsureExecutable(md.cfg.AWSIAMAuthenticatorPath); err != nil {
			return nil, err
		}
	}

	md.lg.Info(
		"checking kubectl and aws-iam-authenticator",
		zap.String("kubectl", md.cfg.KubectlPath),
		zap.String("kubectl-download-url", md.cfg.KubectlDownloadURL),
		zap.String("aws-iam-authenticator", md.cfg.AWSIAMAuthenticatorPath),
		zap.String("aws-iam-authenticator-download-url", md.cfg.AWSIAMAuthenticatorDownloadURL),
	)

	awsCfg := &awsapi.Config{
		Logger:        md.lg,
		DebugAPICalls: md.cfg.LogDebug,
		Region:        md.cfg.AWSRegion,
	}
	md.ss, err = awsapi.New(awsCfg)
	if err != nil {
		return nil, err
	}
	md.im = iam.New(md.ss)
	md.sts = sts.New(md.ss)
	md.cf = cloudformation.New(md.ss)
	md.asg = autoscaling.New(md.ss)
	md.ec2 = ec2.New(md.ss)

	awsCfgEKS := &awsapi.Config{
		Logger:         md.lg,
		DebugAPICalls:  md.cfg.LogDebug,
		Region:         md.cfg.AWSRegion,
		CustomEndpoint: md.cfg.EKSCustomEndpoint,
	}
	md.eksSession, err = awsapi.New(awsCfgEKS)
	if err != nil {
		return nil, err
	}
	md.eks = awseks.New(md.eksSession)

	var stsOutput *sts.GetCallerIdentityOutput
	stsOutput, err = md.sts.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, err
	}
	md.cfg.AWSAccountID = *stsOutput.Account

	// up to 63 characters
	// https://docs.aws.amazon.com/awscloudtrail/latest/userguide/cloudtrail-s3-bucket-naming-requirements.html
	md.cfg.Tag += "-" + strings.ToLower(*stsOutput.UserId)
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
	md.s3Plugin = s3.NewEmbedded(md.lg, md.cfg, awss3.New(md.ss))

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
		StackName: aws.String(md.cfg.CFStackVPCName),
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
				md.cfg.KubectlPath,
				md.cfg.AWSIAMAuthenticatorPath,
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
				md.cfg.KubectlPath,
				"--kubeconfig="+md.cfg.KubeConfigPath,
				"version",
			).CombinedOutput()
			md.lg.Info(
				"checking kubectl after cluster creation",
				zap.String("kubectl", md.cfg.KubectlPath),
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

// KubectlCommand returns "kubectl" command object for API reachability tests.
func (md *embedded) KubectlCommand() (*osexec.Cmd, error) {
	return osexec.Command(md.cfg.KubectlPath, "--kubeconfig="+md.cfg.KubeConfigPath), nil
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
			md.lg.Warn("failed to create EKS, reverting", zap.Error(err))
			md.lg.Warn("failed to create EKS, reverted", zap.Error(md.down()))
		}
	}()

	termChan := make(chan os.Signal)
	signal.Notify(termChan, syscall.SIGTERM, syscall.SIGINT)

	now := time.Now().UTC()

	md.lg.Info("Up",
		zap.String("cluster-name", md.cfg.ClusterName),
		zap.String("custom-endpoint", md.cfg.EKSCustomEndpoint),
		zap.String("KUBECONFIG", md.cfg.KubeConfigPath),
	)
	defer md.cfg.Sync()

	if err = catchStopc(md.lg, md.stopc, termChan, md.createAWSServiceRoleForAmazonEKS); err != nil {
		return err
	}
	if err = catchStopc(md.lg, md.stopc, termChan, md.attachPolicyForAWSServiceRoleForAmazonEKS); err != nil {
		return err
	}
	if err = catchStopc(md.lg, md.stopc, termChan, md.createVPC); err != nil {
		return err
	}
	if err = catchStopc(md.lg, md.stopc, termChan, md.createCluster); err != nil {
		return err
	}
	if err = catchStopc(md.lg, md.stopc, termChan, md.upgradeCNI); err != nil {
		return err
	}
	if err = catchStopc(md.lg, md.stopc, termChan, md.createKeyPair); err != nil {
		return err
	}
	if err = catchStopc(md.lg, md.stopc, termChan, md.createWorkerNode); err != nil {
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

	md.lg.Info("Up finished",
		zap.String("cluster-name", md.cfg.ClusterName),
		zap.String("custom-endpoint", md.cfg.EKSCustomEndpoint),
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
	return nil
}

func catchStopc(lg *zap.Logger, stopc chan struct{}, termc chan os.Signal, run func() error) (err error) {
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
	case sig := <-termc:
		err = fmt.Errorf("operating system: %v", sig)
	case err = <-errc:
	}
	return err
}

// Down terminates and deletes the EKS cluster.
func (md *embedded) Down() (err error) {
	md.mu.Lock()
	defer md.mu.Unlock()
	return md.down()
}

func (md *embedded) down() (err error) {
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
	if err = md.deleteWorkerNode(); err != nil {
		md.lg.Warn("failed to delete node group stack", zap.Error(err))
		errs = append(errs, err.Error())
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

// genS3URL returns S3 URL path.
// e.g. https://s3-us-west-2.amazonaws.com/aws-k8s-tester-20180925/hello-world
func genS3URL(region, bucket, s3Path string) string {
	return fmt.Sprintf("https://s3-%s.amazonaws.com/%s/%s", region, bucket, s3Path)
}

func (md *embedded) createCluster() error {
	if md.cfg.ClusterName == "" {
		return errors.New("cannot create empty cluster")
	}
	if md.cfg.ClusterState.ServiceRoleWithPolicyARN == "" {
		return errors.New("can't create cluster without service role ARN")
	}
	if len(md.cfg.SubnetIDs) == 0 {
		return errors.New("can't create cluster without subnet IDs")
	}
	if md.cfg.SecurityGroupID == "" {
		return errors.New("can't create cluster without security group ID")
	}

	now := time.Now().UTC()

	_, err := md.eks.CreateCluster(&awseks.CreateClusterInput{
		Name:    aws.String(md.cfg.ClusterName),
		Version: aws.String(md.cfg.KubernetesVersion),
		RoleArn: aws.String(md.cfg.ClusterState.ServiceRoleWithPolicyARN),
		ResourcesVpcConfig: &awseks.VpcConfigRequest{
			SubnetIds:        aws.StringSlice(md.cfg.SubnetIDs),
			SecurityGroupIds: aws.StringSlice([]string{md.cfg.SecurityGroupID}),
		},
	})
	if err != nil {
		return err
	}
	md.cfg.ClusterState.StatusClusterCreated = true
	md.cfg.ClusterState.Status = "CREATING"
	md.cfg.Sync()

	if md.cfg.UploadTesterLogs {
		if err = md.uploadTesterLogs(); err != nil {
			md.lg.Warn("failed to upload", zap.Error(err))
		}
	}

	// usually takes 10 minutes
	md.lg.Info("waiting for 7-minute")
	select {
	case <-md.stopc:
		md.lg.Info("interrupted cluster creation")
		return nil
	case <-time.After(7 * time.Minute):
	}

	retryStart := time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 20*time.Minute {
		select {
		case <-md.stopc:
			return nil
		default:
		}

		var do *awseks.DescribeClusterOutput
		do, err = md.eks.DescribeCluster(&awseks.DescribeClusterInput{
			Name: aws.String(md.cfg.ClusterName),
		})
		if err != nil {
			md.lg.Warn("failed to describe cluster", zap.Error(err))
			time.Sleep(10 * time.Second)
			continue
		}

		md.cfg.ClusterState.Status = *do.Cluster.Status
		md.cfg.ClusterState.Created = *do.Cluster.CreatedAt
		md.cfg.PlatformVersion = *do.Cluster.PlatformVersion
		md.cfg.Sync()

		if md.cfg.ClusterState.Status == "FAILED" {
			return fmt.Errorf("failed to create %q", md.cfg.ClusterName)
		}

		if md.cfg.ClusterState.Status == "ACTIVE" {
			if do.Cluster.Endpoint != nil {
				md.cfg.ClusterState.Endpoint = *do.Cluster.Endpoint
			}
			if do.Cluster.CertificateAuthority != nil && do.Cluster.CertificateAuthority.Data != nil {
				md.cfg.ClusterState.CA = *do.Cluster.CertificateAuthority.Data
			}
			md.cfg.Sync()
			break
		}

		md.cfg.Sync()

		md.lg.Info("creating cluster",
			zap.String("status", md.cfg.ClusterState.Status),
			zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
		)

		if md.cfg.UploadTesterLogs {
			if err = md.uploadTesterLogs(); err != nil {
				md.lg.Warn("failed to upload", zap.Error(err))
			}
		}

		time.Sleep(30 * time.Second)
	}

	if md.cfg.ClusterState.Status != "ACTIVE" {
		return fmt.Errorf("cluster creation took too long (status %q, took %v)", md.cfg.ClusterState.Status, time.Now().UTC().Sub(now))
	}
	if md.cfg.ClusterState.Endpoint == "" || md.cfg.ClusterState.CA == "" {
		return errors.New("cannot find cluster endpoint or cluster CA")
	}

	if err = writeKUBECONFIG(
		md.lg,
		md.cfg.KubectlPath,
		md.cfg.AWSIAMAuthenticatorPath,
		md.cfg.ClusterState.Endpoint,
		md.cfg.ClusterState.CA,
		md.cfg.ClusterName,
		md.cfg.KubeConfigPath,
	); err != nil {
		return err
	}

	if md.cfg.UploadKubeConfig {
		if err = md.s3Plugin.UploadToBucketForTests(
			md.cfg.KubeConfigPath,
			md.cfg.KubeConfigPathBucket,
		); err != nil {
			md.lg.Warn("failed to upload KUBECONFIG", zap.Error(err))
		} else {
			md.lg.Info("uploaded KUBECONFIG", zap.String("KUBECONFIG", md.cfg.KubeConfigPath))
		}
	}

	time.Sleep(5 * time.Second)

	retryStart = time.Now().UTC()
	txt, done := "", false
	for time.Now().UTC().Sub(retryStart) < 5*time.Minute {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		var out1 []byte
		out1, err = exec.New().CommandContext(ctx,
			md.cfg.KubectlPath,
			"--kubeconfig="+md.cfg.KubeConfigPath,
			"version",
		).CombinedOutput()
		cancel()
		md.lg.Info("ran kubectl version",
			zap.String("kubectl-path", md.cfg.KubectlPath),
			zap.String("aws-iam-authenticator-path", md.cfg.AWSIAMAuthenticatorPath),
			zap.String("output", string(out1)),
			zap.Error(err),
		)

		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		var out2 []byte
		out2, err = exec.New().CommandContext(ctx,
			md.cfg.KubectlPath,
			"--kubeconfig="+md.cfg.KubeConfigPath,
			"cluster-info",
		).CombinedOutput()
		cancel()
		md.lg.Info("ran kubectl cluster-info",
			zap.String("kubectl-path", md.cfg.KubectlPath),
			zap.String("aws-iam-authenticator-path", md.cfg.AWSIAMAuthenticatorPath),
			zap.String("output", string(out2)),
			zap.Error(err),
		)

		if strings.Contains(string(out2), "is running at") {
			err, done = nil, true
			break
		}

		// Or run
		// kubectl cluster-info dump --kubeconfig /tmp/aws-k8s-tester/kubeconfig

		time.Sleep(10 * time.Second)
	}
	if err != nil || !done {
		return fmt.Errorf("'kubectl get all' output unexpected: %s (%v)", txt, err)
	}

	md.lg.Info("created cluster",
		zap.String("name", md.cfg.ClusterName),
		zap.String("kubernetes-version", md.cfg.KubernetesVersion),
		zap.String("custom-endpoint", md.cfg.EKSCustomEndpoint),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return md.cfg.Sync()
}

func (md *embedded) deleteCluster(deleteKubeconfig bool) error {
	if !md.cfg.ClusterState.StatusClusterCreated {
		return nil
	}
	defer func() {
		md.cfg.ClusterState.StatusClusterCreated = false
		md.cfg.Sync()
	}()

	if md.cfg.ClusterName == "" {
		return errors.New("cannot delete empty cluster")
	}

	now := time.Now().UTC()

	// do not delete kubeconfig on "defer" call
	// only delete on "Down" call
	if deleteKubeconfig && md.cfg.KubeConfigPath != "" {
		rerr := os.RemoveAll(md.cfg.KubeConfigPath)
		md.lg.Info("deleted kubeconfig from local disk", zap.Error(rerr))
	}

	_, err := md.eks.DeleteCluster(&awseks.DeleteClusterInput{
		Name: aws.String(md.cfg.ClusterName),
	})
	if err != nil && !isEKSDeletedGoClient(err) {
		md.cfg.ClusterState.Status = err.Error()
		return err
	}

	// usually takes 5-minute
	md.lg.Info("waiting for 4-minute after cluster delete request")
	time.Sleep(4 * time.Minute)

	retryStart := time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 15*time.Minute {
		var do *awseks.DescribeClusterOutput
		do, err = md.eks.DescribeCluster(&awseks.DescribeClusterInput{
			Name: aws.String(md.cfg.ClusterName),
		})
		if err == nil {
			md.cfg.ClusterState.Status = *do.Cluster.Status
			md.cfg.ClusterState.Created = *do.Cluster.CreatedAt
			md.cfg.PlatformVersion = *do.Cluster.PlatformVersion
			md.cfg.Sync()

			md.lg.Info("deleting cluster",
				zap.String("status", md.cfg.ClusterState.Status),
				zap.String("created-ago", humanize.RelTime(md.cfg.ClusterState.Created, time.Now().UTC(), "ago", "from now")),
				zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
			)

			if md.cfg.UploadTesterLogs {
				if err = md.uploadTesterLogs(); err != nil {
					md.lg.Warn("failed to upload", zap.Error(err))
				}
			}

			time.Sleep(30 * time.Second)
			continue
		}

		if isEKSDeletedGoClient(err) {
			err = nil
			md.cfg.ClusterState.Status = "DELETE_COMPLETE"
			break
		}
		md.cfg.ClusterState.Status = err.Error()
		md.cfg.Sync()

		md.lg.Warn("failed to describe cluster", zap.String("name", md.cfg.ClusterName), zap.Error(err))
		time.Sleep(30 * time.Second)
	}

	if err != nil {
		md.lg.Warn("failed to delete cluster",
			zap.String("name", md.cfg.ClusterName),
			zap.String("status", md.cfg.ClusterState.Status),
			zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
			zap.Error(err),
		)
		return err
	}

	md.lg.Info("deleted cluster",
		zap.String("name", md.cfg.ClusterName),
		zap.String("status", md.cfg.ClusterState.Status),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return md.cfg.Sync()
}

// isEKSDeletedGoClient returns true if error from EKS API indicates that
// the EKS cluster has already been deleted.
func isEKSDeletedGoClient(err error) bool {
	if err == nil {
		return false
	}
	/*
	   https://docs.aws.amazon.com/eks/latest/APIReference/API_Cluster.html#AmazonEKS-Type-Cluster-status

	   CREATING
	   ACTIVE
	   DELETING
	   FAILED
	*/
	// ResourceNotFoundException: No cluster found for name: aws-k8s-tester-155468BC717E03B003\n\tstatus code: 404, request id: 1e3fe41c-b878-11e8-adca-b503e0ba731d
	return strings.Contains(err.Error(), "No cluster found for name: ")
}

const kubeConfigTempl = `---
apiVersion: v1
clusters:
- cluster:
    server: {{ .ClusterEndpoint }}
    certificate-authority-data: {{ .ClusterCA }}
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: aws
  name: aws
current-context: aws
kind: Config
preferences: {}
users:
- name: aws
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1alpha1
      command: {{ .AWSIAMAuthenticatorPath }}
      args:
        - token
        - -i
        - {{ .ClusterName }}

`

type kubeConfig struct {
	AWSIAMAuthenticatorPath string
	ClusterEndpoint         string
	ClusterCA               string
	ClusterName             string
}

func writeKUBECONFIG(
	lg *zap.Logger,
	kubectlPath string,
	awsIAMAuthenticatorPath string,
	ep string,
	ca string,
	clusterName string,
	outputPath string) (err error) {
	kc := kubeConfig{
		AWSIAMAuthenticatorPath: awsIAMAuthenticatorPath,
		ClusterEndpoint:         ep,
		ClusterCA:               ca,
		ClusterName:             clusterName,
	}
	tpl := template.Must(template.New("kubeCfgTempl").Parse(kubeConfigTempl))
	buf := bytes.NewBuffer(nil)
	if err = tpl.Execute(buf, kc); err != nil {
		return err
	}

	// TODO: not working for "kubetest/e2e.go", "getKubectlVersion"
	os.Setenv("KUBECTL", kubectlPath)
	os.Setenv("KUBE_MASTER_URL", ep)
	os.Setenv("KUBECONFIG", outputPath)
	os.Setenv("KUBE_CONFIG_FILE", outputPath)
	lg.Info("set KUBE_* environmental variables for kubetest", zap.Strings("envs", os.Environ()))

	return ioutil.WriteFile(outputPath, buf.Bytes(), 0600)
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
