// Package local implements tester for ConfigMap.
package local

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"reflect"
	"strings"
	"time"

	config_maps "github.com/aws/aws-k8s-tester/eks/configmaps"
	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"go.uber.org/zap"
)

// Config defines configmaps local tester configuration.
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	S3API     s3iface.S3API
}

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg Config
}

func (ts *tester) Create() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnConfigmapsLocal() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnConfigmapsLocal.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnConfigmapsLocal.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnConfigmapsLocal.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnConfigmapsLocal.Namespace,
	); err != nil {
		return err
	}

	loader := config_maps.New(config_maps.Config{
		Logger:                 ts.cfg.Logger,
		Stopc:                  ts.cfg.Stopc,
		S3API:                  ts.cfg.S3API,
		S3BucketName:           ts.cfg.EKSConfig.S3BucketName,
		S3DirName:              path.Join(ts.cfg.EKSConfig.Name, "add-on-configmaps-local"),
		Client:                 ts.cfg.K8SClient,
		ClientTimeout:          ts.cfg.EKSConfig.ClientTimeout,
		Namespace:              ts.cfg.EKSConfig.AddOnConfigmapsLocal.Namespace,
		Objects:                ts.cfg.EKSConfig.AddOnConfigmapsLocal.Objects,
		ObjectSize:             ts.cfg.EKSConfig.AddOnConfigmapsLocal.ObjectSize,
		WritesJSONPath:         ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsWritesJSONPath,
		WritesSummaryJSONPath:  ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsWritesSummaryJSONPath,
		WritesSummaryTablePath: ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsWritesSummaryTablePath,
	})
	loader.Start()
	loader.Stop()

	ts.cfg.Logger.Info("completing configmaps local tester")
	ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsWritesSummary, err = loader.CollectMetrics()
	ts.cfg.EKSConfig.Sync()
	if err != nil {
		ts.cfg.Logger.Warn("failed to get metrics", zap.Error(err))
	} else {
		err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsWritesSummaryJSONPath, []byte(ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsWritesSummary.JSON()), 0600)
		if err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsWritesSummaryTablePath, []byte(ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsWritesSummary.Table()), 0600)
		if err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
	}
	fmt.Printf("\n\nAddOnConfigmapsLocal.RequestsWritesSummary:\n%s\n", ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsWritesSummary.Table())

	waitDur, retryStart := 5*time.Minute, time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("health check aborted")
			return nil
		case <-time.After(5 * time.Second):
		}
		err = ts.cfg.K8SClient.CheckHealth()
		if err == nil {
			break
		}
		ts.cfg.Logger.Warn("health check failed", zap.Error(err))
	}
	ts.cfg.EKSConfig.Sync()
	if err == nil {
		ts.cfg.Logger.Info("health check success after configmaps local tester")
	} else {
		ts.cfg.Logger.Warn("health check failed after configmaps local tester", zap.Error(err))
	}
	return err
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnConfigmapsLocal() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnConfigmapsLocal.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnConfigmapsLocal.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnConfigmapsLocal.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithForceDelete(),
	); err != nil {
		return fmt.Errorf("failed to delete configmaps local tester namespace (%v)", err)
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnConfigmapsLocal.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) AggregateResults() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnConfigmapsLocal() {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnConfigmapsLocal.Created {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.AggregateResults", zap.String("tester", pkgName))
	return nil
}
