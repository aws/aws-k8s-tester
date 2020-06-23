// Package cw implements common CloudWatch utilities.
package cw

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// GetMetricsImage downloads metrics image from CloudWatch.
// ref. https://docs.aws.amazon.com/sdk-for-go/api/service/cloudwatch/#GetMetricWidgetImageInput
// ref. https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/CloudWatch-Metric-Widget-Structure.html
func GetMetricsImage(lg *zap.Logger, cwAPI cloudwatchiface.CloudWatchAPI, widgetQueryJSON string, outputPath string) (err error) {
	_, err = json.Marshal(widgetQueryJSON)
	if err != nil {
		lg.Warn("invalid json format for widget", zap.Error(err))
		return err
	}
	if filepath.Ext(outputPath) != ".png" {
		return fmt.Errorf("%q is not png", outputPath)
	}

	lg.Info("fetching metrics image")
	var cout *cloudwatch.GetMetricWidgetImageOutput
	cout, err = cwAPI.GetMetricWidgetImage(&cloudwatch.GetMetricWidgetImageInput{
		OutputFormat: aws.String("png"),
		MetricWidget: aws.String(widgetQueryJSON),
	})
	if err != nil {
		lg.Warn("failed to fetch metrics image", zap.Error(err))
		return err
	}

	if err = ioutil.WriteFile(outputPath, cout.MetricWidgetImage, 0777); err != nil {
		lg.Warn("failed to write", zap.Error(err))
		return err
	}

	lg.Info("saved metrics image from cw response",
		zap.String("size", humanize.Bytes(uint64(len(cout.MetricWidgetImage)))),
		zap.String("output-path", outputPath),
	)
	return nil
}

// PutData publishes the list of cloudwatch datums in a batch.
func PutData(lg *zap.Logger, cwAPI cloudwatchiface.CloudWatchAPI, namespace string, batchSize int, datums ...*cloudwatch.MetricDatum) (err error) {
	lg.Info("publishing datums",
		zap.String("namespace", namespace),
		zap.Int("batch-size", batchSize),
		zap.Int("datums", len(datums)),
	)
	if len(datums) == 0 {
		return nil
	}
	batch := make([]*cloudwatch.MetricDatum, 0, batchSize)
	for _, cur := range datums {
		batch = append(batch, cur)
		if len(batch) == batchSize {
			lg.Info("sending batch", zap.Int("current-batch", len(batch)))
			req, _ := cwAPI.PutMetricDataRequest(&cloudwatch.PutMetricDataInput{
				Namespace:  aws.String(namespace),
				MetricData: batch,
			})
			if req == nil {
				lg.Info("failed PutMetricDataRequest")
				return errors.New("failed PutMetricDataRequest")
			}
			req.Handlers.Build.PushBack(newCompressPayloadFunc(lg))
			err = req.Send()
			if err == nil {
				lg.Info("sent batch", zap.Int("current-batch", len(batch)))
			} else {
				lg.Warn("failed to send batch", zap.Error(err))
			}

			time.Sleep(500 * time.Millisecond)
		}
	}

	lg.Info("sending last batch", zap.Int("last-batch", len(batch)))
	req, _ := cwAPI.PutMetricDataRequest(&cloudwatch.PutMetricDataInput{
		Namespace:  aws.String(namespace),
		MetricData: batch,
	})
	if req == nil {
		lg.Info("failed PutMetricDataRequest")
		return errors.New("failed PutMetricDataRequest")
	}
	req.Handlers.Build.PushBack(newCompressPayloadFunc(lg))
	err = req.Send()
	if err == nil {
		lg.Info("sent batch", zap.Int("last-batch", len(batch)))
	} else {
		lg.Warn("failed to send batch", zap.Error(err))
	}
	return err
}

func newCompressPayloadFunc(lg *zap.Logger) func(r *request.Request) {
	compressPayload := func(r *request.Request) {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		if _, err := io.Copy(zw, r.GetBody()); err != nil {
			lg.Warn("failed to copy", zap.Error(err))
			return
		}
		if err := zw.Close(); err != nil {
			lg.Warn("failed to close", zap.Error(err))
			return
		}
		r.SetBufferBody(buf.Bytes())
		r.HTTPRequest.Header.Set("Content-Encoding", "gzip")
	}
	return compressPayload
}
