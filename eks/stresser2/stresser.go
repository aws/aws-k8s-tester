package stresser2

import (
	"context"
	"errors"
	"fmt"
	"io"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"reflect"
	"strings"
	"time"

	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	aws_ecr "github.com/aws/aws-k8s-tester/pkg/aws/ecr"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	batch_v1 "k8s.io/api/batch/v1"
	batch_v1beta1 "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
	"sigs.k8s.io/yaml"
)

type Config struct {
	Logger     *zap.Logger
	LogWriter  io.Writer

	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	ECRAPI      ecriface.ECRAPI
}

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg      Config
	ecrImage string
}

const (
	stresserV2AppName = "stresser2-app"
	stresserV2ServiceAccountName          = "stresser2-remote-service-account"
	stresserV2RBACRoleName                = "stresser2-remote-rbac-role"
	stresserV2RBACClusterRoleBindingName  = "stresser2-remote-rbac-role-binding"
)

func (ts *tester) Create() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnStresserRemoteV2() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnStresserRemoteV2.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnStresserRemoteV2.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnStresserRemoteV2.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if ts.ecrImage, _, err = aws_ecr.Check(
		ts.cfg.Logger,
		ts.cfg.ECRAPI,
		ts.cfg.EKSConfig.Partition,
		ts.cfg.EKSConfig.AddOnStresserRemoteV2.RepositoryAccountID,
		ts.cfg.EKSConfig.AddOnStresserRemoteV2.RepositoryRegion,
		ts.cfg.EKSConfig.AddOnStresserRemoteV2.RepositoryName,
		ts.cfg.EKSConfig.AddOnStresserRemoteV2.RepositoryImageTag,
	); err != nil {
		return err
	}

	if err = k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnStresserRemoteV2.Namespace,
	); err != nil {
		return err
	}
	if err = ts.createServiceAccount(); err != nil {
		return err
	}
	if err = ts.createRBACClusterRole(); err != nil {
		return err
	}
	if err = ts.createRBACClusterRoleBinding(); err != nil {
		return err
	}
	if err = ts.createCronJob(); err != nil {
		return err
	}

	// TODO waits for all the job spawned by cronJob up and running


	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) Delete() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnStresserRemoteV2() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnStresserRemoteV2.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnStresserRemoteV2.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteCronJob(); err != nil {
		errs = append(errs, err.Error())
	}
	time.Sleep(2 * time.Minute)

	if err := ts.deleteRBACClusterRoleBinding(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ts.deleteRBACClusterRole(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ts.deleteServiceAccount(); err != nil {
		errs = append(errs, err.Error())
	}

	getAllArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnStresserRemoteV2.Namespace,
		"get",
		"all",
	}
	getAllCmd := strings.Join(getAllArgs, " ")

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnStresserRemoteV2.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithQueryFunc(func() {
			fmt.Fprintf(ts.cfg.LogWriter, "\n")
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			output, err := exec.New().CommandContext(ctx, getAllArgs[0], getAllArgs[1:]...).CombinedOutput()
			cancel()
			out := strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl get all' failed", zap.Error(err))
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", getAllCmd, out)
		}),
		k8s_client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete remote stresser2 namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnStresserRemoteV2.Created = false
	ts.cfg.EKSConfig.Sync()
	return
}

// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
// ref. https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/#foreground-cascading-deletion
func (ts *tester) createServiceAccount() (err error) {
	ts.cfg.Logger.Info("creating stresser2 ServiceAccount")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnStresserRemoteV2.Namespace).
		Create(
			ctx,
			&v1.ServiceAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      stresserV2ServiceAccountName,
					Namespace: ts.cfg.EKSConfig.AddOnStresserRemoteV2.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": stresserV2AppName,
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create stresser2 ServiceAccount (%v)", err)
	}

	ts.cfg.Logger.Info("created stresser2 ServiceAccount")
	ts.cfg.EKSConfig.Sync()
	return
}
func (ts *tester) deleteServiceAccount() (err error) {
	ts.cfg.Logger.Info("deleting stresser2 ServiceAccount")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err = ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnStresserRemoteV2.Namespace).
		Delete(
			ctx,
			stresserV2ServiceAccountName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete stresser2 ServiceAccount (%v)", err)
	}
	ts.cfg.Logger.Info("deleted stresser2 ServiceAccount", zap.Error(err))

	ts.cfg.EKSConfig.Sync()
	return
}
func (ts *tester) createRBACClusterRole() (err error) {
	ts.cfg.Logger.Info("creating stresser RBAC ClusterRole")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoles().
		Create(
			ctx,
			&rbacv1.ClusterRole{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRole",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      stresserV2RBACRoleName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": stresserV2AppName,
					},
				},
				Rules: []rbacv1.PolicyRule{
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
							"stresser",
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
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create stresser2 RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("created stresser2 RBAC ClusterRole")
	ts.cfg.EKSConfig.Sync()
	return
}
func (ts *tester) deleteRBACClusterRole() (err error) {
	ts.cfg.Logger.Info("deleting stresser2 RBAC ClusterRole")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err = ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoles().
		Delete(
			ctx,
			stresserV2RBACRoleName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete stresser2 RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("deleted stresser2 RBAC ClusterRole", zap.Error(err))
	ts.cfg.EKSConfig.Sync()
	return
}

// https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createRBACClusterRoleBinding() (err error) {
	ts.cfg.Logger.Info("creating stresser2 RBAC ClusterRoleBinding")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoleBindings().
		Create(
			ctx,
			&rbacv1.ClusterRoleBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRoleBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      stresserV2RBACClusterRoleBindingName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": stresserV2AppName,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     stresserV2RBACRoleName,
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup:  "",
						Kind:      "ServiceAccount",
						Name:      stresserV2ServiceAccountName,
						Namespace: ts.cfg.EKSConfig.AddOnStresserRemoteV2.Namespace,
					},
					{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "User",
						Name:     "system:node",
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create stresser2 RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("created stresser2 RBAC ClusterRoleBinding")
	ts.cfg.EKSConfig.Sync()
	return
}
func (ts *tester) deleteRBACClusterRoleBinding() (err error) {
	ts.cfg.Logger.Info("deleting stresser2 RBAC ClusterRoleBinding")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err = ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoleBindings().
		Delete(
			ctx,
			stresserV2RBACClusterRoleBindingName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete stresser2 RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("deleted stresser2 RBAC ClusterRoleBinding", zap.Error(err))
	ts.cfg.EKSConfig.Sync()
	return
}

func (ts *tester) createCronJob() (err error) {
	obj, b, err := ts.createObject()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("creating CronJob",
		zap.String("name", stresserV2AppName),
		zap.Int("completes", ts.cfg.EKSConfig.AddOnStresserRemoteV2.Completes),
		zap.Int("parallels", ts.cfg.EKSConfig.AddOnStresserRemoteV2.Parallels),
		zap.String("object-size", humanize.Bytes(uint64(len(b)))),
	)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		BatchV1beta1().
		CronJobs(ts.cfg.EKSConfig.AddOnStresserRemoteV2.Namespace).
		Create(ctx, &obj, metav1.CreateOptions{})
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create CronJob (%v)", err)
	}

	ts.cfg.Logger.Info("created CronJob")
	return nil
}
func (ts *tester) createObject() (batch_v1beta1.CronJob, string, error) {
	testerCmd := fmt.Sprintf("/aws-k8s-tester eks create stresser2 --number=%d --duration=%s --object-size=%d --secret-num=%d --busybox-image=%s",
		ts.cfg.EKSConfig.AddOnStresserRemoteV2.Coroutine,
		ts.cfg.EKSConfig.AddOnStresserRemoteV2.Duration,
		ts.cfg.EKSConfig.AddOnStresserRemoteV2.ObjectSize,
		ts.cfg.EKSConfig.AddOnStresserRemoteV2.SecretNum,
		fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:%s",
			ts.cfg.EKSConfig.AddOnStresserRemoteV2.RepositoryAccountID,
			ts.cfg.EKSConfig.AddOnStresserRemoteV2.RepositoryRegion,
			ts.cfg.EKSConfig.AddOnStresserRemoteV2.RepositoryBusyBoxName,
			ts.cfg.EKSConfig.AddOnStresserRemoteV2.RepositoryBusyBoxImageTag),
	)

	dirOrCreate := v1.HostPathDirectoryOrCreate
	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app.kubernetes.io/name": stresserV2AppName,
			},
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyOnFailure,
			Containers: []v1.Container{
				{
					Name:            stresserV2AppName,
					Image:           ts.ecrImage,
					ImagePullPolicy: v1.PullAlways,
					Command: []string{
						"/bin/sh",
						"-ec",
						testerCmd,
					},
					// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "logging",
							MountPath: "/var/log",
							ReadOnly:  false,
						},
					},
				},
			},
			// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
			Volumes: []v1.Volume{
				{
					Name: "logging",
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
		ObjectMeta: metav1.ObjectMeta{
			Name:      stresserV2AppName,
			Namespace: ts.cfg.EKSConfig.AddOnStresserRemoteV2.Namespace,
		},
		Spec: batch_v1.JobSpec{
			Completions: aws.Int32(int32(ts.cfg.EKSConfig.AddOnStresserRemoteV2.Completes)),
			Parallelism: aws.Int32(int32(ts.cfg.EKSConfig.AddOnStresserRemoteV2.Parallels)),
			Template:    podSpec,
		},
	}
	cronObj := batch_v1beta1.CronJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1beta1",
			Kind:       "CronJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      stresserV2AppName,
			Namespace: ts.cfg.EKSConfig.AddOnStresserRemoteV2.Namespace,
		},
		Spec: batch_v1beta1.CronJobSpec{
			Schedule:                   ts.cfg.EKSConfig.AddOnStresserRemoteV2.Schedule,
			SuccessfulJobsHistoryLimit: aws.Int32(ts.cfg.EKSConfig.AddOnStresserRemoteV2.SuccessfulJobsHistoryLimit),
			FailedJobsHistoryLimit:     aws.Int32(ts.cfg.EKSConfig.AddOnStresserRemoteV2.FailedJobsHistoryLimit),
			JobTemplate:                jobSpec,
			ConcurrencyPolicy:          batch_v1beta1.ReplaceConcurrent,
		},
	}
	b, err := yaml.Marshal(cronObj)
	return cronObj, string(b), err
}
func (ts *tester) deleteCronJob() (err error) {
	propagationBackground := metav1.DeletePropagationBackground
	ts.cfg.Logger.Info("deleting CronJob", zap.String("name", stresserV2AppName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err = ts.cfg.
		K8SClient.KubernetesClientSet().
		BatchV1beta1().
		CronJobs(ts.cfg.EKSConfig.AddOnCronJobs.Namespace).
		Delete(
			ctx,
			stresserV2AppName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &propagationBackground,
			},
		)
	cancel()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("deleted CronJob", zap.String("name", stresserV2AppName))
	return nil
}

