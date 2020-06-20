// Package mng defines AWS EKS Managed Node Group configuration.
package mng

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	version_upgrade "github.com/aws/aws-k8s-tester/eks/mng/version-upgrade"
	"github.com/aws/aws-k8s-tester/eks/mng/wait"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"go.uber.org/zap"
)

// Config defines Managed Node Group configuration.
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	IAMAPI    iamiface.IAMAPI
	CFNAPI    cloudformationiface.CloudFormationAPI
	EC2API    ec2iface.EC2API
	ASGAPI    autoscalingiface.AutoScalingAPI
	EKSAPI    eksiface.EKSAPI
}

// Tester implements EKS "Managed Node Group" for "kubetest2" Deployer.
// ref. https://github.com/kubernetes/test-infra/blob/master/kubetest2/pkg/types/types.go
// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
type Tester interface {
	// Create creates EKS "Managed Node Group", and waits for completion.
	Create() error
	// Delete deletes all EKS "Managed Node Group" resources.
	Delete() error
	// UpgradeVersion upgrades EKS "Managed Node Group" version, and waits for completion.
	UpgradeVersion() error

	// FetchLogs fetches logs from all worker nodes.
	FetchLogs() error
	// DownloadClusterLogs dumps all logs to artifact directory.
	// Let default kubetest log dumper handle all artifact uploads.
	// See https://github.com/kubernetes/test-infra/pull/9811/files#r225776067.
	DownloadClusterLogs(artifactDir string) error
}

// New creates a new Job tester.
func New(cfg Config) Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	return &tester{
		cfg: cfg,
		nodeWaiter: wait.New(wait.Config{
			Logger:    cfg.Logger,
			Stopc:     cfg.Stopc,
			EKSConfig: cfg.EKSConfig,
			K8SClient: cfg.K8SClient,
			EC2API:    cfg.EC2API,
			ASGAPI:    cfg.ASGAPI,
			EKSAPI:    cfg.EKSAPI,
		}),
		versionUpgrader: version_upgrade.New(version_upgrade.Config{
			Logger:    cfg.Logger,
			Stopc:     cfg.Stopc,
			EKSConfig: cfg.EKSConfig,
			K8SClient: cfg.K8SClient,
			EKSAPI:    cfg.EKSAPI,
		}),
		logsMu:          new(sync.RWMutex),
		deleteRequested: make(map[string]struct{}),
	}
}

type tester struct {
	cfg             Config
	nodeWaiter      wait.NodeWaiter
	versionUpgrader version_upgrade.Upgrader
	logsMu          *sync.RWMutex
	deleteRequested map[string]struct{}
}

func (ts *tester) Create() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnManagedNodeGroups() {
		ts.cfg.Logger.Info("managed node group is disabled; skipping creation")
		return nil
	}
	if ts.cfg.EKSConfig.AddOnManagedNodeGroups.Created {
		ts.cfg.Logger.Info("managed node group is already created; skipping creation")
		return nil
	}
	if len(ts.cfg.EKSConfig.Parameters.PublicSubnetIDs) == 0 {
		return errors.New("empty EKSConfig.Parameters.PublicSubnetIDs")
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err = ts.createRole(); err != nil {
		return err
	}
	if err = ts.createASGs(); err != nil {
		return err
	}
	for mngName := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
		if err = ts.createSG(mngName); err != nil {
			return err
		}
	}

	ts.cfg.EKSConfig.AddOnManagedNodeGroups.Created = true
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) UpgradeVersion() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnManagedNodeGroups() {
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnManagedNodeGroups.Created {
		ts.cfg.Logger.Info("ManagedNodeGroup is not created; skipping upgrade")
		return nil
	}

	for _, cur := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
		if err = ts.versionUpgrader.Upgrade(cur.Name); err != nil {
			return err
		}
	}
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnManagedNodeGroups() {
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnManagedNodeGroups.Created {
		ts.cfg.Logger.Info("ManagedNodeGroup is not created; skipping deletion")
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string
	var err error

	for name := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
		err = ts.deleteSG(name)
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	err = nil
	failedMNGs := make(map[string]struct{})
	for name := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
		var derr error
		for i := 0; i < 5; i++ { // retry, leakly ENI may take awhile to be deleted
			derr = ts.deleteASG(name)
			if derr != nil {
				failedMNGs[name] = struct{}{}
				ts.cfg.Logger.Warn("failed to delete mng; retrying", zap.String("name", name), zap.Error(derr))
				select {
				case <-ts.cfg.Stopc:
					ts.cfg.Logger.Warn("aborted")
					return nil
				case <-time.After(time.Minute):
				}
			}
			break
		}
		if derr != nil {
			if err == nil {
				err = derr
			} else {
				err = fmt.Errorf("%v; %v", err, derr)
			}
			continue
		}
		delete(failedMNGs, name)
	}
	if err != nil {
		errs = append(errs, err.Error())
	}

	waitDur := time.Minute
	ts.cfg.Logger.Info("sleeping after MNG deletion", zap.Duration("wait", waitDur))
	time.Sleep(waitDur)

	for name := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
		time.Sleep(10 * time.Second)
		if ok := ts.deleteENIs(name); ok {
			time.Sleep(10 * time.Second)
		}
	}
	err = nil
	for name := range failedMNGs {
		var derr error
		for i := 0; i < 5; i++ { // retry, leakly ENI may take awhile to be deleted
			derr = ts.deleteASG(name)
			if derr != nil {
				ts.cfg.Logger.Warn("failed to retry-delete mng; retrying", zap.String("name", name), zap.Error(derr))
				select {
				case <-ts.cfg.Stopc:
					ts.cfg.Logger.Warn("aborted")
					return nil
				case <-time.After(time.Minute):
				}
			}
			break
		}
		if derr != nil {
			if err == nil {
				err = derr
			} else {
				err = fmt.Errorf("%v; %v", err, derr)
			}
			continue
		}
		delete(failedMNGs, name)
	}
	if err != nil {
		errs = append(errs, err.Error())
	}

	// must be run after deleting node group
	// otherwise, "Cannot delete entity, must remove roles from instance profile first. (Service: AmazonIdentityManagement; Status Code: 409; Error Code: DeleteConflict; Request ID: 197f795b-1003-4386-81cc-44a926c42be7)"
	if err := ts.deleteRole(); err != nil {
		ts.cfg.Logger.Warn("failed to delete mng role", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnManagedNodeGroups.Created = false
	return ts.cfg.EKSConfig.Sync()
}
