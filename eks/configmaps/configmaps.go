// Package configmaps implements tester for ConfigMap.
package configmaps

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"sort"
	"sync"
	"time"

	aws_s3 "github.com/aws/aws-k8s-tester/pkg/aws/s3"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/metrics"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	writeRequestsSuccessTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "configmaps",
			Subsystem: "client",
			Name:      "write_requests_success_total",
			Help:      "Total number of successful write requests.",
		})
	writeRequestsFailureTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "configmaps",
			Subsystem: "client",
			Name:      "write_requests_failure_total",
			Help:      "Total number of successful write requests.",
		})
	writeRequestLatencyMs = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "configmaps",
			Subsystem: "client",
			Name:      "write_request_latency_milliseconds",
			Help:      "Bucketed histogram of client-side write request and response latency.",

			// lowest bucket start of upper bound 0.5 ms with factor 2
			// highest bucket start of 0.5 ms * 2^13 == 4.096 sec
			Buckets: prometheus.ExponentialBuckets(0.5, 2, 14),
		})
)

func init() {
	prometheus.MustRegister(writeRequestsSuccessTotal)
	prometheus.MustRegister(writeRequestsFailureTotal)
	prometheus.MustRegister(writeRequestLatencyMs)
}

// Config configures configmap loader.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}

	S3API        s3iface.S3API
	S3BucketName string
	S3DirName    string

	Client        k8s_client.EKS
	ClientTimeout time.Duration

	Namespace  string
	Objects    int
	ObjectSize int

	WritesJSONPath         string
	WritesSummaryJSONPath  string
	WritesSummaryTablePath string
}

// Loader defines configmap loader operations.
type Loader interface {
	Start()
	Stop()
	CollectMetrics() (writesSummary metrics.RequestsSummary, err error)
}

type loader struct {
	cfg            Config
	donec          chan struct{}
	donecCloseOnce *sync.Once

	writeLatencies metrics.Durations
}

func New(cfg Config) Loader {
	return &loader{
		cfg:            cfg,
		donec:          make(chan struct{}),
		donecCloseOnce: new(sync.Once),
	}
}

func (ld *loader) Start() {
	ld.cfg.Logger.Info("starting write function", zap.String("namespace-write", ld.cfg.Namespace))
	ld.writeLatencies = startWrites(ld.cfg.Logger, ld.cfg.Client.KubernetesClientSet(), ld.cfg.ClientTimeout, ld.cfg.Namespace, ld.cfg.Objects, ld.cfg.ObjectSize, ld.cfg.Stopc, ld.donec)
	ld.cfg.Logger.Info("completed write function", zap.String("namespace-write", ld.cfg.Namespace))
}

func (ld *loader) Stop() {
	ld.cfg.Logger.Info("stopping and waiting for write function")
	ld.donecCloseOnce.Do(func() {
		close(ld.donec)
	})
	ld.cfg.Logger.Info("stopped and waited for write function")
}

// GetMetrics locally fetches output from registered metrics.
// ref. https://pkg.go.dev/github.com/prometheus/client_golang@v1.6.0/prometheus/promhttp?tab=doc#Handler
func (ts *loader) CollectMetrics() (writesSummary metrics.RequestsSummary, err error) {
	writesSummary = metrics.RequestsSummary{TestID: time.Now().UTC().Format(time.RFC3339Nano)}

	// https://pkg.go.dev/github.com/prometheus/client_golang/prometheus?tab=doc#Gatherer
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		ts.cfg.Logger.Warn("failed to gather prometheus metrics", zap.Error(err))
		return metrics.RequestsSummary{}, err
	}
	for _, mf := range mfs {
		if mf == nil {
			continue
		}
		switch *mf.Name {
		case "configmaps_client_write_requests_success_total":
			gg := mf.Metric[0].GetGauge()
			writesSummary.SuccessTotal = gg.GetValue()
		case "configmaps_client_write_requests_failure_total":
			gg := mf.Metric[0].GetGauge()
			writesSummary.FailureTotal = gg.GetValue()
		case "configmaps_client_write_request_latency_milliseconds":
			writesSummary.LatencyHistogram, err = metrics.ParseHistogram("milliseconds", mf.Metric[0].GetHistogram())
			if err != nil {
				return metrics.RequestsSummary{}, err
			}
		}
	}

	ts.cfg.Logger.Info("sorting write latency results", zap.Int("total-data-points", ts.writeLatencies.Len()))
	now := time.Now()
	sort.Sort(ts.writeLatencies)
	ts.cfg.Logger.Info("sorted write latency results", zap.Int("total-data-points", ts.writeLatencies.Len()), zap.String("took", time.Since(now).String()))
	writesSummary.LantencyP50 = ts.writeLatencies.PickLantencyP50()
	writesSummary.LantencyP90 = ts.writeLatencies.PickLantencyP90()
	writesSummary.LantencyP99 = ts.writeLatencies.PickLantencyP99()
	writesSummary.LantencyP999 = ts.writeLatencies.PickLantencyP999()
	writesSummary.LantencyP9999 = ts.writeLatencies.PickLantencyP9999()

	ts.cfg.Logger.Info("writing latency results in JSON to disk", zap.String("path", ts.cfg.WritesJSONPath))
	wb, err := json.Marshal(ts.writeLatencies)
	if err != nil {
		ts.cfg.Logger.Warn("failed to encode latency results in JSON", zap.Error(err))
		return metrics.RequestsSummary{}, err
	}
	if err = ioutil.WriteFile(ts.cfg.WritesJSONPath, wb, 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write latency results in JSON to disk", zap.String("path", ts.cfg.WritesJSONPath), zap.Error(err))
		return metrics.RequestsSummary{}, err
	}
	if err = aws_s3.Upload(ts.cfg.Logger, ts.cfg.S3API, ts.cfg.S3BucketName, path.Join(ts.cfg.S3DirName, "writes", filepath.Base(ts.cfg.WritesJSONPath)), ts.cfg.WritesJSONPath); err != nil {
		return metrics.RequestsSummary{}, err
	}

	if err = ioutil.WriteFile(ts.cfg.WritesSummaryJSONPath, []byte(writesSummary.JSON()), 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return metrics.RequestsSummary{}, err
	}
	if err = aws_s3.Upload(ts.cfg.Logger, ts.cfg.S3API, ts.cfg.S3BucketName, path.Join(ts.cfg.S3DirName, "writes", filepath.Base(ts.cfg.WritesSummaryJSONPath)), ts.cfg.WritesSummaryJSONPath); err != nil {
		return metrics.RequestsSummary{}, err
	}
	if err = ioutil.WriteFile(ts.cfg.WritesSummaryTablePath, []byte(writesSummary.Table()), 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return metrics.RequestsSummary{}, err
	}
	if err = aws_s3.Upload(ts.cfg.Logger, ts.cfg.S3API, ts.cfg.S3BucketName, path.Join(ts.cfg.S3DirName, "writes", filepath.Base(ts.cfg.WritesSummaryTablePath)), ts.cfg.WritesSummaryTablePath); err != nil {
		return metrics.RequestsSummary{}, err
	}
	fmt.Printf("\n\nWritesSummaryTable:\n%s\n", writesSummary.Table())

	return writesSummary, nil
}

func startWrites(lg *zap.Logger, cli *kubernetes.Clientset, timeout time.Duration, namespace string, objects int, objectSize int, stopc chan struct{}, donec chan struct{}) (ds metrics.Durations) {
	lg.Info("starting startWrites", zap.Int("objects", objects), zap.Int("object-size", objectSize))
	ds = make(metrics.Durations, 0, 20000)

	val := randutil.String(objectSize)
	for i := 0; i < objects; i++ {
		select {
		case <-stopc:
			lg.Warn("writes stopped")
			return
		case <-donec:
			lg.Info("writes done")
			return
		default:
		}

		key := fmt.Sprintf("configmap%d%s", i, randutil.String(7))

		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		_, err := cli.
			CoreV1().
			ConfigMaps(namespace).
			Create(ctx, &v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      key,
					Namespace: namespace,
					Labels: map[string]string{
						"name": key,
					},
				},
				Data: map[string]string{key: val},
			}, metav1.CreateOptions{})
		cancel()
		took := time.Since(start)
		tookMS := float64(took / time.Millisecond)
		writeRequestLatencyMs.Observe(tookMS)
		ds = append(ds, took)
		if err != nil {
			writeRequestsFailureTotal.Inc()
			lg.Warn("write configmap failed", zap.String("namespace", namespace), zap.Error(err))
		} else {
			writeRequestsSuccessTotal.Inc()
			if i%20 == 0 {
				lg.Info("wrote configmap", zap.Int("iteration", i), zap.String("namespace", namespace))
			}
		}
	}
	return ds
}
