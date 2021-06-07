// Package stress implements stress tester using "Pod" objects.
// Do not parallelize locally, instead parallelize by distributing workers across nodes.
// It uses "Update" for stressing writes, and "List" for stressing reads.
// Both Kubernetes "Create" and "Update" are same for etcd, as they are etcd mutable transactions.
// See "k8s.io/apiserver/pkg/storage/etcd3/store.go" for "Create" and "GuaranteedUpdate".
// To only test creates, see "k8s-tester/configmaps" and "k8s-tester/secrets".
// To test large-size writes, also see "k8s-tester/jobs-echo".
// Replace https://github.com/aws/aws-k8s-tester/tree/v1.5.9/eks/stresser.
// Replace https://github.com/aws/aws-k8s-tester/tree/v1.5.9/eks/stresser2.
package stress

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester/tester"
	aws_v1 "github.com/aws/aws-k8s-tester/utils/aws/v1"
	aws_v1_ecr "github.com/aws/aws-k8s-tester/utils/aws/v1/ecr"
	"github.com/aws/aws-k8s-tester/utils/latency"
	"github.com/aws/aws-k8s-tester/utils/rand"
	utils_time "github.com/aws/aws-k8s-tester/utils/time"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/dustin/go-humanize"
	"github.com/manifoldco/promptui"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	core_v1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

var (
	writeRequestsSuccessTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "stress",
			Subsystem: "client",
			Name:      "write_requests_success_total",
			Help:      "Total number of successful write requests.",
		})
	writeRequestsFailureTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "stress",
			Subsystem: "client",
			Name:      "write_requests_failure_total",
			Help:      "Total number of successful write requests.",
		})
	writeRequestLatencyMs = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "stress",
			Subsystem: "client",
			Name:      "write_request_latency_milliseconds",
			Help:      "Bucketed histogram of client-side write request and response latency.",

			// lowest bucket start of upper bound 0.5 ms with factor 2
			// highest bucket start of 0.5 ms * 2^13 == 4.096 sec
			Buckets: prometheus.ExponentialBuckets(0.5, 2, 14),
		})

	getRequestsSuccessTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "stress",
			Subsystem: "client",
			Name:      "get_requests_success_total",
			Help:      "Total number of successful get requests.",
		})
	getRequestsFailureTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "stress",
			Subsystem: "client",
			Name:      "get_requests_failure_total",
			Help:      "Total number of successful get requests.",
		})
	getRequestLatencyMs = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "stress",
			Subsystem: "client",
			Name:      "get_request_latency_milliseconds",
			Help:      "Bucketed histogram of client-side get request and response latency.",

			// lowest bucket start of upper bound 0.5 ms with factor 2
			// highest bucket start of 0.5 ms * 2^13 == 4.096 sec
			Buckets: prometheus.ExponentialBuckets(0.5, 2, 14),
		})

	rangeGetRequestsSuccessTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "stress",
			Subsystem: "client",
			Name:      "range_get_requests_success_total",
			Help:      "Total number of successful range get requests.",
		})
	rangeGetRequestsFailureTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "stress",
			Subsystem: "client",
			Name:      "range_get_requests_failure_total",
			Help:      "Total number of successful range get requests.",
		})
	rangeGetRequestLatencyMs = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "stress",
			Subsystem: "client",
			Name:      "range_get_request_latency_milliseconds",
			Help:      "Bucketed histogram of client-side range get request and response latency.",

			// lowest bucket start of upper bound 0.5 ms with factor 2
			// highest bucket start of 0.5 ms * 2^13 == 4.096 sec
			Buckets: prometheus.ExponentialBuckets(0.5, 2, 14),
		})
)

func init() {
	prometheus.MustRegister(writeRequestsSuccessTotal)
	prometheus.MustRegister(writeRequestsFailureTotal)
	prometheus.MustRegister(writeRequestLatencyMs)

	prometheus.MustRegister(getRequestsSuccessTotal)
	prometheus.MustRegister(getRequestsFailureTotal)
	prometheus.MustRegister(getRequestLatencyMs)

	prometheus.MustRegister(rangeGetRequestsSuccessTotal)
	prometheus.MustRegister(rangeGetRequestsFailureTotal)
	prometheus.MustRegister(rangeGetRequestLatencyMs)
}

type Config struct {
	Enable bool `json:"enable"`
	Prompt bool `json:"-"`

	Stopc     chan struct{} `json:"-"`
	Logger    *zap.Logger   `json:"-"`
	LogWriter io.Writer     `json:"-"`
	Client    client.Client `json:"-"`

	// MinimumNodes is the minimum number of Kubernetes nodes required for installing this addon.
	MinimumNodes int `json:"minimum_nodes"`
	// Namespace to create test resources.
	Namespace string `json:"namespace"`

	// Repository defines a custom ECR image repository.
	// For "busybox".
	Repository *aws_v1_ecr.Repository `json:"repository,omitempty"`

	// RunTimeout is the duration of stress runs.
	// After timeout, it stops all stress requests.
	RunTimeout       time.Duration `json:"run_timeout"`
	RunTimeoutString string        `json:"run_timeout_string" read-only:"true"`

	// ObjectKeyPrefix is the key prefix for "Pod" objects.
	ObjectKeyPrefix string `json:"object_key_prefix"`
	// Objects is the desired number of objects to create and update.
	// This doesn't apply to reads.
	// If negative, it creates until timeout.
	Objects int `json:"objects"`
	// ObjectSize is the size in bytes per object.
	ObjectSize int `json:"object_size"`
	// UpdateConcurrency is the number of concurrent routines to issue update requests.
	// Do not set too high, instead distribute this tester as distributed workers to maximize concurrency.
	UpdateConcurrency int `json:"update_concurrency"`
	// ListLimit is the number of objects to return for each list response.
	// If negative, the tester disables list calls (only runs mutable requests).
	ListLimit int64 `json:"list_limit"`

	// LatencySummaryWrites represents latencies for "Create" and "Update" requests.
	LatencySummaryWrites latency.Summary `json:"latency_summary_writes" read-only:"true"`
	// LatencySummaryGets represents latencies for "Get" requests.
	LatencySummaryGets latency.Summary `json:"latency_summary_gets" read-only:"true"`
	// LatencySummaryRangeGets represents latencies for "List" requests.
	LatencySummaryRangeGets latency.Summary `json:"latency_summary_range_gets" read-only:"true"`
}

func (cfg *Config) ValidateAndSetDefaults() error {
	if cfg.MinimumNodes == 0 {
		cfg.MinimumNodes = DefaultMinimumNodes
	}
	if cfg.Namespace == "" {
		return errors.New("empty Namespace")
	}

	if cfg.RunTimeout == time.Duration(0) {
		cfg.RunTimeout = DefaultRunTimeout
	}
	cfg.RunTimeoutString = cfg.RunTimeout.String()

	if cfg.ObjectKeyPrefix == "" {
		cfg.ObjectKeyPrefix = DefaultObjectKeyPrefix()
	}

	if cfg.ObjectSize == 0 {
		return errors.New("zero ObjectSize")
	}
	if cfg.UpdateConcurrency == 0 {
		cfg.UpdateConcurrency = DefaultUpdateConcurrency
	}

	return nil
}

const (
	DefaultMinimumNodes int = 1
	DefaultRunTimeout       = time.Minute

	DefaultObjects    int = -1
	DefaultObjectSize int = 10 * 1024 // 10 KB

	// writes total 300 MB data to etcd
	// Objects: 1000,
	// ObjectSize: 300000, // 0.3 MB

	DefaultUpdateConcurrency int   = 10
	DefaultListLimit         int64 = 1000
)

var defaultObjectKeyPrefix string = fmt.Sprintf("pod%s", rand.String(7))

func DefaultObjectKeyPrefix() string {
	return defaultObjectKeyPrefix
}

func NewDefault() *Config {
	return &Config{
		Enable:            false,
		Prompt:            false,
		MinimumNodes:      DefaultMinimumNodes,
		Namespace:         pkgName + "-" + rand.String(10) + "-" + utils_time.GetTS(10),
		Repository:        &aws_v1_ecr.Repository{},
		RunTimeout:        DefaultRunTimeout,
		RunTimeoutString:  DefaultRunTimeout.String(),
		ObjectKeyPrefix:   DefaultObjectKeyPrefix(),
		Objects:           DefaultObjects,
		ObjectSize:        DefaultObjectSize,
		UpdateConcurrency: DefaultUpdateConcurrency,
		ListLimit:         DefaultListLimit,
	}
}

func New(cfg *Config) k8s_tester.Tester {
	ts := &tester{
		cfg:            cfg,
		donec:          make(chan struct{}),
		donecCloseOnce: new(sync.Once),
	}
	if !cfg.Repository.IsEmpty() {
		awsCfg := aws_v1.Config{
			Logger:        cfg.Logger,
			DebugAPICalls: cfg.Logger.Core().Enabled(zapcore.DebugLevel),
			Partition:     cfg.Repository.Partition,
			Region:        cfg.Repository.Region,
		}
		awsSession, _, _, err := aws_v1.New(&awsCfg)
		if err != nil {
			cfg.Logger.Panic("failed to create aws session", zap.Error(err))
		}
		ts.ecrAPI = ecr.New(awsSession, aws.NewConfig().WithRegion(cfg.Repository.Region))
	}
	return ts
}

type tester struct {
	cfg            *Config
	ecrAPI         ecriface.ECRAPI
	donec          chan struct{}
	donecCloseOnce *sync.Once
}

var pkgName = path.Base(reflect.TypeOf(tester{}).PkgPath())

func Env() string {
	return "ADD_ON_" + strings.ToUpper(strings.Replace(pkgName, "-", "_", -1))
}

func EnvRepository() string {
	return Env() + "_REPOSITORY"
}

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Enabled() bool { return ts.cfg.Enable }

func (ts *tester) Apply() error {
	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}

	podImg, err := ts.checkECRImage()
	if err != nil {
		return err
	}

	if nodes, err := client.ListNodes(ts.cfg.Client.KubernetesClient()); len(nodes) < ts.cfg.MinimumNodes || err != nil {
		return fmt.Errorf("failed to validate minimum nodes requirement %d (nodes %v, error %v)", ts.cfg.MinimumNodes, len(nodes), err)
	}

	if err := client.CreateNamespace(ts.cfg.Logger, ts.cfg.Client.KubernetesClient(), ts.cfg.Namespace); err != nil {
		return err
	}

	latenciesWritesCh, latenciesGetsCh := make(chan latency.Durations), make(chan latency.Durations)
	go func() {
		latenciesWrites, latenciesGets := ts.startUpdates(podImg)
		latenciesWritesCh <- latenciesWrites
		latenciesGetsCh <- latenciesGets
	}()
	latenciesRangeGetsCh := make(chan latency.Durations)
	go func() {
		latenciesRangeGetsCh <- ts.startRangeGets()
	}()

	ts.cfg.Logger.Info("waiting for test run timeout", zap.String("timeout", ts.cfg.RunTimeoutString))
	select {
	case <-ts.cfg.Stopc:
		ts.cfg.Logger.Warn("all stopped")
		ts.donecCloseOnce.Do(func() {
			close(ts.donec)
		})
		return nil
	case <-time.After(ts.cfg.RunTimeout):
		ts.donecCloseOnce.Do(func() {
			close(ts.donec)
		})
		ts.cfg.Logger.Info("run timeout, signaled done channel")
	}

	latenciesWrites := make(latency.Durations, 0)
	select {
	case <-ts.cfg.Stopc:
		ts.cfg.Logger.Warn("stopped while waiting for write results")
		return nil
	case latenciesWrites = <-latenciesWritesCh:
	case <-time.After(3 * time.Minute):
		ts.cfg.Logger.Warn("took too long to receive write results")
	}
	latenciesGets := make(latency.Durations, 0)
	select {
	case <-ts.cfg.Stopc:
		ts.cfg.Logger.Warn("stopped while waiting for write results")
		return nil
	case latenciesGets = <-latenciesGetsCh:
	case <-time.After(3 * time.Minute):
		ts.cfg.Logger.Warn("took too long to receive write results")
	}
	latenciesRangeGets := make(latency.Durations, 0)
	select {
	case <-ts.cfg.Stopc:
		ts.cfg.Logger.Warn("stopped while waiting for read results")
		return nil
	case latenciesRangeGets = <-latenciesRangeGetsCh:
	case <-time.After(3 * time.Minute):
		ts.cfg.Logger.Warn("took too long to receive read results")
	}

	ts.cfg.Logger.Info("sorting write latency results", zap.Int("total-data-points", latenciesWrites.Len()))
	now := time.Now()
	sort.Sort(latenciesWrites)
	ts.cfg.Logger.Info("sorted write latency results", zap.Int("total-data-points", latenciesWrites.Len()), zap.String("took", time.Since(now).String()))

	ts.cfg.Logger.Info("sorting get latency results", zap.Int("total-data-points", latenciesGets.Len()))
	now = time.Now()
	sort.Sort(latenciesGets)
	ts.cfg.Logger.Info("sorted get latency results", zap.Int("total-data-points", latenciesGets.Len()), zap.String("took", time.Since(now).String()))

	ts.cfg.Logger.Info("sorting range get latency results", zap.Int("total-data-points", latenciesRangeGets.Len()))
	now = time.Now()
	sort.Sort(latenciesRangeGets)
	ts.cfg.Logger.Info("sorted range get latency results", zap.Int("total-data-points", latenciesRangeGets.Len()), zap.String("took", time.Since(now).String()))

	testID := time.Now().UTC().Format(time.RFC3339Nano)

	ts.cfg.LatencySummaryWrites.TestID = testID
	ts.cfg.LatencySummaryWrites.P50 = latenciesWrites.PickP50()
	ts.cfg.LatencySummaryWrites.P90 = latenciesWrites.PickP90()
	ts.cfg.LatencySummaryWrites.P99 = latenciesWrites.PickP99()
	ts.cfg.LatencySummaryWrites.P999 = latenciesWrites.PickP999()
	ts.cfg.LatencySummaryWrites.P9999 = latenciesWrites.PickP9999()

	ts.cfg.LatencySummaryGets.TestID = testID
	ts.cfg.LatencySummaryGets.P50 = latenciesGets.PickP50()
	ts.cfg.LatencySummaryGets.P90 = latenciesGets.PickP90()
	ts.cfg.LatencySummaryGets.P99 = latenciesGets.PickP99()
	ts.cfg.LatencySummaryGets.P999 = latenciesGets.PickP999()
	ts.cfg.LatencySummaryGets.P9999 = latenciesGets.PickP9999()

	ts.cfg.LatencySummaryRangeGets.TestID = testID
	ts.cfg.LatencySummaryRangeGets.P50 = latenciesRangeGets.PickP50()
	ts.cfg.LatencySummaryRangeGets.P90 = latenciesRangeGets.PickP90()
	ts.cfg.LatencySummaryRangeGets.P99 = latenciesRangeGets.PickP99()
	ts.cfg.LatencySummaryRangeGets.P999 = latenciesRangeGets.PickP999()
	ts.cfg.LatencySummaryRangeGets.P9999 = latenciesRangeGets.PickP9999()

	// https://pkg.go.dev/github.com/prometheus/client_golang/prometheus?tab=doc#Gatherer
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		ts.cfg.Logger.Warn("failed to gather prometheus metrics", zap.Error(err))
		return err
	}
	for _, mf := range mfs {
		if mf == nil {
			continue
		}

		switch *mf.Name {
		case "stress_client_write_requests_success_total":
			gg := mf.Metric[0].GetGauge()
			ts.cfg.LatencySummaryWrites.SuccessTotal = gg.GetValue()
		case "stress_client_write_requests_failure_total":
			gg := mf.Metric[0].GetGauge()
			ts.cfg.LatencySummaryWrites.FailureTotal = gg.GetValue()
		case "stress_client_write_request_latency_milliseconds":
			ts.cfg.LatencySummaryWrites.Histogram, err = latency.ParseHistogram("milliseconds", mf.Metric[0].GetHistogram())
			if err != nil {
				return err
			}

		case "stress_client_get_requests_success_total":
			gg := mf.Metric[0].GetGauge()
			ts.cfg.LatencySummaryGets.SuccessTotal = gg.GetValue()
		case "stress_client_get_requests_failure_total":
			gg := mf.Metric[0].GetGauge()
			ts.cfg.LatencySummaryGets.FailureTotal = gg.GetValue()
		case "stress_client_get_request_latency_milliseconds":
			ts.cfg.LatencySummaryGets.Histogram, err = latency.ParseHistogram("milliseconds", mf.Metric[0].GetHistogram())
			if err != nil {
				return err
			}

		case "stress_client_range_get_requests_success_total":
			gg := mf.Metric[0].GetGauge()
			ts.cfg.LatencySummaryRangeGets.SuccessTotal = gg.GetValue()
		case "stress_client_range_get_requests_failure_total":
			gg := mf.Metric[0].GetGauge()
			ts.cfg.LatencySummaryRangeGets.FailureTotal = gg.GetValue()
		case "stress_client_range_get_request_latency_milliseconds":
			ts.cfg.LatencySummaryRangeGets.Histogram, err = latency.ParseHistogram("milliseconds", mf.Metric[0].GetHistogram())
			if err != nil {
				return err
			}
		}
	}

	fmt.Fprintf(ts.cfg.LogWriter, "\n\nLatencySummaryWrites:\n%s\n", ts.cfg.LatencySummaryWrites.Table())
	fmt.Fprintf(ts.cfg.LogWriter, "\n\nLatencySummaryGets:\n%s\n", ts.cfg.LatencySummaryGets.Table())
	fmt.Fprintf(ts.cfg.LogWriter, "\n\nLatencySummaryRangeGets:\n%s\n", ts.cfg.LatencySummaryRangeGets.Table())
	return nil
}

func (ts *tester) Delete() error {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}

	ts.donecCloseOnce.Do(func() {
		close(ts.donec)
	})

	var errs []string

	if err := client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
		ts.cfg.Namespace,
		client.DefaultNamespaceDeletionInterval,
		client.DefaultNamespaceDeletionTimeout,
		client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	return nil
}

func (ts *tester) runPrompt(action string) (ok bool) {
	if ts.cfg.Prompt {
		msg := fmt.Sprintf("Ready to %q resources for the namespace %q, should we continue?", action, ts.cfg.Namespace)
		prompt := promptui.Select{
			Label: msg,
			Items: []string{
				"No, cancel it!",
				fmt.Sprintf("Yes, let's %q!", action),
			},
		}
		idx, answer, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx != 1 {
			fmt.Printf("cancelled %q [index %d, answer %q]\n", action, idx, answer)
			return false
		}
	}
	return true
}

func (ts *tester) startUpdates(podImg string) (latenciesWrites latency.Durations, latenciesGets latency.Durations) {
	ts.cfg.Logger.Info("updating",
		zap.Int("objects", ts.cfg.Objects),
		zap.String("object-size", humanize.Bytes(uint64(ts.cfg.ObjectSize))),
		zap.Int("concurrency", ts.cfg.UpdateConcurrency),
	)
	latenciesWrites, latenciesGets = make(latency.Durations, 0, 20000), make(latency.Durations, 0, 20000)
	val := rand.String(ts.cfg.ObjectSize)

	shouldContinue := func(idx int) bool { return idx < ts.cfg.Objects }
	if ts.cfg.Objects < 0 {
		shouldContinue = func(_ int) bool { return true }
	}
	for i := 0; shouldContinue(i); i++ {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("updates stopped")
			return
		case <-ts.donec:
			ts.cfg.Logger.Info("updates done")
			return
		default:
		}

		podName := fmt.Sprintf("%s%d", ts.cfg.ObjectKeyPrefix, i%10)

		wg := &sync.WaitGroup{}
		wg.Add(ts.cfg.UpdateConcurrency)

		updateFunc := func() error {
			defer wg.Done()

			podClient := ts.cfg.Client.KubernetesClient().CoreV1().Pods(ts.cfg.Namespace)

			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), ts.cfg.Client.Config().ClientTimeout)
			pod, err := podClient.Get(ctx, podName, meta_v1.GetOptions{})
			cancel()
			took := time.Since(start)
			tookMS := float64(took / time.Millisecond)
			getRequestLatencyMs.Observe(tookMS)
			latenciesGets = append(latenciesGets, took)
			if err == nil {
				getRequestsSuccessTotal.Inc()
			} else {
				if k8s_errors.IsNotFound(err) {
					start = time.Now()
					ctx, cancel = context.WithTimeout(context.Background(), ts.cfg.Client.Config().ClientTimeout)
					_, err := podClient.Create(ctx, ts.createPodObject(podName, podImg, val), meta_v1.CreateOptions{})
					cancel()
					took = time.Since(start)
					tookMS = float64(took / time.Millisecond)
					writeRequestLatencyMs.Observe(tookMS)
					latenciesWrites = append(latenciesWrites, took)
					if err != nil {
						if !k8s_errors.IsAlreadyExists(err) {
							writeRequestsFailureTotal.Inc()
							ts.cfg.Logger.Warn("create pod failed", zap.String("namespace", ts.cfg.Namespace), zap.Error(err))
						}
					} else {
						writeRequestsSuccessTotal.Inc()
						if i%20 == 0 {
							ts.cfg.Logger.Info("created pod", zap.Int("iteration", i), zap.String("namespace", ts.cfg.Namespace))
						}
					}
					return nil
				}
				getRequestsFailureTotal.Inc()
				ts.cfg.Logger.Warn("get pod failed", zap.String("namespace", ts.cfg.Namespace), zap.Error(err))
				return err
			}

			// only update on "Get" success
			if pod.Annotations == nil {
				pod.Annotations = map[string]string{"key": "value"}
			} else {
				pod.Annotations = nil
			}
			start = time.Now()
			ctx, cancel = context.WithTimeout(context.Background(), ts.cfg.Client.Config().ClientTimeout)
			_, updateErr := podClient.Update(ctx, pod, meta_v1.UpdateOptions{})
			cancel()
			took = time.Since(start)
			tookMS = float64(took / time.Millisecond)
			writeRequestLatencyMs.Observe(tookMS)
			latenciesWrites = append(latenciesWrites, took)
			return updateErr
		}

		// exponential backoff to prevent apiserver overloads
		// conflict happens when other clients overwrites the existing value
		// ref. https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency
		for j := 0; j < ts.cfg.UpdateConcurrency; j++ {
			go retry.RetryOnConflict(retry.DefaultRetry, updateFunc)
		}
		wg.Wait()
	}
	return latenciesWrites, latenciesGets
}

func (ts *tester) startRangeGets() (latenciesRangeGets latency.Durations) {
	if ts.cfg.ListLimit < 0 {
		ts.cfg.Logger.Info("skipping range gets", zap.Int64("list-limit", ts.cfg.ListLimit))
		return latenciesRangeGets
	}

	ts.cfg.Logger.Info("listing for range gets", zap.Int64("list-limit", ts.cfg.ListLimit))
	latenciesRangeGets = make(latency.Durations, 0, 20000)

	for i := 0; true; i++ {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("updates stopped")
			return
		case <-ts.donec:
			ts.cfg.Logger.Info("updates done")
			return
		default:
		}

		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), ts.cfg.Client.Config().ClientTimeout)
		_, err := ts.cfg.Client.KubernetesClient().
			CoreV1().
			Pods(ts.cfg.Namespace).
			List(ctx, meta_v1.ListOptions{Limit: ts.cfg.ListLimit})
		cancel()
		took := time.Since(start)
		tookMS := float64(took / time.Millisecond)
		rangeGetRequestLatencyMs.Observe(tookMS)
		latenciesRangeGets = append(latenciesRangeGets, took)
		if err != nil {
			rangeGetRequestsFailureTotal.Inc()
			ts.cfg.Logger.Warn("list pod failed", zap.String("namespace", ts.cfg.Namespace), zap.Error(err))
		} else {
			rangeGetRequestsSuccessTotal.Inc()
			if i%20 == 0 {
				ts.cfg.Logger.Info("listed pod", zap.Int("iteration", i), zap.String("namespace", ts.cfg.Namespace))
			}
		}
	}
	return latenciesRangeGets
}

const busyboxImageName = "busybox"

func (ts *tester) checkECRImage() (img string, err error) {
	// check ECR permission
	// ref. https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eks/jobs-echo/jobs-echo.go#L75-L90
	img, _, err = ts.cfg.Repository.Describe(ts.cfg.Logger, ts.ecrAPI)
	if err != nil {
		ts.cfg.Logger.Warn("failed to describe ECR image", zap.Error(err))
		img = busyboxImageName
	}
	return img, nil
}

// "string" in Go is just a pointer, so it's not being copied here
func (ts *tester) createPodObject(podName string, busyboxImg string, val string) (po *core_v1.Pod) {
	return &core_v1.Pod{
		TypeMeta: meta_v1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      podName,
			Namespace: ts.cfg.Namespace,
			Labels: map[string]string{
				"name": podName,
			},
		},
		Spec: core_v1.PodSpec{
			// spec.template.spec.restartPolicy: Unsupported value: "Always": supported values: "OnFailure", "Never"
			RestartPolicy: core_v1.RestartPolicyOnFailure,
			Containers: []core_v1.Container{
				{
					Name:            podName,
					Image:           busyboxImg,
					ImagePullPolicy: core_v1.PullAlways,
					Command: []string{
						"/bin/sh",
						"-ec",
						fmt.Sprintf("echo -n '%s' >> /config/output.txt", val),
					},
					VolumeMounts: []core_v1.VolumeMount{
						{
							Name:      "config",
							MountPath: "/config",
						},
					},
				},
			},

			Volumes: []core_v1.Volume{
				{
					Name: "config",
					VolumeSource: core_v1.VolumeSource{
						EmptyDir: &core_v1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}
}
