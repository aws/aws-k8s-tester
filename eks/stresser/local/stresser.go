// Package local implements cluster local load tests.
// ref. https://github.com/kubernetes/perf-tests
package local

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eks/stresser"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Config defines stresser configuration.
// ref. https://github.com/kubernetes/perf-tests
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	S3API     s3iface.S3API
	CWAPI     cloudwatchiface.CloudWatchAPI
}

// TODO: use kubemark
// nodelease.NewController, kubemark.GetHollowKubeletConfig

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
	if !ts.cfg.EKSConfig.IsEnabledAddOnStresserLocal() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnStresserLocal.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnStresserLocal.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnStresserLocal.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnStresserLocal.Namespace,
	); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	nss, err := ts.cfg.K8SClient.KubernetesClientSet().CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	cancel()
	if err != nil {
		ts.cfg.Logger.Warn("list namespaces failed", zap.Error(err))
		return err
	}
	ns := make([]string, 0, len(nss.Items))
	for _, nv := range nss.Items {
		ns = append(ns, nv.GetName())
	}

	loader := stresser.New(stresser.Config{
		Logger:                          ts.cfg.Logger,
		Stopc:                           ts.cfg.Stopc,
		S3API:                           ts.cfg.S3API,
		S3BucketName:                    ts.cfg.EKSConfig.S3BucketName,
		Client:                          ts.cfg.K8SClient,
		ClientTimeout:                   ts.cfg.EKSConfig.ClientTimeout,
		Deadline:                        time.Now().Add(ts.cfg.EKSConfig.AddOnStresserLocal.Duration),
		NamespaceWrite:                  ts.cfg.EKSConfig.AddOnStresserLocal.Namespace,
		NamespacesRead:                  ns,
		ObjectSize:                      ts.cfg.EKSConfig.AddOnStresserLocal.ObjectSize,
		ListLimit:                       ts.cfg.EKSConfig.AddOnStresserLocal.ListLimit,
		RequestsRawWritesJSONPath:       ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawWritesJSONPath,
		RequestsRawWritesJSONS3Key:      ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawWritesJSONS3Key,
		RequestsSummaryWritesJSONPath:   ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWritesJSONPath,
		RequestsSummaryWritesJSONS3Key:  ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWritesJSONS3Key,
		RequestsSummaryWritesTablePath:  ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWritesTablePath,
		RequestsSummaryWritesTableS3Key: ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWritesTableS3Key,
		RequestsRawReadsJSONPath:        ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawReadsJSONPath,
		RequestsRawReadsJSONS3Key:       ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawReadsJSONS3Key,
		RequestsSummaryReadsJSONPath:    ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReadsJSONPath,
		RequestsSummaryReadsJSONS3Key:   ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReadsJSONS3Key,
		RequestsSummaryReadsTablePath:   ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReadsTablePath,
		RequestsSummaryReadsTableS3Key:  ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReadsTableS3Key,
	})
	loader.Start()

	var curWriteLatencies metrics.Durations
	var curReadLatencies metrics.Durations
	select {
	case <-ts.cfg.Stopc:
		ts.cfg.Logger.Warn("cluster stresser aborted")
		loader.Stop()
		curWriteLatencies, ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWrites, curReadLatencies, ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReads, err = loader.CollectMetrics()
		ts.cfg.EKSConfig.Sync()
		if err != nil {
			ts.cfg.Logger.Warn("failed to get metrics", zap.Error(err))
		}
		return err

	case <-time.After(ts.cfg.EKSConfig.AddOnStresserLocal.Duration):
		ts.cfg.Logger.Info("completing load testing", zap.Duration("duration", ts.cfg.EKSConfig.AddOnStresserLocal.Duration))
		loader.Stop()
		curWriteLatencies, ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWrites, curReadLatencies, ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReads, err = loader.CollectMetrics()
		ts.cfg.EKSConfig.Sync()
		if err != nil {
			ts.cfg.Logger.Warn("failed to get metrics", zap.Error(err))
			return err
		}

		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("cluster stresser aborted")
			return nil
		case <-time.After(30 * time.Second):
		}
	}

	if err = ts.compareResults(curWriteLatencies, curReadLatencies); err != nil {
		return err
	}
	if err = ts.publishResults(); err != nil {
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
	ts.cfg.EKSConfig.Sync()
	if err == nil {
		ts.cfg.Logger.Info("health check success after load testing")
	} else {
		ts.cfg.Logger.Warn("health check failed after load testing", zap.Error(err))
	}
	return err
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnStresserLocal() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnStresserLocal.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnStresserLocal.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnStresserLocal.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithForceDelete(true),
	); err != nil {
		return fmt.Errorf("failed to delete stresser namespace (%v)", err)
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnStresserLocal.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) AggregateResults() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnStresserLocal() {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnStresserLocal.Created {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.AggregateResults", zap.String("tester", pkgName))
	return nil
}

// 1. if previous summary exists, download and compare
// 2. upload new summary and overwrite the previous s3 key
func (ts *tester) compareResults(curWriteLatencies metrics.Durations, curReadLatencies metrics.Durations) (err error) {
	tss := time.Now().UTC().Format(time.RFC3339Nano)
	ts.cfg.Logger.Info("comparing results", zap.String("timestamp", tss))

	s3Objects := make([]*s3.Object, 0)
	if ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWritesCompareS3Dir != "" {
		s3Objects, err = aws_s3.ListInDescendingLastModified(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			path.Clean(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWritesCompareS3Dir)+"/",
		)
	}
	if len(s3Objects) > 0 && err == nil {
		reqSummaryS3Key := aws.StringValue(s3Objects[0].Key)
		durRawS3Key := path.Join(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawWritesCompareS3Dir, path.Base(reqSummaryS3Key))

		var prevSummary metrics.RequestsSummary
		prevSummary, err = metrics.DownloadRequestsSummaryFromS3(ts.cfg.Logger, ts.cfg.S3API, ts.cfg.EKSConfig.S3BucketName, reqSummaryS3Key)
		if err != nil {
			ts.cfg.Logger.Warn("failed to download results", zap.Error(err))
			return err
		}
		ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWritesCompare, err = metrics.CompareRequestsSummary(prevSummary, ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWrites)
		if err != nil {
			ts.cfg.Logger.Warn("failed to compare results", zap.Error(err))
			return err
		}
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWritesCompareJSONPath, []byte(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWritesCompare.JSON()), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWritesCompareJSONS3Key,
			ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWritesCompareJSONPath,
		); err != nil {
			return err
		}
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWritesCompareTablePath, []byte(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWritesCompare.Table()), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWritesCompareTableS3Key,
			ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWritesCompareTablePath,
		); err != nil {
			return err
		}
		fmt.Printf("\n\nRequestsSummaryWritesCompare:\n%s\n", ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWritesCompare.Table())

		var prevDurations metrics.Durations
		prevDurations, err = metrics.DownloadDurationsFromS3(ts.cfg.Logger, ts.cfg.S3API, ts.cfg.EKSConfig.S3BucketName, durRawS3Key)
		if err != nil {
			ts.cfg.Logger.Warn("failed to download results", zap.Error(err))
			return err
		}
		prevDurationsWithLabels := metrics.LabelDurations(prevDurations, prevSummary.TestID)
		curDurationsWithLabels := metrics.LabelDurations(curWriteLatencies, ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWrites.TestID)
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
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawWritesCompareAllJSONPath, []byte(allDataJSON), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawWritesCompareAllJSONS3Key,
			ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawWritesCompareAllJSONPath,
		); err != nil {
			return err
		}
		if err = allDurationsWithLabels.CSV(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawWritesCompareAllCSVPath); err != nil {
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawWritesCompareAllCSVS3Key,
			ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawWritesCompareAllCSVPath,
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
		path.Join(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawWritesCompareS3Dir, tss),
		ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawWritesJSONPath,
	); err != nil {
		return err
	}
	if err = aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		path.Join(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWritesCompareS3Dir, tss),
		ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWritesJSONPath,
	); err != nil {
		return err
	}

	s3Objects = make([]*s3.Object, 0)
	if ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReadsCompareS3Dir != "" {
		s3Objects, err = aws_s3.ListInDescendingLastModified(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			path.Clean(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReadsCompareS3Dir)+"/",
		)
	}
	if len(s3Objects) > 0 && err == nil {
		reqSummaryS3Key := aws.StringValue(s3Objects[0].Key)
		durRawS3Key := path.Join(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawReadsCompareS3Dir, path.Base(reqSummaryS3Key))

		var prevSummary metrics.RequestsSummary
		prevSummary, err = metrics.DownloadRequestsSummaryFromS3(ts.cfg.Logger, ts.cfg.S3API, ts.cfg.EKSConfig.S3BucketName, reqSummaryS3Key)
		if err != nil {
			ts.cfg.Logger.Warn("failed to download results", zap.Error(err))
			return err
		}
		ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReadsCompare, err = metrics.CompareRequestsSummary(prevSummary, ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReads)
		if err != nil {
			ts.cfg.Logger.Warn("failed to compare results", zap.Error(err))
			return err
		}
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReadsCompareJSONPath, []byte(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReadsCompare.JSON()), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReadsCompareJSONS3Key,
			ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReadsCompareJSONPath,
		); err != nil {
			return err
		}
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReadsCompareTablePath, []byte(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReadsCompare.Table()), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReadsCompareTableS3Key,
			ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReadsCompareTablePath,
		); err != nil {
			return err
		}
		fmt.Printf("\n\nRequestsSummaryReadsCompare:\n%s\n", ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReadsCompare.Table())

		var prevDurations metrics.Durations
		prevDurations, err = metrics.DownloadDurationsFromS3(ts.cfg.Logger, ts.cfg.S3API, ts.cfg.EKSConfig.S3BucketName, durRawS3Key)
		if err != nil {
			ts.cfg.Logger.Warn("failed to download results", zap.Error(err))
			return err
		}
		prevDurationsWithLabels := metrics.LabelDurations(prevDurations, prevSummary.TestID)
		curDurationsWithLabels := metrics.LabelDurations(curWriteLatencies, ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReads.TestID)
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
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawReadsCompareAllJSONPath, []byte(allDataJSON), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawReadsCompareAllJSONS3Key,
			ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawReadsCompareAllJSONPath,
		); err != nil {
			return err
		}
		if err = allDurationsWithLabels.CSV(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawReadsCompareAllCSVPath); err != nil {
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawReadsCompareAllCSVS3Key,
			ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawReadsCompareAllCSVPath,
		); err != nil {
			return err
		}
	} else {
		ts.cfg.Logger.Warn("previous writes summary not found; skipping comparison", zap.Error(err))
	}
	ts.cfg.Logger.Info("uploading new reads summary to s3 bucket to overwrite the previous")
	if err = aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		path.Join(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawReadsCompareS3Dir, tss),
		ts.cfg.EKSConfig.AddOnStresserLocal.RequestsRawReadsJSONPath,
	); err != nil {
		return err
	}
	if err = aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		path.Join(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReadsCompareS3Dir, tss),
		ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReadsJSONPath,
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
		MetricName: aws.String("add-on-stresser-local-writes-latency-p50"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWrites.LantencyP50.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-stresser-local-writes-latency-p90"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWrites.LantencyP90.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		MetricName: aws.String("add-on-stresser-local-writes-latency-p99"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWrites.LantencyP99.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-stresser-local-writes-latency-p999"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWrites.LantencyP999.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		MetricName: aws.String("add-on-stresser-local-writes-latency-p9999"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryWrites.LantencyP9999.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-stresser-local-reads-latency-p50"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReads.LantencyP50.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-stresser-local-reads-latency-p90"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReads.LantencyP90.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-stresser-local-reads-latency-p99"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReads.LantencyP99.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-stresser-local-reads-latency-p999"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReads.LantencyP999.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-stresser-local-reads-latency-p9999"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnStresserLocal.RequestsSummaryReads.LantencyP9999.Milliseconds())),
	})
	return cw.PutData(ts.cfg.Logger, ts.cfg.CWAPI, ts.cfg.EKSConfig.CWNamespace, 20, datums...)
}
