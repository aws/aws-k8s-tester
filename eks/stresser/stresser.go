// Package stresser implements cluster load tests.
// ref. https://github.com/kubernetes/perf-tests
package stresser

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
			Namespace: "stresser",
			Subsystem: "client",
			Name:      "write_requests_success_total",
			Help:      "Total number of successful write requests.",
		})
	writeRequestsFailureTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "stresser",
			Subsystem: "client",
			Name:      "write_requests_failure_total",
			Help:      "Total number of successful write requests.",
		})
	writeRequestLatencyMs = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "stresser",
			Subsystem: "client",
			Name:      "write_request_latency_milliseconds",
			Help:      "Bucketed histogram of client-side write request and response latency.",

			// lowest bucket start of upper bound 0.5 ms with factor 2
			// highest bucket start of 0.5 ms * 2^13 == 4.096 sec
			Buckets: prometheus.ExponentialBuckets(0.5, 2, 14),
		})

	readRequestsSuccessTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "stresser",
			Subsystem: "client",
			Name:      "read_requests_success_total",
			Help:      "Total number of successful read requests.",
		})
	readRequestsFailureTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "stresser",
			Subsystem: "client",
			Name:      "read_requests_failure_total",
			Help:      "Total number of successful read requests.",
		})
	readRequestLatencyMs = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "stresser",
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

// Config configures cluster loader.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}

	Client        k8s_client.EKS
	ClientTimeout time.Duration

	Deadline time.Time

	NamespaceWrite string
	NamespacesRead []string

	ObjectSize int
}

// Loader defines cluster loader operations.
type Loader interface {
	Start()
	Stop()
	GetMetrics() (writes metrics.RequestsSummary, reads metrics.RequestsSummary, err error)
}

type loader struct {
	cfg            Config
	wg             *sync.WaitGroup
	donec          chan struct{}
	donecCloseOnce *sync.Once
}

func New(cfg Config) Loader {
	return &loader{
		cfg:            cfg,
		wg:             new(sync.WaitGroup),
		donec:          make(chan struct{}),
		donecCloseOnce: new(sync.Once),
	}
}

func (ld *loader) Start() {
	ld.cfg.Logger.Info("starting load functions", zap.String("namespace-write", ld.cfg.NamespaceWrite), zap.Strings("namespaces-read", ld.cfg.NamespacesRead))
	ld.wg.Add(1) // for reads
	if ld.cfg.ObjectSize > 0 {
		ld.wg.Add(1) // for writes
		go startWrites(ld.cfg.Logger, ld.cfg.Client.KubernetesClientSet(), ld.cfg.ClientTimeout, ld.cfg.Deadline, ld.cfg.NamespaceWrite, ld.cfg.ObjectSize, ld.wg, ld.cfg.Stopc, ld.donec)
	}
	go startReads(ld.cfg.Logger, ld.cfg.Client.KubernetesClientSet(), ld.cfg.ClientTimeout, ld.cfg.Deadline, ld.cfg.NamespacesRead, ld.wg, ld.cfg.Stopc, ld.donec)
	ld.cfg.Logger.Info("started load functions", zap.String("namespace-write", ld.cfg.NamespaceWrite), zap.Strings("namespaces-read", ld.cfg.NamespacesRead))
}

func (ld *loader) Stop() {
	ld.cfg.Logger.Info("stopping and waiting for load functions")
	ld.donecCloseOnce.Do(func() {
		close(ld.donec)
	})
	ld.wg.Wait()
	ld.cfg.Logger.Info("stopped and waited for load functions")
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
		case "stresser_client_write_requests_success_total":
			gg := mf.Metric[0].GetGauge()
			writes.SuccessTotal = gg.GetValue()
		case "stresser_client_write_requests_failure_total":
			gg := mf.Metric[0].GetGauge()
			writes.FailureTotal = gg.GetValue()
		case "stresser_client_write_request_latency_milliseconds":
			writes.LatencyHistogram, err = metrics.ParseHistogram("milliseconds", mf.Metric[0].GetHistogram())
			if err != nil {
				return metrics.RequestsSummary{}, metrics.RequestsSummary{}, err
			}

		case "stresser_client_read_requests_success_total":
			gg := mf.Metric[0].GetGauge()
			reads.SuccessTotal = gg.GetValue()
		case "stresser_client_read_requests_failure_total":
			gg := mf.Metric[0].GetGauge()
			reads.FailureTotal = gg.GetValue()
		case "stresser_client_read_request_latency_milliseconds":
			reads.LatencyHistogram, err = metrics.ParseHistogram("milliseconds", mf.Metric[0].GetHistogram())
			if err != nil {
				return metrics.RequestsSummary{}, metrics.RequestsSummary{}, err
			}
		}
	}
	return writes, reads, nil
}

func startWrites(lg *zap.Logger, cli *kubernetes.Clientset, timeout time.Duration, deadline time.Time, namespace string, objectSize int, wg *sync.WaitGroup, stopc chan struct{}, donec chan struct{}) {
	lg.Info("starting startWrites")
	defer wg.Done()

	val := randutil.String(objectSize)
	cnt := 0
	for {
		cnt++
		select {
		case <-stopc:
			lg.Warn("writes stopped")
			return
		case <-donec:
			lg.Info("writes done")
			return
		default:
		}

		key := fmt.Sprintf("secret%d%s", cnt, randutil.String(7))

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
		writeRequestLatencyMs.Observe(float64(time.Since(start) / time.Millisecond))
		if err != nil {
			writeRequestsFailureTotal.Inc()
			lg.Warn("write configmap failed", zap.String("namespace", namespace), zap.Error(err))
		} else {
			writeRequestsSuccessTotal.Inc()
			if cnt%20 == 0 {
				lg.Info("wrote configmap", zap.String("time-left", deadline.Sub(time.Now()).String()), zap.Int("iteration", cnt), zap.String("namespace", namespace))
			}
		}
		select {
		case <-stopc:
			lg.Warn("write configmap stopped")
			return
		case <-donec:
			lg.Info("write configmap done")
			return
		default:
		}

		key = fmt.Sprintf("configmap%d%s", cnt, randutil.String(7))
		start = time.Now()
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
		_, err = cli.
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
		writeRequestLatencyMs.Observe(float64(time.Since(start) / time.Millisecond))
		if err != nil {
			writeRequestsFailureTotal.Inc()
			lg.Warn("write secret failed", zap.String("namespace", namespace), zap.Error(err))
		} else {
			writeRequestsSuccessTotal.Inc()
			if cnt%20 == 0 {
				lg.Info("wrote secret", zap.String("time-left", deadline.Sub(time.Now()).String()), zap.Int("iteration", cnt), zap.String("namespace", namespace))
			}
		}
		select {
		case <-stopc:
			lg.Warn("write secret stopped")
			return
		case <-donec:
			lg.Info("write secret done")
			return
		default:
		}
	}
}

func startReads(lg *zap.Logger, cli *kubernetes.Clientset, timeout time.Duration, deadline time.Time, ns []string, wg *sync.WaitGroup, stopc chan struct{}, donec chan struct{}) {
	lg.Info("starting startReads", zap.Strings("namespaces", ns))
	defer wg.Done()
	cnt := 0
	for {
		cnt++
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
		rs, err := cli.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		cancel()
		readRequestLatencyMs.Observe(float64(time.Since(start) / time.Millisecond))
		if err != nil {
			readRequestsFailureTotal.Inc()
			lg.Warn("list nodes failed", zap.Error(err))
		} else {
			readRequestsSuccessTotal.Inc()
			if cnt%20 == 0 {
				lg.Info("listed nodes", zap.String("time-left", deadline.Sub(time.Now()).String()), zap.Int("iteration", cnt), zap.Int("nodes", len(rs.Items)))
			}
		}

		for _, nv := range ns {
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			pods, err := cli.CoreV1().Pods(nv).List(ctx, metav1.ListOptions{})
			cancel()
			readRequestLatencyMs.Observe(float64(time.Since(start) / time.Millisecond))
			if err != nil {
				readRequestsFailureTotal.Inc()
				lg.Warn("list pods failed", zap.String("namespace", nv), zap.Error(err))
			} else {
				readRequestsSuccessTotal.Inc()
				if cnt%20 == 0 {
					lg.Info("listed pods", zap.String("time-left", deadline.Sub(time.Now()).String()), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("pods", len(pods.Items)))
				}
			}
			select {
			case <-stopc:
				lg.Warn("list pods stopped")
				return
			case <-donec:
				lg.Info("list pods done")
				return
			default:
			}

			start = time.Now()
			ctx, cancel = context.WithTimeout(context.Background(), timeout)
			svcs, err := cli.CoreV1().Services(nv).List(ctx, metav1.ListOptions{})
			cancel()
			readRequestLatencyMs.Observe(float64(time.Since(start) / time.Millisecond))
			if err != nil {
				readRequestsFailureTotal.Inc()
				lg.Warn("list services failed", zap.String("namespace", nv), zap.Error(err))
			} else {
				readRequestsSuccessTotal.Inc()
				if cnt%20 == 0 {
					lg.Info("listed services", zap.String("time-left", deadline.Sub(time.Now()).String()), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("services", len(svcs.Items)))
				}
			}
			select {
			case <-stopc:
				lg.Warn("list services stopped")
				return
			case <-donec:
				lg.Info("list services done")
				return
			default:
			}

			start = time.Now()
			ctx, cancel = context.WithTimeout(context.Background(), timeout)
			eps, err := cli.CoreV1().Endpoints(nv).List(ctx, metav1.ListOptions{})
			cancel()
			readRequestLatencyMs.Observe(float64(time.Since(start) / time.Millisecond))
			if err != nil {
				readRequestsFailureTotal.Inc()
				lg.Warn("list endpoints failed", zap.String("namespace", nv), zap.Error(err))
			} else {
				readRequestsSuccessTotal.Inc()
				if cnt%20 == 0 {
					lg.Info("listed endpoints", zap.String("time-left", deadline.Sub(time.Now()).String()), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("endpoints", len(eps.Items)))
				}
			}
			select {
			case <-stopc:
				lg.Warn("list endpoints stopped")
				return
			case <-donec:
				lg.Info("list endpoints done")
				return
			default:
			}

			start = time.Now()
			ctx, cancel = context.WithTimeout(context.Background(), timeout)
			cms, err := cli.CoreV1().ConfigMaps(nv).List(ctx, metav1.ListOptions{})
			cancel()
			readRequestLatencyMs.Observe(float64(time.Since(start) / time.Millisecond))
			if err != nil {
				readRequestsFailureTotal.Inc()
				lg.Warn("list configmaps failed", zap.String("namespace", nv), zap.Error(err))
			} else {
				readRequestsSuccessTotal.Inc()
				if cnt%20 == 0 {
					lg.Info("listed configmaps", zap.String("time-left", deadline.Sub(time.Now()).String()), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("configmaps", len(cms.Items)))
				}
			}
			select {
			case <-stopc:
				lg.Warn("list configmaps stopped")
				return
			case <-donec:
				lg.Info("list configmaps done")
				return
			default:
			}

			start = time.Now()
			ctx, cancel = context.WithTimeout(context.Background(), timeout)
			ss, err := cli.CoreV1().Secrets(nv).List(ctx, metav1.ListOptions{})
			cancel()
			readRequestLatencyMs.Observe(float64(time.Since(start) / time.Millisecond))
			if err != nil {
				readRequestsFailureTotal.Inc()
				lg.Warn("list secrets failed", zap.String("namespace", nv), zap.Error(err))
				continue
			}
			readRequestsSuccessTotal.Inc()
			if cnt%20 == 0 {
				lg.Info("listed secrets", zap.String("time-left", deadline.Sub(time.Now()).String()), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("secrets", len(ss.Items)))
			}
			select {
			case <-stopc:
				lg.Warn("list secrets stopped")
				return
			case <-donec:
				lg.Info("list secrets done")
				return
			default:
			}

			start = time.Now()
			ctx, cancel = context.WithTimeout(context.Background(), timeout)
			jobs, err := cli.BatchV1().Jobs(nv).List(ctx, metav1.ListOptions{})
			cancel()
			readRequestLatencyMs.Observe(float64(time.Since(start) / time.Millisecond))
			if err != nil {
				readRequestsFailureTotal.Inc()
				lg.Warn("list jobs failed", zap.String("namespace", nv), zap.Error(err))
			} else {
				readRequestsSuccessTotal.Inc()
				if cnt%20 == 0 {
					lg.Info("listed jobs", zap.String("time-left", deadline.Sub(time.Now()).String()), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("jobs", len(jobs.Items)))
				}
			}
			select {
			case <-stopc:
				lg.Warn("list jobs stopped")
				return
			case <-donec:
				lg.Info("list jobs done")
				return
			default:
			}

			start = time.Now()
			ctx, cancel = context.WithTimeout(context.Background(), timeout)
			cjbs, err := cli.BatchV1beta1().CronJobs(nv).List(ctx, metav1.ListOptions{})
			cancel()
			readRequestLatencyMs.Observe(float64(time.Since(start) / time.Millisecond))
			if err != nil {
				readRequestsFailureTotal.Inc()
				lg.Warn("list cronjobs failed", zap.String("namespace", nv), zap.Error(err))
			} else {
				readRequestsSuccessTotal.Inc()
				if cnt%20 == 0 {
					lg.Info("listed cronjobs", zap.String("time-left", deadline.Sub(time.Now()).String()), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("cronjobs", len(cjbs.Items)))
				}
			}
			select {
			case <-stopc:
				lg.Warn("list cronjobs stopped")
				return
			case <-donec:
				lg.Info("list cronjobs done")
				return
			default:
			}
		}
	}
}
