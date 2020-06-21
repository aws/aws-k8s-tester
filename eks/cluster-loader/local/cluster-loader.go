// Package local implements local cluster loader.
// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2
// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2/testing/overrides
package local

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	cluster_loader "github.com/aws/aws-k8s-tester/eks/cluster-loader"
	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
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

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg    Config
	loader cluster_loader.Loader
}

func (ts *tester) Create() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnClusterLoaderLocal() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnClusterLoaderLocal.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	podStartupLatencyTempPath := fileutil.GetTempFilePath()

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.loader = cluster_loader.New(cluster_loader.Config{
		Logger: ts.cfg.Logger,
		Stopc:  ts.cfg.Stopc,

		KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,

		ClusterLoaderPath:           ts.cfg.EKSConfig.AddOnClusterLoaderLocal.ClusterLoaderPath,
		ClusterLoaderDownloadURL:    ts.cfg.EKSConfig.AddOnClusterLoaderLocal.ClusterLoaderDownloadURL,
		TestConfigPath:              ts.cfg.EKSConfig.AddOnClusterLoaderLocal.TestConfigPath,
		ReportDir:                   ts.cfg.EKSConfig.AddOnClusterLoaderLocal.ReportDir,
		ReportTarGzPath:             ts.cfg.EKSConfig.AddOnClusterLoaderLocal.ReportTarGzPath,
		LogPath:                     ts.cfg.EKSConfig.AddOnClusterLoaderLocal.LogPath,
		PodStartupLatencyOutputPath: podStartupLatencyTempPath,

		Runs:    ts.cfg.EKSConfig.AddOnClusterLoaderLocal.Runs,
		Timeout: ts.cfg.EKSConfig.AddOnClusterLoaderLocal.Timeout,

		Nodes: ts.cfg.EKSConfig.AddOnClusterLoaderLocal.Nodes,

		NodesPerNamespace: ts.cfg.EKSConfig.AddOnClusterLoaderLocal.NodesPerNamespace,
		PodsPerNode:       ts.cfg.EKSConfig.AddOnClusterLoaderLocal.PodsPerNode,

		BigGroupSize:    ts.cfg.EKSConfig.AddOnClusterLoaderLocal.BigGroupSize,
		MediumGroupSize: ts.cfg.EKSConfig.AddOnClusterLoaderLocal.MediumGroupSize,
		SmallGroupSize:  ts.cfg.EKSConfig.AddOnClusterLoaderLocal.SmallGroupSize,

		SmallStatefulSetsPerNamespace:  ts.cfg.EKSConfig.AddOnClusterLoaderLocal.SmallStatefulSetsPerNamespace,
		MediumStatefulSetsPerNamespace: ts.cfg.EKSConfig.AddOnClusterLoaderLocal.MediumStatefulSetsPerNamespace,

		CL2LoadTestThroughput:     ts.cfg.EKSConfig.AddOnClusterLoaderLocal.CL2LoadTestThroughput,
		CL2EnablePVS:              ts.cfg.EKSConfig.AddOnClusterLoaderLocal.CL2EnablePVS,
		PrometheusScrapeKubeProxy: ts.cfg.EKSConfig.AddOnClusterLoaderLocal.PrometheusScrapeKubeProxy,
		EnableSystemPodMetrics:    ts.cfg.EKSConfig.AddOnClusterLoaderLocal.EnableSystemPodMetrics,
	})

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

	ts.cfg.EKSConfig.AddOnClusterLoaderLocal.PodStartupLatency, err = cluster_loader.ParsePodStartupLatency(podStartupLatencyTempPath)
	if err != nil {
		return fmt.Errorf("failed to read PodStartupLatency %q (%v)", podStartupLatencyTempPath, err)
	}
	ts.cfg.EKSConfig.Sync()

	waitDur := 10 * time.Minute
	retryStart := time.Now()
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
	if !ts.cfg.EKSConfig.IsEnabledAddOnClusterLoaderLocal() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnClusterLoaderLocal.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
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

func (ts *tester) AggregateResults() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnClusterLoaderLocal() {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnClusterLoaderLocal.Created {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.AggregateResults", zap.String("tester", pkgName))
	return nil
}
