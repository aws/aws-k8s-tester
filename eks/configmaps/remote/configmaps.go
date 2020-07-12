// Package remote implements tester for ConfigMap.
package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/aws/cw"
	aws_ecr "github.com/aws/aws-k8s-tester/pkg/aws/ecr"
	aws_s3 "github.com/aws/aws-k8s-tester/pkg/aws/s3"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/metrics"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines configmaps configuration.
// ref. https://github.com/kubernetes/perf-tests
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	S3API     s3iface.S3API
	CWAPI     cloudwatchiface.CloudWatchAPI
	ECRAPI    ecriface.ECRAPI
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

func (ts *tester) Create() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnConfigmapsRemote() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnConfigmapsRemote.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnConfigmapsRemote.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if ts.ecrImage, err = aws_ecr.Check(
		ts.cfg.Logger,
		ts.cfg.ECRAPI,
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.RepositoryAccountID,
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.RepositoryRegion,
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.RepositoryName,
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.RepositoryImageTag,
	); err != nil {
		return err
	}
	if err = k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.Namespace,
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
	if err = ts.createConfigMap(); err != nil {
		return err
	}

	if err = ts.createJob(); err != nil {
		return err
	}
	timeout := 5*time.Minute + 5*time.Minute*time.Duration(ts.cfg.EKSConfig.AddOnConfigmapsRemote.Completes) + time.Minute*time.Duration(ts.cfg.EKSConfig.AddOnConfigmapsRemote.Objects/100)
	if timeout > 3*time.Hour {
		timeout = 3 * time.Hour
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	var pods []v1.Pod
	_, pods, err = k8s_client.WaitForJobCompletes(
		ctx,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.Stopc,
		ts.cfg.K8SClient,
		time.Minute,
		10*time.Second,
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.Namespace,
		configmapsJobName,
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.Completes,
		k8s_client.WithQueryFunc(func() {
			descArgs := []string{
				ts.cfg.EKSConfig.KubectlPath,
				"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
				"--namespace=" + ts.cfg.EKSConfig.AddOnCSRsRemote.Namespace,
				"describe",
				"job",
				configmapsJobName,
			}
			descCmd := strings.Join(descArgs, " ")
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			descOutput, err := exec.New().CommandContext(ctx, descArgs[0], descArgs[1:]...).CombinedOutput()
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe job' failed", zap.Error(err))
			}
			out := string(descOutput)
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\n\"%s\" output:\n\n%s\n\n", descCmd, out)
		}),
		k8s_client.WithPodFunc(func(pod v1.Pod) {
			switch pod.Status.Phase {
			case v1.PodFailed:
				ts.cfg.Logger.Warn("pod failed",
					zap.String("namespace", pod.Namespace),
					zap.String("pod-name", pod.Name),
					zap.String("pod-status-phase", fmt.Sprintf("%v", pod.Status.Phase)),
				)
				descArgs := []string{
					ts.cfg.EKSConfig.KubectlPath,
					"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
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
					ts.cfg.Logger.Warn("'kubectl describe job' failed", zap.Error(err))
				}
				out := string(cmdOutput)
				fmt.Fprintf(ts.cfg.LogWriter, "\"%s\" output:\n\n%s\n\n", descCmd, out)

				logsArgs := []string{
					ts.cfg.EKSConfig.KubectlPath,
					"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
					"--namespace=" + pod.Namespace,
					"logs",
					fmt.Sprintf("pod/%s", pod.Name),
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
		fmt.Fprintf(ts.cfg.LogWriter, "Job Pod %q: %q\n", item.Name, item.Status.Phase)
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\n")

	if err = ts.checkResults(); err == nil {
		return err
	}
	if err = ts.publishResults(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnConfigmapsRemote() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnConfigmapsRemote.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteJob(); err != nil {
		errs = append(errs, err.Error())
	}
	time.Sleep(2 * time.Minute)

	if err := ts.deleteConfigMap(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ts.deleteRBACClusterRoleBinding(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ts.deleteRBACClusterRole(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ts.deleteServiceAccount(); err != nil {
		errs = append(errs, err.Error())
	}

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete configmaps namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnConfigmapsRemote.Created = false
	return ts.cfg.EKSConfig.Sync()
}

const (
	configmapsServiceAccountName          = "configmaps-remote-service-account"
	configmapsRBACRoleName                = "configmaps-remote-rbac-role"
	configmapsRBACClusterRoleBindingName  = "configmaps-remote-rbac-role-binding"
	configmapsKubeConfigConfigMapName     = "configmaps-remote-kubeconfig-configmap"
	configmapsKubeConfigConfigMapFileName = "configmaps-remote-kubeconfig-configmap.yaml"
	configmapsAppName                     = "configmaps-remote-app"
	configmapsJobName                     = "configmaps-remote-job"
)

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createServiceAccount() error {
	ts.cfg.Logger.Info("creating configmaps ServiceAccount")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnConfigmapsRemote.Namespace).
		Create(
			ctx,
			&v1.ServiceAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      configmapsServiceAccountName,
					Namespace: ts.cfg.EKSConfig.AddOnConfigmapsRemote.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": configmapsAppName,
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create configmaps ServiceAccount (%v)", err)
	}

	ts.cfg.Logger.Info("created configmaps ServiceAccount")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteServiceAccount() error {
	ts.cfg.Logger.Info("deleting configmaps ServiceAccount")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnConfigmapsRemote.Namespace).
		Delete(
			ctx,
			configmapsServiceAccountName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete configmaps ServiceAccount (%v)", err)
	}
	ts.cfg.Logger.Info("deleted configmaps ServiceAccount", zap.Error(err))

	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createRBACClusterRole() error {
	ts.cfg.Logger.Info("creating configmaps RBAC ClusterRole")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
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
					Name:      configmapsRBACRoleName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": configmapsAppName,
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{
							"*",
						},
						Resources: []string{
							"configmaps",
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
		return fmt.Errorf("failed to create configmaps RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("created configmaps RBAC ClusterRole")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteRBACClusterRole() error {
	ts.cfg.Logger.Info("deleting configmaps RBAC ClusterRole")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoles().
		Delete(
			ctx,
			configmapsRBACRoleName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete configmaps RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("deleted configmaps RBAC ClusterRole", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("creating configmaps RBAC ClusterRoleBinding")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
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
					Name:      configmapsRBACClusterRoleBindingName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": configmapsAppName,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     configmapsRBACRoleName,
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup:  "",
						Kind:      "ServiceAccount",
						Name:      configmapsServiceAccountName,
						Namespace: ts.cfg.EKSConfig.AddOnConfigmapsRemote.Namespace,
					},
					{ // https://kubernetes.io/docs/reference/access-authn-authz/rbac/
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
		return fmt.Errorf("failed to create configmaps RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("created configmaps RBAC ClusterRoleBinding")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("deleting configmaps RBAC ClusterRoleBinding")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoleBindings().
		Delete(
			ctx,
			configmapsRBACClusterRoleBindingName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete configmaps RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("deleted configmaps RBAC ClusterRoleBinding", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createConfigMap() error {
	ts.cfg.Logger.Info("creating config map")

	b, err := ioutil.ReadFile(ts.cfg.EKSConfig.KubeConfigPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ConfigMaps(ts.cfg.EKSConfig.AddOnConfigmapsRemote.Namespace).
		Create(
			ctx,
			&v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      configmapsKubeConfigConfigMapName,
					Namespace: ts.cfg.EKSConfig.AddOnConfigmapsRemote.Namespace,
					Labels: map[string]string{
						"name": configmapsKubeConfigConfigMapName,
					},
				},
				Data: map[string]string{
					configmapsKubeConfigConfigMapFileName: string(b),
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("created config map")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteConfigMap() error {
	ts.cfg.Logger.Info("deleting config map")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ConfigMaps(ts.cfg.EKSConfig.AddOnConfigmapsRemote.Namespace).
		Delete(
			ctx,
			configmapsKubeConfigConfigMapName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("deleted config map")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createJob() (err error) {
	obj, b, err := ts.createObject()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("creating Job",
		zap.String("name", configmapsJobName),
		zap.String("object-size", humanize.Bytes(uint64(len(b)))),
	)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		BatchV1().
		Jobs(ts.cfg.EKSConfig.AddOnConfigmapsRemote.Namespace).
		Create(ctx, &obj, metav1.CreateOptions{})
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create Job (%v)", err)
	}

	ts.cfg.Logger.Info("created Job")
	return nil
}

func (ts *tester) createObject() (batchv1.Job, string, error) {
	// "/opt/"+configmapsKubeConfigConfigMapFileName,
	// do not specify "kubeconfig", and use in-cluster config via "pkg/k8s-client"
	// otherwise, error "namespaces is forbidden: User "system:node:ip-192-168-84..."
	// ref. https://github.com/kubernetes/client-go/blob/master/examples/in-cluster-client-configuration/main.go
	testerCmd := fmt.Sprintf("/aws-k8s-tester eks create configmaps --partition=%s --region=%s --s3-bucket-name=%s --clients=%d --client-qps=%f --client-burst=%d --client-timeout=%s --namespace=%s --objects=%d --object-size=%d --requests-raw-writes-json-s3-dir=%s --requests-summary-writes-json-s3-dir=%s --requests-summary-writes-table-s3-dir=%s --writes-output-name-prefix=%s",
		ts.cfg.EKSConfig.Partition,
		ts.cfg.EKSConfig.Region,
		ts.cfg.EKSConfig.S3BucketName,
		ts.cfg.EKSConfig.Clients,
		ts.cfg.EKSConfig.ClientQPS,
		ts.cfg.EKSConfig.ClientBurst,
		ts.cfg.EKSConfig.ClientTimeout,
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.Namespace,
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.Objects,
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.ObjectSize,
		path.Dir(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsRawWritesJSONS3Key),
		path.Dir(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesJSONS3Key),
		path.Dir(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesTableS3Key),
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesOutputNamePrefix,
	)

	dirOrCreate := v1.HostPathDirectoryOrCreate
	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app.kubernetes.io/name": configmapsAppName,
			},
		},
		Spec: v1.PodSpec{
			ServiceAccountName: configmapsServiceAccountName,

			// spec.template.spec.restartPolicy: Unsupported value: "Always": supported values: "OnFailure", "Never"
			// ref. https://github.com/kubernetes/kubernetes/issues/54870
			RestartPolicy: v1.RestartPolicyNever,

			// TODO: set resource limits
			Containers: []v1.Container{
				{
					Name:            configmapsAppName,
					Image:           ts.ecrImage,
					ImagePullPolicy: v1.PullAlways,

					Command: []string{
						"/bin/sh",
						"-ec",
						testerCmd,
					},

					// grant access "/dev/kmsg"
					SecurityContext: &v1.SecurityContext{
						Privileged: aws.Bool(true),
					},

					// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
					VolumeMounts: []v1.VolumeMount{
						{ // to execute
							Name:      configmapsKubeConfigConfigMapName,
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
			Volumes: []v1.Volume{
				{ // to execute
					Name: configmapsKubeConfigConfigMapName,
					VolumeSource: v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: configmapsKubeConfigConfigMapName,
							},
							DefaultMode: aws.Int32(0777),
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

			NodeSelector: map[string]string{
				// do not deploy in fake nodes, obviously
				"NodeType": "regular",
			},
		},
	}

	jobObj := batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmapsJobName,
			Namespace: ts.cfg.EKSConfig.AddOnConfigmapsRemote.Namespace,
		},
		Spec: batchv1.JobSpec{
			Completions: aws.Int32(int32(ts.cfg.EKSConfig.AddOnConfigmapsRemote.Completes)),
			Parallelism: aws.Int32(int32(ts.cfg.EKSConfig.AddOnConfigmapsRemote.Parallels)),
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
	ts.cfg.Logger.Info("deleting Job", zap.String("name", configmapsJobName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err = ts.cfg.
		K8SClient.KubernetesClientSet().
		BatchV1().
		Jobs(ts.cfg.EKSConfig.AddOnConfigmapsRemote.Namespace).
		Delete(
			ctx,
			configmapsJobName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err == nil {
		ts.cfg.Logger.Info("deleted Job", zap.String("name", configmapsJobName))
	} else {
		ts.cfg.Logger.Warn("failed to delete Job", zap.Error(err))
	}
	return err
}

// 1. if previous summary exists, download and compare
// 2. upload new summary and overwrite the previous s3 key
func (ts *tester) checkResults() (err error) {
	curTS := time.Now().UTC().Format(time.RFC3339Nano)
	ts.cfg.Logger.Info("checking results", zap.String("timestamp", curTS))

	writesSummary := metrics.RequestsSummary{TestID: curTS}
	curWriteLatencies := make(metrics.Durations, 0, 20000)
	writesDirRaw := ""
	writesDirSummary := ""

	writesDirRaw, err = aws_s3.DownloadDir(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		path.Dir(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsRawWritesJSONS3Key),
	)
	if err == nil {
		ts.cfg.Logger.Info("reading writes results raw",
			zap.String("writes-dir", writesDirRaw),
			zap.String("s3-dir", path.Dir(ts.cfg.EKSConfig.AddOnCSRsRemote.RequestsRawWritesJSONS3Key)),
		)
		cnt := 0
		err = filepath.Walk(writesDirRaw, func(fpath string, info os.FileInfo, werr error) error {
			if werr != nil {
				return werr
			}
			if info.IsDir() {
				return nil
			}
			cnt++
			switch {
			case strings.HasSuffix(fpath, "-writes-raw.json"):
				b, err := ioutil.ReadFile(fpath)
				if err != nil {
					return fmt.Errorf("failed to open %q (%v)", fpath, err)
				}
				var r metrics.Durations
				if err = json.Unmarshal(b, &r); err != nil {
					return fmt.Errorf("failed to unmarshal %q (%s, %v)", fpath, string(b), err)
				}
				curWriteLatencies = append(curWriteLatencies, r...)
			}
			return nil
		})
		if err != nil || cnt == 0 {
			ts.cfg.Logger.Warn("failed to read writes results", zap.Int("file-count", cnt), zap.Error(err))
			os.RemoveAll(writesDirRaw)
			writesDirRaw = ""
		}
	}
	writesDirSummary, err = aws_s3.DownloadDir(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		path.Dir(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesJSONS3Key),
	)
	if err == nil {
		ts.cfg.Logger.Info("reading writes results summary",
			zap.String("writes-dir", writesDirSummary),
			zap.String("s3-dir", path.Dir(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesJSONS3Key)),
		)
		cnt := 0
		err = filepath.Walk(writesDirSummary, func(fpath string, info os.FileInfo, werr error) error {
			if werr != nil {
				return werr
			}
			if info.IsDir() {
				return nil
			}
			cnt++
			switch {
			case strings.HasSuffix(fpath, "-writes-summary.json"):
				b, err := ioutil.ReadFile(fpath)
				if err != nil {
					return fmt.Errorf("failed to open %q (%v)", fpath, err)
				}
				var r metrics.RequestsSummary
				if err = json.Unmarshal(b, &r); err != nil {
					return fmt.Errorf("failed to unmarshal %q (%s, %v)", fpath, string(b), err)
				}
				writesSummary.SuccessTotal += r.SuccessTotal
				writesSummary.FailureTotal += r.FailureTotal
				if writesSummary.LatencyHistogram == nil || len(writesSummary.LatencyHistogram) == 0 {
					writesSummary.LatencyHistogram = r.LatencyHistogram
				} else {
					writesSummary.LatencyHistogram, err = metrics.MergeHistograms(writesSummary.LatencyHistogram, r.LatencyHistogram)
					if err != nil {
						return fmt.Errorf("failed to merge histograms (%v)", err)
					}
				}
			}
			return nil
		})
		if err != nil || cnt == 0 {
			ts.cfg.Logger.Warn("failed to read writes results", zap.Int("file-count", cnt), zap.Error(err))
			os.RemoveAll(writesDirSummary)
			writesDirSummary = ""
		}
	}

	sortStart := time.Now()
	ts.cfg.Logger.Info("sorting write latencies", zap.Int("data", len(curWriteLatencies)))
	sort.Sort(curWriteLatencies)
	ts.cfg.Logger.Info("sorted write latencies", zap.String("took", time.Since(sortStart).String()))
	writesSummary.LantencyP50 = curWriteLatencies.PickLantencyP50()
	writesSummary.LantencyP90 = curWriteLatencies.PickLantencyP90()
	writesSummary.LantencyP99 = curWriteLatencies.PickLantencyP99()
	writesSummary.LantencyP999 = curWriteLatencies.PickLantencyP999()
	writesSummary.LantencyP9999 = curWriteLatencies.PickLantencyP9999()
	ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWrites = writesSummary
	ts.cfg.EKSConfig.Sync()

	wb, err := json.Marshal(curWriteLatencies)
	if err != nil {
		ts.cfg.Logger.Warn("failed to encode JSON", zap.Error(err))
		return err
	}
	if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsRawWritesJSONPath, wb, 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return err
	}
	if err = aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsRawWritesJSONS3Key,
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsRawWritesJSONPath,
	); err != nil {
		return err
	}
	if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesJSONPath, []byte(writesSummary.JSON()), 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return err
	}
	if err = aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesJSONS3Key,
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesJSONPath,
	); err != nil {
		return err
	}
	if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesTablePath, []byte(writesSummary.Table()), 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return err
	}
	if err = aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesTableS3Key,
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesTablePath,
	); err != nil {
		return err
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\n\nRequestsSummaryWrites:\n%s\n", writesSummary.Table())

	s3Objects := make([]*s3.Object, 0)
	if ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesCompareS3Dir != "" {
		s3Objects, err = aws_s3.ListInDescendingLastModified(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			path.Clean(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesCompareS3Dir)+"/",
		)
	}
	canCompare := len(s3Objects) > 0 && err == nil
	if canCompare {
		reqSummaryS3Key := aws.StringValue(s3Objects[0].Key)
		durRawS3Key := path.Join(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsRawWritesCompareS3Dir, path.Base(reqSummaryS3Key))

		var prevSummary metrics.RequestsSummary
		prevSummary, err = metrics.DownloadRequestsSummaryFromS3(ts.cfg.Logger, ts.cfg.S3API, ts.cfg.EKSConfig.S3BucketName, reqSummaryS3Key)
		if err != nil {
			ts.cfg.Logger.Warn("failed to download results", zap.Error(err))
			return err
		}
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesCompare, err = metrics.CompareRequestsSummary(prevSummary, ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWrites)
		if err != nil {
			ts.cfg.Logger.Warn("failed to compare results", zap.Error(err))
			return err
		}
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesCompareJSONPath, []byte(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesCompare.JSON()), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesCompareJSONS3Key,
			ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesCompareJSONPath,
		); err != nil {
			return err
		}
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesCompareTablePath, []byte(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesCompare.Table()), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesCompareTableS3Key,
			ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesCompareTablePath,
		); err != nil {
			return err
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n\nRequestsSummaryWritesCompare:\n%s\n", ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesCompare.Table())

		var prevDurations metrics.Durations
		prevDurations, err = metrics.DownloadDurationsFromS3(ts.cfg.Logger, ts.cfg.S3API, ts.cfg.EKSConfig.S3BucketName, durRawS3Key)
		if err != nil {
			ts.cfg.Logger.Warn("failed to download results", zap.Error(err))
			return err
		}
		prevDurationsWithLabels := metrics.LabelDurations(prevDurations, prevSummary.TestID)
		curDurationsWithLabels := metrics.LabelDurations(curWriteLatencies, ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWrites.TestID)
		allDurationsWithLabels := append(prevDurationsWithLabels, curDurationsWithLabels...)
		sortStart := time.Now()
		ts.cfg.Logger.Info("sorting before and after durations with label",
			zap.Int("before-data-points", len(prevDurationsWithLabels)),
			zap.Int("after-data-points", len(curDurationsWithLabels)),
			zap.Int("total-points", len(allDurationsWithLabels)),
		)
		sort.Sort(allDurationsWithLabels)
		ts.cfg.Logger.Info("sorted before and after durations with label",
			zap.Int("before-data-points", len(prevDurationsWithLabels)),
			zap.Int("after-data-points", len(curDurationsWithLabels)),
			zap.Int("total-points", len(allDurationsWithLabels)),
			zap.String("took", time.Since(sortStart).String()),
		)
		allDataJSON, err := json.Marshal(allDurationsWithLabels)
		if err != nil {
			ts.cfg.Logger.Warn("failed to marshal results", zap.Error(err))
			return err
		}
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsRawWritesCompareAllJSONPath, []byte(allDataJSON), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsRawWritesCompareAllJSONS3Key,
			ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsRawWritesCompareAllJSONPath,
		); err != nil {
			return err
		}
		if err = allDurationsWithLabels.CSV(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsRawWritesCompareAllCSVPath); err != nil {
			return err
		}
		if err = aws_s3.Upload(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsRawWritesCompareAllCSVS3Key,
			ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsRawWritesCompareAllCSVPath,
		); err != nil {
			return err
		}
	} else {
		ts.cfg.Logger.Warn("previous writes summary not found; skipping comparison", zap.Error(err))
	}
	ts.cfg.Logger.Info("uploading new writes summary to s3 bucket to overwrite the previous")
	if err = aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		path.Join(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsRawWritesCompareS3Dir, curTS),
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsRawWritesJSONPath,
	); err != nil {
		return err
	}
	if err = aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		path.Join(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesCompareS3Dir, curTS),
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWritesJSONPath,
	); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) publishResults() (err error) {
	tv := aws.Time(time.Now().UTC())
	datums := make([]*cloudwatch.MetricDatum, 0)
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-configmaps-remote-writes-latency-p50"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWrites.LantencyP50.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-configmaps-remote-writes-latency-p90"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWrites.LantencyP90.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-configmaps-remote-writes-latency-p99"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWrites.LantencyP99.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-configmaps-remote-writes-latency-p999"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWrites.LantencyP999.Milliseconds())),
	})
	datums = append(datums, &cloudwatch.MetricDatum{
		Timestamp:  tv,
		MetricName: aws.String("add-on-configmaps-remote-writes-latency-p9999"),
		Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		Value:      aws.Float64(float64(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsSummaryWrites.LantencyP9999.Milliseconds())),
	})
	return cw.PutData(ts.cfg.Logger, ts.cfg.CWAPI, ts.cfg.EKSConfig.CWNamespace, 20, datums...)
}
