// Package local implements tester for ConfigMap.
package local

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"reflect"
	"sort"
	"strings"
	"time"

	config_maps "github.com/aws/aws-k8s-tester/eks/configmaps"
	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/aws/cw"
	aws_s3 "github.com/aws/aws-k8s-tester/pkg/aws/s3"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/metrics"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"go.uber.org/zap"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
)

// Config defines configmaps local tester configuration.
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
		Logger:                          ts.cfg.Logger,
		LogWriter:                       ts.cfg.LogWriter,
		Stopc:                           ts.cfg.Stopc,
		S3API:                           ts.cfg.S3API,
		S3BucketName:                    ts.cfg.EKSConfig.S3BucketName,
		Client:                          ts.cfg.K8SClient,
		ClientTimeout:                   ts.cfg.EKSConfig.ClientTimeout,
		Namespace:                       ts.cfg.EKSConfig.AddOnConfigmapsLocal.Namespace,
		Objects:                         ts.cfg.EKSConfig.AddOnConfigmapsLocal.Objects,
		ObjectSize:                      ts.cfg.EKSConfig.AddOnConfigmapsLocal.ObjectSize,
		RequestsRawWritesJSONPath:       ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsRawWritesJSONPath,
		RequestsRawWritesJSONS3Key:      ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsRawWritesJSONS3Key,
		RequestsSummaryWritesJSONPath:   ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWritesJSONPath,
		RequestsSummaryWritesJSONS3Key:  ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWritesJSONS3Key,
		RequestsSummaryWritesTablePath:  ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWritesTablePath,
		RequestsSummaryWritesTableS3Key: ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWritesTableS3Key,
	})
	loader.Start()
	loader.Stop()

	ts.cfg.Logger.Info("completing configmaps local tester")
	var curWriteLatencies metrics.Durations
	curWriteLatencies, ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWrites, err = loader.CollectMetrics()
	ts.cfg.EKSConfig.Sync()
	if err != nil {
		ts.cfg.Logger.Warn("failed to get metrics", zap.Error(err))
		return err
	}

	if err = ts.checkResults(curWriteLatencies); err != nil {
		return err
	}
	if err = ts.publishResults(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
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
		k8s_client.WithForceDelete(true),
	); err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		errs = append(errs, fmt.Sprintf("failed to delete configmaps local tester namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnConfigmapsLocal.Created = false
	return ts.cfg.EKSConfig.Sync()
}

// 1. if previous summary exists, download and compare
// 2. upload new summary and overwrite the previous s3 key
func (ts *tester) checkResults(curWriteLatencies metrics.Durations) (err error) {
	curTS := time.Now().UTC().Format(time.RFC3339Nano)
	ts.cfg.Logger.Info("checking results", zap.String("timestamp", curTS))

	s3Objects := make([]*s3.Object, 0)
	if ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWritesCompareS3Dir != "" {
		s3Objects, err = aws_s3.ListInDescendingLastModified(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			path.Clean(ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWritesCompareS3Dir)+"/",
		)
	}
	canCompare := len(s3Objects) > 0 && err == nil
	if canCompare {
		reqSummaryS3Key := aws.StringValue(s3Objects[0].Key)
		durRawS3Key := path.Join(ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsRawWritesCompareS3Dir, path.Base(reqSummaryS3Key))

		var prevSummary metrics.RequestsSummary
		prevSummary, err = metrics.DownloadRequestsSummaryFromS3(ts.cfg.Logger, ts.cfg.S3API, ts.cfg.EKSConfig.S3BucketName, reqSummaryS3Key)
		if err != nil {
			ts.cfg.Logger.Warn("failed to download results", zap.Error(err))
			return err
		}
		ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWritesCompare, err = metrics.CompareRequestsSummary(prevSummary, ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWrites)
		if err != nil {
			ts.cfg.Logger.Warn("failed to compare results", zap.Error(err))
			return err
		}
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWritesCompareJSONPath, []byte(ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWritesCompare.JSON()), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWritesCompareJSONS3Key,
			ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWritesCompareJSONPath,
		); err != nil {
			return err
		}
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWritesCompareTablePath, []byte(ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWritesCompare.Table()), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWritesCompareTableS3Key,
			ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWritesCompareTablePath,
		); err != nil {
			return err
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n\nRequestsSummaryWritesCompare:\n%s\n", ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWritesCompare.Table())

		var prevDurations metrics.Durations
		prevDurations, err = metrics.DownloadDurationsFromS3(ts.cfg.Logger, ts.cfg.S3API, ts.cfg.EKSConfig.S3BucketName, durRawS3Key)
		if err != nil {
			ts.cfg.Logger.Warn("failed to download results", zap.Error(err))
			return err
		}
		prevDurationsWithLabels := metrics.LabelDurations(prevDurations, prevSummary.TestID)
		curDurationsWithLabels := metrics.LabelDurations(curWriteLatencies, ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWrites.TestID)
		allDurationsWithLabels := append(prevDurationsWithLabels, curDurationsWithLabels...)
		sortStart := time.Now()
		ts.cfg.Logger.Info("sorting before and after durations with label",
			zap.Int("before-data-points", len(prevDurationsWithLabels)),
			zap.Int("after-data-points", len(curDurationsWithLabels)),
			zap.Int("total-points", len(allDurationsWithLabels)),
		)
		sort.Sort(allDurationsWithLabels)
		ts.cfg.Logger.Info("sorted before and after durations with label",
			zap.Int("before-data-points", len(prevDurationsWithLabels)),
			zap.Int("after-data-points", len(curDurationsWithLabels)),
			zap.Int("total-points", len(allDurationsWithLabels)),
			zap.String("took", time.Since(sortStart).String()),
		)
		allDataJSON, err := json.Marshal(allDurationsWithLabels)
		if err != nil {
			ts.cfg.Logger.Warn("failed to marshal results", zap.Error(err))
			return err
		}
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsRawWritesCompareAllJSONPath, []byte(allDataJSON), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsRawWritesCompareAllJSONS3Key,
			ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsRawWritesCompareAllJSONPath,
		); err != nil {
			return err
		}
		if err = allDurationsWithLabels.CSV(ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsRawWritesCompareAllCSVPath); err != nil {
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsRawWritesCompareAllCSVS3Key,
			ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsRawWritesCompareAllCSVPath,
		); err != nil {
			return err
		}
	} else {
		ts.cfg.Logger.Warn("previous writes summary not found; skipping comparison", zap.Error(err))
	}
	ts.cfg.Logger.Info("uploading new writes summary to s3 bucket to overwrite the previous")
	if err = aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		path.Join(ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsRawWritesCompareS3Dir, curTS),
		ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsRawWritesJSONPath,
	); err != nil {
		return err
	}
	if err = aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		path.Join(ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWritesCompareS3Dir, curTS),
		ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWritesJSONPath,
	); err != nil {
		return err
	}

	return nil
}

func (ts *tester) publishResults() (err error) {
	tv := aws.Time(time.Now().UTC())
	datums := make([]*cloudwatch.MetricDatum, 0)
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-configmaps-local-writes-latency-p50"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWrites.LantencyP50.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-configmaps-local-writes-latency-p90"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWrites.LantencyP90.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		MetricName: aws.String("add-on-configmaps-local-writes-latency-p99"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWrites.LantencyP99.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-configmaps-local-writes-latency-p999"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWrites.LantencyP999.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-configmaps-local-writes-latency-p9999"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnConfigmapsLocal.RequestsSummaryWrites.LantencyP9999.Milliseconds())),
	})
	return cw.PutData(ts.cfg.Logger, ts.cfg.CWAPI, ts.cfg.EKSConfig.CWNamespace, 20, datums...)
}
