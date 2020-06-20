// Package stresser implements cluster load tests.
// ref. https://github.com/kubernetes/perf-tests
package stresser

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
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
	ListLimit  int64

	WritesJSONPath string
	ReadsJSONPath  string
}

// Loader defines cluster loader operations.
type Loader interface {
	Start()
	Stop()
	CollectMetrics() (writes metrics.RequestsSummary, reads metrics.RequestsSummary, err error)
}

type loader struct {
	cfg            Config
	donec          chan struct{}
	donecCloseOnce *sync.Once

	writeLatencies chan metrics.Durations
	readLatencies  chan metrics.Durations
}

func New(cfg Config) Loader {
	return &loader{
		cfg:            cfg,
		donec:          make(chan struct{}),
		donecCloseOnce: new(sync.Once),
		writeLatencies: make(chan metrics.Durations, 1), // buffer to not block send
		readLatencies:  make(chan metrics.Durations, 1), // buffer to not block send
	}
}

func (ld *loader) Start() {
	ld.cfg.Logger.Info("starting load functions", zap.String("namespace-write", ld.cfg.NamespaceWrite), zap.Strings("namespaces-read", ld.cfg.NamespacesRead))
	if ld.cfg.ObjectSize > 0 {
		go startWrites(
			ld.cfg.Logger,
			ld.cfg.Client.KubernetesClientSet(),
			ld.cfg.ClientTimeout,
			ld.cfg.Deadline,
			ld.cfg.NamespaceWrite,
			ld.cfg.ObjectSize,
			ld.cfg.Stopc,
			ld.donec,
			ld.writeLatencies,
		)
	}
	go startReads(
		ld.cfg.Logger,
		ld.cfg.Client.KubernetesClientSet(),
		ld.cfg.ClientTimeout,
		ld.cfg.Deadline,
		ld.cfg.NamespacesRead,
		ld.cfg.ListLimit,
		ld.cfg.Stopc,
		ld.donec,
		ld.readLatencies,
	)
	ld.cfg.Logger.Info("started load functions", zap.String("namespace-write", ld.cfg.NamespaceWrite), zap.Strings("namespaces-read", ld.cfg.NamespacesRead))
}

func (ld *loader) Stop() {
	ld.cfg.Logger.Info("stopping and waiting for load functions")
	ld.donecCloseOnce.Do(func() {
		close(ld.donec)
	})
	time.Sleep(5 * time.Second) // enough time to stop goroutines
	ld.cfg.Logger.Info("stopped and waited for load functions")
}

// GetMetrics locally fetches output from registered metrics.
// ref. https://pkg.go.dev/github.com/prometheus/client_golang@v1.6.0/prometheus/promhttp?tab=doc#Handler
func (ld *loader) CollectMetrics() (writes metrics.RequestsSummary, reads metrics.RequestsSummary, err error) {
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

	ld.cfg.Logger.Info("receiving write latency results")
	select {
	case lats := <-ld.writeLatencies:
		ld.cfg.Logger.Info("received and sorting write latency results", zap.Int("total-data-points", lats.Len()))
		now := time.Now()
		sort.Sort(lats)
		ld.cfg.Logger.Info("sorted write latency results", zap.Int("total-data-points", lats.Len()), zap.String("took", time.Since(now).String()))
		writes.LantencyP50 = lats.PickLantencyP50()
		writes.LantencyP90 = lats.PickLantencyP90()
		writes.LantencyP99 = lats.PickLantencyP99()
		writes.LantencyP999 = lats.PickLantencyP999()
		writes.LantencyP9999 = lats.PickLantencyP9999()

		ld.cfg.Logger.Info("writing latency results in JSON to disk", zap.String("path", ld.cfg.WritesJSONPath))
		wb, err := json.Marshal(lats)
		if err != nil {
			ld.cfg.Logger.Warn("failed to encode latency results in JSON", zap.Error(err))
			return metrics.RequestsSummary{}, metrics.RequestsSummary{}, err
		}
		if err = ioutil.WriteFile(ld.cfg.WritesJSONPath, wb, 0600); err != nil {
			ld.cfg.Logger.Warn("failed to write latency results in JSON to disk", zap.String("path", ld.cfg.WritesJSONPath), zap.Error(err))
			return metrics.RequestsSummary{}, metrics.RequestsSummary{}, err
		}
		ld.cfg.Logger.Info("wrote latency results in JSON to disk", zap.String("path", ld.cfg.WritesJSONPath))

	case <-time.After(2 * time.Minute):
		ld.cfg.Logger.Warn("took too long to receive write latency results")
	}

	ld.cfg.Logger.Info("receiving read latency results")
	select {
	case lats := <-ld.readLatencies:
		ld.cfg.Logger.Info("received and sorting read latency results", zap.Int("total-data-points", lats.Len()))
		now := time.Now()
		sort.Sort(lats)
		ld.cfg.Logger.Info("sorted read latency results", zap.Int("total-data-points", lats.Len()), zap.String("took", time.Since(now).String()))
		reads.LantencyP50 = lats.PickLantencyP50()
		reads.LantencyP90 = lats.PickLantencyP90()
		reads.LantencyP99 = lats.PickLantencyP99()
		reads.LantencyP999 = lats.PickLantencyP999()
		reads.LantencyP9999 = lats.PickLantencyP9999()

		ld.cfg.Logger.Info("writing latency results in JSON to disk", zap.String("path", ld.cfg.ReadsJSONPath))
		wb, err := json.Marshal(lats)
		if err != nil {
			ld.cfg.Logger.Warn("failed to encode latency results in JSON", zap.Error(err))
			return metrics.RequestsSummary{}, metrics.RequestsSummary{}, err
		}
		if err = ioutil.WriteFile(ld.cfg.ReadsJSONPath, wb, 0600); err != nil {
			ld.cfg.Logger.Warn("failed to write latency results in JSON to disk", zap.String("path", ld.cfg.ReadsJSONPath), zap.Error(err))
			return metrics.RequestsSummary{}, metrics.RequestsSummary{}, err
		}
		ld.cfg.Logger.Info("wrote latency results in JSON to disk", zap.String("path", ld.cfg.ReadsJSONPath))

	case <-time.After(2 * time.Minute):
		ld.cfg.Logger.Warn("took too long to receive read latency results")
	}

	return writes, reads, nil
}

func startWrites(
	lg *zap.Logger,
	cli *kubernetes.Clientset,
	timeout time.Duration,
	deadline time.Time,
	namespace string,
	objectSize int,
	stopc chan struct{},
	donec chan struct{},
	writeLatencies chan<- metrics.Durations,
) {
	lg.Info("starting writes")
	ds := make(metrics.Durations, 0, 20000)
	defer func() {
		lg.Info("sending write latency results", zap.Int("total-results", len(ds)))
		select {
		case writeLatencies <- ds:
			lg.Info("sent write latency results", zap.Int("total-results", len(ds)))
		case <-time.After(2 * time.Minute):
			lg.Warn("took to long to send write latency results")
			// in case, receiving takes long...
			select {
			case <-stopc:
				return
			case <-donec:
				return
			}
		}
	}()

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
		took := time.Since(start)
		tookMS := float64(took / time.Millisecond)
		writeRequestLatencyMs.Observe(tookMS)
		ds = append(ds, took)
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
		took = time.Since(start)
		tookMS = float64(took / time.Millisecond)
		writeRequestLatencyMs.Observe(tookMS)
		ds = append(ds, took)
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

func startReads(
	lg *zap.Logger,
	cli *kubernetes.Clientset,
	timeout time.Duration,
	deadline time.Time,
	ns []string,
	listLimit int64,
	stopc chan struct{},
	donec chan struct{},
	readLatencies chan<- metrics.Durations,
) {
	lg.Info("starting reads", zap.Strings("namespaces", ns))
	ds := make(metrics.Durations, 0, 20000)
	defer func() {
		lg.Info("sending read latency results", zap.Int("total-results", len(ds)))
		select {
		case readLatencies <- ds:
			lg.Info("sent read latency results", zap.Int("total-results", len(ds)))
		case <-time.After(2 * time.Minute):
			lg.Warn("took to long to send read latency results")
			// in case, receiving takes long...
			select {
			case <-stopc:
				return
			case <-donec:
				return
			}
		}
	}()

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
		rs, err := cli.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: listLimit})
		cancel()
		took := time.Since(start)
		tookMS := float64(took / time.Millisecond)
		readRequestLatencyMs.Observe(tookMS)
		ds = append(ds, took)
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
			pods, err := cli.CoreV1().Pods(nv).List(ctx, metav1.ListOptions{Limit: listLimit})
			cancel()
			took := time.Since(start)
			tookMS := float64(took / time.Millisecond)
			readRequestLatencyMs.Observe(tookMS)
			ds = append(ds, took)
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
			svcs, err := cli.CoreV1().Services(nv).List(ctx, metav1.ListOptions{Limit: listLimit})
			cancel()
			took = time.Since(start)
			tookMS = float64(took / time.Millisecond)
			readRequestLatencyMs.Observe(tookMS)
			ds = append(ds, took)
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
			eps, err := cli.CoreV1().Endpoints(nv).List(ctx, metav1.ListOptions{Limit: listLimit})
			cancel()
			took = time.Since(start)
			tookMS = float64(took / time.Millisecond)
			readRequestLatencyMs.Observe(tookMS)
			ds = append(ds, took)
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
			cms, err := cli.CoreV1().ConfigMaps(nv).List(ctx, metav1.ListOptions{Limit: listLimit})
			cancel()
			took = time.Since(start)
			tookMS = float64(took / time.Millisecond)
			readRequestLatencyMs.Observe(tookMS)
			ds = append(ds, took)
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
			ss, err := cli.CoreV1().Secrets(nv).List(ctx, metav1.ListOptions{Limit: listLimit})
			cancel()
			took = time.Since(start)
			tookMS = float64(took / time.Millisecond)
			readRequestLatencyMs.Observe(tookMS)
			ds = append(ds, took)
			if err != nil {
				readRequestsFailureTotal.Inc()
				lg.Warn("list secrets failed", zap.String("namespace", nv), zap.Error(err))
			} else {
				readRequestsSuccessTotal.Inc()
				if cnt%20 == 0 {
					lg.Info("listed secrets", zap.String("time-left", deadline.Sub(time.Now()).String()), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("secrets", len(ss.Items)))
				}
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
			jobs, err := cli.BatchV1().Jobs(nv).List(ctx, metav1.ListOptions{Limit: listLimit})
			cancel()
			took = time.Since(start)
			tookMS = float64(took / time.Millisecond)
			readRequestLatencyMs.Observe(tookMS)
			ds = append(ds, took)
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
			cjbs, err := cli.BatchV1beta1().CronJobs(nv).List(ctx, metav1.ListOptions{Limit: listLimit})
			cancel()
			took = time.Since(start)
			tookMS = float64(took / time.Millisecond)
			readRequestLatencyMs.Observe(tookMS)
			ds = append(ds, took)
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
