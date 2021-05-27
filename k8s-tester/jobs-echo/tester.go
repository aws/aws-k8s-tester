// Package jobs_echo installs a simple echo Pod with Job.
package jobs_echo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester/tester"
	aws_v1_ecr "github.com/aws/aws-k8s-tester/utils/aws/v1/ecr"
	"github.com/aws/aws-k8s-tester/utils/rand"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/dustin/go-humanize"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
	batch_v1 "k8s.io/api/batch/v1"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_client "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
)

type Config struct {
	EnablePrompt bool

	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}

	ClientConfig *client.Config
	ECRAPI       ecriface.ECRAPI

	// Namespace to create test resources.
	Namespace string `json:"namespace"`

	// RepositoryBusyboxPartition is used for deciding between "amazonaws.com" and "amazonaws.com.cn".
	RepositoryBusyboxPartition string `json:"repository-busybox-partition,omitempty"`
	// RepositoryBusyboxAccountID is the account ID for tester ECR image.
	// e.g. "busybox" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/busybox"
	RepositoryBusyboxAccountID string `json:"repository-busybox-account-id,omitempty"`
	// RepositoryBusyboxRegion is the ECR repository region to pull from.
	RepositoryBusyboxRegion string `json:"repository-busybox-region,omitempty"`
	// RepositoryBusyboxName is the repositoryName for tester ECR image.
	// e.g. "busybox" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/busybox"
	RepositoryBusyboxName string `json:"repository-busybox-name,omitempty"`
	// RepositoryBusyboxImageTag is the image tag for tester ECR image.
	// e.g. "latest" for image URI "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/busybox:latest"
	RepositoryBusyboxImageTag string `json:"repository-busybox-image-tag,omitempty"`

	// Completes is the desired number of successfully finished pods.
	Completes int32 `json:"completes"`
	// Parallels is the the maximum desired number of pods the
	// job should run at any given time.
	Parallels int32 `json:"parallels"`
	// EchoSize is the job object size in bytes.
	// "Request entity too large: limit is 3145728" (3.1 MB).
	// "The Job "echo" is invalid: metadata.annotations:
	// Too long: must have at most 262144 characters". (0.26 MB)
	EchoSize int32 `json:"echo-size"`
}

// writes total 100 MB data to etcd
// Completes: 1000,
// Parallels: 100,
// EchoSize: 100 * 1024, // 100 KB
const (
	DefaultCompletes int32 = 10
	DefaultParallels int32 = 10
	DefaultEchoSize  int32 = 100 * 1024
)

func New(cfg Config) k8s_tester.Tester {
	ccfg, err := client.CreateConfig(cfg.ClientConfig)
	if err != nil {
		cfg.Logger.Panic("failed to create client config", zap.Error(err))
	}
	cli, err := k8s_client.NewForConfig(ccfg)
	if err != nil {
		cfg.Logger.Panic("failed to create client", zap.Error(err))
	}

	return &tester{
		cfg: cfg,
		cli: cli,
	}
}

type tester struct {
	cfg Config
	cli k8s_client.Interface
}

var pkgName = path.Base(reflect.TypeOf(tester{}).PkgPath())

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Apply() (err error) {
	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}

	// check ECR permission
	// ref. https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eks/jobs-echo/jobs-echo.go#L75-L90
	bsyImg := jobBusyboxImageName
	if ts.cfg.RepositoryBusyboxAccountID != "" &&
		ts.cfg.RepositoryBusyboxRegion != "" &&
		ts.cfg.RepositoryBusyboxName != "" &&
		ts.cfg.RepositoryBusyboxImageTag != "" &&
		ts.cfg.ECRAPI != nil {
		bsyImg, _, err = aws_v1_ecr.Check(
			ts.cfg.Logger,
			ts.cfg.ECRAPI,
			ts.cfg.RepositoryBusyboxPartition,
			ts.cfg.RepositoryBusyboxAccountID,
			ts.cfg.RepositoryBusyboxRegion,
			ts.cfg.RepositoryBusyboxName,
			ts.cfg.RepositoryBusyboxImageTag,
		)
		if err != nil {
			return err
		}
	}

	if err := client.CreateNamespace(ts.cfg.Logger, ts.cli, ts.cfg.Namespace); err != nil {
		return err
	}

	if err := ts.createJob(bsyImg); err != nil {
		return err
	}

	timeout := 5*time.Minute + 5*time.Minute*time.Duration(ts.cfg.Completes)
	if timeout > 3*time.Hour {
		timeout = 3 * time.Hour
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	var pods []core_v1.Pod
	_, pods, err = client.WaitForJobCompletes(
		ctx,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.Stopc,
		ts.cli,
		time.Minute,
		5*time.Second,
		ts.cfg.Namespace,
		jobName,
		int(ts.cfg.Completes),
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

func (ts *tester) Delete() error {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}

	var errs []string

	foreground := meta_v1.DeletePropagationForeground
	ts.cfg.Logger.Info("deleting Job", zap.String("name", jobName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cli.
		BatchV1().
		Jobs(ts.cfg.Namespace).
		Delete(
			ctx,
			jobName,
			meta_v1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err == nil {
		ts.cfg.Logger.Info("deleted a Job", zap.String("name", jobName))
	} else {
		ts.cfg.Logger.Warn("failed to delete a Job", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if err := client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cli,
		ts.cfg.Namespace,
		client.DefaultNamespaceDeletionInterval,
		client.DefaultNamespaceDeletionTimeout,
		client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	return nil
}

func (ts *tester) runPrompt(action string) (ok bool) {
	if ts.cfg.EnablePrompt {
		msg := fmt.Sprintf("Ready to %q resources for the namespace %q, should we continue?", action, ts.cfg.Namespace)
		prompt := promptui.Select{
			Label: msg,
			Items: []string{
				"No, cancel it!",
				fmt.Sprintf("Yes, let's %q!", action),
			},
		}
		idx, answer, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx != 1 {
			fmt.Printf("cancelled %q [index %d, answer %q]\n", action, idx, answer)
			return false
		}
	}
	return true
}

const (
	jobName             = "job-echo"
	jobBusyboxImageName = "busybox"
)

func (ts *tester) createObject(busyboxImg string) (batch_v1.Job, string, error) {
	podSpec := core_v1.PodTemplateSpec{
		Spec: core_v1.PodSpec{
			// spec.template.spec.restartPolicy: Unsupported value: "Always": supported values: "OnFailure", "Never"
			RestartPolicy: core_v1.RestartPolicyOnFailure,
			Containers: []core_v1.Container{
				{
					Name:            jobName,
					Image:           busyboxImg,
					ImagePullPolicy: core_v1.PullAlways,
					Command: []string{
						"/bin/sh",
						"-ec",
						fmt.Sprintf("echo -n '%s' >> /config/output.txt", rand.String(int(ts.cfg.EchoSize))),
					},
					VolumeMounts: []core_v1.VolumeMount{
						{
							Name:      "config",
							MountPath: "/config",
						},
					},
				},
			},
		},
	}
	jobObj := batch_v1.Job{
		TypeMeta: meta_v1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      jobName,
			Namespace: ts.cfg.Namespace,
		},
		Spec: batch_v1.JobSpec{
			Completions: &ts.cfg.Completes,
			Parallelism: &ts.cfg.Parallels,
			Template:    podSpec,
			// TODO: 'TTLSecondsAfterFinished' is still alpha
			// https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/
		},
	}
	b, err := yaml.Marshal(jobObj)
	return jobObj, string(b), err
}

func (ts *tester) createJob(busyboxImg string) (err error) {
	obj, b, err := ts.createObject(busyboxImg)
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("creating a Job object",
		zap.String("image-name", busyboxImg),
		zap.String("job-name", jobName),
		zap.Int32("completes", ts.cfg.Completes),
		zap.Int32("parallels", ts.cfg.Parallels),
		zap.String("object-size", humanize.Bytes(uint64(len(b)))),
	)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cli.
		BatchV1().
		Jobs(ts.cfg.Namespace).
		Create(ctx, &obj, meta_v1.CreateOptions{})
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create Job (%v)", err)
	}

	ts.cfg.Logger.Info("created a Job object")
	return nil
}
