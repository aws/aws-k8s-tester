// Package jobspi creates example Job objects in Kubernetes.
package jobspi

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// Config defines 'Job' configuration.
type Config struct {
	Logger *zap.Logger

	Stopc chan struct{}
	Sig   chan os.Signal

	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
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
	if ts.cfg.EKSConfig.AddOnJobsPi.Created {
		ts.cfg.Logger.Info("skipping create AddOnJobsPi")
		return nil
	}

	ts.cfg.EKSConfig.AddOnJobsPi.Created = true
	ts.cfg.EKSConfig.Sync()

	createStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnJobsPi.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnJobsPi.CreateTookString = ts.cfg.EKSConfig.AddOnJobsPi.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(ts.cfg.Logger, ts.cfg.K8SClient.KubernetesClientSet(), ts.cfg.EKSConfig.AddOnJobsPi.Namespace); err != nil {
		return err
	}
	obj, b, err := ts.createObject()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("creating Job",
		zap.String("name", jobName),
		zap.Int("completes", ts.cfg.EKSConfig.AddOnJobsPi.Completes),
		zap.Int("parallels", ts.cfg.EKSConfig.AddOnJobsPi.Parallels),
		zap.String("object-size", humanize.Bytes(uint64(len(b)))),
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		BatchV1().
		Jobs(ts.cfg.EKSConfig.AddOnJobsPi.Namespace).
		Create(ctx, &obj, metav1.CreateOptions{})
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create Job (%v)", err)
	}
	ts.cfg.Logger.Info("created Job")

	waitDur := 3*time.Minute + 10*time.Duration(ts.cfg.EKSConfig.AddOnJobsPi.Completes)*time.Second

	completedJobs, err := waitJobs(
		ts.cfg.Logger,
		ts.cfg.Stopc,
		ts.cfg.Sig,
		ts.cfg.K8SClient.KubernetesClientSet(),
		waitDur,
		5*time.Second,
		ts.cfg.EKSConfig.AddOnJobsPi.Namespace,
		jobName,
		int(ts.cfg.EKSConfig.AddOnJobsPi.Completes),
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
	if !ts.cfg.EKSConfig.AddOnJobsPi.Created {
		ts.cfg.Logger.Info("skipping delete AddOnJobsPi")
		return nil
	}
	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnJobsPi.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnJobsPi.DeleteTookString = ts.cfg.EKSConfig.AddOnJobsPi.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	ts.cfg.Logger.Info("deleting Job", zap.String("name", jobName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.
		K8SClient.KubernetesClientSet().
		BatchV1().
		Jobs(ts.cfg.EKSConfig.AddOnJobsPi.Namespace).
		Delete(
			ctx,
			jobName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &propagationBackground,
			},
		)
	cancel()
	if err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Job pi %q (%v)", jobName, err))
	} else {
		ts.cfg.Logger.Info("deleted Job", zap.String("name", jobName))
	}

	if err := k8s_client.DeleteNamespaceAndWait(ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnJobsPi.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Jobs oi namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnJobsPi.Created = false
	return ts.cfg.EKSConfig.Sync()
}

const (
	// https://github.com/kubernetes/kubernetes/blob/d379ab2697251334774b7bd6f41b26cf39de470d/pkg/apis/batch/v1/conversion.go#L30-L41
	jobsFieldSelector = "status.phase!=Running"
	jobName           = "job-pi"
	jobPiImageName    = "perl"
)

func (ts *tester) createObject() (batchv1.Job, string, error) {
	podSpec := v1.PodTemplateSpec{
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
	jobObj := batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: ts.cfg.EKSConfig.AddOnJobsPi.Namespace,
		},
		Spec: batchv1.JobSpec{
			Completions: aws.Int32(int32(ts.cfg.EKSConfig.AddOnJobsPi.Completes)),
			Parallelism: aws.Int32(int32(ts.cfg.EKSConfig.AddOnJobsPi.Parallels)),
			Template:    podSpec,
			// TODO: 'TTLSecondsAfterFinished' is still alpha
			// https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/
		},
	}
	b, err := yaml.Marshal(jobObj)
	return jobObj, string(b), err
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
