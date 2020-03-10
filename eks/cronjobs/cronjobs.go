// Package cronjobs creates CronJob objects in Kubernetes.
package cronjobs

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
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
)

// Config defines 'CronJob' configuration.
type Config struct {
	Logger *zap.Logger

	Stopc chan struct{}
	Sig   chan os.Signal

	EKSConfig *eksconfig.Config
	K8SClient k8sClientSetGetter
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
	if ts.cfg.EKSConfig.AddOnCronJob.Created {
		ts.cfg.Logger.Info("skipping create AddOnCronJob")
		return nil
	}

	ts.cfg.EKSConfig.AddOnCronJob.Created = true
	ts.cfg.EKSConfig.Sync()

	createStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnCronJob.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnCronJob.CreateTookString = ts.cfg.EKSConfig.AddOnCronJob.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := ts.createNamespace(); err != nil {
		return err
	}
	obj, b, err := ts.createObject()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("creating CronJob",
		zap.String("name", cronJobName),
		zap.Int("completes", ts.cfg.EKSConfig.AddOnCronJob.Completes),
		zap.Int("parallels", ts.cfg.EKSConfig.AddOnCronJob.Parallels),
		zap.String("object-size", humanize.Bytes(uint64(len(b)))),
	)

	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		BatchV1beta1().
		CronJobs(ts.cfg.EKSConfig.AddOnCronJob.Namespace).
		Create(&obj)
	if err != nil {
		return fmt.Errorf("failed to create Job (%v)", err)
	}
	ts.cfg.Logger.Info("created Job")

	waitDur := 3*time.Minute + 10*time.Duration(ts.cfg.EKSConfig.AddOnCronJob.Completes)*time.Second

	completedJobs, err := waitJobs(
		ts.cfg.Logger,
		ts.cfg.Stopc,
		ts.cfg.Sig,
		ts.cfg.K8SClient.KubernetesClientSet(),
		waitDur,
		5*time.Second,
		ts.cfg.EKSConfig.AddOnCronJob.Namespace,
		cronJobName,
		int(ts.cfg.EKSConfig.AddOnCronJob.Completes),
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
	if !ts.cfg.EKSConfig.AddOnCronJob.Created {
		ts.cfg.Logger.Info("skipping delete AddOnCronJob")
		return nil
	}
	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnCronJob.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnCronJob.DeleteTookString = ts.cfg.EKSConfig.AddOnCronJob.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	ts.cfg.Logger.Info("deleting Job", zap.String("name", cronJobName))
	err := ts.cfg.
		K8SClient.KubernetesClientSet().
		BatchV1beta1().
		CronJobs(ts.cfg.EKSConfig.AddOnCronJob.Namespace).
		Delete(
			cronJobName,
			&metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &propagationBackground,
			},
		)
	if err != nil {
		return fmt.Errorf("failed to delete CronJob %q (%v)", cronJobName, err)
	}
	ts.cfg.Logger.Info("deleted CronJob", zap.String("name", cronJobName))

	if err := ts.deleteNamespace(); err != nil {
		return err
	}
	ts.cfg.EKSConfig.AddOnCronJob.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createNamespace() error {
	ts.cfg.Logger.Info("creating namespace", zap.String("namespace", ts.cfg.EKSConfig.AddOnCronJob.Namespace))
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Namespaces().
		Create(&v1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: ts.cfg.EKSConfig.AddOnCronJob.Namespace,
				Labels: map[string]string{
					"name": ts.cfg.EKSConfig.AddOnCronJob.Namespace,
				},
			},
		})
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("created namespace", zap.String("namespace", ts.cfg.EKSConfig.AddOnCronJob.Namespace))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteNamespace() error {
	ts.cfg.Logger.Info("deleting namespace", zap.String("namespace", ts.cfg.EKSConfig.AddOnCronJob.Namespace))
	foreground := metav1.DeletePropagationForeground
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Namespaces().
		Delete(
			ts.cfg.EKSConfig.AddOnCronJob.Namespace,
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
	jobsFieldSelector    = "status.phase!=Running"
	jobName              = "job-echo"
	cronJobName          = "cronjob-echo"
	cronJobEchoImageName = "busybox"
)

func (ts *tester) createObject() (batchv1beta1.CronJob, string, error) {
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
						fmt.Sprintf("echo -n '%s' >> /config/output.txt", randString(ts.cfg.EKSConfig.AddOnCronJob.EchoSize)),
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
			Namespace: ts.cfg.EKSConfig.AddOnCronJob.Namespace,
		},
		Spec: batchv1.JobSpec{
			Completions: aws.Int32(int32(ts.cfg.EKSConfig.AddOnCronJob.Completes)),
			Parallelism: aws.Int32(int32(ts.cfg.EKSConfig.AddOnCronJob.Parallels)),
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
			Namespace: ts.cfg.EKSConfig.AddOnCronJob.Namespace,
		},
		Spec: batchv1beta1.CronJobSpec{
			Schedule:    ts.cfg.EKSConfig.AddOnCronJob.Schedule,
			JobTemplate: jobSpec,
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
