// Package cronjobs creates CronJob objects in Kubernetes.
package cronjobs

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	batch_v1 "k8s.io/api/batch/v1"
	batch_v1beta1 "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// Config defines 'CronJob' configuration.
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

// New creates a new Job tester.
func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg Config
}

func (ts *tester) Create() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnCronJobs() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnCronJobs.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnCronJobs.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnCronJobs.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnCronJobs.Namespace,
	); err != nil {
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
		ts.cfg.K8SClient,
		waitDur,
		5*time.Second,
		ts.cfg.EKSConfig.AddOnCronJobs.Namespace,
		cronJobName,
		int(ts.cfg.EKSConfig.AddOnCronJobs.Completes),
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
	if !ts.cfg.EKSConfig.IsEnabledAddOnCronJobs() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnCronJobs.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnCronJobs.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
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

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnCronJobs.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete CronJobs namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnCronJobs.Created = false
	return ts.cfg.EKSConfig.Sync()
}

const (
	jobName              = "job-echo"
	cronJobName          = "cronjob-echo"
	cronJobEchoImageName = "busybox"
)

func (ts *tester) createCronJobs() (batch_v1beta1.CronJob, string, error) {
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
						fmt.Sprintf("echo -n '%s' >> /config/output.txt", randutil.String(ts.cfg.EKSConfig.AddOnCronJobs.EchoSize)),
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
	jobSpec := batch_v1beta1.JobTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: ts.cfg.EKSConfig.AddOnCronJobs.Namespace,
		},
		Spec: batch_v1.JobSpec{
			Completions: aws.Int32(int32(ts.cfg.EKSConfig.AddOnCronJobs.Completes)),
			Parallelism: aws.Int32(int32(ts.cfg.EKSConfig.AddOnCronJobs.Parallels)),
			Template:    podSpec,
			// TODO: 'TTLSecondsAfterFinished' is still alpha
			// https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/
		},
	}
	cronObj := batch_v1beta1.CronJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1beta1",
			Kind:       "CronJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronJobName,
			Namespace: ts.cfg.EKSConfig.AddOnCronJobs.Namespace,
		},
		Spec: batch_v1beta1.CronJobSpec{
			Schedule:                   ts.cfg.EKSConfig.AddOnCronJobs.Schedule,
			SuccessfulJobsHistoryLimit: aws.Int32(ts.cfg.EKSConfig.AddOnCronJobs.SuccessfulJobsHistoryLimit),
			FailedJobsHistoryLimit:     aws.Int32(ts.cfg.EKSConfig.AddOnCronJobs.FailedJobsHistoryLimit),
			JobTemplate:                jobSpec,
		},
	}
	b, err := yaml.Marshal(cronObj)
	return cronObj, string(b), err
}

func (ts *tester) AggregateResults() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnCronJobs() {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnCronJobs.Created {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.AggregateResults", zap.String("tester", pkgName))
	return nil
}

// TODO: use field selector, "status.phase!=Running"
// https://github.com/kubernetes/kubernetes/blob/d379ab2697251334774b7bd6f41b26cf39de470d/pkg/apis/batch/v1/conversion.go#L30-L41
func waitJobs(
	lg *zap.Logger,
	stopc chan struct{},
	k8sClient k8s_client.EKS,
	timeout time.Duration,
	interval time.Duration,
	namespace string,
	jobName string,
	targets int,
	desiredPodPhase v1.PodPhase,
) (pods []v1.Pod, err error) {
	lg.Info("waiting Pod",
		zap.String("namespace", namespace),
		zap.String("job-name", jobName),
	)
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < timeout {
		select {
		case <-stopc:
			return nil, errors.New("Pod polling aborted")
		case <-time.After(interval):
		}

		pods, err := k8sClient.ListPods(namespace, 150, 5*time.Second)
		if err != nil {
			lg.Warn("failed to list Pod", zap.Error(err))
			continue
		}
		if len(pods) == 0 {
			lg.Warn("got an empty list of Pod",
				zap.String("namespace", namespace),
				zap.String("job-name", jobName),
			)
			continue
		}

		count := 0
		for _, item := range pods {
			jv, ok := item.Labels["job-name"]
			match := ok && jv == jobName
			if !match {
				match = strings.HasPrefix(item.Name, jobName)
			}
			if !match {
				continue
			}
			if item.Status.Phase != desiredPodPhase {
				continue
			}
			count++
		}
		if count >= targets {
			lg.Info("found all targets", zap.Int("target", targets))
			break
		}

		lg.Info("polling",
			zap.String("namespace", namespace),
			zap.String("job-name", jobName),
			zap.Int("count", count),
			zap.Int("target", targets),
		)
	}

	return pods, nil
}
