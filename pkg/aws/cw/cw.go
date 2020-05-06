// Package cw implements common CloudWatch utilities.
package cw

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
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
