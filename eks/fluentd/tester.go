// Package fluentd implements Fluentd plugin.
// ref. https://github.com/aws-samples/amazon-cloudwatch-container-insights/tree/master/k8s-deployment-manifest-templates/deployment-mode/daemonset/container-insights-monitoring/quickstart
// ref. https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-setup-metrics.html
// ref. https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-setup-EKS-quickstart.html
// ref. https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-setup-logs.html
//
// Publishes worker nodes logs to:
//  - /aws/containerinsights/[CLUSTER-NAME]/application
//  - /aws/containerinsights/[CLUSTER-NAME]/dataplane
//  - /aws/containerinsights/[CLUSTER-NAME]/host
//
package fluentd

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	aws_ecr "github.com/aws/aws-k8s-tester/pkg/aws/ecr"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"go.uber.org/zap"
)

// Config defines fluentd configuration.
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	ECRAPI    ecriface.ECRAPI
}

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
	return &tester{cfg: cfg, busyboxImg: "busybox"}
}

type tester struct {
	cfg Config

	busyboxImg string
}

func (ts *tester) Create() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnFluentd() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnFluentd.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnFluentd.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnFluentd.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if ts.cfg.EKSConfig.AddOnFluentd.RepositoryBusyboxAccountID != "" &&
		ts.cfg.EKSConfig.AddOnFluentd.RepositoryBusyboxRegion != "" &&
		ts.cfg.EKSConfig.AddOnFluentd.RepositoryBusyboxName != "" &&
		ts.cfg.EKSConfig.AddOnFluentd.RepositoryBusyboxImageTag != "" {
		if ts.busyboxImg, err = aws_ecr.Check(
			ts.cfg.Logger,
			ts.cfg.ECRAPI,
			ts.cfg.EKSConfig.AddOnFluentd.RepositoryBusyboxAccountID,
			ts.cfg.EKSConfig.AddOnFluentd.RepositoryBusyboxRegion,
			ts.cfg.EKSConfig.AddOnFluentd.RepositoryBusyboxName,
			ts.cfg.EKSConfig.AddOnFluentd.RepositoryBusyboxImageTag,
		); err != nil {
			return err
		}
	}

	if err = k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnFluentd.Namespace,
	); err != nil {
		return err
	}

	// create Fluentd components
	if err = ts.createFluentdServiceAccount(); err != nil {
		return err
	}
	if err = ts.createFluentdRBACClusterRole(); err != nil {
		return err
	}
	if err = ts.createFluentdRBACClusterRoleBinding(); err != nil {
		return err
	}
	if err = ts.createFluentdConfigMapClusterInfo(); err != nil {
		return err
	}
	if err = ts.createFluentdConfigMapConfig(); err != nil {
		return err
	}
	if err = ts.createFluentdDaemonSet(); err != nil {
		return err
	}
	if err = ts.checkFluentdPods(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnFluentd() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnFluentd.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnFluentd.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteFluentdDaemonSet(); err != nil {
		errs = append(errs, err.Error())
	}
	time.Sleep(time.Minute)

	if err := ts.deleteFluentdConfigMapConfig(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ts.deleteFluentdConfigMapClusterInfo(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ts.deleteFluentdRBACClusterRoleBinding(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ts.deleteFluentdRBACClusterRole(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ts.deleteFluentdServiceAccount(); err != nil {
		errs = append(errs, err.Error())
	}

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnFluentd.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete fluentd namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnFluentd.Created = false
	return ts.cfg.EKSConfig.Sync()
}
