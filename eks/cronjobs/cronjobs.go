// Package cronjobs creates CronJob objects in Kubernetes.
package cronjobs

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
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// Config defines 'CronJob' configuration.
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
	if ts.cfg.EKSConfig.AddOnCronJobs.Created {
		ts.cfg.Logger.Info("skipping create AddOnCronJob")
		return nil
	}

	ts.cfg.EKSConfig.AddOnCronJobs.Created = true
	ts.cfg.EKSConfig.Sync()

	createStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnCronJobs.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnCronJobs.CreateTookString = ts.cfg.EKSConfig.AddOnCronJobs.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(ts.cfg.Logger, ts.cfg.K8SClient.KubernetesClientSet(), ts.cfg.EKSConfig.AddOnCronJobs.Namespace); err != nil {
		return err
	}
	obj, b, err := ts.createCronJobs()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("creating CronJob",
		zap.String("name", cronJobName),
		zap.Int("completes", ts.cfg.EKSConfig.AddOnCronJobs.Completes),
		zap.Int("parallels", ts.cfg.EKSConfig.AddOnCronJobs.Parallels),
		zap.String("object-size", humanize.Bytes(uint64(len(b)))),
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		BatchV1beta1().
		CronJobs(ts.cfg.EKSConfig.AddOnCronJobs.Namespace).
		Create(ctx, &obj, metav1.CreateOptions{})
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create CronJob (%v)", err)
	}
	ts.cfg.Logger.Info("created CronJob")

	// take about 4-min for 10 cron jobs to trigger
	select {
	case <-ts.cfg.Stopc:
		ts.cfg.Logger.Warn("wait aborted")
		return nil
	case <-time.After(2 * time.Minute):
	}

	waitDur := 3*time.Minute + 10*time.Duration(ts.cfg.EKSConfig.AddOnCronJobs.Completes)*time.Second

	completedJobs, err := waitJobs(
		ts.cfg.Logger,
		ts.cfg.Stopc,
		ts.cfg.Sig,
		ts.cfg.K8SClient.KubernetesClientSet(),
		waitDur,
		5*time.Second,
		ts.cfg.EKSConfig.AddOnCronJobs.Namespace,
		cronJobName,
		int(ts.cfg.EKSConfig.AddOnCronJobs.Completes),
		jobsFieldSelector,
		v1.PodSucceeded,
	)
	if err != nil {
		return err
	}

	println()
	for _, item := range completedJobs {
		fmt.Printf("CronJob Pod %q: %q\n", item.Name, item.Status.Phase)
	}
	println()

	return nil
}

var propagationBackground = metav1.DeletePropagationBackground

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnCronJobs.Created {
		ts.cfg.Logger.Info("skipping delete AddOnCronJob")
		return nil
	}
	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnCronJobs.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnCronJobs.DeleteTookString = ts.cfg.EKSConfig.AddOnCronJobs.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	ts.cfg.Logger.Info("deleting Job", zap.String("name", cronJobName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.
		K8SClient.KubernetesClientSet().
		BatchV1beta1().
		CronJobs(ts.cfg.EKSConfig.AddOnCronJobs.Namespace).
		Delete(
			ctx,
			cronJobName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &propagationBackground,
			},
		)
	cancel()
	if err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete CronJob %q (%v)", cronJobName, err))
	}
	ts.cfg.Logger.Info("deleted CronJob", zap.String("name", cronJobName))

	if err := k8s_client.DeleteNamespaceAndWait(ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnCronJobs.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete CronJobs namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnCronJobs.Created = false
	return ts.cfg.EKSConfig.Sync()
}

const (
	// https://github.com/kubernetes/kubernetes/blob/d379ab2697251334774b7bd6f41b26cf39de470d/pkg/apis/batch/v1/conversion.go#L30-L41
	jobsFieldSelector    = "status.phase!=Running"
	jobName              = "job-echo"
	cronJobName          = "cronjob-echo"
	cronJobEchoImageName = "busybox"
)

func (ts *tester) createCronJobs() (batchv1beta1.CronJob, string, error) {
	podSpec := v1.PodTemplateSpec{
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            cronJobName,
					Image:           cronJobEchoImageName,
					ImagePullPolicy: v1.PullAlways,
					Command: []string{
						"/bin/sh",
						"-ec",
						fmt.Sprintf("echo -n '%s' >> /config/output.txt", randString(ts.cfg.EKSConfig.AddOnCronJobs.EchoSize)),
					},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "config",
							MountPath: "/config",
						},
					},
				},
			},
			// spec.template.spec.restartPolicy: Unsupported value: "Always": supported values: "OnFailure", "Never"
			RestartPolicy: v1.RestartPolicyOnFailure,

			Volumes: []v1.Volume{
				{
					Name: "config",
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}
	jobSpec := batchv1beta1.JobTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: ts.cfg.EKSConfig.AddOnCronJobs.Namespace,
		},
		Spec: batchv1.JobSpec{
			Completions: aws.Int32(int32(ts.cfg.EKSConfig.AddOnCronJobs.Completes)),
			Parallelism: aws.Int32(int32(ts.cfg.EKSConfig.AddOnCronJobs.Parallels)),
			Template:    podSpec,
			// TODO: 'TTLSecondsAfterFinished' is still alpha
			// https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/
		},
	}
	cronObj := batchv1beta1.CronJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1beta1",
			Kind:       "CronJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronJobName,
			Namespace: ts.cfg.EKSConfig.AddOnCronJobs.Namespace,
		},
		Spec: batchv1beta1.CronJobSpec{
			Schedule:                   ts.cfg.EKSConfig.AddOnCronJobs.Schedule,
			SuccessfulJobsHistoryLimit: aws.Int32(ts.cfg.EKSConfig.AddOnCronJobs.SuccessfulJobsHistoryLimit),
			FailedJobsHistoryLimit:     aws.Int32(ts.cfg.EKSConfig.AddOnCronJobs.FailedJobsHistoryLimit),
			JobTemplate:                jobSpec,
		},
	}
	b, err := yaml.Marshal(cronObj)
	return cronObj, string(b), err
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
