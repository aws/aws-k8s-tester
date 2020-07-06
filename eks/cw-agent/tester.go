// Package cwagent implements CloudWatch agent plugin.
// ref. https://github.com/aws-samples/amazon-cloudwatch-container-insights/tree/master/k8s-deployment-manifest-templates/deployment-mode/daemonset/container-insights-monitoring/quickstart
// ref. https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-setup-metrics.html
// ref. https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-setup-EKS-quickstart.html
// ref. https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-setup-logs.html
//
// Publishes worker nodes logs to:
//  - /aws/containerinsights/[CLUSTER-NAME]/performance
//
package cwagent

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"go.uber.org/zap"
)

// Config defines fluentd configuration.
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
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
	if !ts.cfg.EKSConfig.IsEnabledAddOnCWAgent() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnCWAgent.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnCWAgent.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnCWAgent.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err = k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnCWAgent.Namespace,
	); err != nil {
		return err
	}

	// create CloudWatch agent components
	if err = ts.createCWAgentServiceAccount(); err != nil {
		return err
	}
	if err = ts.createCWAgentRBACClusterRole(); err != nil {
		return err
	}
	if err = ts.createCWAgentRBACClusterRoleBinding(); err != nil {
		return err
	}
	if err = ts.createCWAgentConfigMapConfig(); err != nil {
		return err
	}
	if err = ts.createCWAgentDaemonSet(); err != nil {
		return err
	}
	if err = ts.checkCWAgentPods(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnCWAgent() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnCWAgent.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnCWAgent.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteCWAgentDaemonSet(); err != nil {
		errs = append(errs, err.Error())
	}
	time.Sleep(time.Minute)

	if err := ts.deleteCWAgentConfigMapConfig(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ts.deleteCWAgentRBACClusterRoleBinding(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ts.deleteCWAgentRBACClusterRole(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ts.deleteCWAgentServiceAccount(); err != nil {
		errs = append(errs, err.Error())
	}

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnCWAgent.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete fluentd namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnCWAgent.Created = false
	return ts.cfg.EKSConfig.Sync()
}
