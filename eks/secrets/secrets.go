// Package secrets implements Secrets plugin.
package secrets

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

// Config defines "Secrets" configuration.
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

// Tester defines "Secret" tester.
type Tester interface {
	// Create creates "Secret" objects to test "Secret" writes,
	// and "Pod" objects to test "Secret" reads.
	// ref. https://kubernetes.io/docs/concepts/workloads/controllers/deployment/
	// Then, aggregate all reads results from remote nodes.
	Create() error
	// Delete deletes Secrets and Pods by deleting the namespace.
	Delete() error
	// AggregateResults aggregates all test results from remote nodes.
	AggregateResults() error
}

// New creates a new Job tester.
func New(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg Config
}

func (ts *tester) Create() error {
	if ts.cfg.EKSConfig.AddOnSecrets.Created {
		ts.cfg.Logger.Info("skipping create AddOnSecrets")
		return nil
	}

	ts.cfg.EKSConfig.AddOnSecrets.Created = true
	ts.cfg.EKSConfig.Sync()

	createStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnSecrets.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnSecrets.CreateTookString = ts.cfg.EKSConfig.AddOnSecrets.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := ts.createNamespace(); err != nil {
		return err
	}
	if err := ts.createSecrets(); err != nil {
		return err
	}
	if err := ts.createPods(); err != nil {
		return err
	}
	if err := ts.waitForPodsCompleted(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnSecrets.Created {
		ts.cfg.Logger.Info("skipping delete AddOnSecrets")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnSecrets.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnSecrets.DeleteTookString = ts.cfg.EKSConfig.AddOnSecrets.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := ts.deleteNamespace(); err != nil {
		return err
	}

	ts.cfg.EKSConfig.AddOnSecrets.Created = false
	return ts.cfg.EKSConfig.Sync()
}

// ResultSuffixRead is the suffix of the result file for "Secret" reads.
const ResultSuffixRead = "-secret-read.csv"

// only letters and numbers for Secret key names
var regex = regexp.MustCompile("[^a-zA-Z0-9]+")

const secretWritesFailThreshold = 20

func (ts *tester) createSecrets() error {
	size := humanize.Bytes(uint64(ts.cfg.EKSConfig.AddOnSecrets.Size))
	ts.cfg.Logger.Info("creating Secrets",
		zap.Int("objects", ts.cfg.EKSConfig.AddOnSecrets.Objects),
		zap.String("each-size", size),
	)

	// valid config key must consist of alphanumeric characters
	pfx := strings.ToLower(regex.ReplaceAllString(ts.cfg.EKSConfig.Name, ""))

	// no need generate random bytes in goroutine,
	// which can pressure host machine
	var valSfx string
	if ts.cfg.EKSConfig.AddOnSecrets.Size > 6 {
		valSfx = strings.Repeat("0", ts.cfg.EKSConfig.AddOnSecrets.Size-6)
	}

	// overwrite if any
	ts.cfg.EKSConfig.AddOnSecrets.CreatedSecretNames = make([]string, 0, ts.cfg.EKSConfig.AddOnSecrets.Objects)
	ts.cfg.EKSConfig.Sync()

	if ts.cfg.EKSConfig.AddOnSecrets.SecretQPS <= 1 {
		if err := ts.createSecretsSequential(pfx, valSfx, secretWritesFailThreshold); err != nil {
			return err
		}
		return ts.cfg.EKSConfig.Sync()
	}

	if err := ts.createSecretsParallel(pfx, valSfx, secretWritesFailThreshold); err != nil {
		return err
	}
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createSecretsSequential(pfx, valSfx string, failThreshold int) error {
	qps := float64(ts.cfg.EKSConfig.AddOnSecrets.SecretQPS)
	burst := int(ts.cfg.EKSConfig.AddOnSecrets.SecretBurst)
	rateLimiter := rate.NewLimiter(rate.Limit(qps), burst)
	ts.cfg.Logger.Info("creating Secret sequential",
		zap.Float64("qps", qps),
		zap.Int("burst", burst),
	)

	f, err := os.OpenFile(ts.cfg.EKSConfig.AddOnSecrets.WritesResultPath, os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		f, err = os.Create(ts.cfg.EKSConfig.AddOnSecrets.WritesResultPath)
		if err != nil {
			return err
		}
	}
	defer f.Close()
	wr := csv.NewWriter(f)
	if err = wr.Write([]string{"secret-name", "write-took-in-seconds", "start", "end"}); err != nil {
		return err
	}

	fails := 0
	for i := 0; i < ts.cfg.EKSConfig.AddOnSecrets.Objects; i++ {
		if !rateLimiter.Allow() {
			ts.cfg.Logger.Debug("waiting for rate limiter creating Secret", zap.Int("index", i))
			werr := rateLimiter.Wait(context.Background())
			ts.cfg.Logger.Debug("waited for rate limiter", zap.Int("index", i), zap.Error(werr))
		}

		key := fmt.Sprintf("%s%06d", pfx, i)
		val := []byte(fmt.Sprintf("%06d", i) + valSfx)

		secret := &v1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      key,
				Namespace: ts.cfg.EKSConfig.AddOnSecrets.Namespace,
			},
			Type: v1.SecretTypeOpaque,
			Data: map[string][]byte{key: val},
		}

		t1 := time.Now()
		_, err := ts.cfg.K8SClient.KubernetesClientSet().
			CoreV1().
			Secrets(ts.cfg.EKSConfig.AddOnSecrets.Namespace).
			Create(secret)
		t2 := time.Now()
		if err != nil {
			select {
			case <-ts.cfg.Stopc:
				return errors.New("Secret creation aborted")
			default:
				fails++
				ts.cfg.Logger.Warn("create Secret failed",
					zap.Int("fails", fails),
					zap.Int("threshold", failThreshold),
					zap.Error(err),
				)
				if fails >= failThreshold {
					return fmt.Errorf("exceeded secret writes fail threshold %d (%v)", failThreshold, err)
				}
			}
			continue
		}
		fails = 0

		secretName := secret.GetObjectMeta().GetName()

		ts.cfg.EKSConfig.AddOnSecrets.CreatedSecretNames = append(ts.cfg.EKSConfig.AddOnSecrets.CreatedSecretNames, secretName)
		ts.cfg.EKSConfig.Sync()

		if err = wr.Write([]string{
			secretName,
			fmt.Sprintf("%f", t2.Sub(t1).Seconds()),
			t1.String(),
			t2.String(),
		}); err != nil {
			return err
		}
		if i%100 == 0 {
			wr.Flush()
		}

		if ts.cfg.EKSConfig.LogLevel == "debug" || i%200 == 0 {
			ts.cfg.Logger.Info("created Secret",
				zap.String("key", secret.GetObjectMeta().GetName()),
			)
		}
	}
	wr.Flush()

	ts.cfg.Logger.Info("created Secrets sequential",
		zap.Int("objects", ts.cfg.EKSConfig.AddOnSecrets.Objects),
		zap.Int("success", len(ts.cfg.EKSConfig.AddOnSecrets.CreatedSecretNames)),
		zap.String("writes-result-path", ts.cfg.EKSConfig.AddOnSecrets.WritesResultPath),
		zap.Error(wr.Error()),
	)
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createSecretsParallel(pfx, valSfx string, failThreshold int) error {
	qps := float64(ts.cfg.EKSConfig.AddOnSecrets.SecretQPS)
	burst := int(ts.cfg.EKSConfig.AddOnSecrets.SecretBurst)
	rateLimiter := rate.NewLimiter(rate.Limit(qps), burst)
	ts.cfg.Logger.Info("creating Secrets parallel",
		zap.Float64("qps", qps),
		zap.Int("burst", burst),
	)

	rch := make(chan result, int(qps))
	for i := 0; i < ts.cfg.EKSConfig.AddOnSecrets.Objects; i++ {
		go func(i int) {
			if !rateLimiter.Allow() {
				ts.cfg.Logger.Debug("waiting for rate limiter creating Secret", zap.Int("index", i))
				werr := rateLimiter.Wait(context.Background())
				ts.cfg.Logger.Debug("waited for rate limiter", zap.Int("index", i), zap.Error(werr))
			}

			key := fmt.Sprintf("%s%06d", pfx, i)
			val := []byte(fmt.Sprintf("%06d", i) + valSfx)

			secret := &v1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      key,
					Namespace: ts.cfg.EKSConfig.AddOnSecrets.Namespace,
				},
				Type: v1.SecretTypeOpaque,
				Data: map[string][]byte{key: val},
			}

			t1 := time.Now()
			_, err := ts.cfg.K8SClient.KubernetesClientSet().
				CoreV1().
				Secrets(ts.cfg.EKSConfig.AddOnSecrets.Namespace).
				Create(secret)
			t2 := time.Now()
			if err != nil {
				select {
				case <-ts.cfg.Stopc:
					ts.cfg.Logger.Warn("exiting")
					return
				case rch <- result{secret: secret, err: err, took: t2.Sub(t1), start: t1, end: t2}:
				}
				return
			}

			select {
			case <-ts.cfg.Stopc:
				ts.cfg.Logger.Warn("exiting")
				return
			case rch <- result{secret: secret, err: nil, took: t2.Sub(t1), start: t1, end: t2}:
			}

			if ts.cfg.EKSConfig.LogLevel == "debug" || i%200 == 0 {
				ts.cfg.Logger.Info("created Secret",
					zap.String("key", secret.GetObjectMeta().GetName()),
				)
			}
		}(i)
	}

	f, err := os.OpenFile(ts.cfg.EKSConfig.AddOnSecrets.WritesResultPath, os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		f, err = os.Create(ts.cfg.EKSConfig.AddOnSecrets.WritesResultPath)
		if err != nil {
			return err
		}
	}
	defer f.Close()
	wr := csv.NewWriter(f)
	if err = wr.Write([]string{"secret-name", "write-took-in-seconds", "start", "end"}); err != nil {
		return err
	}

	fails := 0
	for i := 0; i < ts.cfg.EKSConfig.AddOnSecrets.Objects; i++ {
		var rv result
		select {
		case rv = <-rch:
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("exiting")
			return errors.New("aborted")
		}
		if rv.err != nil {
			fails++
			ts.cfg.Logger.Warn("create Secret failed",
				zap.Int("fails", fails),
				zap.Int("threshold", failThreshold),
				zap.Error(rv.err),
			)
			if fails >= failThreshold {
				return fmt.Errorf("exceeded secret writes fail threshold %d (%v)", failThreshold, err)
			}
			continue
		}
		fails = 0

		secretName := rv.secret.GetObjectMeta().GetName()

		ts.cfg.EKSConfig.AddOnSecrets.CreatedSecretNames = append(ts.cfg.EKSConfig.AddOnSecrets.CreatedSecretNames, secretName)
		ts.cfg.EKSConfig.Sync()

		if err = wr.Write([]string{secretName, fmt.Sprintf("%f", rv.took.Seconds()), rv.start.String(), rv.end.String()}); err != nil {
			return err
		}
		if i%100 == 0 {
			wr.Flush()
		}
	}
	wr.Flush()

	ts.cfg.Logger.Info("created Secrets parallel",
		zap.Int("objects", ts.cfg.EKSConfig.AddOnSecrets.Objects),
		zap.Int("success", len(ts.cfg.EKSConfig.AddOnSecrets.CreatedSecretNames)),
		zap.String("writes-result-path", ts.cfg.EKSConfig.AddOnSecrets.WritesResultPath),
		zap.Error(wr.Error()),
	)
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createPods() error {
	ts.cfg.Logger.Info("mounting and read Secrets using Pod")

	fileOrCreate := v1.HostPathFileOrCreate
	pods := make([]*v1.Pod, len(ts.cfg.EKSConfig.AddOnSecrets.CreatedSecretNames))
	for i, secretName := range ts.cfg.EKSConfig.AddOnSecrets.CreatedSecretNames {
		podName := "pod-" + secretName
		csvFilePath := fmt.Sprintf("/var/log/%s%s", secretName, ResultSuffixRead)

		pods[i] = &v1.Pod{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Pod",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: ts.cfg.EKSConfig.AddOnSecrets.Namespace,
			},
			Spec: v1.PodSpec{
				RestartPolicy: v1.RestartPolicyOnFailure,
				Containers: []v1.Container{
					{
						Name:            podName,
						Image:           "amazonlinux",
						ImagePullPolicy: v1.PullIfNotPresent,
						Command: []string{
							"/bin/sh",
							"-c",
						},
						Args: []string{
							`date +%s.%N | tr -d '\n'` + " > " + csvFilePath + "; " +
								`echo , | tr -d '\n'` + " >> " + csvFilePath + "; " +

								fmt.Sprintf(`echo '%s-' | tr -d '\n'`, podName) + " >> " + csvFilePath + "; " +
								fmt.Sprintf(`cat /etc/secret-volume/%s | head -c 5`, secretName) + " >> " + csvFilePath + "; " +
								`echo , | tr -d '\n'` + " >> " + csvFilePath + "; " +

								`date +%s.%N | tr -d '\n'` + " >> " + csvFilePath + ";",
						},

						// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
						VolumeMounts: []v1.VolumeMount{
							{
								Name:      "secret-volume",
								MountPath: "/etc/secret-volume",
								ReadOnly:  true,
							},
							{
								Name:      "csv-file",
								MountPath: csvFilePath,
								ReadOnly:  false,
							},
							{
								Name:      "var-log",
								MountPath: "/var/log",
								ReadOnly:  false,
							},
						},
					},
				},

				// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
				Volumes: []v1.Volume{
					{ // to read
						Name: "secret-volume",
						VolumeSource: v1.VolumeSource{
							Secret: &v1.SecretVolumeSource{
								SecretName: secretName,
							},
						},
					},
					{ // to write
						Name: "csv-file",
						VolumeSource: v1.VolumeSource{
							HostPath: &v1.HostPathVolumeSource{
								Path: csvFilePath,
								Type: &fileOrCreate,
							},
						},
					},
					{ // to write
						Name: "var-log",
						VolumeSource: v1.VolumeSource{
							EmptyDir: &v1.EmptyDirVolumeSource{},
						},
					},
				},
			},
		}
	}

	// overwrite if any
	ts.cfg.EKSConfig.AddOnSecrets.CreatedPodNames = make([]string, 0, ts.cfg.EKSConfig.AddOnSecrets.Objects)
	ts.cfg.EKSConfig.Sync()

	if ts.cfg.EKSConfig.AddOnSecrets.PodQPS <= 1 {
		if err := ts.createPodsSequential(pods); err != nil {
			return err
		}
	} else {
		if err := ts.createPodsParallel(pods); err != nil {
			return err
		}
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createPodsSequential(pods []*v1.Pod) error {
	qps := float64(ts.cfg.EKSConfig.AddOnSecrets.PodQPS)
	burst := int(ts.cfg.EKSConfig.AddOnSecrets.PodBurst)
	rateLimiter := rate.NewLimiter(rate.Limit(qps), burst)
	ts.cfg.Logger.Info("creating Pods sequential",
		zap.Float64("qps", qps),
		zap.Int("burst", burst),
		zap.Int("pods", len(pods)),
	)

	for idx, pod := range pods {
		if !rateLimiter.Allow() {
			ts.cfg.Logger.Debug("waiting for rate limiter creating Pod")
			werr := rateLimiter.Wait(context.Background())
			ts.cfg.Logger.Debug("waited for rate limiter", zap.Error(werr))
		}

		_, err := ts.cfg.K8SClient.KubernetesClientSet().
			CoreV1().
			Pods(ts.cfg.EKSConfig.AddOnSecrets.Namespace).
			Create(pod)
		if err != nil {
			select {
			case <-ts.cfg.Stopc:
				ts.cfg.Logger.Warn("exiting")
				return errors.New("aborted")
			default:
				ts.cfg.Logger.Warn("create Pod failed", zap.Error(err))
			}
			continue
		}

		ts.cfg.EKSConfig.AddOnSecrets.CreatedPodNames = append(ts.cfg.EKSConfig.AddOnSecrets.CreatedPodNames, pod.GetObjectMeta().GetName())
		ts.cfg.EKSConfig.Sync()

		if ts.cfg.EKSConfig.LogLevel == "debug" || idx%200 == 0 {
			ts.cfg.Logger.Info("created Pod", zap.String("name", pod.GetObjectMeta().GetName()))
		}
	}

	ts.cfg.Logger.Info("created Pods sequential",
		zap.Int("objects", ts.cfg.EKSConfig.AddOnSecrets.Objects),
		zap.Int("success", len(ts.cfg.EKSConfig.AddOnSecrets.CreatedPodNames)),
	)
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createPodsParallel(pods []*v1.Pod) error {
	qps := float64(ts.cfg.EKSConfig.AddOnSecrets.PodQPS)
	burst := int(ts.cfg.EKSConfig.AddOnSecrets.PodBurst)
	rateLimiter := rate.NewLimiter(rate.Limit(qps), burst)
	ts.cfg.Logger.Info("creating Pods parallel",
		zap.Float64("qps", qps),
		zap.Int("burst", burst),
		zap.Int("pods", len(pods)),
	)

	rch := make(chan result, int(qps))
	for idx, s := range pods {
		go func(i int, pod *v1.Pod) {
			if !rateLimiter.Allow() {
				ts.cfg.Logger.Debug("waiting for rate limiter creating Pod")
				werr := rateLimiter.Wait(context.Background())
				ts.cfg.Logger.Debug("waited for rate limiter", zap.Error(werr))
			}

			_, err := ts.cfg.K8SClient.KubernetesClientSet().
				CoreV1().
				Pods(ts.cfg.EKSConfig.AddOnSecrets.Namespace).
				Create(pod)
			if err != nil {
				select {
				case <-ts.cfg.Stopc:
					ts.cfg.Logger.Warn("exiting")
					return
				case rch <- result{pod: pod, err: err}:
				}
				return
			}

			select {
			case <-ts.cfg.Stopc:
				ts.cfg.Logger.Warn("exiting")
				return
			case rch <- result{pod: pod, err: nil}:
			}

			if ts.cfg.EKSConfig.LogLevel == "debug" || i%200 == 0 {
				ts.cfg.Logger.Info("created Pod", zap.String("name", pod.GetObjectMeta().GetName()))
			}
		}(idx, s)
	}

	for range pods {
		var rv result
		select {
		case rv = <-rch:
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("exiting")
			return ts.cfg.EKSConfig.Sync()
		}
		if rv.err != nil {
			ts.cfg.Logger.Warn("create Pod failed", zap.Error(rv.err))
			continue
		}
		ts.cfg.EKSConfig.AddOnSecrets.CreatedPodNames = append(ts.cfg.EKSConfig.AddOnSecrets.CreatedPodNames, rv.pod.GetObjectMeta().GetName())
		ts.cfg.EKSConfig.Sync()
	}

	ts.cfg.Logger.Info("created Pods parallel",
		zap.Int("objects", ts.cfg.EKSConfig.AddOnSecrets.Objects),
		zap.Int("success", len(ts.cfg.EKSConfig.AddOnSecrets.CreatedPodNames)),
	)
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitForPodsCompleted() error {
	interval, timeout := 30*time.Second, 20*time.Minute
	target := len(ts.cfg.EKSConfig.AddOnSecrets.CreatedPodNames)
	if target > 3000 { // takes 20-min to create 3K pods, 0.4 seconds per Pod
		timeout += 500 * time.Millisecond * time.Duration(target-3000)
	}
	ts.cfg.Logger.Info("waiting for completed Pods", zap.Duration("timeout", timeout))

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	retryStart := time.Now()
	for time.Now().Sub(retryStart) < timeout {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("aborted")
		case sig := <-ts.cfg.Sig:
			return fmt.Errorf("aborted with %v", sig)
		case <-ticker.C:
		}

		pods, err := ts.cfg.K8SClient.KubernetesClientSet().
			CoreV1().
			Pods(ts.cfg.EKSConfig.AddOnSecrets.Namespace).
			List(metav1.ListOptions{})
		if err != nil {
			ts.cfg.Logger.Error("failed to list Pod", zap.Error(err))
			continue
		}
		if len(pods.Items) == 0 {
			ts.cfg.Logger.Warn("got an empty list of Pod")
			continue
		}

		completed := 0
		for _, p := range pods.Items {
			if p.Status.Phase == v1.PodSucceeded {
				completed++
			}
		}

		ts.cfg.Logger.Info("polling",
			zap.Int("completed", completed),
			zap.Int("target", target),
		)
		if completed == target {
			ts.cfg.Logger.Info("found all targets", zap.Int("target", target))
			break
		}
	}

	ts.cfg.Logger.Info("waited for completed Pods")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createNamespace() error {
	ts.cfg.Logger.Info("creating namespace", zap.String("namespace", ts.cfg.EKSConfig.AddOnSecrets.Namespace))
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Namespaces().
		Create(&v1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: ts.cfg.EKSConfig.AddOnSecrets.Namespace,
				Labels: map[string]string{
					"name": ts.cfg.EKSConfig.AddOnSecrets.Namespace,
				},
			},
		})
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("created namespace", zap.String("namespace", ts.cfg.EKSConfig.AddOnSecrets.Namespace))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteNamespace() error {
	ts.cfg.Logger.Info("deleting namespace", zap.String("namespace", ts.cfg.EKSConfig.AddOnSecrets.Namespace))
	foreground := metav1.DeletePropagationForeground
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Namespaces().
		Delete(
			ts.cfg.EKSConfig.AddOnSecrets.Namespace,
			&metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("deleted namespace", zap.String("namespace", ts.cfg.EKSConfig.AddOnSecrets.Namespace))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) AggregateResults() error {
	if !ts.cfg.EKSConfig.AddOnSecrets.Created {
		ts.cfg.Logger.Info("skipping aggregating AddOnSecrets")
		return nil
	}

	ts.cfg.Logger.Info("aggregating results from Pods")

	f, err := os.OpenFile(ts.cfg.EKSConfig.AddOnSecrets.ReadsResultPath, os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		f, err = os.Create(ts.cfg.EKSConfig.AddOnSecrets.ReadsResultPath)
		if err != nil {
			return err
		}
	}
	defer f.Close()
	wr := csv.NewWriter(f)
	if err = wr.Write([]string{"secret-name", "read-took-in-seconds", "start", "end"}); err != nil {
		return err
	}
	for _, v := range ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes {
		for _, fpaths := range v.Logs {
			for _, fpath := range fpaths {
				if !strings.HasSuffix(fpath, ResultSuffixRead) {
					continue
				}
				rf, err := os.OpenFile(fpath, os.O_RDONLY, 0444)
				if err != nil {
					return fmt.Errorf("failed to open %q (%v)", fpath, err)
				}
				rd := csv.NewReader(rf)

				rows, err := rd.ReadAll()
				if err != nil {
					return fmt.Errorf("failed to read CSV %q (%v)", fpath, err)
				}
				if len(rows) != 1 {
					ts.cfg.Logger.Warn("unexpected rows", zap.String("path", fpath), zap.String("rows", fmt.Sprintf("%v", rows)))
					wr.Flush()
					rf.Close()
					continue
				}

				row := rows[0]
				if len(row) != 3 {
					ts.cfg.Logger.Warn("unexpected column", zap.String("path", fpath), zap.String("row", fmt.Sprintf("%v", row)))
					wr.Flush()
					rf.Close()
					continue
				}

				start, secretName, end := row[0], row[1], row[2]
				startFv, err := strconv.ParseFloat(start, 64)
				if err != nil {
					ts.cfg.Logger.Warn("unexpected float value", zap.String("path", fpath), zap.String("start", start))
					wr.Flush()
					rf.Close()
					continue
				}
				endFv, err := strconv.ParseFloat(end, 64)
				if err != nil {
					ts.cfg.Logger.Warn("unexpected float value", zap.String("path", fpath), zap.String("end", end))
					wr.Flush()
					rf.Close()
					continue
				}

				if err = wr.Write([]string{secretName, fmt.Sprintf("%f", endFv-startFv), start, end}); err != nil {
					return err
				}

				wr.Flush()
				if err = wr.Error(); err != nil {
					return fmt.Errorf("CSV %q has unexpected error %v", fpath, err)
				}
				rf.Close()
			}
		}
	}

	ts.cfg.Logger.Info("aggregated results from Pods",
		zap.String("reads-result-path", ts.cfg.EKSConfig.AddOnSecrets.ReadsResultPath),
	)
	return ts.cfg.EKSConfig.Sync()
}

type result struct {
	secret *v1.Secret
	pod    *v1.Pod
	err    error
	took   time.Duration
	start  time.Time
	end    time.Time
}

// mountAWSCred mounts AWS credentials as a "Secret" object.
// TODO: not used for now
func (ts *tester) mountAWSCred() error {
	d, err := ioutil.ReadFile(ts.cfg.EKSConfig.Status.AWSCredentialPath)
	if err != nil {
		return err
	}
	size := humanize.Bytes(uint64(len(d)))

	ts.cfg.Logger.Info("creating and mounting AWS credential as Secret",
		zap.String("path", ts.cfg.EKSConfig.Status.AWSCredentialPath),
		zap.String("size", size),
	)

	// awsCredName is the name of the mounted AWS Credential Secret.
	const awsCredName = "aws-cred-aws-k8s-tester"

	/*
	  kubectl \
	    --namespace=[NAMESPACE] \
	    create secret generic aws-cred-aws-k8s-tester \
	    --from-file=aws-cred-aws-k8s-tester/[FILE-PATH]
	*/
	so, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Secrets(ts.cfg.EKSConfig.AddOnSecrets.Namespace).
		Create(&v1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      awsCredName,
				Namespace: ts.cfg.EKSConfig.AddOnSecrets.Namespace,
			},
			Type: v1.SecretTypeOpaque,
			Data: map[string][]byte{
				awsCredName: d,
			},
		})
	if err != nil {
		return fmt.Errorf("failed to create AWS credential as Secret (%v)", err)
	}

	/*
	  kubectl \
	    --namespace=[NAMESPACE] \
	    get secret aws-cred-aws-k8s-tester \
	    --output=yaml
	*/
	ts.cfg.Logger.Info("mounted AWS credential as Secret",
		zap.String("path", ts.cfg.EKSConfig.Status.AWSCredentialPath),
		zap.String("size", size),
		zap.String("created-timestamp", so.GetCreationTimestamp().String()),
	)
	return ts.cfg.EKSConfig.Sync()
}
