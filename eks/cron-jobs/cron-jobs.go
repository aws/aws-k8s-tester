// Package cronjobs creates CronJob objects in Kubernetes.
package cronjobs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	aws_ecr "github.com/aws/aws-k8s-tester/pkg/aws/ecr"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
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
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	ECRAPI    ecriface.ECRAPI
}

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

// New creates a new Job tester.
func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
	return &tester{cfg: cfg, busyboxImg: "busybox"}
}

type tester struct {
	cfg Config

	busyboxImg string
}

func (ts *tester) Create() (err error) {
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

	if ts.cfg.EKSConfig.AddOnCronJobs.RepositoryBusyboxAccountID != "" &&
		ts.cfg.EKSConfig.AddOnCronJobs.RepositoryBusyboxRegion != "" &&
		ts.cfg.EKSConfig.AddOnCronJobs.RepositoryBusyboxName != "" &&
		ts.cfg.EKSConfig.AddOnCronJobs.RepositoryBusyboxImageTag != "" {
		if ts.busyboxImg, _, err = aws_ecr.Check(
			ts.cfg.Logger,
			ts.cfg.ECRAPI,
			ts.cfg.EKSConfig.Partition,
			ts.cfg.EKSConfig.AddOnCronJobs.RepositoryBusyboxAccountID,
			ts.cfg.EKSConfig.AddOnCronJobs.RepositoryBusyboxRegion,
			ts.cfg.EKSConfig.AddOnCronJobs.RepositoryBusyboxName,
			ts.cfg.EKSConfig.AddOnCronJobs.RepositoryBusyboxImageTag,
		); err != nil {
			return err
		}
	}
	if err = k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnCronJobs.Namespace,
	); err != nil {
		return err
	}

	if err = ts.createCronJob(); err != nil {
		return err
	}
	timeout := 10*time.Minute + 5*time.Minute*time.Duration(ts.cfg.EKSConfig.AddOnCronJobs.Completes)
	if timeout > 3*time.Hour {
		timeout = 3 * time.Hour
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	var pods []v1.Pod
	_, pods, err = k8s_client.WaitForCronJobCompletes(
		ctx,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.Stopc,
		ts.cfg.K8SClient,
		3*time.Minute,
		5*time.Second,
		ts.cfg.EKSConfig.AddOnCronJobs.Namespace,
		cronJobName,
		int(ts.cfg.EKSConfig.AddOnCronJobs.Completes),
	)
	cancel()
	if err != nil {
		return err
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\n")
	for _, item := range pods {
		fmt.Fprintf(ts.cfg.LogWriter, "CronJob Pod %q: %q\n", item.Name, item.Status.Phase)
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\n")

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
	ts.cfg.EKSConfig.Sync()
	return nil
}

const cronJobName = "cronjob-echo"

func (ts *tester) createCronJob() (err error) {
	obj, b, err := ts.createObject()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("creating CronJob",
		zap.String("name", cronJobName),
		zap.String("schedule", ts.cfg.EKSConfig.AddOnCronJobs.Schedule),
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
	return nil
}

func (ts *tester) createObject() (batch_v1beta1.CronJob, string, error) {
	podSpec := v1.PodTemplateSpec{
		Spec: v1.PodSpec{
			// spec.template.spec.restartPolicy: Unsupported value: "Always": supported values: "OnFailure", "Never"
			RestartPolicy: v1.RestartPolicyOnFailure,
			Containers: []v1.Container{
				{
					Name:            cronJobName,
					Image:           ts.busyboxImg,
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
			Name:      cronJobName,
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
			ConcurrencyPolicy:          batch_v1beta1.ReplaceConcurrent,
		},
	}
	b, err := yaml.Marshal(cronObj)
	return cronObj, string(b), err
}
