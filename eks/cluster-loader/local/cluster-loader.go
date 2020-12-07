// Package local implements local cluster loader.
// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2
// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2/testing/overrides
package local

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	cluster_loader "github.com/aws/aws-k8s-tester/eks/cluster-loader"
	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/aws/cw"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"go.uber.org/zap"
)

// Config defines cluster loader configuration.
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	S3API     s3iface.S3API
	CWAPI     cloudwatchiface.CloudWatchAPI
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

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.loader = cluster_loader.New(cluster_loader.Config{
		Logger:    ts.cfg.Logger,
		LogWriter: ts.cfg.LogWriter,

		Stopc: ts.cfg.Stopc,

		S3API:        ts.cfg.S3API,
		S3BucketName: ts.cfg.EKSConfig.S3BucketName,

		KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,

		ClusterLoaderPath:        ts.cfg.EKSConfig.AddOnClusterLoaderLocal.ClusterLoaderPath,
		ClusterLoaderDownloadURL: ts.cfg.EKSConfig.AddOnClusterLoaderLocal.ClusterLoaderDownloadURL,
		TestConfigPath:           ts.cfg.EKSConfig.AddOnClusterLoaderLocal.TestConfigPath,
		ReportDir:                ts.cfg.EKSConfig.AddOnClusterLoaderLocal.ReportDir,
		ReportTarGzPath:          ts.cfg.EKSConfig.AddOnClusterLoaderLocal.ReportTarGzPath,
		ReportTarGzS3Key:         ts.cfg.EKSConfig.AddOnClusterLoaderLocal.ReportTarGzS3Key,
		LogPath:                  ts.cfg.EKSConfig.AddOnClusterLoaderLocal.LogPath,
		LogS3Key:                 ts.cfg.EKSConfig.AddOnClusterLoaderLocal.LogS3Key,
		PodStartupLatencyPath:    ts.cfg.EKSConfig.AddOnClusterLoaderLocal.PodStartupLatencyPath,
		PodStartupLatencyS3Key:   ts.cfg.EKSConfig.AddOnClusterLoaderLocal.PodStartupLatencyS3Key,

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

		CL2UseHostNetworkPods:     ts.cfg.EKSConfig.AddOnClusterLoaderLocal.CL2UseHostNetworkPods,
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

	ts.cfg.EKSConfig.AddOnClusterLoaderLocal.PodStartupLatency, err = cluster_loader.ParsePodStartupLatency(ts.cfg.EKSConfig.AddOnClusterLoaderLocal.PodStartupLatencyPath)
	if err != nil {
		return fmt.Errorf("failed to read PodStartupLatency %q (%v)", ts.cfg.EKSConfig.AddOnClusterLoaderLocal.PodStartupLatencyPath, err)
	}
	ts.cfg.EKSConfig.Sync()

	if err = ts.publishResults(); err != nil {
		return err
	}

	ts.cfg.EKSConfig.Sync()
	return nil
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
	ts.cfg.EKSConfig.Sync()
	return nil
}

/*
e.g.

  pod-startup-latency:
    dataItems:
    - data:
        Perc50: 0
        Perc90: 0
        Perc99: 250
      labels:
        Metric: create_to_schedule
      unit: ms
    - data:
        Perc50: 1000
        Perc90: 1062.5
        Perc99: 2062.5
      labels:
        Metric: schedule_to_run
      unit: ms
    - data:
        Perc50: 1200.219827375
        Perc90: 1741.2740193125
        Perc99: 1992.869426375
      labels:
        Metric: run_to_watch
      unit: ms
    - data:
        Perc50: 2289.84256025
        Perc90: 2848.8128514375
        Perc99: 3143.05340125
      labels:
        Metric: schedule_to_watch
      unit: ms
    - data:
        Perc50: 2297.02802625
        Perc90: 2861.429472625
        Perc99: 3156.2773068125
      labels:
        Metric: pod_startup
      unit: ms
    version: "1.0"
*/

func (ts *tester) publishResults() (err error) {
	tv := aws.Time(time.Now().UTC())
	datums := make([]*cloudwatch.MetricDatum, 0)
	for _, item := range ts.cfg.EKSConfig.AddOnClusterLoaderLocal.PodStartupLatency.DataItems {
		name := "add-on-cluster-loader-local-pod-startup-latency"
		if mv, ok := item.Labels["Metric"]; ok {
			name += "-" + mv
		}
		for k, fv := range item.Data {
			key := name + "-" + k
			ts.cfg.Logger.Info("adding", zap.String("key", key), zap.Float64("value", fv))
			datums = append(datums, &cloudwatch.MetricDatum{
				Timestamp:  tv,
				MetricName: aws.String(key),
				Unit:       toUnit(item.Unit),
				Value:      aws.Float64(fv),
			})
		}
	}
	return cw.PutData(ts.cfg.Logger, ts.cfg.CWAPI, ts.cfg.EKSConfig.CWNamespace, 20, datums...)
}

func toUnit(k string) *string {
	switch k {
	case "ms":
		return aws.String(cloudwatch.StandardUnitMilliseconds)
	default:
		return nil
	}
}
