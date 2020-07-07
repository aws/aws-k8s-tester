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
	ts := &tester{
		cfg:        cfg,
		busyboxImg: "busybox",
	}
	ts.creates = []func() error{
		func() (err error) {
			if ts.cfg.EKSConfig.AddOnFluentd.RepositoryBusyboxAccountID != "" &&
				ts.cfg.EKSConfig.AddOnFluentd.RepositoryBusyboxRegion != "" &&
				ts.cfg.EKSConfig.AddOnFluentd.RepositoryBusyboxName != "" &&
				ts.cfg.EKSConfig.AddOnFluentd.RepositoryBusyboxImageTag != "" {
				ts.busyboxImg, err = aws_ecr.Check(
					ts.cfg.Logger,
					ts.cfg.ECRAPI,
					ts.cfg.EKSConfig.AddOnFluentd.RepositoryBusyboxAccountID,
					ts.cfg.EKSConfig.AddOnFluentd.RepositoryBusyboxRegion,
					ts.cfg.EKSConfig.AddOnFluentd.RepositoryBusyboxName,
					ts.cfg.EKSConfig.AddOnFluentd.RepositoryBusyboxImageTag,
				)
				return err
			}
			return nil
		},
		func() error {
			return k8s_client.CreateNamespace(ts.cfg.Logger, ts.cfg.K8SClient.KubernetesClientSet(), ts.cfg.EKSConfig.AddOnFluentd.Namespace)
		},
		func() error { return ts.createFluentdServiceAccount() },
		func() error { return ts.createFluentdRBACClusterRole() },
		func() error { return ts.createFluentdRBACClusterRoleBinding() },
		func() error { return ts.createFluentdConfigMapClusterInfo() },
		func() error { return ts.createFluentdConfigMapConfig() },
		func() error { return ts.createFluentdDaemonSet() },
		func() error { return ts.checkFluentdPods() },
	}
	ts.deletes = []func() error{
		func() error {
			// wait some time for delete completion
			defer time.Sleep(time.Minute)
			return ts.deleteFluentdDaemonSet()
		},
		func() error { return ts.deleteFluentdConfigMapConfig() },
		func() error { return ts.deleteFluentdRBACClusterRoleBinding() },
		func() error { return ts.deleteFluentdRBACClusterRole() },
		func() error { return ts.deleteFluentdServiceAccount() },
		func() error {
			return k8s_client.DeleteNamespaceAndWait(
				ts.cfg.Logger,
				ts.cfg.K8SClient.KubernetesClientSet(),
				ts.cfg.EKSConfig.AddOnFluentd.Namespace,
				k8s_client.DefaultNamespaceDeletionInterval,
				k8s_client.DefaultNamespaceDeletionTimeout,
				k8s_client.WithForceDelete(true),
			)
		},
	}
	return ts
}

type tester struct {
	cfg Config

	busyboxImg string

	creates []func() error
	deletes []func() error
}

// TODO: add "ShouldCreate/Delete" and dedup redundant code

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

	for _, createFunc := range ts.creates {
		if err = createFunc(); err != nil {
			return err
		}
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
	for _, deleteFunc := range ts.deletes {
		if err := deleteFunc(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnFluentd.Created = false
	return ts.cfg.EKSConfig.Sync()
}
