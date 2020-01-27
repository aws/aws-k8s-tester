// Package jobs creates example Job objects in Kubernetes.
package jobs

import (
	"fmt"
	"math/rand"
	"os"
	"time"

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

	K8SClient k8sClientSetGetter

	// Namespace is the namespace to create Jobs.
	Namespace string
	// JobName is the example Job type.
	JobName string

	// Completes the desired number of successfully finished pods.
	Completes int
	// Parallels is the the maximum desired number of pods the
	// job should run at any given time.
	Parallels int

	// EchoSize is the size of payload for echo Job.
	EchoSize int
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
	obj, b, err := ts.createObject()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("creating Job",
		zap.String("name", ts.cfg.JobName),
		zap.Int("completes", ts.cfg.Completes),
		zap.Int("parallels", ts.cfg.Parallels),
		zap.String("object-size", humanize.Bytes(uint64(len(b)))),
	)

	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		BatchV1().
		Jobs(ts.cfg.Namespace).
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
		ts.cfg.Namespace,
		ts.cfg.JobName,
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
	ts.cfg.Logger.Info("deleting Job", zap.String("name", ts.cfg.JobName))
	err := ts.cfg.
		K8SClient.KubernetesClientSet().
		BatchV1().
		Jobs(ts.cfg.Namespace).
		Delete(
			ts.cfg.JobName,
			&metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &propagationBackground,
			},
		)
	if err != nil {
		return fmt.Errorf("failed to delete Job %q (%v)", ts.cfg.JobName, err)
	}
	ts.cfg.Logger.Info("deleted Job", zap.String("name", ts.cfg.JobName))
	return nil
}

const (
	// https://github.com/kubernetes/kubernetes/blob/d379ab2697251334774b7bd6f41b26cf39de470d/pkg/apis/batch/v1/conversion.go#L30-L41
	jobsFieldSelector = "status.phase!=Running"

	// JobNamePi creates basic Job object using Perl.
	// https://kubernetes.io/docs/concepts/workloads/controllers/jobs-run-to-completion/
	JobNamePi      = "pi"
	jobPiImageName = "perl"

	// JobNameEcho creates Job object that simply echoes data.
	JobNameEcho      = "echo"
	jobEchoImageName = "busybox"
)

func (ts *tester) createObject() (batchv1.Job, string, error) {
	var spec v1.PodTemplateSpec
	switch ts.cfg.JobName {
	case JobNamePi:
		spec = v1.PodTemplateSpec{
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:            JobNamePi,
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
	case JobNameEcho:
		spec = v1.PodTemplateSpec{
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:            JobNameEcho,
						Image:           jobEchoImageName,
						ImagePullPolicy: v1.PullAlways,
						Command: []string{
							"/bin/sh",
							"-ec",
							fmt.Sprintf("echo -n '%s' >> /config/output.txt", randString(ts.cfg.EchoSize)),
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
	default:
		return batchv1.Job{}, "", fmt.Errorf("%q unknown Job name", ts.cfg.JobName)
	}

	obj := batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ts.cfg.JobName,
			Namespace: ts.cfg.Namespace,
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
