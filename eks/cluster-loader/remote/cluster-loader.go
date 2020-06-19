// Package remote implements remote cluster loader.
// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2
// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2/testing/overrides
package remote

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"time"

	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	aws_ecr "github.com/aws/aws-k8s-tester/pkg/aws/ecr"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-k8s-tester/ssh"
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

// Config defines cluster loader configuration.
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	ECRAPI    ecriface.ECRAPI
}

func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg      Config
	ecrImage string
}

func (ts *tester) Create() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnClusterLoaderRemote() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if ts.ecrImage, err = aws_ecr.Check(
		ts.cfg.Logger,
		ts.cfg.ECRAPI,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.RepositoryAccountID,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.RepositoryName,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.RepositoryImageTag,
	); err != nil {
		return err
	}
	if err = k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace,
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
	if err = ts.checkClusterLoader(); err != nil {
		return err
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
	if err == nil {
		ts.cfg.Logger.Info("health check success after load testing")
	} else {
		ts.cfg.Logger.Warn("health check failed after load testing", zap.Error(err))
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnClusterLoaderRemote() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
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
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete cluster loader namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Created = false
	return ts.cfg.EKSConfig.Sync()
}

const (
	clusterLoaderServiceAccountName          = "cluster-loader-remote-service-account"
	clusterLoaderRBACRoleName                = "cluster-loader-remote-rbac-role"
	clusterLoaderRBACClusterRoleBindingName  = "cluster-loader-remote-rbac-role-binding"
	clusterLoaderKubeConfigConfigMapName     = "cluster-loader-remote-kubeconfig-configmap"
	clusterLoaderKubeConfigConfigMapFileName = "cluster-loader-remote-kubeconfig-configmap.yaml"
	clusterLoaderDeploymentName              = "cluster-loader-remote-deployment"
	clusterLoaderAppName                     = "cluster-loader-remote-app"
)

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createServiceAccount() error {
	ts.cfg.Logger.Info("creating cluster loader ServiceAccount")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace).
		Create(
			ctx,
			&v1.ServiceAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterLoaderServiceAccountName,
					Namespace: ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": clusterLoaderAppName,
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to create cluster loader ServiceAccount (%v)", err)
	}

	ts.cfg.Logger.Info("created cluster loader ServiceAccount")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteServiceAccount() error {
	ts.cfg.Logger.Info("deleting cluster loader ServiceAccount")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace).
		Delete(
			ctx,
			clusterLoaderServiceAccountName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete cluster loader ServiceAccount (%v)", err)
	}
	ts.cfg.Logger.Info("deleted cluster loader ServiceAccount", zap.Error(err))

	return ts.cfg.EKSConfig.Sync()
}

// need RBAC, otherwise
// kubelet_node_status.go:92] Unable to register node "fake-node-000000-8pkvl" with API server: nodes "fake-node-000000-8pkvl" is forbidden: node "ip-192-168-83-61.us-west-2.compute.internal" is not allowed to modify node "fake-node-000000-8pkvl"
// ref. https://github.com/kubernetes/kubernetes/issues/47695
// ref. https://kubernetes.io/docs/reference/access-authn-authz/node
// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createALBRBACClusterRole() error {
	ts.cfg.Logger.Info("creating cluster loader RBAC ClusterRole")
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
					Name:      clusterLoaderRBACRoleName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": clusterLoaderAppName,
					},
				},
				Rules: []rbacv1.PolicyRule{
					{ // TODO: make it more restrictive
						APIGroups: []string{
							"*",
						},
						Resources: []string{
							"*"},
						Verbs: []string{
							"*",
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create cluster loader RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("created cluster loader RBAC ClusterRole")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteALBRBACClusterRole() error {
	ts.cfg.Logger.Info("deleting cluster loader RBAC ClusterRole")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoles().
		Delete(
			ctx,
			clusterLoaderRBACRoleName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete cluster loader RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("deleted cluster loader RBAC ClusterRole", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createALBRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("creating cluster loader RBAC ClusterRoleBinding")
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
					Name:      clusterLoaderRBACClusterRoleBindingName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": clusterLoaderAppName,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     clusterLoaderRBACRoleName,
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup:  "",
						Kind:      "ServiceAccount",
						Name:      clusterLoaderServiceAccountName,
						Namespace: ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace,
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
		return fmt.Errorf("failed to create cluster loader RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("created cluster loader RBAC ClusterRoleBinding")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteALBRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("deleting cluster loader RBAC ClusterRoleBinding")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoleBindings().
		Delete(
			ctx,
			clusterLoaderRBACClusterRoleBindingName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete cluster loader RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("deleted cluster loader RBAC ClusterRoleBinding", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createConfigMap() error {
	ts.cfg.Logger.Info("creating configmap")

	b, err := ioutil.ReadFile(ts.cfg.EKSConfig.KubeConfigPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ConfigMaps(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace).
		Create(
			ctx,
			&v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterLoaderKubeConfigConfigMapName,
					Namespace: ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace,
					Labels: map[string]string{
						"name": clusterLoaderKubeConfigConfigMapName,
					},
				},
				Data: map[string]string{
					clusterLoaderKubeConfigConfigMapFileName: string(b),
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("created configmap")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteConfigMap() error {
	ts.cfg.Logger.Info("deleting configmap")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ConfigMaps(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace).
		Delete(
			ctx,
			clusterLoaderKubeConfigConfigMapName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("deleted configmap")
	return ts.cfg.EKSConfig.Sync()
}

// TODO: use "ReplicationController" to max out

func (ts *tester) createDeployment() error {
	// "/opt/"+clusterLoaderKubeConfigConfigMapFileName,
	// do not specify "kubeconfig", and use in-cluster config via "pkg/k8s-client"
	// ref. https://github.com/kubernetes/client-go/blob/master/examples/in-cluster-client-configuration/main.go

	// ref. https://github.com/kubernetes/perf-tests/pull/1295
	testerCmd := fmt.Sprintf("/aws-k8s-tester eks create cluster-loader --cluster-loader-path=/clusterloader2 --test-config-path=/clusterloader2-test-config.yaml --report-dir=/var/log/cluster-loader-remote --report-tar-gz-path=/var/log/cluster-loader-remote.tar.gz --logs-path=/var/log/cluster-loader-remote.log --runs=%d --timeout=%v --nodes=%d --nodes-per-namespace=%d --pods-per-node=%d --big-group-size=%d --medium-group-size=%d --small-group-size=%d --small-stateful-sets-per-namespace=%d --medium-stateful-sets-per-namespace=%d --cl2-load-test-throughput=%d --cl2-enable-pvs=%v --prometheus-scrape-kube-proxy=%v --enable-system-pod-metrics=%v",
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Runs,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Timeout,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Nodes,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.NodesPerNamespace,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.PodsPerNode,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.BigGroupSize,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.MediumGroupSize,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.SmallGroupSize,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.SmallStatefulSetsPerNamespace,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.MediumStatefulSetsPerNamespace,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.CL2LoadTestThroughput,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.CL2EnablePVS,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.PrometheusScrapeKubeProxy,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.EnableSystemPodMetrics,
	)

	ts.cfg.Logger.Info("creating cluster loader Deployment", zap.String("image", ts.ecrImage), zap.String("tester-command", testerCmd))
	dirOrCreate := v1.HostPathDirectoryOrCreate
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace).
		Create(
			ctx,
			&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterLoaderDeploymentName,
					Namespace: ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": clusterLoaderAppName,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: aws.Int32(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.DeploymentReplicas),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": clusterLoaderAppName,
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": clusterLoaderAppName,
							},
						},
						Spec: v1.PodSpec{
							ServiceAccountName: clusterLoaderServiceAccountName,

							// TODO: set resource limits
							Containers: []v1.Container{
								{
									Name:            clusterLoaderAppName,
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
											Name:      clusterLoaderKubeConfigConfigMapName,
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
									Name: clusterLoaderKubeConfigConfigMapName,
									VolumeSource: v1.VolumeSource{
										ConfigMap: &v1.ConfigMapVolumeSource{
											LocalObjectReference: v1.LocalObjectReference{
												Name: clusterLoaderKubeConfigConfigMapName,
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
		return fmt.Errorf("failed to create hollow node Deployment (%v)", err)
	}
	return nil
}

func (ts *tester) deleteDeployment() error {
	ts.cfg.Logger.Info("deleting deployment")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace).
		Delete(
			ctx,
			clusterLoaderDeploymentName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("deleted deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitDeployment() error {
	ts.cfg.Logger.Info("waiting for cluster loader Deployment")
	args := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace,
		"describe",
		"deployment",
		clusterLoaderDeploymentName,
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
	waitDur := 7*time.Minute + time.Duration(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.DeploymentReplicas)*time.Minute
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
			Deployments(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace).
			Get(ctx, clusterLoaderDeploymentName, metav1.GetOptions{})
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
		if available && dresp.Status.AvailableReplicas >= ts.cfg.EKSConfig.AddOnClusterLoaderRemote.DeploymentReplicas {
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

	ts.cfg.Logger.Info("waited for cluster loader Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) AggregateResults() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnClusterLoaderRemote() {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Created {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.AggregateResults", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	if ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() && ts.cfg.EKSConfig.AddOnNodeGroups.FetchLogs {
		ts.cfg.Logger.Info("fetching logs from ngs")
		for _, v := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
			for _, fpaths := range v.Logs {
				for _, fpath := range fpaths {
					if strings.HasSuffix(fpath, "cluster-loader-remote.tar.gz") {
						if !fileutil.Exist(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.ReportTarGzPath) {
							if cerr := fileutil.Copy(fpath, ts.cfg.EKSConfig.AddOnClusterLoaderRemote.ReportTarGzPath); cerr != nil {
								ts.cfg.Logger.Warn("found AddOnClusterLoaderRemote cluster loader report dir .tar.gz file but failed to copy", zap.String("original-file-path", fpath), zap.Error(cerr))
							} else {
								ts.cfg.Logger.Info("successfully copied AddOnClusterLoaderRemote cluster loader report dir .tar.gz file", zap.String("original-file-path", fpath), zap.String("copy-file-path", ts.cfg.EKSConfig.AddOnClusterLoaderRemote.ReportTarGzPath))
							}
						} else {
							ts.cfg.Logger.Info("AddOnClusterLoaderRemote cluster loader report dir .tar.gz file already exists; skipping copy", zap.String("original-file-path", fpath), zap.String("copy-file-path", ts.cfg.EKSConfig.AddOnClusterLoaderRemote.ReportTarGzPath))
						}
					}
					if strings.HasSuffix(fpath, "cluster-loader-remote.log") {
						if !fileutil.Exist(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.LogPath) {
							if cerr := fileutil.CopyAppend(fpath, ts.cfg.EKSConfig.AddOnClusterLoaderRemote.LogPath); cerr != nil {
								ts.cfg.Logger.Warn("found AddOnClusterLoaderRemote cluster loader logs file but failed to copy", zap.String("original-file-path", fpath), zap.Error(cerr))
							} else {
								ts.cfg.Logger.Info("successfully copied AddOnClusterLoaderRemote cluster loader logs file", zap.String("original-file-path", fpath), zap.String("copy-file-path", ts.cfg.EKSConfig.AddOnClusterLoaderRemote.LogPath))
							}
						} else {
							ts.cfg.Logger.Info("AddOnClusterLoaderRemote cluster loader report logs file already exists; skipping copy", zap.String("original-file-path", fpath), zap.String("copy-file-path", ts.cfg.EKSConfig.AddOnClusterLoaderRemote.LogPath))
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
					if strings.HasSuffix(fpath, "cluster-loader-remote.tar.gz") {
						if cerr := fileutil.Copy(fpath, ts.cfg.EKSConfig.AddOnClusterLoaderRemote.ReportTarGzPath); cerr != nil {
							ts.cfg.Logger.Warn("found AddOnClusterLoaderRemote cluster loader report dir .tar.gz file but failed to copy", zap.String("original-file-path", fpath), zap.Error(cerr))
						} else {
							ts.cfg.Logger.Info("successfully copied AddOnClusterLoaderRemote cluster loader report dir .tar.gz file", zap.String("original-file-path", fpath), zap.String("copy-file-path", ts.cfg.EKSConfig.AddOnClusterLoaderRemote.LogPath))
						}
					}
					if strings.HasSuffix(fpath, "cluster-loader-remote.log") {
						if cerr := fileutil.CopyAppend(fpath, ts.cfg.EKSConfig.AddOnClusterLoaderRemote.LogPath); cerr != nil {
							ts.cfg.Logger.Warn("found AddOnClusterLoaderRemote cluster loader logs file but failed to copy", zap.String("original-file-path", fpath), zap.Error(cerr))
						} else {
							ts.cfg.Logger.Info("successfully copied AddOnClusterLoaderRemote cluster loader logs file", zap.String("original-file-path", fpath), zap.String("copy-file-path", ts.cfg.EKSConfig.AddOnClusterLoaderRemote.LogPath))
						}
					}
				}
			}
		}
	}

	ts.cfg.Logger.Info("aggregated results from Pods")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) checkClusterLoader() (err error) {
	ts.cfg.Logger.Info("checking pod/cluster-loader-remote-deployment")
	clusterloaderPod := ""
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < 10*time.Minute {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("cluster loader check stopped")
			return nil
		case <-time.After(10 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		var pods *v1.PodList
		pods, err = ts.cfg.K8SClient.
			KubernetesClientSet().
			CoreV1().
			Pods(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace).
			List(ctx, metav1.ListOptions{})
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to list pods", zap.Error(err))
			continue
		}

		for _, pv := range pods.Items {
			ts.cfg.Logger.Info("found pod", zap.String("name", pv.GetName()))
			if strings.HasPrefix(pv.GetName(), "cluster-loader-remote-deployment-") {
				clusterloaderPod = pv.GetName()
				break
			}
		}
		if clusterloaderPod != "" {
			break
		}
	}
	if clusterloaderPod == "" {
		return fmt.Errorf("failed to find pod/cluster-loader-remote-deployment in %q", ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace)
	}
	ts.cfg.Logger.Info("found pod/cluster-loader-remote-deployment", zap.String("pod-name", clusterloaderPod))

	ts.cfg.Logger.Info("checking node name for pod/cluster-loader-remote-deployment")
	nodeName, podPhase := "", v1.PodPending
	retryStart = time.Now()
	for time.Now().Sub(retryStart) < 10*time.Minute {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("cluster loader check stopped")
			return nil
		case <-time.After(10 * time.Second):
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		var pod *v1.Pod
		pod, err = ts.cfg.K8SClient.
			KubernetesClientSet().
			CoreV1().
			Pods(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace).
			Get(ctx, clusterloaderPod, metav1.GetOptions{})
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to get pod", zap.Error(err))
			continue
		}
		podPhase = pod.Status.Phase
		nodeName = pod.Spec.NodeName
		ts.cfg.Logger.Info("pod status",
			zap.String("pod-name", pod.Name),
			zap.String("pod-phase", fmt.Sprintf("%v", podPhase)),
			zap.String("node-name", nodeName),
		)
		if podPhase == v1.PodRunning && nodeName != "" {
			break
		}
	}
	if podPhase != v1.PodRunning || nodeName == "" {
		return fmt.Errorf("failed to find running pod and assigned node for %q", clusterloaderPod)
	}

	sshConfig, ok := ts.cfg.EKSConfig.Status.PrivateDNSToSSHConfig[nodeName]
	if !ok {
		ts.cfg.Logger.Warn("got pod/cluster-loader-remote-deployment, but no SSH config found for private DNS",
			zap.String("pod-name", clusterloaderPod),
			zap.String("pod-phase", fmt.Sprintf("%v", podPhase)),
			zap.String("node-name", nodeName),
		)
		return fmt.Errorf("no SSH config found for node name (private DNS) %q", nodeName)
	}
	ts.cfg.Logger.Info("found node name for running pod/cluster-loader-remote-deployment",
		zap.String("pod-name", clusterloaderPod),
		zap.String("pod-phase", fmt.Sprintf("%v", podPhase)),
		zap.String("node-name", nodeName),
		zap.String("ssh-config", sshConfig.ToString()),
	)

	argsLogsPod := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace,
		"logs",
		fmt.Sprintf("pod/%s", clusterloaderPod),
		"--tail=30",
	}
	cmdLogsPod := strings.Join(argsLogsPod, " ")

	ts.cfg.Logger.Info("checking cluster loader logs", zap.String("logs-command-pod", cmdLogsPod))
	waitDur := 20 * time.Minute * time.Duration(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Runs)
	start := time.Now()
	ready := false
	for time.Now().Sub(start) < waitDur {
		ts.cfg.Logger.Info("waiting for cluster loader run")
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("cluster loader check stopped")
			return nil
		case <-time.After(15 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		output, err := exec.New().CommandContext(ctx, argsLogsPod[0], argsLogsPod[1:]...).CombinedOutput()
		cancel()
		out := strings.TrimSpace(string(output))
		if err != nil {
			ts.cfg.Logger.Warn("failed to fetch cluster loader logs", zap.String("command", cmdLogsPod), zap.Error(err))
		}
		lines := strings.Split(out, "\n")
		linesN := len(lines)
		if linesN > 30 { // tail 30 lines
			out = strings.Join(lines[linesN-30:], "\n")
		}
		fmt.Printf("\n'%s' output (total lines %d, last 30 lines):\n\n%s\n\n", cmdLogsPod, linesN, out)

		if strings.Contains(out, `"waiting for OS signal after test completion"`) {
			ready = true
			break
		}
	}
	if !ready {
		return errors.New("cluster loader remote failed to complete")
	}

	ts.cfg.Logger.Info("checking cluster loader report .tar.gz")
	sh, err := ssh.New(ssh.Config{
		Logger:        ts.cfg.Logger,
		KeyPath:       ts.cfg.EKSConfig.RemoteAccessPrivateKeyPath,
		PublicIP:      sshConfig.PublicIP,
		PublicDNSName: sshConfig.PublicDNSName,
		UserName:      sshConfig.UserName,
	})
	if err != nil {
		return fmt.Errorf("failed to create SSH config for %q (%v)", nodeName, err)
	}
	defer sh.Close()
	if err = sh.Connect(); err != nil {
		return fmt.Errorf("failed to SSH connect to %q (%v)", nodeName, err)
	}
	var out []byte
	out, err = sh.Download(
		"/var/log/cluster-loader-remote.tar.gz",
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.ReportTarGzPath,
		ssh.WithVerbose(ts.cfg.EKSConfig.LogLevel == "debug"),
		ssh.WithRetry(3, 10*time.Second),
	)
	if err != nil {
		os.RemoveAll(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.ReportTarGzPath)
		ts.cfg.Logger.Warn("failed to download '/var/log/cluster-loader-remote.tar.gz'", zap.Error(err))
	} else {
		ts.cfg.Logger.Info("downloaded cluster loader results from remote node",
			zap.String("tar-gz-path", ts.cfg.EKSConfig.AddOnClusterLoaderRemote.ReportTarGzPath),
		)
		fmt.Printf("\nDownloaded '/var/log/cluster-loader-remote.tar.gz' output (%q):\n%s\n", ts.cfg.EKSConfig.AddOnClusterLoaderRemote.ReportTarGzPath, string(out))
	}

	out, err = sh.Download(
		"/var/log/cluster-loader-remote.log",
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.LogPath,
		ssh.WithVerbose(ts.cfg.EKSConfig.LogLevel == "debug"),
		ssh.WithRetry(3, 10*time.Second),
	)
	if err != nil {
		os.RemoveAll(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.LogPath)
		ts.cfg.Logger.Warn("failed to download '/var/log/cluster-loader-remote.log'", zap.Error(err))
	} else {
		ts.cfg.Logger.Info("downloaded cluster loader results from remote node",
			zap.String("log-path", ts.cfg.EKSConfig.AddOnClusterLoaderRemote.LogPath),
		)
		fmt.Printf("\nDownloaded '/var/log/cluster-loader-remote.log' output (%q):\n%s\n", ts.cfg.EKSConfig.AddOnClusterLoaderRemote.LogPath, string(out))
	}

	return ts.cfg.EKSConfig.Sync()
}
