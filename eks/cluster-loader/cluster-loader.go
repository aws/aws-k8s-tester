// Package clusterloader implements cluster load tests.
// ref. https://github.com/kubernetes/perf-tests
package clusterloader

import (
	"context"
	"sync"
	"time"

	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/metrics"
	"github.com/dustin/go-humanize"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	clientReqLatencyMs = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "cluster_loader",
			Subsystem: "client",
			Name:      "request_latency_milliseconds",
			Help:      "Bucketed histogram of client-side request and response latency.",

			// lowest bucket start of upper bound 0.5 ms with factor 2
			// highest bucket start of 0.5 ms * 2^13 == 4.096 sec
			Buckets: prometheus.ExponentialBuckets(0.5, 2, 14),
		})

	requestsSuccessTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "cluster_loader",
			Subsystem: "client",
			Name:      "requests_success_total",
			Help:      "Total number of successful requests.",
		})

	requestsFailureTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "cluster_loader",
			Subsystem: "client",
			Name:      "requests_failure_total",
			Help:      "Total number of successful requests.",
		})
)

func init() {
	prometheus.MustRegister(clientReqLatencyMs)
	prometheus.MustRegister(requestsSuccessTotal)
	prometheus.MustRegister(requestsFailureTotal)
}

// Config configures cluster loader.
type Config struct {
	Logger *zap.Logger

	Client k8s_client.EKS

	// Groups is the number of loader groups to launch.
	// The total number of goroutines is Groups * number of
	// load functons defined below.
	Groups int

	Stopc chan struct{}

	Deadline time.Time
	Timeout  time.Duration

	Namespaces []string
}

// Loader defines cluster loader operations.
type Loader interface {
	Start()
	Stop()
	GetMetrics() (success float64, failure float64, hs metrics.HistogramBuckets, err error)
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
	n := ld.cfg.Groups * opFuncN
	ld.wg.Add(n)

	ld.cfg.Logger.Info("starting load functions", zap.Strings("namespaces", ld.cfg.Namespaces), zap.Int("workers", n))
	for i := 0; i < ld.cfg.Groups; i++ {
		cli := ld.cfg.Client.KubernetesClientSet()
		go listNodes(ld.cfg.Logger, cli, ld.cfg.Deadline, ld.cfg.Timeout, ld.wg, ld.cfg.Stopc, ld.donec)
		go listPods(ld.cfg.Logger, cli, ld.cfg.Namespaces, ld.cfg.Deadline, ld.cfg.Timeout, ld.wg, ld.cfg.Stopc, ld.donec)
		go listServices(ld.cfg.Logger, cli, ld.cfg.Namespaces, ld.cfg.Deadline, ld.cfg.Timeout, ld.wg, ld.cfg.Stopc, ld.donec)
		go listEndpoints(ld.cfg.Logger, cli, ld.cfg.Namespaces, ld.cfg.Deadline, ld.cfg.Timeout, ld.wg, ld.cfg.Stopc, ld.donec)
		go listSecrets(ld.cfg.Logger, cli, ld.cfg.Namespaces, ld.cfg.Deadline, ld.cfg.Timeout, ld.wg, ld.cfg.Stopc, ld.donec)
		go listConfigMaps(ld.cfg.Logger, cli, ld.cfg.Namespaces, ld.cfg.Deadline, ld.cfg.Timeout, ld.wg, ld.cfg.Stopc, ld.donec)
		go listJobs(ld.cfg.Logger, cli, ld.cfg.Namespaces, ld.cfg.Deadline, ld.cfg.Timeout, ld.wg, ld.cfg.Stopc, ld.donec)
		go listCronJobs(ld.cfg.Logger, cli, ld.cfg.Namespaces, ld.cfg.Deadline, ld.cfg.Timeout, ld.wg, ld.cfg.Stopc, ld.donec)
	}
	ld.cfg.Logger.Info("started load functions", zap.Strings("namespaces", ld.cfg.Namespaces), zap.Int("workers", n))
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
func (ld *loader) GetMetrics() (success float64, failure float64, hs metrics.HistogramBuckets, err error) {
	// https://pkg.go.dev/github.com/prometheus/client_golang/prometheus?tab=doc#Gatherer
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		ld.cfg.Logger.Warn("failed to gather prometheus metrics", zap.Error(err))
		return 0, 0, nil, err
	}
	for _, mf := range mfs {
		if mf == nil {
			continue
		}
		switch *mf.Name {
		case "cluster_loader_client_request_latency_milliseconds":
			hs, err = metrics.ParseHistogram("milliseconds", mf.Metric[0].GetHistogram())
			if err != nil {
				return 0, 0, nil, err
			}
		case "cluster_loader_client_requests_success_total":
			gg := mf.Metric[0].GetGauge()
			success = gg.GetValue()

		case "cluster_loader_client_requests_failure_total":
			gg := mf.Metric[0].GetGauge()
			failure = gg.GetValue()
		}
	}
	return success, failure, hs, nil
}

const opFuncN = 8

func listNodes(lg *zap.Logger, cli *kubernetes.Clientset, deadline time.Time, timeout time.Duration, wg *sync.WaitGroup, stopc chan struct{}, donec chan struct{}) {
	lg.Info("starting listNodes")
	defer wg.Done()
	cnt := 0
	for {
		cnt++
		select {
		case <-stopc:
			lg.Warn("list nodes stopped")
			return
		case <-donec:
			lg.Info("list nodes done")
			return
		default:
		}

		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		rs, err := cli.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		cancel()
		clientReqLatencyMs.Observe(float64(time.Since(start) / time.Millisecond))
		if err != nil {
			requestsFailureTotal.Inc()
			lg.Warn("list nodes failed", zap.Error(err))
		} else {
			requestsSuccessTotal.Inc()
			if cnt%20 == 0 {
				lg.Info("listed nodes", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.Int("nodes", len(rs.Items)))
			}
		}
	}
}

func listPods(lg *zap.Logger, cli *kubernetes.Clientset, ns []string, deadline time.Time, timeout time.Duration, wg *sync.WaitGroup, stopc chan struct{}, donec chan struct{}) {
	lg.Info("starting listPods", zap.Strings("namespaces", ns))
	defer wg.Done()
	cnt := 0
	for {
		cnt++
		select {
		case <-stopc:
			lg.Warn("list pods stopped")
			return
		case <-donec:
			lg.Info("list pods done")
			return
		default:
		}

		for _, nv := range ns {
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			pods, err := cli.CoreV1().Pods(nv).List(ctx, metav1.ListOptions{})
			cancel()
			clientReqLatencyMs.Observe(float64(time.Since(start) / time.Millisecond))
			if err != nil {
				requestsFailureTotal.Inc()
				lg.Warn("list pods failed", zap.String("namespace", nv), zap.Error(err))
				continue
			}
			requestsSuccessTotal.Inc()
			if cnt%20 == 0 {
				lg.Info("listed pods", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("pods", len(pods.Items)))
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
		}
	}
}

func listServices(lg *zap.Logger, cli *kubernetes.Clientset, ns []string, deadline time.Time, timeout time.Duration, wg *sync.WaitGroup, stopc chan struct{}, donec chan struct{}) {
	lg.Info("starting listServices", zap.Strings("namespaces", ns))
	defer wg.Done()
	cnt := 0
	for {
		cnt++
		select {
		case <-stopc:
			lg.Warn("list services stopped")
			return
		case <-donec:
			lg.Info("list services done")
			return
		default:
		}

		for _, nv := range ns {
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			es, err := cli.CoreV1().Services(nv).List(ctx, metav1.ListOptions{})
			cancel()
			clientReqLatencyMs.Observe(float64(time.Since(start) / time.Millisecond))
			if err != nil {
				requestsFailureTotal.Inc()
				lg.Warn("list services failed", zap.String("namespace", nv), zap.Error(err))
				continue
			}
			requestsSuccessTotal.Inc()
			if cnt%20 == 0 {
				lg.Info("listed services", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("services", len(es.Items)))
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
		}
	}
}

func listEndpoints(lg *zap.Logger, cli *kubernetes.Clientset, ns []string, deadline time.Time, timeout time.Duration, wg *sync.WaitGroup, stopc chan struct{}, donec chan struct{}) {
	lg.Info("starting listEndpoints", zap.Strings("namespaces", ns))
	defer wg.Done()
	cnt := 0
	for {
		cnt++
		select {
		case <-stopc:
			lg.Warn("list endpoints stopped")
			return
		case <-donec:
			lg.Info("list endpoints done")
			return
		default:
		}

		for _, nv := range ns {
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			es, err := cli.CoreV1().Endpoints(nv).List(ctx, metav1.ListOptions{})
			cancel()
			clientReqLatencyMs.Observe(float64(time.Since(start) / time.Millisecond))
			if err != nil {
				requestsFailureTotal.Inc()
				lg.Warn("list endpoints failed", zap.String("namespace", nv), zap.Error(err))
				continue
			}
			requestsSuccessTotal.Inc()
			if cnt%20 == 0 {
				lg.Info("listed endpoints", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("endpoints", len(es.Items)))
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
		}
	}
}

func listSecrets(lg *zap.Logger, cli *kubernetes.Clientset, ns []string, deadline time.Time, timeout time.Duration, wg *sync.WaitGroup, stopc chan struct{}, donec chan struct{}) {
	lg.Info("starting listSecrets", zap.Strings("namespaces", ns))
	defer wg.Done()
	cnt := 0
	for {
		cnt++
		select {
		case <-stopc:
			lg.Warn("list secrets stopped")
			return
		case <-donec:
			lg.Info("list secrets done")
			return
		default:
		}

		for _, nv := range ns {
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			ss, err := cli.CoreV1().Secrets(nv).List(ctx, metav1.ListOptions{})
			cancel()
			clientReqLatencyMs.Observe(float64(time.Since(start) / time.Millisecond))
			if err != nil {
				requestsFailureTotal.Inc()
				lg.Warn("list secrets failed", zap.String("namespace", nv), zap.Error(err))
				continue
			}
			requestsSuccessTotal.Inc()
			if cnt%20 == 0 {
				lg.Info("listed secrets", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("secrets", len(ss.Items)))
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
		}
	}
}

func listConfigMaps(lg *zap.Logger, cli *kubernetes.Clientset, ns []string, deadline time.Time, timeout time.Duration, wg *sync.WaitGroup, stopc chan struct{}, donec chan struct{}) {
	lg.Info("starting listConfigMaps", zap.Strings("namespaces", ns))
	defer wg.Done()
	cnt := 0
	for {
		cnt++
		select {
		case <-stopc:
			lg.Warn("list configmaps stopped")
			return
		case <-donec:
			lg.Info("list configmaps done")
			return
		default:
		}

		for _, nv := range ns {
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			ss, err := cli.CoreV1().ConfigMaps(nv).List(ctx, metav1.ListOptions{})
			cancel()
			clientReqLatencyMs.Observe(float64(time.Since(start) / time.Millisecond))
			if err != nil {
				requestsFailureTotal.Inc()
				lg.Warn("list configmaps failed", zap.String("namespace", nv), zap.Error(err))
				continue
			}
			requestsSuccessTotal.Inc()
			if cnt%20 == 0 {
				lg.Info("listed configmaps", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("configmaps", len(ss.Items)))
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
		}
	}
}

func listJobs(lg *zap.Logger, cli *kubernetes.Clientset, ns []string, deadline time.Time, timeout time.Duration, wg *sync.WaitGroup, stopc chan struct{}, donec chan struct{}) {
	lg.Info("starting listJobs", zap.Strings("namespaces", ns))
	defer wg.Done()
	cnt := 0
	for {
		cnt++
		select {
		case <-stopc:
			lg.Warn("list jobs stopped")
			return
		case <-donec:
			lg.Info("list jobs done")
			return
		default:
		}

		for _, nv := range ns {
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			ss, err := cli.BatchV1().Jobs(nv).List(ctx, metav1.ListOptions{})
			cancel()
			clientReqLatencyMs.Observe(float64(time.Since(start) / time.Millisecond))
			if err != nil {
				requestsFailureTotal.Inc()
				lg.Warn("list jobs failed", zap.String("namespace", nv), zap.Error(err))
				continue
			}
			requestsSuccessTotal.Inc()
			if cnt%20 == 0 {
				lg.Info("listed jobs", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("jobs", len(ss.Items)))
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
		}
	}
}

func listCronJobs(lg *zap.Logger, cli *kubernetes.Clientset, ns []string, deadline time.Time, timeout time.Duration, wg *sync.WaitGroup, stopc chan struct{}, donec chan struct{}) {
	lg.Info("starting listCronJobs", zap.Strings("namespaces", ns))
	defer wg.Done()
	cnt := 0
	for {
		cnt++
		select {
		case <-stopc:
			lg.Warn("list cronjobs stopped")
			return
		case <-donec:
			lg.Info("list cronjobs done")
			return
		default:
		}

		for _, nv := range ns {
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			ss, err := cli.BatchV1beta1().CronJobs(nv).List(ctx, metav1.ListOptions{})
			cancel()
			clientReqLatencyMs.Observe(float64(time.Since(start) / time.Millisecond))
			if err != nil {
				requestsFailureTotal.Inc()
				lg.Warn("list cronjobs failed", zap.String("namespace", nv), zap.Error(err))
				continue
			}
			requestsSuccessTotal.Inc()
			if cnt%20 == 0 {
				lg.Info("listed cronjobs", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("cronjobs", len(ss.Items)))
			}

			select {
			case <-stopc:
				lg.Warn("list cronjobs stopped")
				return
			case <-donec:
				lg.Info("list jobs done")
				return
			default:
			}
		}
	}
}
