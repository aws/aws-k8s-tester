// Package local implements local cluster loader.
// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2
package local

import (
	"errors"
	"strings"
	"time"

	cluster_loader "github.com/aws/aws-k8s-tester/eks/cluster-loader"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"go.uber.org/zap"
)

// Config defines cluster loader configuration.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}

	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

// Tester defines cluster loader tester.
type Tester interface {
	// Create installs hollow nodes.
	Create() error
	// Delete deletes hollow nodes.
	Delete() error
}

func New(cfg Config) Tester {
	return &tester{
		cfg: cfg,

		loader: cluster_loader.New(cluster_loader.Config{
			Logger: cfg.Logger,
			Stopc:  cfg.Stopc,

			KubeConfigPath: cfg.EKSConfig.KubeConfigPath,

			ClusterLoaderPath:           cfg.EKSConfig.AddOnClusterLoaderLocal.ClusterLoaderPath,
			ClusterLoaderDownloadURL:    cfg.EKSConfig.AddOnClusterLoaderLocal.ClusterLoaderDownloadURL,
			ClusterLoaderTestConfigPath: cfg.EKSConfig.AddOnClusterLoaderLocal.ClusterLoaderTestConfigPath,
			ClusterLoaderReportDir:      cfg.EKSConfig.AddOnClusterLoaderLocal.ClusterLoaderReportDir,
			ClusterLoaderLogsPath:       cfg.EKSConfig.AddOnClusterLoaderLocal.ClusterLoaderLogsPath,

			Runs:    cfg.EKSConfig.AddOnClusterLoaderLocal.Runs,
			Timeout: cfg.EKSConfig.AddOnClusterLoaderLocal.Timeout,

			Nodes: cfg.EKSConfig.AddOnClusterLoaderLocal.Nodes,

			NodesPerNamespace: cfg.EKSConfig.AddOnClusterLoaderLocal.NodesPerNamespace,
			PodsPerNode:       cfg.EKSConfig.AddOnClusterLoaderLocal.PodsPerNode,

			BigGroupSize:    cfg.EKSConfig.AddOnClusterLoaderLocal.BigGroupSize,
			MediumGroupSize: cfg.EKSConfig.AddOnClusterLoaderLocal.MediumGroupSize,
			SmallGroupSize:  cfg.EKSConfig.AddOnClusterLoaderLocal.SmallGroupSize,

			SmallStatefulSetsPerNamespace:  cfg.EKSConfig.AddOnClusterLoaderLocal.SmallStatefulSetsPerNamespace,
			MediumStatefulSetsPerNamespace: cfg.EKSConfig.AddOnClusterLoaderLocal.MediumStatefulSetsPerNamespace,

			CL2EnablePVS:              cfg.EKSConfig.AddOnClusterLoaderLocal.CL2EnablePVS,
			PrometheusScrapeKubeProxy: cfg.EKSConfig.AddOnClusterLoaderLocal.PrometheusScrapeKubeProxy,
			EnableSystemPodMetrics:    cfg.EKSConfig.AddOnClusterLoaderLocal.EnableSystemPodMetrics,
		}),
	}
}

type tester struct {
	cfg    Config
	loader cluster_loader.Loader
}

func (ts *tester) Create() (err error) {
	if ts.cfg.EKSConfig.AddOnClusterLoaderLocal.Created {
		ts.cfg.Logger.Info("skipping create AddOnClusterLoaderLocal")
		return nil
	}

	ts.cfg.Logger.Info("starting cluster loader testing")
	ts.cfg.EKSConfig.AddOnClusterLoaderLocal.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnClusterLoaderLocal.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err = ts.loader.Start(); err != nil {
		return err
	}

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
	if err == nil {
		ts.cfg.Logger.Info("health check success after load testing")
	} else {
		ts.cfg.Logger.Warn("health check failed after load testing", zap.Error(err))
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() (err error) {
	if !ts.cfg.EKSConfig.AddOnClusterLoaderLocal.Created {
		ts.cfg.Logger.Info("skipping delete AddOnClusterLoaderLocal")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnClusterLoaderLocal.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if ts.loader != nil {
		ts.loader.Stop()
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnClusterLoaderLocal.Created = false
	return ts.cfg.EKSConfig.Sync()
}
