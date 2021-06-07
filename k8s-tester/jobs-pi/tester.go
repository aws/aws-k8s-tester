// Package jobs_pi a simple pi Pod with Job.
// Replace https://github.com/aws/aws-k8s-tester/tree/v1.5.9/eks/jobs-pi.
package jobs_pi

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
	"github.com/aws/aws-k8s-tester/utils/rand"
	utils_time "github.com/aws/aws-k8s-tester/utils/time"
	"github.com/dustin/go-humanize"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
	batch_v1 "k8s.io/api/batch/v1"
	core_v1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

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

	// Completes is the desired number of successfully finished pods.
	Completes int32 `json:"completes"`
	// Parallels is the the maximum desired number of pods the
	// job should run at any given time.
	Parallels int32 `json:"parallels"`
}

func (cfg *Config) ValidateAndSetDefaults() error {
	if cfg.MinimumNodes == 0 {
		cfg.MinimumNodes = DefaultMinimumNodes
	}
	if cfg.Namespace == "" {
		return errors.New("empty Namespace")
	}

	return nil
}

const (
	DefaultMinimumNodes int   = 1
	DefaultCompletes    int32 = 10
	DefaultParallels    int32 = 10
)

func NewDefault() *Config {
	return &Config{
		Enable:       false,
		Prompt:       false,
		MinimumNodes: DefaultMinimumNodes,
		Namespace:    pkgName + "-" + rand.String(10) + "-" + utils_time.GetTS(10),
		Completes:    DefaultCompletes,
		Parallels:    DefaultParallels,
	}
}

func New(cfg *Config) k8s_tester.Tester {
	return &tester{
		cfg: cfg,
	}
}

type tester struct {
	cfg *Config
}

var pkgName = path.Base(reflect.TypeOf(tester{}).PkgPath())

func Env() string {
	return "ADD_ON_" + strings.ToUpper(strings.Replace(pkgName, "-", "_", -1))
}

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Enabled() bool { return ts.cfg.Enable }

func (ts *tester) Apply() error {
	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}

	if nodes, err := client.ListNodes(ts.cfg.Client.KubernetesClient()); len(nodes) < ts.cfg.MinimumNodes || err != nil {
		return fmt.Errorf("failed to validate minimum nodes requirement %d (nodes %v, error %v)", ts.cfg.MinimumNodes, len(nodes), err)
	}

	if err := client.CreateNamespace(ts.cfg.Logger, ts.cfg.Client.KubernetesClient(), ts.cfg.Namespace); err != nil {
		return err
	}

	if err := ts.createJob(); err != nil {
		return err
	}

	if err := ts.checkJob(); err != nil {
		return err
	}

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
	err := ts.cfg.Client.KubernetesClient().
		BatchV1().
		Jobs(ts.cfg.Namespace).
		Delete(
			ctx,
			jobName,
			meta_v1.DeleteOptions{
				GracePeriodSeconds: int64Ref(0),
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
	jobName        = "job-pi"
	jobPiImageName = "perl"
)

func (ts *tester) createObject() (batch_v1.Job, string, error) {
	podSpec := core_v1.PodTemplateSpec{
		Spec: core_v1.PodSpec{
			// spec.template.spec.restartPolicy: Unsupported value: "Always": supported values: "OnFailure", "Never"
			RestartPolicy: core_v1.RestartPolicyOnFailure,
			Containers: []core_v1.Container{
				{
					Name:            jobName,
					Image:           jobPiImageName,
					ImagePullPolicy: core_v1.PullAlways,
					Command: []string{
						"perl",
						"-Mbignum=bpi",
						"-wle",
						"print bpi(2000)",
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

func (ts *tester) createJob() (err error) {
	obj, b, err := ts.createObject()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("creating a Job object",
		zap.String("name", jobName),
		zap.Int32("completes", ts.cfg.Completes),
		zap.Int32("parallels", ts.cfg.Parallels),
		zap.String("object-size", humanize.Bytes(uint64(len(b)))),
	)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.Client.KubernetesClient().
		BatchV1().
		Jobs(ts.cfg.Namespace).
		Create(ctx, &obj, meta_v1.CreateOptions{})
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

func (ts *tester) checkJob() error {
	timeout := 5*time.Minute + 5*time.Minute*time.Duration(ts.cfg.Completes)
	if timeout > 3*time.Hour {
		timeout = 3 * time.Hour
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	var pods []core_v1.Pod
	_, pods, err := client.WaitForJobCompletes(
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

func int64Ref(v int64) *int64 {
	return &v
}
