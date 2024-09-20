package metric

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/aws/aws-k8s-tester/e2e2/internal/utils"
	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prometheus/prompb"
)

type MetricManager struct {
	Metadata              map[string]string
	AwsConfig             aws.Config
	AmpRemoteWriteURL     string
	AmpRemoteWriteRoleArn string
}

func NewMetricManager(metadata map[string]string, awsConfig aws.Config, url string, role string) *MetricManager {

	return &MetricManager{
		Metadata:              metadata,
		AwsConfig:             awsConfig,
		AmpRemoteWriteURL:     url,
		AmpRemoteWriteRoleArn: role,
	}
}

// PushMetricsToAMP pushes metric data to AWS Managed Prometheus (AMP) using SigV4 authentication
func (m *MetricManager) PushMetricsToAMP(name string, help string, value float64) error {
	metric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        name,
		Help:        help,
		ConstLabels: m.Metadata,
	})
	metric.Set(value)
	registry := prometheus.NewRegistry()
	registry.MustRegister(metric)

	mfs, err := registry.Gather()
	if err != nil {
		return fmt.Errorf("failed to gather metrics: %v", err)
	}
	// Collect Prometheus metrics
	metricsData, err := convertMetrics(mfs)
	if err != nil {
		return fmt.Errorf("failed to collect metrics: %v", err)
	}

	// Create an STS client
	stsClient := sts.NewFromConfig(m.AwsConfig)
	var creds aws.Credentials
	if m.AmpRemoteWriteRoleArn != "" {
		// Assume the specified role
		session := stscreds.NewAssumeRoleProvider(stsClient, m.AmpRemoteWriteRoleArn)
		creds, err = session.Retrieve(context.TODO())
	} else {
		// Use the default credentials
		creds, err = m.AwsConfig.Credentials.Retrieve(context.TODO())
	}
	if err != nil {
		return fmt.Errorf("failed to retrieve credentials: %w", err)
	}

	// Create a new HTTP request for the AMP remote_write API
	req, err := http.NewRequest(http.MethodPost, m.AmpRemoteWriteURL, metricsData)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set appropriate headers for the request
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

	// Create a SigV4 signer
	signer := v4.NewSigner()
	payloadHash, err := utils.CreatePayloadHash(req)
	if err != nil {
		return fmt.Errorf("failed to create payload hash: %w", err)
	}

	//extract region from remoteWriteURL
	region, err := extractRegionFromURL(m.AmpRemoteWriteURL)
	if err != nil {
		return fmt.Errorf("failed to extract region from URL: %w", err)
	}

	// Sign the HTTP request using the credentials from the assumed role
	err = signer.SignHTTP(context.TODO(), creds, req, payloadHash, "aps", region, time.Now())
	if err != nil {
		return fmt.Errorf("failed to sign the request: %w", err)
	}

	// Send the HTTP request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Read and print the response body
	res, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-OK response code: %d, response body: %s", resp.StatusCode, string(res))
	}

	return nil
}

// convertMetrics converts a slice of MetricFamily to snappy compressed Protobuf data
// Reference: https://github.com/prometheus/prometheus/blob/1c624c58ca934f618be737b4995e22051f5724c1/prompb/remote.pb.go#L64
// https://github.com/aws-observability/observability-best-practices/blob/main/sandbox/CWMetricStreamExporter/lambda/main.go#L264
func convertMetrics(mfs []*dto.MetricFamily) (*bytes.Reader, error) {
	var timeSeries []prompb.TimeSeries

	// Loop over each MetricFamily and convert it into Prometheus TimeSeries format
	for _, mf := range mfs {
		for _, metric := range mf.Metric {
			labels := []prompb.Label{
				{Name: "__name__", Value: mf.GetName()},
			}

			// Add ConstLabels to TimeSeries
			for _, v := range metric.GetLabel() {
				labels = append(labels, prompb.Label{
					Name:  v.GetName(),
					Value: v.GetValue(),
				})
			}
			ts := prompb.TimeSeries{
				Labels: labels,
				Samples: []prompb.Sample{
					{
						Value:     metric.GetGauge().GetValue(),
						Timestamp: time.Now().UnixMilli(), // Use the timestamp from the metric
					},
				},
			}
			timeSeries = append(timeSeries, ts)
		}
	}

	// Build WriteRequest with TimeSeries data
	writeRequest := &prompb.WriteRequest{
		Timeseries: timeSeries,
	}

	// Marshal WriteRequest to Protobuf
	data, err := proto.Marshal(writeRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal write request: %w", err)
	}

	// Snappy compress the Protobuf data
	compressed := snappy.Encode(nil, data)
	body := bytes.NewReader(compressed)

	return body, nil
}

func extractRegionFromURL(remoteWriteURL string) (string, error) {
	// Define the regular expression pattern to capture the region
	re := regexp.MustCompile(`https://aps-workspaces\.([a-z0-9-]+)\.amazonaws\.com`)

	// Find the region in the URL
	matches := re.FindStringSubmatch(remoteWriteURL)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not extract region from URL")
	}

	// The region will be in the first capture group
	region := matches[1]
	return region, nil
}
