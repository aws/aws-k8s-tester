// Package configmaps implements tester for ConfigMap.
package configmaps

import (
	"context"
	"fmt"
	"sync"
	"time"

	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/metrics"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
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

	Client        k8s_client.EKS
	ClientTimeout time.Duration

	Namespace  string
	Objects    int
	ObjectSize int
}

// Loader defines configmap loader operations.
type Loader interface {
	Start()
	Stop()
	GetMetrics() (writes metrics.RequestsSummary, err error)
}

type loader struct {
	cfg            Config
	donec          chan struct{}
	donecCloseOnce *sync.Once
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
	startWrites(ld.cfg.Logger, ld.cfg.Client.KubernetesClientSet(), ld.cfg.ClientTimeout, ld.cfg.Namespace, ld.cfg.Objects, ld.cfg.ObjectSize, ld.cfg.Stopc, ld.donec)
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
func (ld *loader) GetMetrics() (writes metrics.RequestsSummary, err error) {
	// https://pkg.go.dev/github.com/prometheus/client_golang/prometheus?tab=doc#Gatherer
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		ld.cfg.Logger.Warn("failed to gather prometheus metrics", zap.Error(err))
		return metrics.RequestsSummary{}, err
	}
	for _, mf := range mfs {
		if mf == nil {
			continue
		}
		switch *mf.Name {
		case "configmaps_client_write_requests_success_total":
			gg := mf.Metric[0].GetGauge()
			writes.SuccessTotal = gg.GetValue()
		case "configmaps_client_write_requests_failure_total":
			gg := mf.Metric[0].GetGauge()
			writes.FailureTotal = gg.GetValue()
		case "configmaps_client_write_request_latency_milliseconds":
			writes.LatencyHistogram, err = metrics.ParseHistogram("milliseconds", mf.Metric[0].GetHistogram())
			if err != nil {
				return metrics.RequestsSummary{}, err
			}
		}
	}
	return writes, nil
}

func startWrites(lg *zap.Logger, cli *kubernetes.Clientset, timeout time.Duration, namespace string, objects int, objectSize int, stopc chan struct{}, donec chan struct{}) {
	lg.Info("starting startWrites", zap.Int("objects", objects), zap.Int("object-size", objectSize))

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
}
