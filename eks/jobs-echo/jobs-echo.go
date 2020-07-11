// Package jobsecho creates Job objects in Kubernetes.
package jobsecho

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
	batch1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// Config defines 'Job' configuration.
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
	if !ts.cfg.EKSConfig.IsEnabledAddOnJobsEcho() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnJobsEcho.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnJobsEcho.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnJobsEcho.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if ts.cfg.EKSConfig.AddOnJobsEcho.RepositoryBusyboxAccountID != "" &&
		ts.cfg.EKSConfig.AddOnJobsEcho.RepositoryBusyboxRegion != "" &&
		ts.cfg.EKSConfig.AddOnJobsEcho.RepositoryBusyboxName != "" &&
		ts.cfg.EKSConfig.AddOnJobsEcho.RepositoryBusyboxImageTag != "" {
		if ts.busyboxImg, err = aws_ecr.Check(
			ts.cfg.Logger,
			ts.cfg.ECRAPI,
			ts.cfg.EKSConfig.AddOnJobsEcho.RepositoryBusyboxAccountID,
			ts.cfg.EKSConfig.AddOnJobsEcho.RepositoryBusyboxRegion,
			ts.cfg.EKSConfig.AddOnJobsEcho.RepositoryBusyboxName,
			ts.cfg.EKSConfig.AddOnJobsEcho.RepositoryBusyboxImageTag,
		); err != nil {
			return err
		}
	}
	if err = k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnJobsEcho.Namespace,
	); err != nil {
		return err
	}

	if err = ts.createJob(); err != nil {
		return err
	}
	timeout := 5*time.Minute + 5*time.Minute*time.Duration(ts.cfg.EKSConfig.AddOnJobsEcho.Completes)
	if timeout > 3*time.Hour {
		timeout = 3 * time.Hour
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	var pods []v1.Pod
	_, pods, err = k8s_client.WaitForJobCompletes(
		ctx,
		ts.cfg.Logger,
		ts.cfg.Stopc,
		ts.cfg.K8SClient,
		2*time.Minute,
		5*time.Second,
		ts.cfg.EKSConfig.AddOnJobsEcho.Namespace,
		jobName,
		int(ts.cfg.EKSConfig.AddOnJobsEcho.Completes),
	)
	cancel()
	if err != nil {
		return err
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\n")
	for _, item := range pods {
		fmt.Fprintf(ts.cfg.LogWriter, "Job Pod %q: %q\n", item.Name, item.Status.Phase)
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\n")

	return nil
}

func (ts *tester) Delete() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnJobsEcho() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnJobsEcho.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnJobsEcho.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err = ts.deleteJob(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Job %v", err))
	}

	if err = k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnJobsEcho.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Jobs echo namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnJobsEcho.Created = false
	return ts.cfg.EKSConfig.Sync()
}

const jobName = "job-echo"

func (ts *tester) createJob() (err error) {
	obj, b, err := ts.createObject()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("creating Job",
		zap.String("name", jobName),
		zap.Int("completes", ts.cfg.EKSConfig.AddOnJobsEcho.Completes),
		zap.Int("parallels", ts.cfg.EKSConfig.AddOnJobsEcho.Parallels),
		zap.String("object-size", humanize.Bytes(uint64(len(b)))),
	)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		BatchV1().
		Jobs(ts.cfg.EKSConfig.AddOnJobsEcho.Namespace).
		Create(ctx, &obj, metav1.CreateOptions{})
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create Job (%v)", err)
	}

	ts.cfg.Logger.Info("created Job")
	return nil
}

func (ts *tester) createObject() (batch1.Job, string, error) {
	podSpec := v1.PodTemplateSpec{
		Spec: v1.PodSpec{
			// spec.template.spec.restartPolicy: Unsupported value: "Always": supported values: "OnFailure", "Never"
			RestartPolicy: v1.RestartPolicyOnFailure,
			Containers: []v1.Container{
				{
					Name:            jobName,
					Image:           ts.busyboxImg,
					ImagePullPolicy: v1.PullAlways,
					Command: []string{
						"/bin/sh",
						"-ec",
						fmt.Sprintf("echo -n '%s' >> /config/output.txt", randutil.String(ts.cfg.EKSConfig.AddOnJobsEcho.EchoSize)),
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
	jobObj := batch1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: ts.cfg.EKSConfig.AddOnJobsEcho.Namespace,
		},
		Spec: batch1.JobSpec{
			Completions: aws.Int32(int32(ts.cfg.EKSConfig.AddOnJobsEcho.Completes)),
			Parallelism: aws.Int32(int32(ts.cfg.EKSConfig.AddOnJobsEcho.Parallels)),
			Template:    podSpec,
			// TODO: 'TTLSecondsAfterFinished' is still alpha
			// https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/
		},
	}
	b, err := yaml.Marshal(jobObj)
	return jobObj, string(b), err
}

func (ts *tester) deleteJob() (err error) {
	foreground := metav1.DeletePropagationForeground
	ts.cfg.Logger.Info("deleting Job", zap.String("name", jobName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err = ts.cfg.
		K8SClient.KubernetesClientSet().
		BatchV1().
		Jobs(ts.cfg.EKSConfig.AddOnJobsEcho.Namespace).
		Delete(
			ctx,
			jobName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err == nil {
		ts.cfg.Logger.Info("deleted Job", zap.String("name", jobName))
	} else {
		ts.cfg.Logger.Warn("failed to delete Job", zap.Error(err))
	}
	return err
}
