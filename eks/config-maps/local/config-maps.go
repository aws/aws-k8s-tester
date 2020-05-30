// Package local implements tester for ConfigMap.
package local

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	config_maps "github.com/aws/aws-k8s-tester/eks/config-maps"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"go.uber.org/zap"
)

// Config defines configmaps local tester configuration.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}

	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

// Tester defines configmaps local tester.
type Tester interface {
	// Create installs configmaps local tester.
	Create() error
	// Delete deletes configmaps local tester.
	Delete() error
}

func New(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg Config
}

func (ts *tester) Create() (err error) {
	if ts.cfg.EKSConfig.AddOnConfigMapsLocal.Created {
		ts.cfg.Logger.Info("skipping create AddOnConfigMapsLocal")
		return nil
	}

	ts.cfg.Logger.Info("starting configmaps local tester")
	ts.cfg.EKSConfig.AddOnConfigMapsLocal.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnConfigMapsLocal.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnConfigMapsLocal.Namespace,
	); err != nil {
		return err
	}

	loader := config_maps.New(config_maps.Config{
		Logger:         ts.cfg.Logger,
		Stopc:          ts.cfg.Stopc,
		Client:         ts.cfg.K8SClient,
		ClientTimeout:  ts.cfg.EKSConfig.ClientTimeout,
		Namespace:      ts.cfg.EKSConfig.AddOnConfigMapsLocal.Namespace,
		Objects:        ts.cfg.EKSConfig.AddOnConfigMapsLocal.Objects,
		ObjectSize:     ts.cfg.EKSConfig.AddOnConfigMapsLocal.ObjectSize,
		WritesJSONPath: ts.cfg.EKSConfig.AddOnConfigMapsLocal.RequestsWritesJSONPath,
	})
	loader.Start()
	loader.Stop()

	ts.cfg.Logger.Info("completing configmaps local tester")
	ts.cfg.EKSConfig.AddOnConfigMapsLocal.RequestsWritesSummary, err = loader.CollectMetrics()
	ts.cfg.EKSConfig.Sync()
	if err != nil {
		ts.cfg.Logger.Warn("failed to get metrics", zap.Error(err))
	} else {
		err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnConfigMapsLocal.RequestsWritesSummaryJSONPath, []byte(ts.cfg.EKSConfig.AddOnConfigMapsLocal.RequestsWritesSummary.JSON()), 0600)
		if err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnConfigMapsLocal.RequestsWritesSummaryTablePath, []byte(ts.cfg.EKSConfig.AddOnConfigMapsLocal.RequestsWritesSummary.Table()), 0600)
		if err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
	}
	fmt.Printf("\n\nAddOnConfigMapsLocal.RequestsWritesSummary:\n%s\n", ts.cfg.EKSConfig.AddOnConfigMapsLocal.RequestsWritesSummary.Table())

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
	if !ts.cfg.EKSConfig.AddOnConfigMapsLocal.Created {
		ts.cfg.Logger.Info("skipping delete AddOnConfigMapsLocal")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnConfigMapsLocal.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnConfigMapsLocal.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
	); err != nil {
		return fmt.Errorf("failed to delete configmaps local tester namespace (%v)", err)
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnConfigMapsLocal.Created = false
	return ts.cfg.EKSConfig.Sync()
}
