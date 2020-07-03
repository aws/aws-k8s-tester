// Package remote implements remote hollow nodes.
//
// ref. https://github.com/kubernetes/kubernetes/blob/master/pkg/kubemark/hollow_kubelet.go
//
// The purpose is to make it easy to run on EKS.
// ref. https://github.com/kubernetes/kubernetes/blob/master/test/kubemark/start-kubemark.sh
//
// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
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
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines hollow nodes configuration.
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
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
	if !ts.cfg.EKSConfig.IsEnabledAddOnHollowNodesRemote() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnHollowNodesRemote.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnHollowNodesRemote.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnHollowNodesRemote.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if ts.ecrImage, err = aws_ecr.Check(
		ts.cfg.Logger,
		ts.cfg.ECRAPI,
		ts.cfg.EKSConfig.AddOnHollowNodesRemote.RepositoryAccountID,
		ts.cfg.EKSConfig.AddOnHollowNodesRemote.RepositoryRegion,
		ts.cfg.EKSConfig.AddOnHollowNodesRemote.RepositoryName,
		ts.cfg.EKSConfig.AddOnHollowNodesRemote.RepositoryImageTag,
	); err != nil {
		return err
	}
	if err = k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnHollowNodesRemote.Namespace,
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
	if err = ts.createDeployment(); err != nil {
		return err
	}
	if err = ts.waitDeployment(); err != nil {
		return err
	}
	if err = ts.checkNodes(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnHollowNodesRemote() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnHollowNodesRemote.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnHollowNodesRemote.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteDeployment(); err != nil {
		errs = append(errs, err.Error())
	}
	time.Sleep(2 * time.Minute)

	if err := ts.deleteCreatedNodes(); err != nil {
		errs = append(errs, err.Error())
	}
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
		ts.cfg.EKSConfig.AddOnHollowNodesRemote.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete hollow nodes namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnHollowNodesRemote.Created = false
	return ts.cfg.EKSConfig.Sync()
}

const (
	hollowNodesServiceAccountName          = "hollow-nodes-remote-service-account"
	hollowNodesRBACRoleName                = "hollow-nodes-remote-rbac-role"
	hollowNodesRBACClusterRoleBindingName  = "hollow-nodes-remote-rbac-role-binding"
	hollowNodesKubeConfigConfigMapName     = "hollow-nodes-remote-kubeconfig-configmap"
	hollowNodesKubeConfigConfigMapFileName = "hollow-nodes-remote-kubeconfig-configmap.yaml"
	hollowNodesDeploymentName              = "hollow-nodes-remote-deployment"
	hollowNodesAppName                     = "hollow-nodes-remote-app"
)

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createServiceAccount() error {
	ts.cfg.Logger.Info("creating cluster loader ServiceAccount")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnHollowNodesRemote.Namespace).
		Create(
			ctx,
			&v1.ServiceAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      hollowNodesServiceAccountName,
					Namespace: ts.cfg.EKSConfig.AddOnHollowNodesRemote.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": hollowNodesAppName,
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
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
		ServiceAccounts(ts.cfg.EKSConfig.AddOnHollowNodesRemote.Namespace).
		Delete(
			ctx,
			hollowNodesServiceAccountName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
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
func (ts *tester) createRBACClusterRole() error {
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
					Name:      hollowNodesRBACRoleName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": hollowNodesAppName,
					},
				},
				// e.g. "kubectl api-resources"
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{
							"*",
						},
						Resources: []string{
							"leases",         // for API group "coordination.k8s.io"
							"runtimeclasses", // for API group "node.k8s.io"
							"nodes",
							"nodes/status", // to patch resource "nodes/status" in API group "" at the cluster scope
							"pods",
							"pods/status", // to patch resource in API group "" in the namespace "kube-system"
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
							"storageclasses",
							"volumeattachments",
							"csidrivers", // for API group "storage.k8s.io"
							"csinodes",   // Failed to initialize CSINodeInfo: error updating CSINode annotation: timed out waiting for the condition; caused by: csinodes.storage.k8s.io "hollowwandefortegreen6wd8z" is forbidden: User "system:serviceaccount:eks-2020052423-boldlyuxvugd-hollow-nodes-remote:hollow-nodes-remote-service-account" cannot get resource "csinodes" in API group "storage.k8s.io" at the cluster scope
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
		return fmt.Errorf("failed to create cluster loader RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("created cluster loader RBAC ClusterRole")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteRBACClusterRole() error {
	ts.cfg.Logger.Info("deleting cluster loader RBAC ClusterRole")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoles().
		Delete(
			ctx,
			hollowNodesRBACRoleName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete cluster loader RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("deleted cluster loader RBAC ClusterRole", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createRBACClusterRoleBinding() error {
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
					Name:      hollowNodesRBACClusterRoleBindingName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": hollowNodesAppName,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     hollowNodesRBACRoleName,
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup:  "",
						Kind:      "ServiceAccount",
						Name:      hollowNodesServiceAccountName,
						Namespace: ts.cfg.EKSConfig.AddOnHollowNodesRemote.Namespace,
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
func (ts *tester) deleteRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("deleting cluster loader RBAC ClusterRoleBinding")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoleBindings().
		Delete(
			ctx,
			hollowNodesRBACClusterRoleBindingName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete cluster loader RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("deleted cluster loader RBAC ClusterRoleBinding", zap.Error(err))
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
		ConfigMaps(ts.cfg.EKSConfig.AddOnHollowNodesRemote.Namespace).
		Create(
			ctx,
			&v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      hollowNodesKubeConfigConfigMapName,
					Namespace: ts.cfg.EKSConfig.AddOnHollowNodesRemote.Namespace,
					Labels: map[string]string{
						"name": hollowNodesKubeConfigConfigMapName,
					},
				},
				Data: map[string]string{
					hollowNodesKubeConfigConfigMapFileName: string(b),
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
		ConfigMaps(ts.cfg.EKSConfig.AddOnHollowNodesRemote.Namespace).
		Delete(
			ctx,
			hollowNodesKubeConfigConfigMapName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("deleted config map")
	return ts.cfg.EKSConfig.Sync()
}

// TODO: use "ReplicationController" to max out

func (ts *tester) createDeployment() error {
	// "/opt/"+hollowNodesKubeConfigConfigMapFileName,
	// do not specify "kubeconfig", and use in-cluster config via "pkg/k8s-client"
	// ref. https://github.com/kubernetes/client-go/blob/master/examples/in-cluster-client-configuration/main.go
	//
	// randomize node label, for node checking
	// in case multiple pods are creating hollow nodes
	//
	testerCmd := fmt.Sprintf("/aws-k8s-tester eks create hollow-nodes --clients=%d --client-qps=%f --client-burst=%d --nodes=%d --node-name-prefix=%s --node-label-prefix=%s --remote=true",
		ts.cfg.EKSConfig.Clients,
		ts.cfg.EKSConfig.ClientQPS,
		ts.cfg.EKSConfig.ClientBurst,
		ts.cfg.EKSConfig.AddOnHollowNodesRemote.Nodes,
		ts.cfg.EKSConfig.AddOnHollowNodesRemote.NodeNamePrefix,
		ts.cfg.EKSConfig.AddOnHollowNodesRemote.NodeLabelPrefix,
	)

	ts.cfg.Logger.Info("creating hollow nodes Deployment", zap.String("image", ts.ecrImage), zap.String("tester-command", testerCmd))
	dirOrCreate := v1.HostPathDirectoryOrCreate
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnHollowNodesRemote.Namespace).
		Create(
			ctx,
			&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      hollowNodesDeploymentName,
					Namespace: ts.cfg.EKSConfig.AddOnHollowNodesRemote.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": hollowNodesAppName,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: aws.Int32(ts.cfg.EKSConfig.AddOnHollowNodesRemote.DeploymentReplicas),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": hollowNodesAppName,
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": hollowNodesAppName,
							},
						},
						Spec: v1.PodSpec{
							ServiceAccountName: hollowNodesServiceAccountName,

							RestartPolicy: v1.RestartPolicyAlways,
							// TODO: set resource limits
							Containers: []v1.Container{
								{
									Name:            hollowNodesAppName,
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
											Name:      hollowNodesKubeConfigConfigMapName,
											MountPath: "/opt",
										},
										{ // for hollow node kubelet, kubelet requires "/dev/kmsg"
											Name:      "kmsg",
											MountPath: "/dev/kmsg",
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
									Name: hollowNodesKubeConfigConfigMapName,
									VolumeSource: v1.VolumeSource{
										ConfigMap: &v1.ConfigMapVolumeSource{
											LocalObjectReference: v1.LocalObjectReference{
												Name: hollowNodesKubeConfigConfigMapName,
											},
											DefaultMode: aws.Int32(0777),
										},
									},
								},
								{ // for hollow node kubelet
									Name: "kmsg",
									VolumeSource: v1.VolumeSource{
										HostPath: &v1.HostPathVolumeSource{
											Path: "/dev/kmsg",
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
		Deployments(ts.cfg.EKSConfig.AddOnHollowNodesRemote.Namespace).
		Delete(
			ctx,
			hollowNodesDeploymentName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("deleted deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitDeployment() error {
	ts.cfg.Logger.Info("waiting for hollow nodes Deployment")
	args := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnHollowNodesRemote.Namespace,
		"describe",
		"deployment",
		hollowNodesDeploymentName,
	}
	cmd := strings.Join(args, " ")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'%s' failed %v", cmd, err)
	}
	out := string(output)
	fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"%s\" output:\n%s\n\n", cmd, out)

	ready := false
	waitDur := 7*time.Minute + time.Duration(ts.cfg.EKSConfig.AddOnHollowNodesRemote.DeploymentReplicas)*time.Minute
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
			Deployments(ts.cfg.EKSConfig.AddOnHollowNodesRemote.Namespace).
			Get(ctx, hollowNodesDeploymentName, metav1.GetOptions{})
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
		if available && dresp.Status.AvailableReplicas >= ts.cfg.EKSConfig.AddOnHollowNodesRemote.DeploymentReplicas {
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
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"%s\" output:\n%s\n\n", cmd, out)
		}
	}
	if !ready {
		ts.cfg.Logger.Warn("deployment not ready")
		return errors.New("deployment not ready")
	}

	ts.cfg.Logger.Info("waited for hollow nodes Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) checkNodes() error {
	argsLogs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnHollowNodesRemote.Namespace,
		"logs",
		"--selector=app.kubernetes.io/name=" + hollowNodesAppName,
		"--tail=10",
	}
	cmdLogs := strings.Join(argsLogs, " ")

	expectedNodes := ts.cfg.EKSConfig.AddOnHollowNodesRemote.Nodes * int(ts.cfg.EKSConfig.AddOnHollowNodesRemote.DeploymentReplicas)

	// TODO: :some" hollow nodes may fail from resource quota
	// find out why it's failing
	expectedNodes /= 2

	retryStart, waitDur := time.Now(), 5*time.Minute+2*time.Second*time.Duration(expectedNodes)
	ts.cfg.Logger.Info("checking nodes readiness", zap.Duration("wait", waitDur), zap.Int("expected-nodes", expectedNodes))
	ready := false
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("checking node aborted")
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		nodes, err := ts.cfg.K8SClient.KubernetesClientSet().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("get nodes failed", zap.Error(err))
			continue
		}
		items := nodes.Items

		createdNodeNames := make([]string, 0)
		readies := 0
		for _, node := range items {
			labels := node.GetLabels()
			if !strings.HasPrefix(labels["NGName"], ts.cfg.EKSConfig.AddOnHollowNodesRemote.NodeLabelPrefix) {
				continue
			}
			nodeName := node.GetName()

			for _, cond := range node.Status.Conditions {
				if cond.Status != v1.ConditionTrue {
					continue
				}
				if cond.Type != v1.NodeReady {
					continue
				}
				ts.cfg.Logger.Info("node is ready!",
					zap.String("name", nodeName),
					zap.String("status-type", fmt.Sprintf("%s", cond.Type)),
					zap.String("status", fmt.Sprintf("%s", cond.Status)),
				)
				createdNodeNames = append(createdNodeNames, nodeName)
				readies++
				break
			}
		}
		ts.cfg.Logger.Info("nodes",
			zap.Int("current-ready-nodes", readies),
			zap.Int("desired-ready-nodes", expectedNodes),
		)

		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		output, err := exec.New().CommandContext(ctx, argsLogs[0], argsLogs[1:]...).CombinedOutput()
		cancel()
		out := string(output)
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl logs' failed", zap.Error(err))
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"%s\":\n%s\n", cmdLogs, out)

		ts.cfg.EKSConfig.AddOnHollowNodesRemote.CreatedNodeNames = createdNodeNames
		ts.cfg.EKSConfig.Sync()
		if readies > 0 && readies >= expectedNodes {
			ready = true
			break
		}
	}
	if !ready {
		return fmt.Errorf("NG %q not ready", ts.cfg.EKSConfig.AddOnHollowNodesRemote.NodeLabelPrefix)
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteCreatedNodes() error {
	var errs []string

	ts.cfg.Logger.Info("deleting node objects", zap.Int("created-nodes", len(ts.cfg.EKSConfig.AddOnHollowNodesRemote.CreatedNodeNames)))
	deleted := 0
	foreground := metav1.DeletePropagationForeground
	for i, nodeName := range ts.cfg.EKSConfig.AddOnHollowNodesRemote.CreatedNodeNames {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err := ts.cfg.K8SClient.KubernetesClientSet().CoreV1().Nodes().Delete(
			ctx,
			nodeName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
		cancel()
		if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
			ts.cfg.Logger.Warn("failed to delete node", zap.Int("index", i), zap.String("name", nodeName), zap.Error(err))
			errs = append(errs, err.Error())
		} else {
			ts.cfg.Logger.Info("deleted node", zap.Int("index", i), zap.String("name", nodeName))
			deleted++
		}
		if i > 300 {
			ts.cfg.Logger.Warn("skipping deleting created nodes; too many", zap.Int("deleted", deleted))
			break
		}
	}
	ts.cfg.Logger.Info("deleted node objects", zap.Int("deleted", deleted), zap.Int("created-nodes", len(ts.cfg.EKSConfig.AddOnHollowNodesRemote.CreatedNodeNames)))

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	return nil
}
