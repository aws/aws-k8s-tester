// Package configmaps a simple pi Pod with Job.
// Replace https://github.com/aws/aws-k8s-tester/tree/v1.5.9/eks/configmaps.
package configmaps

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
	"github.com/aws/aws-k8s-tester/utils/latency"
	"github.com/aws/aws-k8s-tester/utils/rand"
	utils_time "github.com/aws/aws-k8s-tester/utils/time"
	"github.com/manifoldco/promptui"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// Objects is the desired number of objects to create.
	Objects int `json:"objects"`
	// ObjectSize is the size in bytes per object.
	ObjectSize int `json:"object_size"`

	LatencySummary latency.Summary `json:"latency_summary" read-only:"true"`
}

const (
	DefaultMinimumNodes int = 1
	DefaultObjects      int = 10
	DefaultObjectSize   int = 10 * 1024 // 10 KB

	// writes total 300 MB data to etcd
	// Objects: 1000,
	// ObjectSize: 300000, // 0.3 MB
)

func NewDefault() *Config {
	return &Config{
		Enable:       false,
		Prompt:       false,
		MinimumNodes: DefaultMinimumNodes,
		Namespace:    pkgName + "-" + rand.String(10) + "-" + utils_time.GetTS(10),
		Objects:      DefaultObjects,
		ObjectSize:   DefaultObjectSize,
	}
}

func New(cfg *Config) k8s_tester.Tester {
	return &tester{
		cfg:            cfg,
		donec:          make(chan struct{}),
		donecCloseOnce: new(sync.Once),
	}
}

type tester struct {
	cfg            *Config
	donec          chan struct{}
	donecCloseOnce *sync.Once
}

var pkgName = path.Base(reflect.TypeOf(tester{}).PkgPath())

func Env() string {
	return "ADD_ON_" + strings.ToUpper(strings.Replace(pkgName, "-", "_", -1))
}

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Enabled() bool { return ts.cfg.Enable }

func (ts *tester) Apply() error {
	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}

	if nodes, err := client.ListNodes(ts.cfg.Client.KubernetesClient()); len(nodes) < ts.cfg.MinimumNodes || err != nil {
		return fmt.Errorf("failed to validate minimum nodes requirement %d (nodes %v, error %v)", ts.cfg.MinimumNodes, len(nodes), err)
	}

	if err := client.CreateNamespace(ts.cfg.Logger, ts.cfg.Client.KubernetesClient(), ts.cfg.Namespace); err != nil {
		return err
	}

	latencies := ts.startWrites()
	if len(latencies) == 0 {
		ts.cfg.Logger.Warn("no latency collected")
		return nil
	}

	ts.cfg.Logger.Info("sorting write latency results", zap.Int("total-data-points", latencies.Len()))
	now := time.Now()
	sort.Sort(latencies)
	ts.cfg.Logger.Info("sorted write latency results", zap.Int("total-data-points", latencies.Len()), zap.String("took", time.Since(now).String()))
	ts.cfg.LatencySummary.TestID = time.Now().UTC().Format(time.RFC3339Nano)
	ts.cfg.LatencySummary.P50 = latencies.PickP50()
	ts.cfg.LatencySummary.P90 = latencies.PickP90()
	ts.cfg.LatencySummary.P99 = latencies.PickP99()
	ts.cfg.LatencySummary.P999 = latencies.PickP999()
	ts.cfg.LatencySummary.P9999 = latencies.PickP9999()

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
		case "configmaps_client_write_requests_success_total":
			gg := mf.Metric[0].GetGauge()
			ts.cfg.LatencySummary.SuccessTotal = gg.GetValue()
		case "configmaps_client_write_requests_failure_total":
			gg := mf.Metric[0].GetGauge()
			ts.cfg.LatencySummary.FailureTotal = gg.GetValue()
		case "configmaps_client_write_request_latency_milliseconds":
			ts.cfg.LatencySummary.Histogram, err = latency.ParseHistogram("milliseconds", mf.Metric[0].GetHistogram())
			if err != nil {
				return err
			}
		}
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\n\nLatencySummary:\n%s\n", ts.cfg.LatencySummary.Table())

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

func (ts *tester) startWrites() (latencies latency.Durations) {
	ts.cfg.Logger.Info("writing", zap.Int("objects", ts.cfg.Objects), zap.Int("object-size", ts.cfg.Objects))
	latencies = make(latency.Durations, 0, 20000)

	val := rand.String(ts.cfg.ObjectSize)
	for i := 0; i < ts.cfg.Objects; i++ {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("writes stopped")
			return
		case <-ts.donec:
			ts.cfg.Logger.Info("writes done")
			return
		default:
		}

		key := fmt.Sprintf("configmap%d%s", i, rand.String(7))

		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), ts.cfg.Client.Config().ClientTimeout)
		_, err := ts.cfg.Client.KubernetesClient().
			CoreV1().
			ConfigMaps(ts.cfg.Namespace).
			Create(ctx, &core_v1.ConfigMap{
				TypeMeta: meta_v1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      key,
					Namespace: ts.cfg.Namespace,
					Labels: map[string]string{
						"name": key,
					},
				},
				Data: map[string]string{key: val},
			}, meta_v1.CreateOptions{})
		cancel()
		took := time.Since(start)
		tookMS := float64(took / time.Millisecond)
		writeRequestLatencyMs.Observe(tookMS)
		latencies = append(latencies, took)
		if err != nil {
			writeRequestsFailureTotal.Inc()
			ts.cfg.Logger.Warn("write configmap failed", zap.String("namespace", ts.cfg.Namespace), zap.Error(err))
		} else {
			writeRequestsSuccessTotal.Inc()
			if i%20 == 0 {
				ts.cfg.Logger.Info("wrote configmap", zap.Int("iteration", i), zap.String("namespace", ts.cfg.Namespace))
			}
		}
	}
	return latencies
}
