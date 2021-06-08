// Package jobs_echo installs a simple echo Pod with Job or CronJob.
// Replace https://github.com/aws/aws-k8s-tester/tree/v1.5.9/eks/jobs-echo.
// Replace https://github.com/aws/aws-k8s-tester/tree/v1.5.9/eks/cron-jobs.
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
	aws_v1 "github.com/aws/aws-k8s-tester/utils/aws/v1"
	aws_v1_ecr "github.com/aws/aws-k8s-tester/utils/aws/v1/ecr"
	"github.com/aws/aws-k8s-tester/utils/rand"
	utils_time "github.com/aws/aws-k8s-tester/utils/time"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/dustin/go-humanize"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	batch_v1 "k8s.io/api/batch/v1"
	batch_v1beta1 "k8s.io/api/batch/v1beta1"
	core_v1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// Config defines Job/CronJob spec.
type Config struct {
	Enable bool `json:"enable"`
	Prompt bool `json:"-"`

	Stopc     chan struct{} `json:"-"`
	Logger    *zap.Logger   `json:"-"`
	LogWriter io.Writer     `json:"-"`
	Client    client.Client `json:"-"`

	// MinimumNodes is the minimum number of Kubernetes nodes required for installing this addon.
	MinimumNodes int `json:"minimum_nodes"`
	// Namespace to create test resources.
	Namespace string `json:"namespace"`

	// Repository defines a custom ECR image repository.
	// For "busybox".
	Repository *aws_v1_ecr.Repository `json:"repository,omitempty"`

	// JobType is either "Job" or "CronJob".
	JobType string `json:"job_type"`

	// Completes is the desired number of successfully finished pods.
	Completes int32 `json:"completes"`
	// Parallels is the the maximum desired number of pods the
	// job should run at any given time.
	Parallels int32 `json:"parallels"`
	// EchoSize is the job object size in bytes.
	// "Request entity too large: limit is 3145728" (3.1 MB).
	// "The Job "echo" is invalid: metadata.annotations:
	// Too long: must have at most 262144 characters". (0.26 MB)
	EchoSize int32 `json:"echo_size"`

	// Schedule is the CronJob schedule.
	Schedule string `json:"schedule"`
	// SuccessfulJobsHistoryLimit is the number of successful finished CronJobs to retain.
	// Defaults to 3.
	SuccessfulJobsHistoryLimit int32 `json:"successful_jobs_history_limit"`
	// FailedJobsHistoryLimit is the number of failed finished CronJobs to retain.
	// Defaults to 1.
	FailedJobsHistoryLimit int32 `json:"failed_jobs_history_limit"`
}

func (cfg *Config) ValidateAndSetDefaults() error {
	if cfg.MinimumNodes == 0 {
		cfg.MinimumNodes = DefaultMinimumNodes
	}
	if cfg.Namespace == "" {
		return errors.New("empty Namespace")
	}

	if cfg.Completes == 0 {
		cfg.Completes = DefaultCompletes
	}
	if cfg.Parallels == 0 {
		cfg.Parallels = DefaultParallels
	}
	if cfg.EchoSize == 0 {
		cfg.EchoSize = DefaultEchoSize
	}
	if cfg.Schedule == "" {
		cfg.Schedule = DefaultSchedule
	}
	if cfg.SuccessfulJobsHistoryLimit == 0 {
		cfg.SuccessfulJobsHistoryLimit = DefaultSuccessfulJobsHistoryLimit
	}
	if cfg.FailedJobsHistoryLimit == 0 {
		cfg.FailedJobsHistoryLimit = DefaultFailedJobsHistoryLimit
	}

	return nil
}

// writes total 100 MB data to etcd
// Completes: 1000,
// Parallels: 100,
// EchoSize: 100 * 1024, // 100 KB
const (
	DefaultMinimumNodes               int    = 1
	DefaultJobType                    string = "Job"
	DefaultCompletes                  int32  = 10
	DefaultParallels                  int32  = 10
	DefaultEchoSize                   int32  = 100 * 1024
	DefaultSchedule                   string = "*/10 * * * *" // every 10-min
	DefaultSuccessfulJobsHistoryLimit int32  = 3
	DefaultFailedJobsHistoryLimit     int32  = 1
)

func NewDefault(jobType string) *Config {
	return &Config{
		Enable:                     false,
		Prompt:                     false,
		MinimumNodes:               DefaultMinimumNodes,
		Namespace:                  pkgName + "-" + rand.String(10) + "-" + utils_time.GetTS(10),
		Repository:                 &aws_v1_ecr.Repository{},
		JobType:                    jobType,
		Completes:                  DefaultCompletes,
		Parallels:                  DefaultParallels,
		EchoSize:                   DefaultEchoSize,
		Schedule:                   DefaultSchedule,
		SuccessfulJobsHistoryLimit: DefaultSuccessfulJobsHistoryLimit,
		FailedJobsHistoryLimit:     DefaultFailedJobsHistoryLimit,
	}
}

func New(cfg *Config) k8s_tester.Tester {
	ts := &tester{
		cfg: cfg,
	}
	if !cfg.Repository.IsEmpty() {
		awsCfg := aws_v1.Config{
			Logger:        cfg.Logger,
			DebugAPICalls: cfg.Logger.Core().Enabled(zapcore.DebugLevel),
			Partition:     cfg.Repository.Partition,
			Region:        cfg.Repository.Region,
		}
		awsSession, _, _, err := aws_v1.New(&awsCfg)
		if err != nil {
			cfg.Logger.Panic("failed to create aws session", zap.Error(err))
		}
		ts.ecrAPI = ecr.New(awsSession, aws.NewConfig().WithRegion(cfg.Repository.Region))
	}
	return ts
}

type tester struct {
	cfg    *Config
	ecrAPI ecriface.ECRAPI
}

var pkgName = path.Base(reflect.TypeOf(tester{}).PkgPath())

func Env(jobType string) string {
	if jobType == "Job" {
		return "ADD_ON_" + strings.ToUpper(strings.Replace(pkgName, "-", "_", -1))
	}
	return "ADD_ON_CRON_" + strings.ToUpper(strings.Replace(pkgName, "-", "_", -1))
}

func EnvRepository(jobType string) string {
	return Env(jobType) + "_REPOSITORY"
}

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Enabled() bool { return ts.cfg.Enable }

func (ts *tester) Apply() (err error) {
	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}

	img, err := ts.checkECRImage()
	if err != nil {
		return err
	}

	if nodes, err := client.ListNodes(ts.cfg.Client.KubernetesClient()); len(nodes) < ts.cfg.MinimumNodes || err != nil {
		return fmt.Errorf("failed to validate minimum nodes requirement %d (nodes %v, error %v)", ts.cfg.MinimumNodes, len(nodes), err)
	}

	if err := client.CreateNamespace(ts.cfg.Logger, ts.cfg.Client.KubernetesClient(), ts.cfg.Namespace); err != nil {
		return err
	}

	if err := ts.createJob(img); err != nil {
		return err
	}

	if err := ts.checkJob(); err != nil {
		return err
	}

	return nil
}

func (ts *tester) Delete() (err error) {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}

	foreground := meta_v1.DeletePropagationForeground

	var errs []string

	ts.cfg.Logger.Info("deleting Job", zap.String("job-type", ts.cfg.JobType), zap.String("name", jobName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)

	if ts.cfg.JobType == "Job" {
		err = ts.cfg.Client.KubernetesClient().
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
	} else {
		err = ts.cfg.Client.KubernetesClient().
			BatchV1beta1().
			CronJobs(ts.cfg.Namespace).
			Delete(
				ctx,
				jobName,
				meta_v1.DeleteOptions{
					GracePeriodSeconds: aws.Int64(0),
					PropagationPolicy:  &foreground,
				},
			)
	}

	cancel()
	if err == nil {
		ts.cfg.Logger.Info("deleted a Job", zap.String("job-type", ts.cfg.JobType), zap.String("name", jobName))
	} else {
		ts.cfg.Logger.Warn("failed to delete a Job", zap.String("job-type", ts.cfg.JobType), zap.Error(err))
		errs = append(errs, err.Error())
	}

	if err := client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
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
	if ts.cfg.Prompt {
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

func (ts *tester) checkECRImage() (img string, err error) {
	// check ECR permission
	// ref. https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eks/jobs-echo/jobs-echo.go#L75-L90
	img, _, err = ts.cfg.Repository.Describe(ts.cfg.Logger, ts.ecrAPI)
	if err != nil {
		ts.cfg.Logger.Warn("failed to describe ECR image", zap.Error(err))
		img = jobBusyboxImageName
	}
	return img, nil
}

func (ts *tester) createJobObject(busyboxImg string) (batch_v1.Job, batch_v1beta1.CronJob, string, error) {
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

			Volumes: []core_v1.Volume{
				{
					Name: "config",
					VolumeSource: core_v1.VolumeSource{
						EmptyDir: &core_v1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}

	if ts.cfg.JobType == "Job" {
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
		return jobObj, batch_v1beta1.CronJob{}, string(b), err
	}

	jobSpec := batch_v1beta1.JobTemplateSpec{
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
	jobObj := batch_v1beta1.CronJob{
		TypeMeta: meta_v1.TypeMeta{
			APIVersion: "batch/v1beta1",
			Kind:       "CronJob",
		},
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      jobName,
			Namespace: ts.cfg.Namespace,
		},
		Spec: batch_v1beta1.CronJobSpec{
			Schedule:                   ts.cfg.Schedule,
			SuccessfulJobsHistoryLimit: &ts.cfg.SuccessfulJobsHistoryLimit,
			FailedJobsHistoryLimit:     &ts.cfg.FailedJobsHistoryLimit,
			JobTemplate:                jobSpec,
			ConcurrencyPolicy:          batch_v1beta1.ReplaceConcurrent,
		},
	}
	b, err := yaml.Marshal(jobObj)
	return batch_v1.Job{}, jobObj, string(b), err
}

func (ts *tester) createJob(busyboxImg string) (err error) {
	jobObj, cronObj, b, err := ts.createJobObject(busyboxImg)
	if err != nil {
		return err
	}

	if ts.cfg.JobType == "Job" {
		ts.cfg.Logger.Info("creating a Job object",
			zap.String("image-name", busyboxImg),
			zap.String("job-name", jobName),
			zap.Int32("completes", ts.cfg.Completes),
			zap.Int32("parallels", ts.cfg.Parallels),
			zap.String("object-size", humanize.Bytes(uint64(len(b)))),
		)
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		_, err = ts.cfg.Client.KubernetesClient().
			BatchV1().
			Jobs(ts.cfg.Namespace).
			Create(ctx, &jobObj, meta_v1.CreateOptions{})
		cancel()
		if err != nil {
			if k8s_errors.IsAlreadyExists(err) {
				ts.cfg.Logger.Info("job already exists")
				return nil
			}
			return fmt.Errorf("failed to create Job (%v)", err)
		}
		ts.cfg.Logger.Info("created a Job object")
		return nil
	}

	ts.cfg.Logger.Info("creating a CronJob object",
		zap.String("image-name", busyboxImg),
		zap.String("job-name", jobName),
		zap.Int32("completes", ts.cfg.Completes),
		zap.Int32("parallels", ts.cfg.Parallels),
		zap.String("schedule", ts.cfg.Schedule),
		zap.Int32("successful-job-history-limit", ts.cfg.SuccessfulJobsHistoryLimit),
		zap.Int32("failed-job-history-limit", ts.cfg.FailedJobsHistoryLimit),
		zap.String("object-size", humanize.Bytes(uint64(len(b)))),
	)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.Client.KubernetesClient().
		BatchV1beta1().
		CronJobs(ts.cfg.Namespace).
		Create(ctx, &cronObj, meta_v1.CreateOptions{})
	cancel()
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("job already exists")
			return nil
		}
		return fmt.Errorf("failed to create CronJob (%v)", err)
	}
	ts.cfg.Logger.Info("created a CronJob object")
	return nil
}

func (ts *tester) checkJob() (err error) {
	timeout := 5*time.Minute + 5*time.Minute*time.Duration(ts.cfg.Completes)
	if ts.cfg.JobType == "CronJob" {
		timeout += 10 * time.Minute
	}
	if timeout > 3*time.Hour {
		timeout = 3 * time.Hour
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	var pods []core_v1.Pod
	if ts.cfg.JobType == "Job" {
		_, pods, err = client.WaitForJobCompletes(
			ctx,
			ts.cfg.Logger,
			ts.cfg.LogWriter,
			ts.cfg.Stopc,
			ts.cfg.Client.KubernetesClient(),
			time.Minute,
			5*time.Second,
			ts.cfg.Namespace,
			jobName,
			int(ts.cfg.Completes),
		)
	} else {
		_, pods, err = client.WaitForCronJobCompletes(
			ctx,
			ts.cfg.Logger,
			ts.cfg.LogWriter,
			ts.cfg.Stopc,
			ts.cfg.Client.KubernetesClient(),
			3*time.Minute,
			5*time.Second,
			ts.cfg.Namespace,
			jobName,
			int(ts.cfg.Completes),
		)
	}
	cancel()
	if err != nil {
		return err
	}

	fmt.Fprintf(ts.cfg.LogWriter, "\n")
	for _, item := range pods {
		fmt.Fprintf(ts.cfg.LogWriter, "%s Pod %q: %q\n", ts.cfg.JobType, item.Name, item.Status.Phase)
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\n")

	return nil
}
