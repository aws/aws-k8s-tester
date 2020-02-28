// Package metrics implements Kubernetes metrics operations.
package metrics

import (
	"bytes"
	"errors"
	"io"
	"os"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"go.uber.org/zap"
	clientset "k8s.io/client-go/kubernetes"
)

// Config defines metrics configuration.
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	Sig       chan os.Signal
	EKSConfig *eksconfig.Config
	K8SClient k8sClientSetGetter
}

type k8sClientSetGetter interface {
	KubernetesClientSet() *clientset.Clientset
}

// Tester defines metrics tester.
type Tester interface {
	// Fetch fetches all metrics output.
	Fetch() (map[string]*dto.MetricFamily, error)
}

// New creates a new Job tester.
func New(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg Config
}

func (ts *tester) Fetch() (map[string]*dto.MetricFamily, error) {
	ts.cfg.Logger.Info("fetching /metrics")

	var output []byte
	var err error

	waitDur := 5 * time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return nil, errors.New("/metric fetch aborted")
		case <-ts.cfg.Sig:
			return nil, errors.New("/metric fetch aborted")
		case <-time.After(5 * time.Second):
		}

		output, err = ts.cfg.K8SClient.KubernetesClientSet().
			CoreV1().
			RESTClient().
			Get().
			RequestURI("/metrics").
			Do().
			Raw()
		if err != nil {
			ts.cfg.Logger.Warn("failed to fetch /metrics", zap.Error(err))
			continue
		}
		break
	}
	if len(output) == 0 {
		return nil, errors.New("empty response from /metrics")
	}
	ts.cfg.Logger.Info("fetched /metrics")

	return parse(bytes.NewReader(output))
}

func parse(rd io.Reader) (map[string]*dto.MetricFamily, error) {
	tp := new(expfmt.TextParser)
	return tp.TextToMetricFamilies(rd)
}
