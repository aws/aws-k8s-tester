// Package configmaps implements tester for ConfigMap.
package configmaps

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	k8sclient "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

// Config defines "ConfigMap" configuration.
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

// Tester defines ConfigMap tester.
type Tester interface {
	// Create creates "ConfigMap" objects to test ConfigMap.
	Create() error
	// Delete deletes "ConfigMap" and Pods.
	Delete() error
}

// New creates a new ConfigMap tester.
func New(cfg Config) (Tester, error) {
	return &tester{cfg: cfg, cancel: make(chan struct{})}, nil
}

type tester struct {
	cfg               Config
	deploymentCreated time.Time
	cancel            chan struct{}
}

func (ts *tester) Create() error {
	if ts.cfg.EKSConfig.AddOnConfigMaps.Created {
		ts.cfg.Logger.Info("skipping create AddOnConfigMaps")
		return nil
	}

	ts.cfg.EKSConfig.AddOnConfigMaps.Created = true
	ts.cfg.EKSConfig.Sync()

	createStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnConfigMaps.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnConfigMaps.CreateTookString = ts.cfg.EKSConfig.AddOnConfigMaps.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8sclient.CreateNamespace(ts.cfg.Logger, ts.cfg.K8SClient.KubernetesClientSet(), ts.cfg.EKSConfig.AddOnConfigMaps.Namespace); err != nil {
		return err
	}
	if err := ts.createConfigMaps(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnConfigMaps.Created {
		ts.cfg.Logger.Info("skipping delete AddOnConfigMaps")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnConfigMaps.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnConfigMaps.DeleteTookString = ts.cfg.EKSConfig.AddOnConfigMaps.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8sclient.DeleteNamespaceAndWait(ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnConfigMaps.Namespace,
		k8sclient.DefaultNamespaceDeletionInterval,
		k8sclient.DefaultNamespaceDeletionTimeout); err != nil {
		return fmt.Errorf("failed to delete ConfigMaps namespace (%v)", err)
	}

	ts.cfg.EKSConfig.AddOnConfigMaps.Created = false
	return ts.cfg.EKSConfig.Sync()
}

// only letters and numbers for ConfigMap key names
var regex = regexp.MustCompile("[^a-zA-Z0-9]+")

const writesFailThreshold = 10

func (ts *tester) createConfigMaps() (err error) {
	size := humanize.Bytes(uint64(ts.cfg.EKSConfig.AddOnConfigMaps.Size))
	ts.cfg.Logger.Info("creating ConfigMaps",
		zap.Int("objects", ts.cfg.EKSConfig.AddOnConfigMaps.Objects),
		zap.String("each-size", size),
	)

	// valid config key must consist of alphanumeric characters
	pfx := strings.ToLower(regex.ReplaceAllString(ts.cfg.EKSConfig.Name, ""))
	val := randString(ts.cfg.EKSConfig.AddOnConfigMaps.Size)

	// overwrite if any
	ts.cfg.EKSConfig.AddOnConfigMaps.CreatedNames = make([]string, 0, ts.cfg.EKSConfig.AddOnConfigMaps.Objects)
	ts.cfg.EKSConfig.Sync()

	if ts.cfg.EKSConfig.AddOnConfigMaps.QPS <= 1 {
		err = ts.createConfigMapsSequential(pfx, val, writesFailThreshold)
	} else {
		err = ts.createConfigMapsParallel(pfx, val, writesFailThreshold)
	}
	ts.cfg.EKSConfig.Sync()
	return err
}

func (ts *tester) createConfigMapsSequential(pfx, val string, failThreshold int) error {
	qps := float64(ts.cfg.EKSConfig.AddOnConfigMaps.QPS)
	burst := int(ts.cfg.EKSConfig.AddOnConfigMaps.Burst)
	rateLimiter := rate.NewLimiter(rate.Limit(qps), burst)
	ts.cfg.Logger.Info("creating ConfigMaps sequential",
		zap.Float64("qps", qps),
		zap.Int("burst", burst),
	)

	fails := 0
	for i := 0; i < ts.cfg.EKSConfig.AddOnConfigMaps.Objects; i++ {
		if !rateLimiter.Allow() {
			ts.cfg.Logger.Debug("waiting for rate limiter creating ConfigMap", zap.Int("index", i))
			werr := rateLimiter.Wait(context.Background())
			ts.cfg.Logger.Debug("waited for rate limiter", zap.Int("index", i), zap.Error(werr))
		}

		key := fmt.Sprintf("%s%06d", pfx, i)
		configMap := &v1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      key,
				Namespace: ts.cfg.EKSConfig.AddOnConfigMaps.Namespace,
				Labels: map[string]string{
					"name": key,
				},
			},
			Data: map[string]string{key: val},
		}

		t1 := time.Now()
		_, err := ts.cfg.K8SClient.KubernetesClientSet().
			CoreV1().
			ConfigMaps(ts.cfg.EKSConfig.AddOnConfigMaps.Namespace).
			Create(configMap)
		t2 := time.Now()
		if err != nil {
			select {
			case <-ts.cancel:
				return errors.New("ConfigMap creation aborted")
			case <-ts.cfg.Stopc:
				return errors.New("ConfigMap creation aborted")
			default:
				fails++
				ts.cfg.Logger.Warn("create ConfigMap failed",
					zap.Int("fails", fails),
					zap.Int("threshold", failThreshold),
					zap.Error(err),
				)
				if fails >= failThreshold {
					close(ts.cancel)
					return fmt.Errorf("exceeded ConfigMap writes fail threshold %d (%v)", failThreshold, err)
				}
			}
			continue
		}
		fails = 0

		configMapName := configMap.GetObjectMeta().GetName()
		ts.cfg.EKSConfig.AddOnConfigMaps.CreatedNames = append(ts.cfg.EKSConfig.AddOnConfigMaps.CreatedNames, configMapName)
		ts.cfg.EKSConfig.Sync()

		if ts.cfg.EKSConfig.LogLevel == "debug" || i%200 == 0 {
			ts.cfg.Logger.Info("created ConfigMap", zap.String("key", configMapName), zap.Duration("took", t2.Sub(t1)))
		}
	}

	ts.cfg.Logger.Info("created ConfigMaps sequential",
		zap.Int("objects", ts.cfg.EKSConfig.AddOnConfigMaps.Objects),
		zap.Int("success", len(ts.cfg.EKSConfig.AddOnConfigMaps.CreatedNames)),
	)
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createConfigMapsParallel(pfx, val string, failThreshold int) error {
	qps := float64(ts.cfg.EKSConfig.AddOnConfigMaps.QPS)
	burst := int(ts.cfg.EKSConfig.AddOnConfigMaps.Burst)
	rateLimiter := rate.NewLimiter(rate.Limit(qps), burst)
	ts.cfg.Logger.Info("creating ConfigMaps parallel",
		zap.Float64("qps", qps),
		zap.Int("burst", burst),
	)

	rch := make(chan result, int(qps))
	for i := 0; i < ts.cfg.EKSConfig.AddOnConfigMaps.Objects; i++ {
		go func(i int) {
			if !rateLimiter.Allow() {
				ts.cfg.Logger.Debug("waiting for rate limiter creating ConfigMap", zap.Int("index", i))
				werr := rateLimiter.Wait(context.Background())
				ts.cfg.Logger.Debug("waited for rate limiter", zap.Int("index", i), zap.Error(werr))
			}
			select {
			case <-ts.cancel:
				return
			case <-ts.cfg.Stopc:
				return
			default:
			}

			key := fmt.Sprintf("%s%06d", pfx, i)
			configMap := &v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      key,
					Namespace: ts.cfg.EKSConfig.AddOnConfigMaps.Namespace,
					Labels: map[string]string{
						"name": key,
					},
				},
				Data: map[string]string{key: val},
			}

			t1 := time.Now()
			_, err := ts.cfg.K8SClient.KubernetesClientSet().
				CoreV1().
				ConfigMaps(ts.cfg.EKSConfig.AddOnConfigMaps.Namespace).
				Create(configMap)
			t2 := time.Now()
			if err != nil {
				select {
				case <-ts.cancel:
					ts.cfg.Logger.Warn("exiting")
					return
				case <-ts.cfg.Stopc:
					ts.cfg.Logger.Warn("exiting")
					return
				case rch <- result{configMap: configMap, err: err, took: t2.Sub(t1), start: t1, end: t2}:
				}
				return
			}

			select {
			case <-ts.cancel:
				ts.cfg.Logger.Warn("exiting")
				return
			case <-ts.cfg.Stopc:
				ts.cfg.Logger.Warn("exiting")
				return
			case rch <- result{configMap: configMap, err: nil, took: t2.Sub(t1), start: t1, end: t2}:
			}

			if ts.cfg.EKSConfig.LogLevel == "debug" || i%200 == 0 {
				ts.cfg.Logger.Info("created ConfigMap", zap.String("key", configMap.GetObjectMeta().GetName()), zap.Duration("took", t2.Sub(t1)))
			}
		}(i)
	}

	fails := 0
	for i := 0; i < ts.cfg.EKSConfig.AddOnConfigMaps.Objects; i++ {
		var rv result
		select {
		case rv = <-rch:
		case <-ts.cancel:
			ts.cfg.Logger.Warn("exiting")
			return errors.New("aborted")
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("exiting")
			return errors.New("aborted")
		}
		if rv.err != nil {
			fails++
			ts.cfg.Logger.Warn("create ConfigMap failed",
				zap.Int("fails", fails),
				zap.Int("threshold", failThreshold),
				zap.Error(rv.err),
			)
			if fails >= failThreshold {
				close(ts.cancel)
				return fmt.Errorf("exceeded ConfigMap writes fail threshold %d (%v)", failThreshold, rv.err)
			}
			continue
		}
		fails = 0

		configMapName := rv.configMap.GetObjectMeta().GetName()
		ts.cfg.EKSConfig.AddOnConfigMaps.CreatedNames = append(ts.cfg.EKSConfig.AddOnConfigMaps.CreatedNames, configMapName)
		ts.cfg.EKSConfig.Sync()
	}

	ts.cfg.Logger.Info("created ConfigMaps parallel",
		zap.Int("objects", ts.cfg.EKSConfig.AddOnConfigMaps.Objects),
		zap.Int("success", len(ts.cfg.EKSConfig.AddOnConfigMaps.CreatedNames)),
	)
	return ts.cfg.EKSConfig.Sync()
}

type result struct {
	configMap *v1.ConfigMap
	err       error
	took      time.Duration
	start     time.Time
	end       time.Time
}

const ll = "0123456789abcdefghijklmnopqrstuvwxyz"

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		rand.Seed(time.Now().UnixNano())
		b[i] = ll[rand.Intn(len(ll))]
	}
	return string(b)
}
