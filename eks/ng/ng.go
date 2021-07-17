// Package ng implements EKS worker nodes with a custom AMI.
package ng

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/eks/ng/autoscaler"
	"github.com/aws/aws-k8s-tester/eks/ng/wait"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	aws_asg_v2 "github.com/aws/aws-sdk-go-v2/service/autoscaling"
	aws_ec2_v2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	aws_iam_v2 "github.com/aws/aws-sdk-go-v2/service/iam"
	aws_ssm_v2 "github.com/aws/aws-sdk-go-v2/service/ssm"
	"go.uber.org/zap"
)

// Config defines Node Group configuration.
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS

	IAMAPIV2 *aws_iam_v2.Client
	EC2APIV2 *aws_ec2_v2.Client
	SSMAPIV2 *aws_ssm_v2.Client
	ASGAPIV2 *aws_asg_v2.Client
}

// Tester implements EKS "Node Group" for "kubetest2" Deployer.
// ref. https://github.com/kubernetes/test-infra/blob/master/kubetest2/pkg/types/types.go
// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
type Tester interface {
	// Name returns the name of the tester.
	Name() string
	// Create creates EKS "Node Group", and waits for completion.
	Create() error
	// Delete deletes all EKS "Node Group" resources.
	Delete() error

	// FetchLogs fetches logs from all worker nodes.
	FetchLogs() error
	// DownloadClusterLogs dumps all logs to artifact directory.
	// Let default kubetest log dumper handle all artifact uploads.
	// See https://github.com/kubernetes/test-infra/pull/9811/files#r225776067.
	DownloadClusterLogs(artifactDir string) error
}

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

// New creates a new Job tester.
func New(cfg Config) Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
	return &tester{
		cfg: cfg,
		nodeWaiter: wait.New(wait.Config{
			Logger:    cfg.Logger,
			LogWriter: cfg.LogWriter,
			Stopc:     cfg.Stopc,
			EKSConfig: cfg.EKSConfig,
			K8SClient: cfg.K8SClient,
			EC2APIV2:  cfg.EC2APIV2,
			ASGAPIV2:  cfg.ASGAPIV2,
		}),
		logsMu:     new(sync.RWMutex),
		failedOnce: false,
		clusterAutoscaler: autoscaler.New(autoscaler.Config{
			Logger:    cfg.Logger,
			LogWriter: cfg.LogWriter,
			Stopc:     cfg.Stopc,
			EKSConfig: cfg.EKSConfig,
			K8SClient: cfg.K8SClient,
		}),
	}
}

type tester struct {
	cfg               Config
	nodeWaiter        wait.NodeWaiter
	logsMu            *sync.RWMutex
	failedOnce        bool
	clusterAutoscaler autoscaler.ClusterAutoscaler
}

func (ts *tester) Create() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() {
		ts.cfg.Logger.Info("node group is disabled; skipping creation")
		return nil
	}
	if ts.cfg.EKSConfig.AddOnNodeGroups.Created {
		ts.cfg.Logger.Info("node group is already created; skipping creation")
		return nil
	}
	if len(ts.cfg.EKSConfig.VPC.PublicSubnetIDs) == 0 {
		return errors.New("empty EKSConfig.VPC.PublicSubnetIDs")
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnNodeGroups.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err = ts.createSecurityGroups(); err != nil {
		return err
	}
	if err = ts.authorizeSecurityGroup(); err != nil {
		return err
	}
	if err = ts.createRole(); err != nil {
		return err
	}
	if err = ts.createConfigMap(); err != nil {
		return err
	}
	if err = ts.createASGs(); err != nil {
		return err
	}
	if err = ts.createSSM(); err != nil {
		return err
	}
	if err = ts.clusterAutoscaler.Create(); err != nil {
		return err
	}

	ts.cfg.EKSConfig.AddOnNodeGroups.Created = true
	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() {
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnNodeGroups.Created {
		ts.cfg.Logger.Info("ManagedNodeGroup is not created; skipping deletion")
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnNodeGroups.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string
	if err := ts.deleteSSM(); err != nil {
		ts.cfg.Logger.Warn("failed to delete SSM", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if err := ts.deleteRole(); err != nil {
		ts.cfg.Logger.Warn("failed to delete role", zap.Error(err))
		errs = append(errs, err.Error())
	}

	select {
	case <-time.After(10 * time.Second):
	case <-ts.cfg.Stopc:
		return errors.New("stopped")
	}

	if err := ts.deleteASGs(); err != nil {
		ts.cfg.Logger.Warn("failed to delete ASGs", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if ok := ts.deleteENIs(); ok {
		time.Sleep(10 * time.Second)
	}

	select {
	case <-time.After(10 * time.Second):
	case <-ts.cfg.Stopc:
		return errors.New("stopped")
	}

	if ok := ts.deleteENIs(); ok {
		time.Sleep(10 * time.Second)
	}

	select {
	case <-time.After(10 * time.Second):
	case <-ts.cfg.Stopc:
		return errors.New("stopped")
	}

	if err := ts.revokeSecurityGroups(); err != nil {
		ts.cfg.Logger.Warn("failed to revoke SG", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if err := ts.deleteSecurityGroups(); err != nil {
		ts.cfg.Logger.Warn("failed to delete SG", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if ok := ts.deleteENIs(); ok {
		time.Sleep(10 * time.Second)
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnNodeGroups.Created = false
	ts.cfg.EKSConfig.Sync()
	return nil
}
