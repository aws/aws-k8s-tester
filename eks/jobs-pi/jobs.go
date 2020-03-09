// Package jobspi creates example Job objects in Kubernetes.
package jobspi

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
)

// Config defines 'Job' configuration.
type Config struct {
	Logger *zap.Logger

	Stopc chan struct{}
	Sig   chan os.Signal

	EKSConfig *eksconfig.Config
	K8SClient k8sClientSetGetter

	// Completes the desired number of successfully finished pods.
	Completes int
	// Parallels is the the maximum desired number of pods the
	// job should run at any given time.
	Parallels int
}

type k8sClientSetGetter interface {
	KubernetesClientSet() *clientset.Clientset
}

// Tester defines Job tester.
type Tester interface {
	// Create creates Job objects, and waits for completion.
	Create() error
	// Delete deletes all Job objects.
	Delete() error
}

// New creates a new Job tester.
func New(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg Config
}

func (ts *tester) Create() error {
	if ts.cfg.EKSConfig.AddOnJobPi.Created {
		ts.cfg.Logger.Info("skipping create AddOnJobPi")
		return nil
	}

	ts.cfg.EKSConfig.AddOnJobPi.Created = true
	ts.cfg.EKSConfig.Sync()

	createStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnJobPi.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnJobPi.CreateTookString = ts.cfg.EKSConfig.AddOnJobPi.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := ts.createNamespace(); err != nil {
		return err
	}
	obj, b, err := ts.createObject()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("creating Job",
		zap.String("name", jobName),
		zap.Int("completes", ts.cfg.Completes),
		zap.Int("parallels", ts.cfg.Parallels),
		zap.String("object-size", humanize.Bytes(uint64(len(b)))),
	)

	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		BatchV1().
		Jobs(ts.cfg.EKSConfig.AddOnJobPi.Namespace).
		Create(&obj)
	if err != nil {
		return fmt.Errorf("failed to create Job (%v)", err)
	}
	ts.cfg.Logger.Info("created Job")

	waitDur := 3*time.Minute + 10*time.Duration(ts.cfg.Completes)*time.Second

	completedJobs, err := waitJobs(
		ts.cfg.Logger,
		ts.cfg.Stopc,
		ts.cfg.Sig,
		ts.cfg.K8SClient.KubernetesClientSet(),
		waitDur,
		5*time.Second,
		ts.cfg.EKSConfig.AddOnJobPi.Namespace,
		jobName,
		int(ts.cfg.Completes),
		jobsFieldSelector,
		v1.PodSucceeded,
	)
	if err != nil {
		return err
	}

	println()
	for _, item := range completedJobs {
		fmt.Printf("Job Pod %q: %q\n", item.Name, item.Status.Phase)
	}
	println()

	return nil
}

var propagationBackground = metav1.DeletePropagationBackground

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnJobPi.Created {
		ts.cfg.Logger.Info("skipping delete AddOnJobPi")
		return nil
	}
	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnJobPi.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnJobPi.DeleteTookString = ts.cfg.EKSConfig.AddOnJobPi.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	ts.cfg.Logger.Info("deleting Job", zap.String("name", jobName))
	err := ts.cfg.
		K8SClient.KubernetesClientSet().
		BatchV1().
		Jobs(ts.cfg.EKSConfig.AddOnJobPi.Namespace).
		Delete(
			jobName,
			&metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &propagationBackground,
			},
		)
	if err != nil {
		return fmt.Errorf("failed to delete Job %q (%v)", jobName, err)
	}
	ts.cfg.Logger.Info("deleted Job", zap.String("name", jobName))

	if err := ts.deleteNamespace(); err != nil {
		return err
	}

	ts.cfg.EKSConfig.AddOnJobPi.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createNamespace() error {
	ts.cfg.Logger.Info("creating namespace", zap.String("namespace", ts.cfg.EKSConfig.AddOnJobPi.Namespace))
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Namespaces().
		Create(&v1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: ts.cfg.EKSConfig.AddOnJobPi.Namespace,
				Labels: map[string]string{
					"name": ts.cfg.EKSConfig.AddOnJobPi.Namespace,
				},
			},
		})
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("created namespace", zap.String("namespace", ts.cfg.EKSConfig.AddOnJobPi.Namespace))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteNamespace() error {
	ts.cfg.Logger.Info("deleting namespace", zap.String("namespace", ts.cfg.EKSConfig.AddOnJobPi.Namespace))
	foreground := metav1.DeletePropagationForeground
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Namespaces().
		Delete(
			ts.cfg.EKSConfig.AddOnJobPi.Namespace,
			&metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	if err != nil {
		// ref. https://github.com/aws/aws-k8s-tester/issues/79
		if !strings.Contains(err.Error(), ` not found`) {
			return err
		}
	}
	ts.cfg.Logger.Info("deleted namespace", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

const (
	// https://github.com/kubernetes/kubernetes/blob/d379ab2697251334774b7bd6f41b26cf39de470d/pkg/apis/batch/v1/conversion.go#L30-L41
	jobsFieldSelector = "status.phase!=Running"
	jobName           = "job-pi"
	jobPiImageName    = "perl"
)

func (ts *tester) createObject() (batchv1.Job, string, error) {
	spec := v1.PodTemplateSpec{
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            jobName,
					Image:           jobPiImageName,
					ImagePullPolicy: v1.PullAlways,
					Command: []string{
						"perl",
						"-Mbignum=bpi",
						"-wle",
						"print bpi(2000)",
					},
				},
			},
			// spec.template.spec.restartPolicy: Unsupported value: "Always": supported values: "OnFailure", "Never"
			RestartPolicy: v1.RestartPolicyOnFailure,
		},
	}

	obj := batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: ts.cfg.EKSConfig.AddOnJobPi.Namespace,
		},
		Spec: batchv1.JobSpec{
			Completions: aws.Int32(int32(ts.cfg.Completes)),
			Parallelism: aws.Int32(int32(ts.cfg.Parallels)),

			// TODO: 'TTLSecondsAfterFinished' is still alpha
			// https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/

			Template: spec,
		},
	}
	b, err := yaml.Marshal(obj)
	return obj, string(b), err
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
