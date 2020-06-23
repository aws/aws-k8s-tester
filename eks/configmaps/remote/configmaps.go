// Package remote implements tester for ConfigMap.
package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	aws_ecr "github.com/aws/aws-k8s-tester/pkg/aws/ecr"
	aws_s3 "github.com/aws/aws-k8s-tester/pkg/aws/s3"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/metrics"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines configmaps configuration.
// ref. https://github.com/kubernetes/perf-tests
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	S3API     s3iface.S3API
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
	if err = ts.createALBRBACClusterRole(); err != nil {
		return err
	}
	if err = ts.createALBRBACClusterRoleBinding(); err != nil {
		return err
	}
	if err = ts.createConfigMap(); err != nil {
		return err
	}
	if err = ts.createDeployment(); err != nil {
		return err
	}
	if err = ts.waitDeployment(); err != nil {
		return err
	}

	select {
	case <-ts.cfg.Stopc:
		ts.cfg.Logger.Warn("configmaps tester aborted")
		return errors.New("configmaps tester aborted")
	case <-time.After(15 * time.Second):
		// wait for results writes
	}

	waitDur, retryStart := 5*time.Minute, time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("health check aborted")
			return nil
		case <-time.After(5 * time.Second):
		}
		err = ts.cfg.K8SClient.CheckHealth()
		if err == nil {
			break
		}
		ts.cfg.Logger.Warn("health check failed", zap.Error(err))
	}
	ts.cfg.EKSConfig.Sync()
	if err == nil {
		ts.cfg.Logger.Info("health check success after configmap testing")
	} else {
		ts.cfg.Logger.Warn("health check failed after configmap testing", zap.Error(err))
	}
	return err
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

	if err := ts.deleteDeployment(); err != nil {
		errs = append(errs, err.Error())
	}
	time.Sleep(2 * time.Minute)

	if err := ts.deleteConfigMap(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ts.deleteALBRBACClusterRoleBinding(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ts.deleteALBRBACClusterRole(); err != nil {
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
	configmapsDeploymentName              = "configmaps-remote-deployment"
	configmapsAppName                     = "configmaps-remote-app"
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
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete configmaps ServiceAccount (%v)", err)
	}
	ts.cfg.Logger.Info("deleted configmaps ServiceAccount", zap.Error(err))

	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createALBRBACClusterRole() error {
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
func (ts *tester) deleteALBRBACClusterRole() error {
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
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete configmaps RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("deleted configmaps RBAC ClusterRole", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createALBRBACClusterRoleBinding() error {
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
func (ts *tester) deleteALBRBACClusterRoleBinding() error {
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
	if err != nil && !api_errors.IsNotFound(err) {
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

func (ts *tester) createDeployment() error {
	// "/opt/"+configmapsKubeConfigConfigMapFileName,
	// do not specify "kubeconfig", and use in-cluster config via "pkg/k8s-client"
	// otherwise, error "namespaces is forbidden: User "system:node:ip-192-168-84..."
	// ref. https://github.com/kubernetes/client-go/blob/master/examples/in-cluster-client-configuration/main.go
	testerCmd := fmt.Sprintf("/aws-k8s-tester eks create configmaps --partition=%s --region=%s --s3-bucket-name=%s --s3-dir-name=%s --clients=%d --client-qps=%f --client-burst=%d --client-timeout=%s --namespace=%s --objects=%d --object-size=%d --writes-output-name-prefix=%s --block=true",
		ts.cfg.EKSConfig.Partition,
		ts.cfg.EKSConfig.Region,
		ts.cfg.EKSConfig.S3BucketName,
		path.Join(ts.cfg.EKSConfig.Name, "add-on-configmaps-remote"),
		ts.cfg.EKSConfig.Clients,
		ts.cfg.EKSConfig.ClientQPS,
		ts.cfg.EKSConfig.ClientBurst,
		ts.cfg.EKSConfig.ClientTimeout,
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.Namespace,
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.Objects,
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.ObjectSize,
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryOutputNamePrefix,
	)

	ts.cfg.Logger.Info("creating configmaps Deployment", zap.String("image", ts.ecrImage), zap.String("tester-command", testerCmd))
	dirOrCreate := v1.HostPathDirectoryOrCreate
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnConfigmapsRemote.Namespace).
		Create(
			ctx,
			&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      configmapsDeploymentName,
					Namespace: ts.cfg.EKSConfig.AddOnConfigmapsRemote.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": configmapsAppName,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: aws.Int32(ts.cfg.EKSConfig.AddOnConfigmapsRemote.DeploymentReplicas),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": configmapsAppName,
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": configmapsAppName,
							},
						},
						Spec: v1.PodSpec{
							ServiceAccountName: configmapsServiceAccountName,

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
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create configmaps Deployment (%v)", err)
	}
	return nil
}

func (ts *tester) deleteDeployment() error {
	ts.cfg.Logger.Info("deleting deployment")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnConfigmapsRemote.Namespace).
		Delete(
			ctx,
			configmapsDeploymentName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("deleted deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitDeployment() error {
	ts.cfg.Logger.Info("waiting for configmaps Deployment")
	args := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnConfigmapsRemote.Namespace,
		"describe",
		"deployment",
		configmapsDeploymentName,
	}
	cmd := strings.Join(args, " ")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'%s' failed %v", cmd, err)
	}
	out := string(output)
	fmt.Printf("\n\n\"%s\" output:\n%s\n\n", cmd, out)

	ready := false
	waitDur := 7*time.Minute + time.Duration(ts.cfg.EKSConfig.AddOnConfigmapsRemote.DeploymentReplicas)*time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("check aborted")
		case <-time.After(15 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		dresp, err := ts.cfg.K8SClient.KubernetesClientSet().
			AppsV1().
			Deployments(ts.cfg.EKSConfig.AddOnConfigmapsRemote.Namespace).
			Get(ctx, configmapsDeploymentName, metav1.GetOptions{})
		cancel()
		if err != nil {
			return fmt.Errorf("failed to get Deployment (%v)", err)
		}
		ts.cfg.Logger.Info("get deployment",
			zap.Int32("desired-replicas", dresp.Status.Replicas),
			zap.Int32("available-replicas", dresp.Status.AvailableReplicas),
			zap.Int32("unavailable-replicas", dresp.Status.UnavailableReplicas),
			zap.Int32("ready-replicas", dresp.Status.ReadyReplicas),
		)
		available := false
		for _, cond := range dresp.Status.Conditions {
			ts.cfg.Logger.Info("condition",
				zap.String("last-updated", cond.LastUpdateTime.String()),
				zap.String("type", string(cond.Type)),
				zap.String("status", string(cond.Status)),
				zap.String("reason", cond.Reason),
				zap.String("message", cond.Message),
			)
			if cond.Status != v1.ConditionTrue {
				continue
			}
			if cond.Type == appsv1.DeploymentAvailable {
				available = true
				break
			}
		}
		if available && dresp.Status.AvailableReplicas >= ts.cfg.EKSConfig.AddOnConfigmapsRemote.DeploymentReplicas {
			ready = true
			break
		}

		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		output, err = exec.New().CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
		cancel()
		out := string(output)
		if err != nil {
			ts.cfg.Logger.Warn("describe failed", zap.String("command", cmd), zap.Error(err))
		} else {
			fmt.Printf("\n\n\"%s\" output:\n%s\n\n", cmd, out)
		}
	}
	if !ready {
		ts.cfg.Logger.Warn("deployment not ready")
		return errors.New("deployment not ready")
	}

	ts.cfg.Logger.Info("waited for configmaps Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) AggregateResults() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnConfigmapsRemote() {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnConfigmapsRemote.Created {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.AggregateResults", zap.String("tester", pkgName))
	writesSummary := metrics.RequestsSummary{TestID: time.Now().UTC().Format(time.RFC3339Nano)}
	writeLatencies := make(metrics.Durations, 0, 20000)

	writesDir := ""

	writesDir, err = aws_s3.DownloadDir(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		path.Join(ts.cfg.EKSConfig.Name, "add-on-configmaps-remote", "writes"),
	)
	if err == nil {
		ts.cfg.Logger.Info("reading writes results", zap.String("writes-dir", writesDir))
		cnt := 0
		err = filepath.Walk(writesDir, func(fpath string, info os.FileInfo, werr error) error {
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

			case strings.HasSuffix(fpath, "-writes.json"):
				b, err := ioutil.ReadFile(fpath)
				if err != nil {
					return fmt.Errorf("failed to open %q (%v)", fpath, err)
				}
				var r metrics.Durations
				if err = json.Unmarshal(b, &r); err != nil {
					return fmt.Errorf("failed to unmarshal %q (%s, %v)", fpath, string(b), err)
				}
				writeLatencies = append(writeLatencies, r...)
			}
			return nil
		})
		if err != nil || cnt == 0 {
			ts.cfg.Logger.Warn("failed to read writes results", zap.Int("file-count", cnt), zap.Error(err))
			os.RemoveAll(writesDir)
			writesDir = ""
		}
	}

	aggSucceed := writesDir != ""
	if !aggSucceed {
		writesSummary = metrics.RequestsSummary{TestID: time.Now().UTC().Format(time.RFC3339Nano)}
		writeLatencies = make(metrics.Durations, 0, 20000)

		if ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() && ts.cfg.EKSConfig.AddOnNodeGroups.FetchLogs {
			ts.cfg.Logger.Info("fetching logs from ngs")
			for _, v := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
				for _, fpaths := range v.Logs {
					for _, fpath := range fpaths {
						if strings.Contains(fpath, ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryOutputNamePrefix) {
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

							case strings.HasSuffix(fpath, "-writes.json"):
								b, err := ioutil.ReadFile(fpath)
								if err != nil {
									return fmt.Errorf("failed to open %q (%v)", fpath, err)
								}
								var r metrics.Durations
								if err = json.Unmarshal(b, &r); err != nil {
									return fmt.Errorf("failed to unmarshal %q (%s, %v)", fpath, string(b), err)
								}
								writeLatencies = append(writeLatencies, r...)
							}
						}
					}
				}
			}
		}
		if ts.cfg.EKSConfig.IsEnabledAddOnManagedNodeGroups() && ts.cfg.EKSConfig.AddOnManagedNodeGroups.FetchLogs {
			ts.cfg.Logger.Info("fetching logs from mngs")
			for _, cur := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
				for _, fpaths := range cur.Logs {
					for _, fpath := range fpaths {
						if strings.Contains(fpath, ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryOutputNamePrefix) {
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

							case strings.HasSuffix(fpath, "-writes.json"):
								b, err := ioutil.ReadFile(fpath)
								if err != nil {
									return fmt.Errorf("failed to open %q (%v)", fpath, err)
								}
								var r metrics.Durations
								if err = json.Unmarshal(b, &r); err != nil {
									return fmt.Errorf("failed to unmarshal %q (%s, %v)", fpath, string(b), err)
								}
								writeLatencies = append(writeLatencies, r...)
							}
						}
					}
				}
			}
		}
	}

	sortStart := time.Now()
	ts.cfg.Logger.Info("sorting write latencies")
	sort.Sort(writeLatencies)
	ts.cfg.Logger.Info("sorted write latencies", zap.String("took", time.Since(sortStart).String()))
	writesSummary.LantencyP50 = writeLatencies.PickLantencyP50()
	writesSummary.LantencyP90 = writeLatencies.PickLantencyP90()
	writesSummary.LantencyP99 = writeLatencies.PickLantencyP99()
	writesSummary.LantencyP999 = writeLatencies.PickLantencyP999()
	writesSummary.LantencyP9999 = writeLatencies.PickLantencyP9999()
	ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummary = writesSummary
	ts.cfg.EKSConfig.Sync()

	wb, err := json.Marshal(writeLatencies)
	if err != nil {
		ts.cfg.Logger.Warn("failed to encode JSON", zap.Error(err))
		return err
	}
	if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesJSONPath, wb, 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return err
	}
	if err = aws_s3.Upload(ts.cfg.Logger, ts.cfg.S3API, ts.cfg.EKSConfig.S3BucketName, path.Join(ts.cfg.EKSConfig.Name, "add-on-configmaps-remote", "writes", filepath.Base(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesJSONPath)), ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesJSONPath); err != nil {
		return err
	}
	if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryJSONPath, []byte(writesSummary.JSON()), 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return err
	}
	if err = aws_s3.Upload(ts.cfg.Logger, ts.cfg.S3API, ts.cfg.EKSConfig.S3BucketName, path.Join(ts.cfg.EKSConfig.Name, "add-on-configmaps-remote", "writes", filepath.Base(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryJSONPath)), ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryJSONPath); err != nil {
		return err
	}
	if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryTablePath, []byte(writesSummary.Table()), 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return err
	}
	if err = aws_s3.Upload(ts.cfg.Logger, ts.cfg.S3API, ts.cfg.EKSConfig.S3BucketName, path.Join(ts.cfg.EKSConfig.Name, "add-on-configmaps-remote", "writes", filepath.Base(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryTablePath)), ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryTablePath); err != nil {
		return err
	}
	fmt.Printf("\n\nRequestsWritesSummary:\n%s\n", writesSummary.Table())

	ts.cfg.Logger.Info("aggregated results from Pods; now comparing previous results")
	if err = ts.compareResults(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

// 1. if previous summary exists, download and compare
// 2. upload new summary and overwrite the previous s3 key
func (ts *tester) compareResults() (err error) {
	tss := time.Now().UTC().Format(time.RFC3339Nano)
	ts.cfg.Logger.Info("comparing results", zap.String("timestamp", tss))

	s3Objects := make([]*s3.Object, 0)
	if ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryS3Dir != "" {
		s3Objects, err = aws_s3.ListInDescendingLastModified(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryS3Dir,
		)
	}
	if len(s3Objects) > 0 && err == nil {
		var localPath string
		localPath, err = aws_s3.DownloadToTempFile(
			ts.cfg.Logger,
			ts.cfg.S3API,
			ts.cfg.EKSConfig.S3BucketName,
			aws.StringValue(s3Objects[0].Key),
		)
		if err != nil {
			return fmt.Errorf("failed to download previous writes summary %v", err)
		}
		defer os.RemoveAll(localPath)
		rf, err := os.OpenFile(localPath, os.O_RDONLY, 0444)
		if err != nil {
			ts.cfg.Logger.Warn("failed to read a file", zap.Error(err))
			return err
		}
		defer rf.Close()
		var prev metrics.RequestsSummary
		if err = json.NewDecoder(rf).Decode(&prev); err != nil {
			ts.cfg.Logger.Warn("failed to decode a JSON file", zap.Error(err))
			return err
		}
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryCompare, err = metrics.CompareRequestsSummary(prev, ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummary)
		if err != nil {
			ts.cfg.Logger.Warn("failed to compare results", zap.Error(err))
			return err
		}
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryCompareJSONPath, []byte(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryCompare.JSON()), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(ts.cfg.Logger, ts.cfg.S3API, ts.cfg.EKSConfig.S3BucketName, path.Join(ts.cfg.EKSConfig.Name, "add-on-stresser-local", "writes", filepath.Base(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryCompareJSONPath)), ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryCompareJSONPath); err != nil {
			return err
		}
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryCompareTablePath, []byte(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryCompare.Table()), 0600); err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		if err = aws_s3.Upload(ts.cfg.Logger, ts.cfg.S3API, ts.cfg.EKSConfig.S3BucketName, path.Join(ts.cfg.EKSConfig.Name, "add-on-stresser-local", "writes", filepath.Base(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryCompareTablePath)), ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryCompareTablePath); err != nil {
			return err
		}
		fmt.Printf("\n\nRequestsWritesSummaryCompare:\n%s\n", ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryCompare.Table())
	} else {
		ts.cfg.Logger.Warn("previous writes summary not found; skipping comparison", zap.Error(err))
	}
	ts.cfg.Logger.Info("uploading new writes summary to s3 bucket to overwrite the previous")
	if err = aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		path.Join(ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryS3Dir, tss),
		ts.cfg.EKSConfig.AddOnConfigmapsRemote.RequestsWritesSummaryJSONPath,
	); err != nil {
		return err
	}

	return nil
}
