// Package local implements Secrets plugin.
package local

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eks/secrets"
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

// Config defines secrets local tester configuration.
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
	if !ts.cfg.EKSConfig.IsEnabledAddOnSecretsLocal() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnSecretsLocal.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnSecretsLocal.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnSecretsLocal.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnSecretsLocal.Namespace,
	); err != nil {
		return err
	}

	loader := secrets.New(secrets.Config{
		Logger:                  ts.cfg.Logger,
		Stopc:                   ts.cfg.Stopc,
		S3API:                   ts.cfg.S3API,
		S3BucketName:            ts.cfg.EKSConfig.S3BucketName,
		Client:                  ts.cfg.K8SClient,
		ClientTimeout:           ts.cfg.EKSConfig.ClientTimeout,
		Namespace:               ts.cfg.EKSConfig.AddOnSecretsLocal.Namespace,
		NamePrefix:              ts.cfg.EKSConfig.AddOnSecretsLocal.NamePrefix,
		Objects:                 ts.cfg.EKSConfig.AddOnSecretsLocal.Objects,
		ObjectSize:              ts.cfg.EKSConfig.AddOnSecretsLocal.ObjectSize,
		WritesRawJSONPath:       ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesRawJSONPath,
		WritesRawJSONS3Key:      ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesRawJSONS3Key,
		WritesSummaryJSONPath:   ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesSummaryJSONPath,
		WritesSummaryJSONS3Key:  ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesSummaryJSONS3Key,
		WritesSummaryTablePath:  ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesSummaryTablePath,
		WritesSummaryTableS3Key: ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesSummaryTableS3Key,
		ReadsRawJSONPath:        ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsRawJSONPath,
		ReadsRawJSONS3Key:       ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsRawJSONS3Key,
		ReadsSummaryJSONPath:    ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsSummaryJSONPath,
		ReadsSummaryJSONS3Key:   ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsSummaryJSONS3Key,
		ReadsSummaryTablePath:   ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsSummaryTablePath,
		ReadsSummaryTableS3Key:  ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsSummaryTableS3Key,
	})
	loader.Start()
	loader.Stop()

	ts.cfg.Logger.Info("completing secrets local tester")
	ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesSummary, ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsSummary, err = loader.CollectMetrics()
	ts.cfg.EKSConfig.Sync()
	if err != nil {
		ts.cfg.Logger.Warn("failed to get metrics", zap.Error(err))
		return err
	}

	if err = ts.compareResults(); err != nil {
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
		ts.cfg.Logger.Info("health check success after secrets local tester")
	} else {
		ts.cfg.Logger.Warn("health check failed after secrets local tester", zap.Error(err))
	}
	return err
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnSecretsLocal() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnSecretsLocal.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnSecretsLocal.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnSecretsLocal.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithForceDelete(true),
	); err != nil {
		return fmt.Errorf("failed to delete secrets local tester namespace (%v)", err)
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnSecretsLocal.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) AggregateResults() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnSecretsLocal() {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnSecretsLocal.Created {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.AggregateResults", zap.String("tester", pkgName))
	return nil
}

// 1. if previous summary exists, download and compare
// 2. upload new summary and overwrite the previous s3 key
func (ts *tester) compareResults() (err error) {
	tss := time.Now().UTC().Format(time.RFC3339Nano)
	ts.cfg.Logger.Info("comparing results", zap.String("timestamp", tss))

	s3Objects := make([]*s3.Object, 0)
	if ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesCompareS3Dir != "" {
		s3Objects, err = aws_s3.ListInDescendingLastModified(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesCompareS3Dir,
		)
	}
	if len(s3Objects) > 0 && err == nil {
		var localPath string
		localPath, err = aws_s3.DownloadToTempFile(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			aws.StringValue(s3Objects[0].Key),
		)
		if err != nil {
			return fmt.Errorf("failed to download previous writes summary %v", err)
		}
		defer os.RemoveAll(localPath)
		rf, err := os.OpenFile(localPath, os.O_RDONLY, 0444)
		if err != nil {
			ts.cfg.Logger.Warn("failed to read a file", zap.Error(err))
			return err
		}
		defer rf.Close()
		var prev metrics.RequestsSummary
		if err = json.NewDecoder(rf).Decode(&prev); err != nil {
			ts.cfg.Logger.Warn("failed to decode a JSON file", zap.Error(err))
			return err
		}
		ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesCompare, err = metrics.CompareRequestsSummary(prev, ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesSummary)
		if err != nil {
			ts.cfg.Logger.Warn("failed to compare results", zap.Error(err))
			return err
		}
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesCompareJSONPath, []byte(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesCompare.JSON()), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			path.Join(ts.cfg.EKSConfig.Name, "add-on-secrets-local", "writes", filepath.Base(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesCompareJSONPath)),
			ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesCompareJSONPath,
		); err != nil {
			return err
		}
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesCompareTablePath, []byte(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesCompare.Table()), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			path.Join(ts.cfg.EKSConfig.Name, "add-on-secrets-local", "writes", filepath.Base(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesCompareTablePath)),
			ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesCompareTablePath,
		); err != nil {
			return err
		}
		fmt.Printf("\n\nRequestsWritesCompare:\n%s\n", ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesCompare.Table())
	} else {
		ts.cfg.Logger.Warn("previous writes summary not found; skipping comparison", zap.Error(err))
	}
	ts.cfg.Logger.Info("uploading new writes summary to s3 bucket to overwrite the previous")
	if err = aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		path.Join(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesCompareS3Dir, tss),
		ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesSummaryJSONPath,
	); err != nil {
		return err
	}

	s3Objects = make([]*s3.Object, 0)
	if ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsCompareS3Dir != "" {
		s3Objects, err = aws_s3.ListInDescendingLastModified(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsCompareS3Dir,
		)
	}
	if len(s3Objects) > 0 && err == nil {
		var localPath string
		localPath, err = aws_s3.DownloadToTempFile(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			aws.StringValue(s3Objects[0].Key),
		)
		if err != nil {
			return fmt.Errorf("failed to download previous reads summary %v", err)
		}
		defer os.RemoveAll(localPath)
		rf, err := os.OpenFile(localPath, os.O_RDONLY, 0444)
		if err != nil {
			ts.cfg.Logger.Warn("failed to read a file", zap.Error(err))
			return err
		}
		defer rf.Close()
		var prev metrics.RequestsSummary
		if err = json.NewDecoder(rf).Decode(&prev); err != nil {
			ts.cfg.Logger.Warn("failed to decode a JSON file", zap.Error(err))
			return err
		}
		ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsCompare, err = metrics.CompareRequestsSummary(prev, ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsSummary)
		if err != nil {
			ts.cfg.Logger.Warn("failed to compare results", zap.Error(err))
			return err
		}
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsCompareJSONPath, []byte(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsCompare.JSON()), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsCompareJSONS3Key,
			ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsCompareJSONPath,
		); err != nil {
			return err
		}
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsCompareTablePath, []byte(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsCompare.Table()), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsCompareTableS3Key,
			ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsCompareTablePath,
		); err != nil {
			return err
		}
		fmt.Printf("\n\nRequestsReadsCompare:\n%s\n", ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsCompare.Table())
	} else {
		ts.cfg.Logger.Warn("previous writes summary not found; skipping comparison", zap.Error(err))
	}
	ts.cfg.Logger.Info("uploading new reads summary to s3 bucket to overwrite the previous")
	if err = aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		path.Join(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsCompareS3Dir, tss),
		ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsSummaryJSONPath,
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
		MetricName: aws.String("add-on-secrets-local-writes-latency-p50"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesSummary.LantencyP50.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-secrets-local-writes-latency-p90"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesSummary.LantencyP90.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-secrets-local-writes-latency-p99"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesSummary.LantencyP99.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-secrets-local-writes-latency-p999"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesSummary.LantencyP999.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-secrets-local-writes-latency-p9999"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsWritesSummary.LantencyP9999.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-secrets-local-reads-latency-p50"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsSummary.LantencyP50.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-secrets-local-reads-latency-p90"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsSummary.LantencyP90.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-secrets-local-reads-latency-p99"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsSummary.LantencyP99.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-secrets-local-reads-latency-p999"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsSummary.LantencyP999.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-secrets-local-reads-latency-p9999"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsReadsSummary.LantencyP9999.Milliseconds())),
	})
	return cw.PutData(ts.cfg.Logger, ts.cfg.CWAPI, ts.cfg.EKSConfig.CWNamespace, 20, datums...)
}
