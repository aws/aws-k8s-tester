// Package remote implements cluster remote load tests.
// ref. https://github.com/kubernetes/perf-tests
// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	aws_ecr "github.com/aws/aws-k8s-tester/pkg/aws/ecr"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/metrics"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines stresser configuration.
// ref. https://github.com/kubernetes/perf-tests
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	ECRAPI    ecriface.ECRAPI
}

// Tester defines stresser tester.
// ref. https://github.com/kubernetes/perf-tests
type Tester interface {
	// Create installs stresser tester.
	Create() error
	// Delete deletes stresser tester.
	Delete() error
	// AggregateResults aggregates all test results from remote nodes.
	AggregateResults() error
}

// TODO: use kubemark
// nodelease.NewController, kubemark.GetHollowKubeletConfig

func New(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg      Config
	ecrImage string
}

func (ts *tester) Create() (err error) {
	if ts.cfg.EKSConfig.AddOnStresserRemote.Created {
		ts.cfg.Logger.Info("skipping create AddOnStresserRemote")
		return nil
	}

	ts.cfg.Logger.Info("starting load testing", zap.Duration("duration", ts.cfg.EKSConfig.AddOnStresserRemote.Duration))
	ts.cfg.EKSConfig.AddOnStresserRemote.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnStresserRemote.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if ts.ecrImage, err = aws_ecr.Check(
		ts.cfg.Logger,
		ts.cfg.ECRAPI,
		ts.cfg.EKSConfig.AddOnStresserRemote.RepositoryAccountID,
		ts.cfg.EKSConfig.AddOnStresserRemote.RepositoryName,
		ts.cfg.EKSConfig.AddOnStresserRemote.RepositoryImageTag,
	); err != nil {
		return err
	}
	if err = k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnStresserRemote.Namespace,
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
		ts.cfg.Logger.Warn("stresser aborted")
		return nil

	case <-time.After(ts.cfg.EKSConfig.AddOnStresserRemote.Duration):
		ts.cfg.Logger.Info("completing load testing", zap.Duration("duration", ts.cfg.EKSConfig.AddOnStresserRemote.Duration))

		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("stresser aborted")
			return nil
		case <-time.After(30 * time.Second):
		}
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
		ts.cfg.Logger.Info("health check success after load testing")
	} else {
		ts.cfg.Logger.Warn("health check failed after load testing", zap.Error(err))
	}
	return err
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnStresserRemote.Created {
		ts.cfg.Logger.Info("skipping delete AddOnStresserRemote")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnStresserRemote.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteDeployment(); err != nil {
		errs = append(errs, err.Error())
	}
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
		ts.cfg.EKSConfig.AddOnStresserRemote.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete stresser namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnStresserRemote.Created = false
	return ts.cfg.EKSConfig.Sync()
}

const (
	stresserServiceAccountName          = "stresser-remote-service-account"
	stresserRBACRoleName                = "stresser-remote-rbac-role"
	stresserRBACClusterRoleBindingName  = "stresser-remote-rbac-role-binding"
	stresserKubeConfigConfigMapName     = "stresser-remote-kubeconfig-config-map"
	stresserKubeConfigConfigMapFileName = "stresser-remote-kubeconfig-config-map.yaml"
	stresserDeploymentName              = "stresser-remote-deployment"
	stresserAppName                     = "stresser-remote-app"
)

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createServiceAccount() error {
	ts.cfg.Logger.Info("creating stresser ServiceAccount")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnStresserRemote.Namespace).
		Create(
			ctx,
			&v1.ServiceAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      stresserServiceAccountName,
					Namespace: ts.cfg.EKSConfig.AddOnStresserRemote.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": stresserAppName,
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create stresser ServiceAccount (%v)", err)
	}

	ts.cfg.Logger.Info("created stresser ServiceAccount")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteServiceAccount() error {
	ts.cfg.Logger.Info("deleting stresser ServiceAccount")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnStresserRemote.Namespace).
		Delete(
			ctx,
			stresserServiceAccountName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete stresser ServiceAccount (%v)", err)
	}
	ts.cfg.Logger.Info("deleted stresser ServiceAccount", zap.Error(err))

	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createALBRBACClusterRole() error {
	ts.cfg.Logger.Info("creating stresser RBAC ClusterRole")
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
					Name:      stresserRBACRoleName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": stresserAppName,
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{
							"*",
						},
						Resources: []string{
							"leases",
							"nodes",
							"pods",
							"secrets",
							"services",
							"namespaces",
							"configmaps",
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
		return fmt.Errorf("failed to create stresser RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("created stresser RBAC ClusterRole")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteALBRBACClusterRole() error {
	ts.cfg.Logger.Info("deleting stresser RBAC ClusterRole")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoles().
		Delete(
			ctx,
			stresserRBACRoleName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete stresser RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("deleted stresser RBAC ClusterRole", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createALBRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("creating stresser RBAC ClusterRoleBinding")
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
					Name:      stresserRBACClusterRoleBindingName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": stresserAppName,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     stresserRBACRoleName,
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup:  "",
						Kind:      "ServiceAccount",
						Name:      stresserServiceAccountName,
						Namespace: ts.cfg.EKSConfig.AddOnStresserRemote.Namespace,
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
		return fmt.Errorf("failed to create stresser RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("created stresser RBAC ClusterRoleBinding")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteALBRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("deleting stresser RBAC ClusterRoleBinding")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoleBindings().
		Delete(
			ctx,
			stresserRBACClusterRoleBindingName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete stresser RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("deleted stresser RBAC ClusterRoleBinding", zap.Error(err))
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
		ConfigMaps(ts.cfg.EKSConfig.AddOnStresserRemote.Namespace).
		Create(
			ctx,
			&v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      stresserKubeConfigConfigMapName,
					Namespace: ts.cfg.EKSConfig.AddOnStresserRemote.Namespace,
					Labels: map[string]string{
						"name": stresserKubeConfigConfigMapName,
					},
				},
				Data: map[string]string{
					stresserKubeConfigConfigMapFileName: string(b),
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
		ConfigMaps(ts.cfg.EKSConfig.AddOnStresserRemote.Namespace).
		Delete(
			ctx,
			stresserKubeConfigConfigMapName,
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	nss, err := ts.cfg.K8SClient.KubernetesClientSet().CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	cancel()
	if err != nil {
		ts.cfg.Logger.Warn("list namespaces failed", zap.Error(err))
		return err
	}
	ns := make([]string, 0, len(nss.Items))
	for _, nv := range nss.Items {
		ns = append(ns, nv.GetName())
	}

	// "/opt/"+stresserKubeConfigConfigMapFileName,
	// do not specify "kubeconfig", and use in-cluster config via "pkg/k8s-client"
	// otherwise, error "namespaces is forbidden: User "system:node:ip-192-168-84..."
	// ref. https://github.com/kubernetes/client-go/blob/master/examples/in-cluster-client-configuration/main.go
	testerCmd := fmt.Sprintf("/aws-k8s-tester eks create stresser --clients=%d --client-qps=%f --client-burst=%d --client-timeout=%s --object-size=%d --list-limit=%d --duration=%s --namespace-write=%s --namespaces-read=%s --writes-output-name-prefix=%s --reads-output-name-prefix=%s --block=true",
		ts.cfg.EKSConfig.Clients,
		ts.cfg.EKSConfig.ClientQPS,
		ts.cfg.EKSConfig.ClientBurst,
		ts.cfg.EKSConfig.ClientTimeout,
		ts.cfg.EKSConfig.AddOnStresserRemote.ObjectSize,
		ts.cfg.EKSConfig.AddOnStresserRemote.ListLimit,
		ts.cfg.EKSConfig.AddOnStresserRemote.Duration,
		ts.cfg.EKSConfig.AddOnStresserRemote.Namespace,
		strings.Join(ns, ","),
		ts.cfg.EKSConfig.AddOnStresserRemote.RequestsSummaryWritesOutputNamePrefix,
		ts.cfg.EKSConfig.AddOnStresserRemote.RequestsSummaryReadsOutputNamePrefix,
	)

	ts.cfg.Logger.Info("creating stresser Deployment", zap.String("image", ts.ecrImage), zap.String("tester-command", testerCmd))
	dirOrCreate := v1.HostPathDirectoryOrCreate
	ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnStresserRemote.Namespace).
		Create(
			ctx,
			&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      stresserDeploymentName,
					Namespace: ts.cfg.EKSConfig.AddOnStresserRemote.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": stresserAppName,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: aws.Int32(ts.cfg.EKSConfig.AddOnStresserRemote.DeploymentReplicas),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": stresserAppName,
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": stresserAppName,
							},
						},
						Spec: v1.PodSpec{
							ServiceAccountName: stresserServiceAccountName,

							// TODO: set resource limits
							Containers: []v1.Container{
								{
									Name:            stresserAppName,
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
											Name:      stresserKubeConfigConfigMapName,
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
									Name: stresserKubeConfigConfigMapName,
									VolumeSource: v1.VolumeSource{
										ConfigMap: &v1.ConfigMapVolumeSource{
											LocalObjectReference: v1.LocalObjectReference{
												Name: stresserKubeConfigConfigMapName,
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
		return fmt.Errorf("failed to create stresser Deployment (%v)", err)
	}
	return nil
}

func (ts *tester) deleteDeployment() error {
	ts.cfg.Logger.Info("deleting deployment")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnStresserRemote.Namespace).
		Delete(
			ctx,
			stresserDeploymentName,
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
	ts.cfg.Logger.Info("waiting for stresser Deployment")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(
		ctx,
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace="+ts.cfg.EKSConfig.AddOnStresserRemote.Namespace,
		"describe",
		"deployment",
		stresserDeploymentName,
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl describe deployment' failed %v", err)
	}
	out := string(output)
	fmt.Printf("\n\n\"kubectl describe deployment\" output:\n%s\n\n", out)

	ready := false
	waitDur := 5*time.Minute + time.Duration(ts.cfg.EKSConfig.AddOnStresserRemote.DeploymentReplicas)*time.Minute
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
			Deployments(ts.cfg.EKSConfig.AddOnStresserRemote.Namespace).
			Get(ctx, stresserDeploymentName, metav1.GetOptions{})
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
		if available && dresp.Status.AvailableReplicas >= ts.cfg.EKSConfig.AddOnStresserRemote.DeploymentReplicas {
			ready = true
			break
		}
	}
	if !ready {
		// TODO: return error...
		// return errors.New("Deployment not ready")
		ts.cfg.Logger.Warn("Deployment not ready")
	}

	ts.cfg.Logger.Info("waited for stresser Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) AggregateResults() (err error) {
	if !ts.cfg.EKSConfig.AddOnStresserRemote.Created {
		ts.cfg.Logger.Info("skipping aggregating AddOnStresserRemote")
		return nil
	}

	ts.cfg.Logger.Info("aggregating results from Pods")
	writes, reads := metrics.RequestsSummary{}, metrics.RequestsSummary{}
	if ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() && ts.cfg.EKSConfig.AddOnNodeGroups.FetchLogs {
		ts.cfg.Logger.Info("fetching logs from ngs")
		for _, v := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
			for _, fpaths := range v.Logs {
				for _, fpath := range fpaths {
					if strings.Contains(fpath, ts.cfg.EKSConfig.AddOnStresserRemote.RequestsSummaryWritesOutputNamePrefix) && strings.HasSuffix(fpath, "-writes.json") {
						b, err := ioutil.ReadFile(fpath)
						if err != nil {
							return fmt.Errorf("failed to open %q (%v)", fpath, err)
						}
						var r metrics.RequestsSummary
						if err = json.Unmarshal(b, &r); err != nil {
							return fmt.Errorf("failed to unmarshal %q (%s, %v)", fpath, string(b), err)
						}
						writes.SuccessTotal += r.SuccessTotal
						writes.FailureTotal += r.FailureTotal
						if writes.LatencyHistogram == nil || len(writes.LatencyHistogram) == 0 {
							writes.LatencyHistogram = r.LatencyHistogram
						} else {
							writes.LatencyHistogram, err = metrics.MergeHistograms(writes.LatencyHistogram, r.LatencyHistogram)
							if err != nil {
								return fmt.Errorf("failed to merge histograms (%v)", err)
							}
						}
					}
					if strings.Contains(fpath, ts.cfg.EKSConfig.AddOnStresserRemote.RequestsSummaryReadsOutputNamePrefix) && strings.HasSuffix(fpath, "-reads.json") {
						b, err := ioutil.ReadFile(fpath)
						if err != nil {
							return fmt.Errorf("failed to open %q (%v)", fpath, err)
						}
						var r metrics.RequestsSummary
						if err = json.Unmarshal(b, &r); err != nil {
							return fmt.Errorf("failed to unmarshal %q (%s, %v)", fpath, string(b), err)
						}
						reads.SuccessTotal += r.SuccessTotal
						reads.FailureTotal += r.FailureTotal
						if reads.LatencyHistogram == nil || len(reads.LatencyHistogram) == 0 {
							reads.LatencyHistogram = r.LatencyHistogram
						} else {
							reads.LatencyHistogram, err = metrics.MergeHistograms(reads.LatencyHistogram, r.LatencyHistogram)
							if err != nil {
								return fmt.Errorf("failed to merge histograms (%v)", err)
							}
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
					if strings.Contains(fpath, ts.cfg.EKSConfig.AddOnStresserRemote.RequestsSummaryWritesOutputNamePrefix) && strings.HasSuffix(fpath, "-writes.json") {
						b, err := ioutil.ReadFile(fpath)
						if err != nil {
							return fmt.Errorf("failed to open %q (%v)", fpath, err)
						}
						var r metrics.RequestsSummary
						if err = json.Unmarshal(b, &r); err != nil {
							return fmt.Errorf("failed to unmarshal %q (%s, %v)", fpath, string(b), err)
						}
						writes.SuccessTotal += r.SuccessTotal
						writes.FailureTotal += r.FailureTotal
						if writes.LatencyHistogram == nil || len(writes.LatencyHistogram) == 0 {
							writes.LatencyHistogram = r.LatencyHistogram
						} else {
							writes.LatencyHistogram, err = metrics.MergeHistograms(writes.LatencyHistogram, r.LatencyHistogram)
							if err != nil {
								return fmt.Errorf("failed to merge histograms (%v)", err)
							}
						}
					}
					if strings.Contains(fpath, ts.cfg.EKSConfig.AddOnStresserRemote.RequestsSummaryReadsOutputNamePrefix) && strings.HasSuffix(fpath, "-reads.json") {
						b, err := ioutil.ReadFile(fpath)
						if err != nil {
							return fmt.Errorf("failed to open %q (%v)", fpath, err)
						}
						var r metrics.RequestsSummary
						if err = json.Unmarshal(b, &r); err != nil {
							return fmt.Errorf("failed to unmarshal %q (%s, %v)", fpath, string(b), err)
						}
						reads.SuccessTotal += r.SuccessTotal
						reads.FailureTotal += r.FailureTotal
						if reads.LatencyHistogram == nil || len(reads.LatencyHistogram) == 0 {
							reads.LatencyHistogram = r.LatencyHistogram
						} else {
							reads.LatencyHistogram, err = metrics.MergeHistograms(reads.LatencyHistogram, r.LatencyHistogram)
							if err != nil {
								return fmt.Errorf("failed to merge histograms (%v)", err)
							}
						}
					}
				}
			}
		}
	}

	ts.cfg.EKSConfig.AddOnStresserRemote.RequestsSummaryWrites = writes
	ts.cfg.EKSConfig.AddOnStresserRemote.RequestsSummaryReads = reads
	ts.cfg.EKSConfig.Sync()

	if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnStresserRemote.RequestsSummaryWritesJSONPath, []byte(writes.JSON()), 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return err
	}
	if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnStresserRemote.RequestsSummaryWritesTablePath, []byte(writes.Table()), 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return err
	}
	fmt.Printf("\n\nAddOnStresserRemote.RequestsSummaryWrites:\n%s\n", writes.Table())
	if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnStresserRemote.RequestsSummaryReadsJSONPath, []byte(reads.JSON()), 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return err
	}
	if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnStresserRemote.RequestsSummaryReadsTablePath, []byte(reads.Table()), 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return err
	}
	fmt.Printf("\n\nAddOnStresserRemote.RequestsSummaryReads:\n%s\n", reads.Table())

	ts.cfg.Logger.Info("aggregated results from Pods")
	return ts.cfg.EKSConfig.Sync()
}
