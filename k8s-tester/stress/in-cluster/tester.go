// Package in_cluster implements stress tester in remote worker nodes
// which runs workloads against its Kubernetes control plane, thus "in cluster".
// See "Dockerfile.k8s-tester-stress" for an example docker image.
// Replace https://github.com/aws/aws-k8s-tester/tree/v1.5.9/eks/stresser/remote.
package in_cluster

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
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
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	batch_v1 "k8s.io/api/batch/v1"
	batch_v1beta1 "k8s.io/api/batch/v1beta1"
	core_v1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbac_v1 "k8s.io/api/rbac/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
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

	// K8sTesterStressRepository defines a custom ECR image repository.
	// For "k8s-tester-stress".
	K8sTesterStressRepository *aws_v1_ecr.Repository `json:"k8s_tester_stress_repository,omitempty"`

	// Completes is the desired number of successfully finished pods.
	Completes int32 `json:"completes"`
	// Parallels is the the maximum desired number of pods the
	// job should run at any given time.
	Parallels int32 `json:"parallels"`
	// Schedule is the CronJob schedule.
	Schedule string `json:"schedule"`
	// SuccessfulJobsHistoryLimit is the number of successful finished CronJobs to retain.
	SuccessfulJobsHistoryLimit int32 `json:"successful_jobs_history_limit"`
	// FailedJobsHistoryLimit is the number of failed finished CronJobs to retain.
	FailedJobsHistoryLimit int32 `json:"failed_jobs_history_limit"`

	// K8sTesterStressCLI defines flags for "k8s-tester-stress".
	K8sTesterStressCLI *K8sTesterStressCLI `json:"k8s_tester_stress_cli"`
}

// K8sTesterStressCLI defines flags for "k8s-tester-stress".
type K8sTesterStressCLI struct {
	// BusyboxRepository defines a custom ECR image repository.
	// For "busybox".
	BusyboxRepository *aws_v1_ecr.Repository `json:"busybox_repository,omitempty"`

	// RunTimeout is the duration of stress runs.
	// After timeout, it stops all stress requests.
	RunTimeout       time.Duration `json:"run_timeout"`
	RunTimeoutString string        `json:"run_timeout_string" read-only:"true"`
	// ObjectKeyPrefix is the key prefix for "Pod" objects.
	ObjectKeyPrefix string `json:"object_key_prefix"`
	// Objects is the desired number of objects to create and update.
	// This doesn't apply to reads.
	// If negative, it creates until timeout.
	Objects int `json:"objects"`
	// ObjectSize is the size in bytes per object.
	ObjectSize int `json:"object_size"`
	// UpdateConcurrency is the number of concurrent routines to issue update requests.
	// Do not set too high, instead distribute this tester as distributed workers to maximize concurrency.
	UpdateConcurrency int `json:"update_concurrency"`
	// ListBatchLimit is the number of objects to return for each list response.
	// If negative, the tester disables list calls (only runs mutable requests).
	ListBatchLimit int64 `json:"list_batch_limit"`
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
	if cfg.Schedule == "" {
		cfg.Schedule = DefaultSchedule
	}
	if cfg.SuccessfulJobsHistoryLimit == 0 {
		cfg.SuccessfulJobsHistoryLimit = DefaultSuccessfulJobsHistoryLimit
	}
	if cfg.FailedJobsHistoryLimit == 0 {
		cfg.FailedJobsHistoryLimit = DefaultFailedJobsHistoryLimit
	}

	if cfg.K8sTesterStressCLI.RunTimeout == time.Duration(0) {
		cfg.K8sTesterStressCLI.RunTimeout = DefaultRunTimeout
	}
	cfg.K8sTesterStressCLI.RunTimeoutString = cfg.K8sTesterStressCLI.RunTimeout.String()

	if cfg.K8sTesterStressCLI.ObjectKeyPrefix == "" {
		cfg.K8sTesterStressCLI.ObjectKeyPrefix = DefaultObjectKeyPrefix()
	}

	if cfg.K8sTesterStressCLI.ObjectSize == 0 {
		return errors.New("zero ObjectSize")
	}
	if cfg.K8sTesterStressCLI.UpdateConcurrency == 0 {
		cfg.K8sTesterStressCLI.UpdateConcurrency = DefaultUpdateConcurrency
	}

	return nil
}

const (
	DefaultMinimumNodes int = 1

	DefaultCompletes                  int32  = 10
	DefaultParallels                  int32  = 10
	DefaultSchedule                   string = "*/10 * * * *" // every 10-min
	DefaultSuccessfulJobsHistoryLimit int32  = 10
	DefaultFailedJobsHistoryLimit     int32  = 10

	DefaultRunTimeout = time.Minute

	DefaultObjects    int = -1
	DefaultObjectSize int = 10 * 1024 // 10 KB

	// writes total 300 MB data to etcd
	// Objects: 1000,
	// ObjectSize: 300000, // 0.3 MB

	DefaultUpdateConcurrency int   = 10
	DefaultListBatchLimit    int64 = 1000
)

var defaultObjectKeyPrefix string = fmt.Sprintf("pod%s", rand.String(7))

func DefaultObjectKeyPrefix() string {
	return defaultObjectKeyPrefix
}

func NewDefault() *Config {
	return &Config{
		Enable:                     false,
		Prompt:                     false,
		MinimumNodes:               DefaultMinimumNodes,
		Namespace:                  pkgName + "-" + rand.String(10) + "-" + utils_time.GetTS(10),
		K8sTesterStressRepository:  &aws_v1_ecr.Repository{},
		Completes:                  DefaultCompletes,
		Parallels:                  DefaultParallels,
		Schedule:                   DefaultSchedule,
		SuccessfulJobsHistoryLimit: DefaultSuccessfulJobsHistoryLimit,
		FailedJobsHistoryLimit:     DefaultFailedJobsHistoryLimit,
		K8sTesterStressCLI:         NewDefaultK8sTesterStressCLI(),
	}
}

func NewDefaultK8sTesterStressCLI() *K8sTesterStressCLI {
	return &K8sTesterStressCLI{
		BusyboxRepository: &aws_v1_ecr.Repository{},
		RunTimeout:        DefaultRunTimeout,
		RunTimeoutString:  DefaultRunTimeout.String(),
		ObjectKeyPrefix:   DefaultObjectKeyPrefix(),
		Objects:           DefaultObjects,
		ObjectSize:        DefaultObjectSize,
		UpdateConcurrency: DefaultUpdateConcurrency,
		ListBatchLimit:    DefaultListBatchLimit,
	}
}

func New(cfg *Config) k8s_tester.Tester {
	ts := &tester{
		cfg: cfg,
	}
	if !cfg.K8sTesterStressRepository.IsEmpty() {
		awsCfg := aws_v1.Config{
			Logger:        cfg.Logger,
			DebugAPICalls: cfg.Logger.Core().Enabled(zapcore.DebugLevel),
			Partition:     cfg.K8sTesterStressRepository.Partition,
			Region:        cfg.K8sTesterStressRepository.Region,
		}
		awsSession, _, _, err := aws_v1.New(&awsCfg)
		if err != nil {
			cfg.Logger.Panic("failed to create aws session", zap.Error(err))
		}
		ts.ecrAPI = ecr.New(awsSession, aws.NewConfig().WithRegion(cfg.K8sTesterStressRepository.Region))
	}
	if ts.ecrAPI == nil && !cfg.K8sTesterStressCLI.BusyboxRepository.IsEmpty() {
		awsCfg := aws_v1.Config{
			Logger:        cfg.Logger,
			DebugAPICalls: cfg.Logger.Core().Enabled(zapcore.DebugLevel),
			Partition:     cfg.K8sTesterStressCLI.BusyboxRepository.Partition,
			Region:        cfg.K8sTesterStressCLI.BusyboxRepository.Region,
		}
		awsSession, _, _, err := aws_v1.New(&awsCfg)
		if err != nil {
			cfg.Logger.Panic("failed to create aws session", zap.Error(err))
		}
		ts.ecrAPI = ecr.New(awsSession, aws.NewConfig().WithRegion(cfg.K8sTesterStressCLI.BusyboxRepository.Region))
	}
	return ts
}

type tester struct {
	cfg    *Config
	ecrAPI ecriface.ECRAPI
}

var pkgName = "stress-" + path.Base(reflect.TypeOf(tester{}).PkgPath())

func Env() string {
	return "ADD_ON_" + strings.ToUpper(strings.Replace(pkgName, "-", "_", -1))
}

func EnvK8sTesterStressRepository() string {
	return Env() + "_K8S_TESTER_STRESS_REPOSITORY"
}

func EnvK8sTesterStressCLI() string {
	return Env() + "_K8S_TESTER_STRESS_CLI"
}

func EnvK8sTesterStressCLIBusyboxRepository() string {
	return EnvK8sTesterStressCLI() + "_BUSYBOX_REPOSITORY"
}

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Enabled() bool { return ts.cfg.Enable }

func (ts *tester) Apply() (err error) {
	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}

	k8sTesterStressImg, busyboxImg, err := ts.checkECRImages()
	if err != nil {
		ts.cfg.Logger.Warn("failed to describe ECR image", zap.Error(err))
		return err
	}

	if nodes, err := client.ListNodes(ts.cfg.Client.KubernetesClient()); len(nodes) < ts.cfg.MinimumNodes || err != nil {
		return fmt.Errorf("failed to validate minimum nodes requirement %d (nodes %v, error %v)", ts.cfg.MinimumNodes, len(nodes), err)
	}

	if err := client.CreateNamespace(ts.cfg.Logger, ts.cfg.Client.KubernetesClient(), ts.cfg.Namespace); err != nil {
		return err
	}

	if err := ts.createServiceAccount(); err != nil {
		return err
	}

	if err = ts.createRBACClusterRole(); err != nil {
		return err
	}

	if err = ts.createRBACClusterRoleBinding(); err != nil {
		return err
	}

	if err = ts.createConfigmap(); err != nil {
		return err
	}

	if err = ts.createCronJob(k8sTesterStressImg, busyboxImg); err != nil {
		return err
	}

	if err = ts.checkCronJob(); err != nil {
		return err
	}

	return nil
}

func (ts *tester) Delete() (err error) {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}

	var errs []string

	if err := client.DeleteCronJob(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
		ts.cfg.Namespace,
		cronJobName,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete CronJob (%v)", err))
	}

	if err := client.DeleteConfigmap(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
		ts.cfg.Namespace,
		kubeconfigConfigmapName,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete config map (%v)", err))
	}

	if err := client.DeleteRBACClusterRoleBinding(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
		rbacClusterRoleBindingName,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete RBAC cluster role binding (%v)", err))
	}

	if err := client.DeleteRBACClusterRole(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
		rbacRoleName,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete RBAC cluster role binding (%v)", err))
	}

	if err := client.DeleteServiceAccount(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
		ts.cfg.Namespace,
		serviceAccountName,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete service account (%v)", err))
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

func (ts *tester) checkECRImages() (k8sTesterStressImg string, busyboxImg string, err error) {
	k8sTesterStressImg, _, err = ts.cfg.K8sTesterStressRepository.Describe(ts.cfg.Logger, ts.ecrAPI)
	if err != nil {
		return "", "", err
	}
	busyboxImg, _, err = ts.cfg.K8sTesterStressCLI.BusyboxRepository.Describe(ts.cfg.Logger, ts.ecrAPI)
	return k8sTesterStressImg, busyboxImg, err
}

const (
	serviceAccountName          = "stress-in-cluster-service-account"
	rbacRoleName                = "stress-in-cluster-rbac-role"
	rbacClusterRoleBindingName  = "stress-in-cluster-rbac-role-binding"
	kubeconfigConfigmapName     = "stress-in-cluster-kubeconfig-configmap"
	kubeconfigConfigmapFileName = "stress-in-cluster-kubeconfig-configmap.yaml"
	appName                     = "stress-in-cluster-app"
	cronJobName                 = "stress-in-cluster-cronjob"
)

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createServiceAccount() error {
	ts.cfg.Logger.Info("creating stress ServiceAccount")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.Client.KubernetesClient().
		CoreV1().
		ServiceAccounts(ts.cfg.Namespace).
		Create(
			ctx,
			&v1.ServiceAccount{
				TypeMeta: meta_v1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
				},
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      serviceAccountName,
					Namespace: ts.cfg.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": appName,
					},
				},
			},
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("stress ServiceAccount already exists")
			return nil
		}
		return fmt.Errorf("failed to create stress ServiceAccount (%v)", err)
	}

	ts.cfg.Logger.Info("created stress ServiceAccount")
	return nil
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
// e.g., nodes is forbidden: User "system:serviceaccount:stress-in-cluster...:default" cannot list resource "nodes" in API group "" at the cluster scope
func (ts *tester) createRBACClusterRole() error {
	ts.cfg.Logger.Info("creating stresser RBAC ClusterRole")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.Client.KubernetesClient().
		RbacV1().
		ClusterRoles().
		Create(
			ctx,
			&rbac_v1.ClusterRole{
				TypeMeta: meta_v1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRole",
				},
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      rbacRoleName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": appName,
					},
				},
				Rules: []rbac_v1.PolicyRule{
					{
						APIGroups: []string{
							"*",
						},
						Resources: []string{
							"configmaps",
							"leases",
							"nodes",
							"pods",
							"secrets",
							"services",
							"namespaces",
							"endpoints",
							"events",
							"ingresses",
							"ingresses/status",
							"services",
							"jobs",
							"cronjobs",
						},
						Verbs: []string{
							"create",
							"get",
							"list",
							"update",
							"watch",
							"patch",
						},
					},
				},
			},
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("stress RBAC ClusterRole already exists")
			return nil
		}
		return fmt.Errorf("failed to create stresser RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("created stresser RBAC ClusterRole")
	return nil
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("creating stresser RBAC ClusterRoleBinding")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.Client.KubernetesClient().
		RbacV1().
		ClusterRoleBindings().
		Create(
			ctx,
			&rbac_v1.ClusterRoleBinding{
				TypeMeta: meta_v1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRoleBinding",
				},
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      rbacClusterRoleBindingName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": appName,
					},
				},
				RoleRef: rbac_v1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     rbacRoleName,
				},
				Subjects: []rbac_v1.Subject{
					{
						APIGroup:  "",
						Kind:      "ServiceAccount",
						Name:      serviceAccountName,
						Namespace: ts.cfg.Namespace,
					},
					{ // https://kubernetes.io/docs/reference/access-authn-authz/rbac/
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "User",
						Name:     "system:node",
					},
				},
			},
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("stress RBAC ClusterRoleBinding already exists")
			return nil
		}
		return fmt.Errorf("failed to create stresser RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("created stresser RBAC ClusterRoleBinding")
	return nil
}

func (ts *tester) createConfigmap() error {
	ts.cfg.Logger.Info("creating config map")

	b, err := ioutil.ReadFile(ts.cfg.Client.Config().KubeconfigPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.Client.KubernetesClient().
		CoreV1().
		ConfigMaps(ts.cfg.Namespace).
		Create(
			ctx,
			&v1.ConfigMap{
				TypeMeta: meta_v1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      kubeconfigConfigmapName,
					Namespace: ts.cfg.Namespace,
					Labels: map[string]string{
						"name": kubeconfigConfigmapName,
					},
				},
				Data: map[string]string{
					kubeconfigConfigmapFileName: string(b),
				},
			},
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("stress config map already exists")
			return nil
		}
		return err
	}

	ts.cfg.Logger.Info("created stress config map")
	return nil
}

func (ts *tester) createCronJobObject(k8sTesterStressImg string, busyboxImg string) (batch_v1beta1.CronJob, string, error) {
	// do not pass kubeconfig to use in-cluster client
	cmd := "/k8s-tester-stress --prompt=false"
	cmd += fmt.Sprintf(" --namespace %s", ts.cfg.Namespace)
	cmd += " --kubectl-path /kubectl"
	cmd += fmt.Sprintf(" apply --ecr-busybox-image %s", busyboxImg)
	cmd += fmt.Sprintf(" --run-timeout %s", ts.cfg.K8sTesterStressCLI.RunTimeout)
	cmd += fmt.Sprintf(" --object-key-prefix %s", ts.cfg.K8sTesterStressCLI.ObjectKeyPrefix)
	cmd += fmt.Sprintf(" --objects %d", ts.cfg.K8sTesterStressCLI.Objects)
	cmd += fmt.Sprintf(" --object-size %d", ts.cfg.K8sTesterStressCLI.ObjectSize)
	cmd += fmt.Sprintf(" --update-concurrency %d", ts.cfg.K8sTesterStressCLI.UpdateConcurrency)
	cmd += fmt.Sprintf(" --list-batch-limit %d", ts.cfg.K8sTesterStressCLI.ListBatchLimit)

	dirOrCreate := core_v1.HostPathDirectoryOrCreate
	podSpec := core_v1.PodTemplateSpec{
		Spec: core_v1.PodSpec{
			ServiceAccountName: serviceAccountName,

			// spec.template.spec.restartPolicy: Unsupported value: "Always": supported values: "OnFailure", "Never"
			// ref. https://github.com/kubernetes/kubernetes/issues/54870
			RestartPolicy: core_v1.RestartPolicyNever,

			Containers: []core_v1.Container{
				{
					Name:            cronJobName,
					Image:           k8sTesterStressImg,
					ImagePullPolicy: core_v1.PullAlways,

					Command: []string{
						"/bin/sh",
						"-ec",
						cmd,
					},

					// grant access "/dev/kmsg"
					SecurityContext: &v1.SecurityContext{
						Privileged: boolRef(true),
					},

					// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
					VolumeMounts: []core_v1.VolumeMount{
						{ // to execute
							Name:      kubeconfigConfigmapName,
							MountPath: "/opt",
						},
						{ // to write
							Name:      "varlog",
							MountPath: "/var/log",
							ReadOnly:  false,
						},
					},
				},
			},

			// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
			Volumes: []core_v1.Volume{
				{ // to execute
					Name: kubeconfigConfigmapName,
					VolumeSource: core_v1.VolumeSource{
						ConfigMap: &core_v1.ConfigMapVolumeSource{
							LocalObjectReference: core_v1.LocalObjectReference{
								Name: kubeconfigConfigmapName,
							},
							DefaultMode: int32Ref(0777),
						},
					},
				},
				{ // to write
					Name: "varlog",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/var/log",
							Type: &dirOrCreate,
						},
					},
				},
			},
		},
	}

	jobSpec := batch_v1beta1.JobTemplateSpec{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      cronJobName,
			Namespace: ts.cfg.Namespace,
			Labels: map[string]string{
				"cronjob-name": cronJobName,
			},
		},
		Spec: batch_v1.JobSpec{
			Completions: &ts.cfg.Completes,
			Parallelism: &ts.cfg.Parallels,
			Template:    podSpec,
			// TODO: 'TTLSecondsAfterFinished' is still alpha
			// https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/
		},
	}
	cronJobObj := batch_v1beta1.CronJob{
		TypeMeta: meta_v1.TypeMeta{
			APIVersion: "batch/v1beta1",
			Kind:       "CronJob",
		},
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      cronJobName,
			Namespace: ts.cfg.Namespace,
			Labels: map[string]string{
				"cronjob-name": cronJobName,
			},
		},
		Spec: batch_v1beta1.CronJobSpec{
			Schedule:                   ts.cfg.Schedule,
			SuccessfulJobsHistoryLimit: &ts.cfg.SuccessfulJobsHistoryLimit,
			FailedJobsHistoryLimit:     &ts.cfg.FailedJobsHistoryLimit,
			JobTemplate:                jobSpec,
			ConcurrencyPolicy:          batch_v1beta1.ReplaceConcurrent,
		},
	}
	b, err := yaml.Marshal(cronJobObj)
	return cronJobObj, string(b), err
}

func (ts *tester) createCronJob(k8sTesterStressImg string, busyboxImg string) error {
	cronObj, css, err := ts.createCronJobObject(k8sTesterStressImg, busyboxImg)
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("creating a CronJob object",
		zap.String("cronjob-name", cronJobName),
		zap.Int32("completes", ts.cfg.Completes),
		zap.Int32("parallels", ts.cfg.Parallels),
		zap.String("schedule", ts.cfg.Schedule),
		zap.Int32("successful-job-history-limit", ts.cfg.SuccessfulJobsHistoryLimit),
		zap.Int32("failed-job-history-limit", ts.cfg.FailedJobsHistoryLimit),
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
	fmt.Fprintf(ts.cfg.LogWriter, "\n%s\n", css)

	return nil
}

func (ts *tester) checkCronJob() (err error) {
	timeout := 15*time.Minute + ts.cfg.K8sTesterStressCLI.RunTimeout*time.Duration(ts.cfg.Completes)
	if timeout > 3*time.Hour {
		timeout = 3 * time.Hour
	}

	ts.cfg.Logger.Info("checking cron job", zap.String("timeout", timeout.String()))
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	var pods []core_v1.Pod
	_, pods, err = client.WaitForCronJobCompletes(
		ctx,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.Stopc,
		ts.cfg.Client.KubernetesClient(),
		3*time.Minute,
		5*time.Second,
		ts.cfg.Namespace,
		cronJobName,
		int(ts.cfg.Completes),

		client.WithQueryFunc(func() {
			descArgs := []string{
				ts.cfg.Client.Config().KubectlPath,
				"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
				"--namespace=" + ts.cfg.Namespace,
				"describe",
				"cronjob.batch/" + cronJobName,
			}
			descCmd := strings.Join(descArgs, " ")
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			descOutput, err := exec.New().CommandContext(ctx, descArgs[0], descArgs[1:]...).CombinedOutput()
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe cronjob' failed", zap.Error(err))
			}
			out := string(descOutput)
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\n\"%s\" output:\n\n%s\n\n", descCmd, out)

			descArgs = []string{
				ts.cfg.Client.Config().KubectlPath,
				"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
				"--namespace=" + ts.cfg.Namespace,
				"describe",
				"job",
				cronJobName,
			}
			descCmd = strings.Join(descArgs, " ")
			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			descOutput, err = exec.New().CommandContext(ctx, descArgs[0], descArgs[1:]...).CombinedOutput()
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe job' failed", zap.Error(err))
			}
			out = string(descOutput)
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\n\"%s\" output:\n\n%s\n\n", descCmd, out)

			argsLogs := []string{
				ts.cfg.Client.Config().KubectlPath,
				"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
				"--namespace=" + ts.cfg.Namespace,
				"logs",
				"--selector=cronjob-name=" + cronJobName,
				"--timestamps",
				"--tail=10",
			}
			cmdLogs := strings.Join(argsLogs, " ")
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			logsOutput, err := exec.New().CommandContext(ctx, argsLogs[0], argsLogs[1:]...).CombinedOutput()
			cancel()
			out = string(logsOutput)
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl logs' failed", zap.Error(err))
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"%s\":\n%s\n", cmdLogs, out)
		}),

		client.WithPodFunc(func(pod core_v1.Pod) {
			switch pod.Status.Phase {
			case core_v1.PodFailed:
				ts.cfg.Logger.Warn("pod failed",
					zap.String("namespace", pod.Namespace),
					zap.String("pod-name", pod.Name),
					zap.String("pod-status-phase", fmt.Sprintf("%v", pod.Status.Phase)),
				)
				descArgs := []string{
					ts.cfg.Client.Config().KubectlPath,
					"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
					"--namespace=" + pod.Namespace,
					"describe",
					"pod",
					pod.Name,
				}
				descCmd := strings.Join(descArgs, " ")
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				cmdOutput, err := exec.New().CommandContext(ctx, descArgs[0], descArgs[1:]...).CombinedOutput()
				cancel()
				if err != nil {
					ts.cfg.Logger.Warn("'kubectl describe pod' failed", zap.Error(err))
				}
				out := string(cmdOutput)
				fmt.Fprintf(ts.cfg.LogWriter, "\"%s\" output:\n\n%s\n\n", descCmd, out)

				logsArgs := []string{
					ts.cfg.Client.Config().KubectlPath,
					"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
					"--namespace=" + pod.Namespace,
					"logs",
					fmt.Sprintf("pod/%s", pod.Name),
					"--timestamps",
				}
				logsCmd := strings.Join(logsArgs, " ")
				ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
				cmdOutput, err = exec.New().CommandContext(ctx, logsArgs[0], logsArgs[1:]...).CombinedOutput()
				cancel()
				if err != nil {
					ts.cfg.Logger.Warn("'kubectl logs' failed", zap.Error(err))
				}
				out = string(cmdOutput)
				fmt.Fprintf(ts.cfg.LogWriter, "\"%s\" output:\n\n%s\n\n", logsCmd, out)
			}
		}),
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

func int32Ref(v int32) *int32 {
	return &v
}

func boolRef(v bool) *bool {
	return &v
}
