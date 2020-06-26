// Package local implements tester for CertificateSigningRequest.
package local

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eks/csrs"
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
)

// Config defines csrs local tester configuration.
type Config struct {
	Logger    *zap.Logger
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
	if !ts.cfg.EKSConfig.IsEnabledAddOnCSRsLocal() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnCSRsLocal.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnCSRsLocal.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnCSRsLocal.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	loader := csrs.New(csrs.Config{
		Logger:                          ts.cfg.Logger,
		Stopc:                           ts.cfg.Stopc,
		S3API:                           ts.cfg.S3API,
		S3BucketName:                    ts.cfg.EKSConfig.S3BucketName,
		Client:                          ts.cfg.K8SClient,
		ClientTimeout:                   ts.cfg.EKSConfig.ClientTimeout,
		Objects:                         ts.cfg.EKSConfig.AddOnCSRsLocal.Objects,
		InitialRequestConditionType:     ts.cfg.EKSConfig.AddOnCSRsLocal.InitialRequestConditionType,
		RequestsRawWritesJSONPath:       ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsRawWritesJSONPath,
		RequestsRawWritesJSONS3Key:      ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsRawWritesJSONS3Key,
		RequestsSummaryWritesJSONPath:   ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWritesJSONPath,
		RequestsSummaryWritesJSONS3Key:  ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWritesJSONS3Key,
		RequestsSummaryWritesTablePath:  ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWritesTablePath,
		RequestsSummaryWritesTableS3Key: ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWritesTableS3Key,
	})
	loader.Start()
	loader.Stop()

	ts.cfg.Logger.Info("completing csrs local tester")
	var curWriteLatencies metrics.Durations
	curWriteLatencies, ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWrites, err = loader.CollectMetrics()
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
	if !ts.cfg.EKSConfig.IsEnabledAddOnCSRsLocal() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnCSRsLocal.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnCSRsLocal.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnCSRsLocal.Created = false
	return ts.cfg.EKSConfig.Sync()
}

// 1. if previous summary exists, download and compare
// 2. upload new summary and overwrite the previous s3 key
func (ts *tester) checkResults(curWriteLatencies metrics.Durations) (err error) {
	curTS := time.Now().UTC().Format(time.RFC3339Nano)
	ts.cfg.Logger.Info("checking results", zap.String("timestamp", curTS))

	s3Objects := make([]*s3.Object, 0)
	if ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWritesCompareS3Dir != "" {
		s3Objects, err = aws_s3.ListInDescendingLastModified(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			path.Clean(ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWritesCompareS3Dir)+"/",
		)
	}
	canCompare := len(s3Objects) > 0 && err == nil
	if canCompare {
		reqSummaryS3Key := aws.StringValue(s3Objects[0].Key)
		durRawS3Key := path.Join(ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsRawWritesCompareS3Dir, path.Base(reqSummaryS3Key))

		var prevSummary metrics.RequestsSummary
		prevSummary, err = metrics.DownloadRequestsSummaryFromS3(ts.cfg.Logger, ts.cfg.S3API, ts.cfg.EKSConfig.S3BucketName, reqSummaryS3Key)
		if err != nil {
			ts.cfg.Logger.Warn("failed to download results", zap.Error(err))
			return err
		}
		ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWritesCompare, err = metrics.CompareRequestsSummary(prevSummary, ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWrites)
		if err != nil {
			ts.cfg.Logger.Warn("failed to compare results", zap.Error(err))
			return err
		}
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWritesCompareJSONPath, []byte(ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWritesCompare.JSON()), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWritesCompareJSONS3Key,
			ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWritesCompareJSONPath,
		); err != nil {
			return err
		}
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWritesCompareTablePath, []byte(ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWritesCompare.Table()), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWritesCompareTableS3Key,
			ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWritesCompareTablePath,
		); err != nil {
			return err
		}
		fmt.Printf("\n\nRequestsSummaryWritesCompare:\n%s\n", ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWritesCompare.Table())

		var prevDurations metrics.Durations
		prevDurations, err = metrics.DownloadDurationsFromS3(ts.cfg.Logger, ts.cfg.S3API, ts.cfg.EKSConfig.S3BucketName, durRawS3Key)
		if err != nil {
			ts.cfg.Logger.Warn("failed to download results", zap.Error(err))
			return err
		}
		prevDurationsWithLabels := metrics.LabelDurations(prevDurations, prevSummary.TestID)
		curDurationsWithLabels := metrics.LabelDurations(curWriteLatencies, ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWrites.TestID)
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
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsRawWritesCompareAllJSONPath, []byte(allDataJSON), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsRawWritesCompareAllJSONS3Key,
			ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsRawWritesCompareAllJSONPath,
		); err != nil {
			return err
		}
		if err = allDurationsWithLabels.CSV(ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsRawWritesCompareAllCSVPath); err != nil {
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsRawWritesCompareAllCSVS3Key,
			ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsRawWritesCompareAllCSVPath,
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
		path.Join(ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsRawWritesCompareS3Dir, curTS),
		ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsRawWritesJSONPath,
	); err != nil {
		return err
	}
	if err = aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		path.Join(ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWritesCompareS3Dir, curTS),
		ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWritesJSONPath,
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
		MetricName: aws.String("add-on-csrs-local-writes-latency-p50"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWrites.LantencyP50.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-csrs-local-writes-latency-p90"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWrites.LantencyP90.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-csrs-local-writes-latency-p99"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWrites.LantencyP99.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-csrs-local-writes-latency-p999"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWrites.LantencyP999.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-csrs-local-writes-latency-p9999"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsSummaryWrites.LantencyP9999.Milliseconds())),
	})
	return cw.PutData(ts.cfg.Logger, ts.cfg.CWAPI, ts.cfg.EKSConfig.CWNamespace, 20, datums...)
}
