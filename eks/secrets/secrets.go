// Package secrets implements Secrets plugin.
package secrets

import (
	"context"
	"fmt"
	"sort"
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
			Namespace: "secrets",
			Subsystem: "client",
			Name:      "write_requests_success_total",
			Help:      "Total number of successful write requests.",
		})
	writeRequestsFailureTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "secrets",
			Subsystem: "client",
			Name:      "write_requests_failure_total",
			Help:      "Total number of successful write requests.",
		})
	writeRequestLatencyMs = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "secrets",
			Subsystem: "client",
			Name:      "write_request_latency_milliseconds",
			Help:      "Bucketed histogram of client-side write request and response latency.",

			// lowest bucket start of upper bound 0.5 ms with factor 2
			// highest bucket start of 0.5 ms * 2^13 == 4.096 sec
			Buckets: prometheus.ExponentialBuckets(0.5, 2, 14),
		})

	readRequestsSuccessTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "secrets",
			Subsystem: "client",
			Name:      "read_requests_success_total",
			Help:      "Total number of successful read requests.",
		})
	readRequestsFailureTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "secrets",
			Subsystem: "client",
			Name:      "read_requests_failure_total",
			Help:      "Total number of successful read requests.",
		})
	readRequestLatencyMs = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "secrets",
			Subsystem: "client",
			Name:      "read_request_latency_milliseconds",
			Help:      "Bucketed histogram of client-side read request and response latency.",

			// lowest bucket start of upper bound 0.5 ms with factor 2
			// highest bucket start of 0.5 ms * 2^13 == 4.096 sec
			Buckets: prometheus.ExponentialBuckets(0.5, 2, 14),
		})
)

func init() {
	prometheus.MustRegister(writeRequestsSuccessTotal)
	prometheus.MustRegister(writeRequestsFailureTotal)
	prometheus.MustRegister(writeRequestLatencyMs)
	prometheus.MustRegister(readRequestsSuccessTotal)
	prometheus.MustRegister(readRequestsFailureTotal)
	prometheus.MustRegister(readRequestLatencyMs)
}

// Config configures Secret loader.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}

	Client        k8s_client.EKS
	ClientTimeout time.Duration

	Namespace string

	// NamePrefix is the prefix of Secret name.
	// If multiple Secret loader is running,
	// this must be unique per worker to avoid name conflicts.
	NamePrefix string

	Objects    int
	ObjectSize int
}

// Loader defines Secret loader operations.
type Loader interface {
	Start()
	Stop()
	GetMetrics() (writes metrics.RequestsSummary, reads metrics.RequestsSummary, err error)
}

type loader struct {
	cfg            Config
	donec          chan struct{}
	donecCloseOnce *sync.Once

	writeLatencies metrics.Durations
	readLatencies  metrics.Durations
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
	var created []string
	ld.writeLatencies, created = startWrites(ld.cfg.Logger, ld.cfg.Client.KubernetesClientSet(), ld.cfg.ClientTimeout, ld.cfg.Namespace, ld.cfg.NamePrefix, ld.cfg.Objects, ld.cfg.ObjectSize, ld.cfg.Stopc, ld.donec)
	ld.cfg.Logger.Info("completed write function", zap.String("namespace-write", ld.cfg.Namespace))

	// TODO: create Pod with created secrets mounted as volume, read them, measure latency
	// as implemented in aws-k8s-tester <= v1.2.1
	// ref. https://github.com/aws/aws-k8s-tester/blob/v1.2.1/eks/secrets/secrets.go#L404-L514

	ld.cfg.Logger.Info("starting read function", zap.String("namespace-read", ld.cfg.Namespace))
	ld.readLatencies = startReads(ld.cfg.Logger, ld.cfg.Client.KubernetesClientSet(), ld.cfg.ClientTimeout, ld.cfg.Namespace, created, ld.cfg.Stopc, ld.donec)
	ld.cfg.Logger.Info("completed read function", zap.String("namespace-read", ld.cfg.Namespace))
}

func (ld *loader) Stop() {
	ld.cfg.Logger.Info("stopping and waiting")
	ld.donecCloseOnce.Do(func() {
		close(ld.donec)
	})
	ld.cfg.Logger.Info("stopped and waited")
}

// GetMetrics locally fetches output from registered metrics.
// ref. https://pkg.go.dev/github.com/prometheus/client_golang@v1.6.0/prometheus/promhttp?tab=doc#Handler
func (ld *loader) GetMetrics() (writes metrics.RequestsSummary, reads metrics.RequestsSummary, err error) {
	// https://pkg.go.dev/github.com/prometheus/client_golang/prometheus?tab=doc#Gatherer
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		ld.cfg.Logger.Warn("failed to gather prometheus metrics", zap.Error(err))
		return metrics.RequestsSummary{}, metrics.RequestsSummary{}, err
	}
	for _, mf := range mfs {
		if mf == nil {
			continue
		}
		switch *mf.Name {
		case "secrets_client_write_requests_success_total":
			gg := mf.Metric[0].GetGauge()
			writes.SuccessTotal = gg.GetValue()
		case "secrets_client_write_requests_failure_total":
			gg := mf.Metric[0].GetGauge()
			writes.FailureTotal = gg.GetValue()
		case "secrets_client_write_request_latency_milliseconds":
			writes.LatencyHistogram, err = metrics.ParseHistogram("milliseconds", mf.Metric[0].GetHistogram())
			if err != nil {
				return metrics.RequestsSummary{}, metrics.RequestsSummary{}, err
			}

		case "secrets_client_read_requests_success_total":
			gg := mf.Metric[0].GetGauge()
			reads.SuccessTotal = gg.GetValue()
		case "secrets_client_read_requests_failure_total":
			gg := mf.Metric[0].GetGauge()
			reads.FailureTotal = gg.GetValue()
		case "secrets_client_read_request_latency_milliseconds":
			reads.LatencyHistogram, err = metrics.ParseHistogram("milliseconds", mf.Metric[0].GetHistogram())
			if err != nil {
				return metrics.RequestsSummary{}, metrics.RequestsSummary{}, err
			}
		}
	}

	ld.cfg.Logger.Info("sorting write latency results", zap.Int("total-data-points", ld.writeLatencies.Len()))
	now := time.Now()
	sort.Sort(ld.writeLatencies)
	ld.cfg.Logger.Info("sorted write latency results", zap.Int("total-data-points", ld.writeLatencies.Len()), zap.String("took", time.Since(now).String()))
	writes.LantencyP50 = ld.writeLatencies.PickLantencyP50()
	writes.LantencyP90 = ld.writeLatencies.PickLantencyP90()
	writes.LantencyP99 = ld.writeLatencies.PickLantencyP99()
	writes.LantencyP999 = ld.writeLatencies.PickLantencyP999()
	writes.LantencyP9999 = ld.writeLatencies.PickLantencyP9999()

	ld.cfg.Logger.Info("sorting read latency results", zap.Int("total-data-points", ld.readLatencies.Len()))
	now = time.Now()
	sort.Sort(ld.readLatencies)
	ld.cfg.Logger.Info("sorted read latency results", zap.Int("total-data-points", ld.readLatencies.Len()), zap.String("took", time.Since(now).String()))
	reads.LantencyP50 = ld.readLatencies.PickLantencyP50()
	reads.LantencyP90 = ld.readLatencies.PickLantencyP90()
	reads.LantencyP99 = ld.readLatencies.PickLantencyP99()
	reads.LantencyP999 = ld.readLatencies.PickLantencyP999()
	reads.LantencyP9999 = ld.readLatencies.PickLantencyP9999()

	return writes, reads, nil
}

func startWrites(lg *zap.Logger, cli *kubernetes.Clientset, timeout time.Duration, namespace string, namePrefix string, objects int, objectSize int, stopc chan struct{}, donec chan struct{}) (ds metrics.Durations, created []string) {
	lg.Info("starting startWrites", zap.Int("objects", objects), zap.Int("object-size", objectSize))
	ds = make(metrics.Durations, 0, 20000)

	val := randutil.String(objectSize)
	for i := 0; i < objects; i++ {
		select {
		case <-stopc:
			lg.Warn("writes stopped")
			return ds, created
		case <-donec:
			lg.Info("writes done")
			return ds, created
		default:
		}

		key := fmt.Sprintf("%s%d", namePrefix, i)

		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		_, err := cli.
			CoreV1().
			Secrets(namespace).
			Create(ctx, &v1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      key,
					Namespace: namespace,
					Labels: map[string]string{
						"name": key,
					},
				},
				Type: v1.SecretTypeOpaque,
				Data: map[string][]byte{key: []byte(val)},
			}, metav1.CreateOptions{})
		cancel()
		took := time.Since(start)
		tookMS := float64(took / time.Millisecond)
		writeRequestLatencyMs.Observe(tookMS)
		ds = append(ds, took)
		if err != nil {
			writeRequestsFailureTotal.Inc()
			lg.Warn("write secret failed", zap.String("namespace", namespace), zap.Error(err))
		} else {
			writeRequestsSuccessTotal.Inc()
			created = append(created, key)
			if i%20 == 0 {
				lg.Info("wrote secret", zap.Int("iteration", i), zap.String("namespace", namespace))
			}
		}
	}
	return ds, created
}

func startReads(lg *zap.Logger, cli *kubernetes.Clientset, timeout time.Duration, namespace string, created []string, stopc chan struct{}, donec chan struct{}) (ds metrics.Durations) {
	lg.Info("starting startReads", zap.Int("created-secrets", len(created)))
	ds = make(metrics.Durations, 0, 20000)

	for i, key := range created {
		select {
		case <-stopc:
			lg.Warn("reads stopped")
			return
		case <-donec:
			lg.Info("reads done")
			return
		default:
		}

		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		_, err := cli.
			CoreV1().
			Secrets(namespace).
			Get(ctx, key, metav1.GetOptions{})
		cancel()
		took := time.Since(start)
		tookMS := float64(took / time.Millisecond)
		readRequestLatencyMs.Observe(tookMS)
		ds = append(ds, took)
		if err != nil {
			readRequestsFailureTotal.Inc()
			lg.Warn("read secret failed", zap.String("namespace", namespace), zap.Error(err))
		} else {
			readRequestsSuccessTotal.Inc()
			if i%20 == 0 {
				lg.Info("read secret", zap.Int("iteration", i), zap.String("namespace", namespace))
			}
		}
	}
	return ds
}
