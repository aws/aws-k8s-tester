// Package cnivpc installs "https://github.com/aws/amazon-vpc-cni-k8s".
package cnivpc

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	apiextensions_v1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/exec"
)

// Config defines CNI configuration.
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
	cfg.Logger.Info("updating tester", zap.String("tester", pkgName))
	ts := &tester{
		cfg:    cfg,
		cniImg: "busybox",
	}
	ts.creates = []func() error{
		func() (err error) {
			if ts.cfg.EKSConfig.AddOnCNIVPC.RepositoryAccountID != "" &&
				ts.cfg.EKSConfig.AddOnCNIVPC.RepositoryRegion != "" &&
				ts.cfg.EKSConfig.AddOnCNIVPC.RepositoryName != "" &&
				ts.cfg.EKSConfig.AddOnCNIVPC.RepositoryImageTag != "" {
				ts.cniImg, _, err = aws_ecr.Check(
					ts.cfg.Logger,
					ts.cfg.ECRAPI,
					ts.cfg.EKSConfig.Partition,
					ts.cfg.EKSConfig.AddOnCNIVPC.RepositoryAccountID,
					ts.cfg.EKSConfig.AddOnCNIVPC.RepositoryRegion,
					ts.cfg.EKSConfig.AddOnCNIVPC.RepositoryName,
					ts.cfg.EKSConfig.AddOnCNIVPC.RepositoryImageTag,
				)
				if err != nil &&
					!strings.Contains(err.Error(), "not authorized to perform: ecr:DescribeRepositories") {
					// e.g. "not authorized to perform: ecr:DescribeRepositories on resource: arn:aws:ecr:us-west-2:602401143452:repository/amazon-k8s-cni"
					return err
				}
				if ts.cniImg == "" {
					return errors.New("no amazon-k8s-cni ECR image found")
				}
				ts.cfg.Logger.Info("found amazon-k8s-cni ECR image", zap.String("image", ts.cniImg))
				return nil
			}
			return nil
		},
		func() error { return ts.updateCNIServiceAccount() },
		func() error { return ts.updateCNIRBACClusterRole() },
		func() error { return ts.updateCNIRBACClusterRoleBinding() },
		func() error { return ts.updateCNICRD() },
		func() error { return ts.updateCNIDaemonSet() },
		func() error { return ts.checkCNIPods() },
	}
	ts.deletes = []func() error{}
	return ts
}

type tester struct {
	cfg Config

	cniImg string

	creates []func() error
	deletes []func() error
}

func (ts *tester) Create() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnCNIVPC() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnCNIVPC.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnCNIVPC.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnCNIVPC.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	for _, createFunc := range ts.creates {
		if err = createFunc(); err != nil {
			return err
		}
	}
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnCNIVPC() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnCNIVPC.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnCNIVPC.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string
	for _, deleteFunc := range ts.deletes {
		if err := deleteFunc(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnCNIVPC.Created = false
	return ts.cfg.EKSConfig.Sync()
}

// https://github.com/aws/amazon-vpc-cni-k8s/blob/master/config/v1.6/aws-k8s-cni.yaml
const (
	cniServiceAccountName         = "aws-node"
	cniRBACRoleName               = "aws-node"
	cniRBACClusterRoleBindingName = "aws-node"
	cniAppName                    = "aws-node"
	cniDaemonSetName              = "aws-node"
	cniCRDNameSingular            = "eniconfig"
	cniCRDNamePlural              = "eniconfigs"
)

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) updateCNIServiceAccount() error {
	ts.cfg.Logger.Info("updating CNI ServiceAccount")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts("kube-system").
		Update(
			ctx,
			&v1.ServiceAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      cniServiceAccountName,
					Namespace: "kube-system",
					Labels: map[string]string{
						"app.kubernetes.io/name": cniAppName,
					},
				},
			},
			metav1.UpdateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create CNI ServiceAccount (%v)", err)
	}

	ts.cfg.Logger.Info("updated CNI ServiceAccount")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteCNIServiceAccount() error {
	ts.cfg.Logger.Info("deleting CNI ServiceAccount")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts("kube-system").
		Delete(
			ctx,
			cniServiceAccountName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete CNI ServiceAccount (%v)", err)
	}
	ts.cfg.Logger.Info("deleted CNI ServiceAccount", zap.Error(err))

	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) updateCNIRBACClusterRole() error {
	ts.cfg.Logger.Info("updating CNI RBAC ClusterRole")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoles().
		Update(
			ctx,
			&rbacv1.ClusterRole{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRole",
				},
				// "ClusterRole" is a non-namespaced resource.
				// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/#role-and-clusterrole
				ObjectMeta: metav1.ObjectMeta{
					Name:      cniRBACRoleName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": cniAppName,
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{
							"crd.k8s.amazonaws.com",
						},
						Resources: []string{
							"*",
						},
						Verbs: []string{
							"*",
						},
					},
					{
						// "" indicates the core API group
						// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/#role-and-clusterrole
						APIGroups: []string{
							"",
						},
						Resources: []string{
							"pods",
							"nodes",
							"namespaces",
						},
						Verbs: []string{
							"get",
							"list",
							"watch",
						},
					},
					{
						APIGroups: []string{
							"extensions",
						},
						Resources: []string{
							"daemonsets",
						},
						Verbs: []string{
							"list",
							"watch",
						},
					},
				},
			},
			metav1.UpdateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create CNI RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("updated CNI RBAC ClusterRole")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteCNIRBACClusterRole() error {
	ts.cfg.Logger.Info("deleting CNI RBAC ClusterRole")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoles().
		Delete(
			ctx,
			cniRBACRoleName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete CNI RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("deleted CNI RBAC ClusterRole", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) updateCNIRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("updating CNI RBAC ClusterRoleBinding")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoleBindings().
		Update(
			ctx,
			&rbacv1.ClusterRoleBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRoleBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      cniRBACClusterRoleBindingName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": cniAppName,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     cniRBACRoleName,
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup:  "",
						Kind:      "ServiceAccount",
						Name:      cniServiceAccountName,
						Namespace: "kube-system",
					},
				},
			},
			metav1.UpdateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create CNI RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("updated CNI RBAC ClusterRoleBinding")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteCNIRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("deleting CNI RBAC ClusterRoleBinding")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoleBindings().
		Delete(
			ctx,
			cniRBACClusterRoleBindingName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete CNI RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("deleted CNI RBAC ClusterRoleBinding", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) updateCNICRD() (err error) {
	ts.cfg.Logger.Info("updating CNI CRD")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.APIExtensionsClientSet().
		ApiextensionsV1beta1().
		CustomResourceDefinitions().
		Update(
			ctx,
			&apiextensions_v1beta1.CustomResourceDefinition{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apiextensions.k8s.io/v1beta1",
					Kind:       "CustomResourceDefinition",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eniconfigs.crd.k8s.amazonaws.com",
					Namespace: "default",
				},
				Spec: apiextensions_v1beta1.CustomResourceDefinitionSpec{
					Scope: apiextensions_v1beta1.ClusterScoped,
					Group: "crd.k8s.amazonaws.com",
					Versions: []apiextensions_v1beta1.CustomResourceDefinitionVersion{
						{
							Name:    "v1alpha1",
							Served:  true,
							Storage: true,
						},
					},
					Names: apiextensions_v1beta1.CustomResourceDefinitionNames{
						Singular: cniCRDNameSingular,
						Plural:   cniCRDNamePlural,
						Kind:     "ENIConfig",
					},
				},
			},
			metav1.UpdateOptions{},
		)
	cancel()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("updated CNI CRD")
	return ts.cfg.EKSConfig.Sync()
}

// https://github.com/aws/amazon-vpc-cni-k8s/blob/master/config/v1.6/aws-k8s-cni.yaml
func (ts *tester) updateCNIDaemonSet() (err error) {
	envVars := []v1.EnvVar{
		{
			Name:  "AWS_VPC_K8S_CNI_LOGLEVEL",
			Value: "DEBUG",
		},
		{
			Name:  "AWS_VPC_K8S_CNI_VETHPREFIX",
			Value: "eni",
		},
		{
			Name:  "AWS_VPC_ENI_MTU",
			Value: "9001",
		},
		{
			Name: "MY_NODE_NAME",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
	}
	if ts.cfg.EKSConfig.AddOnCNIVPC.MinimumIPTarget > 0 {
		envVars = append(envVars, v1.EnvVar{
			Name:  "MINIMUM_IP_TARGET",
			Value: fmt.Sprintf("%d", ts.cfg.EKSConfig.AddOnCNIVPC.MinimumIPTarget),
		})
	}
	if ts.cfg.EKSConfig.AddOnCNIVPC.WarmIPTarget > 0 {
		envVars = append(envVars, v1.EnvVar{
			Name:  "WARM_IP_TARGET",
			Value: fmt.Sprintf("%d", ts.cfg.EKSConfig.AddOnCNIVPC.WarmIPTarget),
		})
	}
	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"k8s-app": cniAppName,
			},
		},
		Spec: v1.PodSpec{
			PriorityClassName: "system-node-critical",
			// Unsupported value: "OnFailure": supported values: "Always"
			RestartPolicy: v1.RestartPolicyAlways,

			// specify both nodeSelector and nodeAffinity,
			// both must be satisfied for the pod to be scheduled
			// onto a candidate node.
			// ref. https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/
			NodeSelector: ts.cfg.EKSConfig.AddOnCNIVPC.NodeSelector,
			Affinity: &v1.Affinity{
				NodeAffinity: &v1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
						// pod can be scheduled onto a node if one of the nodeSelectorTerms can be satisfied
						// ref. https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/
						NodeSelectorTerms: []v1.NodeSelectorTerm{
							{
								MatchExpressions: []v1.NodeSelectorRequirement{
									{
										Key:      "beta.kubernetes.io/os",
										Operator: v1.NodeSelectorOpIn,
										Values:   []string{"linux"},
									},
									{
										Key:      "beta.kubernetes.io/arch",
										Operator: v1.NodeSelectorOpIn,
										Values:   []string{"amd64"},
									},
									{
										Key:      "eks.amazonaws.com/compute-type",
										Operator: v1.NodeSelectorOpNotIn,
										Values:   []string{"fargate"},
									},
								},
							},
							{
								MatchExpressions: []v1.NodeSelectorRequirement{
									{
										Key:      "kubernetes.io/os",
										Operator: v1.NodeSelectorOpIn,
										Values:   []string{"linux"},
									},
									{
										Key:      "kubernetes.io/arch",
										Operator: v1.NodeSelectorOpIn,
										Values:   []string{"amd64"},
									},
									{
										Key:      "eks.amazonaws.com/compute-type",
										Operator: v1.NodeSelectorOpNotIn,
										Values:   []string{"fargate"},
									},
								},
							},
						},
					},
				},
			},

			ServiceAccountName: cniServiceAccountName,
			HostNetwork:        true,
			Tolerations: []v1.Toleration{
				{
					Operator: v1.TolerationOpExists,
				},
			},

			// https://www.eksworkshop.com/intermediate/230_logging/deploy/
			Containers: []v1.Container{
				{
					Name:            cniAppName,
					Image:           ts.cniImg,
					ImagePullPolicy: v1.PullAlways,

					Ports: []v1.ContainerPort{
						{
							ContainerPort: 61678,
							Name:          "metrics",
						},
					},

					ReadinessProbe: &v1.Probe{
						Handler: v1.Handler{
							Exec: &v1.ExecAction{
								Command: []string{"/app/grpc-health-probe", "-addr=:50051"},
							},
						},
						InitialDelaySeconds: 35,
					},
					LivenessProbe: &v1.Probe{
						Handler: v1.Handler{
							Exec: &v1.ExecAction{
								Command: []string{"/app/grpc-health-probe", "-addr=:50051"},
							},
						},
						InitialDelaySeconds: 35,
					},

					Env: envVars,

					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU: resource.MustParse("10m"),
						},
					},

					SecurityContext: &v1.SecurityContext{
						Privileged: aws.Bool(true),
					},

					// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "cni-bin-dir",
							MountPath: "/host/opt/cni/bin",
						},
						{
							Name:      "cni-net-dir",
							MountPath: "/host/var/log",
						},
						{
							Name:      "log-dir",
							MountPath: "/host/var/log",
						},
						{
							Name:      "dockersock",
							MountPath: "/var/run/docker.sock",
						},
						{
							Name:      "dockershim",
							MountPath: "/var/run/dockershim.sock",
						},
					},
				},
			},

			// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
			Volumes: []v1.Volume{
				{
					Name: "cni-bin-dir",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/opt/cni/bin",
						},
					},
				},
				{
					Name: "cni-net-dir",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/etc/cni/net.d",
						},
					},
				},
				{
					Name: "log-dir",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/var/log",
						},
					},
				},
				{
					Name: "dockersock",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/var/run/docker.sock",
						},
					},
				},
				{
					Name: "dockershim",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/var/run/dockershim.sock",
						},
					},
				},
			},
		},
	}

	maxUnavailable := intstr.FromString("10%")
	dsObj := appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "DaemonSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cniDaemonSetName,
			Namespace: "kube-system",
			Labels: map[string]string{
				"k8s-app": "aws-node",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &maxUnavailable,
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app": cniAppName,
				},
			},

			Template: podSpec,
		},
	}

	ts.cfg.Logger.Info("updating CNI DaemonSet", zap.String("name", cniDaemonSetName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		DaemonSets("kube-system").
		Update(ctx, &dsObj, metav1.UpdateOptions{})
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create CNI DaemonSet (%v)", err)
	}

	ts.cfg.Logger.Info("updated CNI DaemonSet")
	return nil
}

func (ts *tester) deleteCNIDaemonSet() (err error) {
	foreground := metav1.DeletePropagationForeground
	ts.cfg.Logger.Info("deleting CNI DaemonSet", zap.String("name", cniDaemonSetName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err = ts.cfg.
		K8SClient.KubernetesClientSet().
		AppsV1().
		DaemonSets("kube-system").
		Delete(
			ctx,
			cniDaemonSetName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete CNI DaemonSet", zap.Error(err))
		return fmt.Errorf("failed to delete CNI DaemonSet (%v)", err)
	}
	return nil
}

func (ts *tester) checkCNIPods() (err error) {
	waitDur := 10*time.Minute + 3*time.Minute*time.Duration(ts.cfg.EKSConfig.TotalNodes)
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("check aborted")
		case <-time.After(15 * time.Second):
		}
		if err = ts._checkCNIPods(); err == nil {
			break
		}
		ts.cfg.Logger.Info("failed to check CNI pods; retrying", zap.Error(err))
	}
	return err
}

func (ts *tester) _checkCNIPods() error {
	pods, err := ts.cfg.K8SClient.ListPods("kube-system", 1000, 5*time.Second)
	if err != nil {
		ts.cfg.Logger.Warn("listing pods failed", zap.Error(err))
		return err
	}
	if len(pods) > 0 {
		ts.cfg.Logger.Info("pods found", zap.Int("pods", len(pods)))
		fmt.Fprintf(ts.cfg.LogWriter, "\n")
		for _, pod := range pods {
			fmt.Fprintf(ts.cfg.LogWriter, "%q Pod using client-go: %q\n", "kube-system", pod.Name)
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n")
	} else {
		ts.cfg.Logger.Info("no pod found", zap.String("namespace", "kube-system"))
		return errors.New("no pod found in " + "kube-system")
	}

	targetPods := int64(1)
	if ts.cfg.EKSConfig.TotalNodes > 1 {
		targetPods = ts.cfg.EKSConfig.TotalNodes / int64(2)
	}
	ts.cfg.Logger.Info("checking CNI pods",
		zap.Int64("target-ready-pods", targetPods),
		zap.Int64("total-nodes", ts.cfg.EKSConfig.TotalNodes),
	)
	readyPods := int64(0)
	for _, pod := range pods {
		appName, ok := pod.Labels["k8s-app"]
		if !ok || appName != cniAppName {
			ts.cfg.Logger.Info("skipping pod, not fluentd", zap.String("labels", fmt.Sprintf("%+v", pod.Labels)))
			continue
		}

		descArgsPods := []string{
			ts.cfg.EKSConfig.KubectlPath,
			"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
			"--namespace=" + "kube-system",
			"describe",
			"pods/" + pod.Name,
		}
		descCmdPods := strings.Join(descArgsPods, " ")

		logArgs := []string{
			ts.cfg.EKSConfig.KubectlPath,
			"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
			"--namespace=" + "kube-system",
			"logs",
			"pods/" + pod.Name,
			"--all-containers=true",
			"--timestamps",
		}
		logsCmd := strings.Join(logArgs, " ")

		ts.cfg.Logger.Debug("checking Pod",
			zap.String("pod-name", pod.Name),
			zap.String("app-name", appName),
			zap.String("command-describe", descCmdPods),
			zap.String("command-logs", logsCmd),
		)

		ready := false
		statusType, status := "", ""
		for _, cond := range pod.Status.Conditions {
			if cond.Status != v1.ConditionTrue {
				continue
			}
			statusType = fmt.Sprintf("%s", cond.Type)
			status = fmt.Sprintf("%s", cond.Status)
			if cond.Type == v1.PodInitialized || cond.Type == v1.PodReady {
				ready = true
				readyPods++
			}
			break
		}
		if !ready {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			output, err := exec.New().CommandContext(ctx, descArgsPods[0], descArgsPods[1:]...).CombinedOutput()
			cancel()
			outDesc := string(output)
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe' failed", zap.Error(err))
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n'%s' output:\n\n%s\n\n", descCmdPods, outDesc)
			ts.cfg.Logger.Warn("pod is not ready yet",
				zap.Int64("current-ready-pods", readyPods),
				zap.Int64("target-ready-pods", targetPods),
				zap.Int64("total-nodes", ts.cfg.EKSConfig.TotalNodes),
				zap.String("pod-name", pod.Name),
				zap.String("app-name", appName),
				zap.String("status-type", statusType),
				zap.String("status", status),
			)
			continue
		}

		if readyPods < 3 { // only first 3 nodes
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			output, err := exec.New().CommandContext(ctx, descArgsPods[0], descArgsPods[1:]...).CombinedOutput()
			cancel()
			outDesc := string(output)
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe' failed", zap.Error(err))
				continue
			}
			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			output, err = exec.New().CommandContext(ctx, logArgs[0], logArgs[1:]...).CombinedOutput()
			cancel()
			outLogs := string(output)
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl logs' failed", zap.Error(err))
				continue
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n'%s' output:\n\n%s\n\n", descCmdPods, outDesc)
			logLines := strings.Split(outLogs, "\n")
			logLinesN := len(logLines)
			if logLinesN > 15 {
				logLines = logLines[logLinesN-15:]
				outLogs = strings.Join(logLines, "\n")
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n'%s' output:\n\n%s\n\n", logsCmd, outLogs)
		}
		if readyPods%100 == 0 {
			ts.cfg.Logger.Info("found a ready pod",
				zap.Int64("current-ready-pods", readyPods),
				zap.Int64("target-ready-pods", targetPods),
				zap.Int64("total-nodes", ts.cfg.EKSConfig.TotalNodes),
				zap.String("pod-name", pod.Name),
				zap.String("app-name", appName),
				zap.String("status-type", statusType),
				zap.String("status", status),
			)
		}
	}
	ts.cfg.Logger.Info("checking CNI pods",
		zap.Int64("current-ready-pods", readyPods),
		zap.Int64("target-ready-pods", targetPods),
		zap.Int64("total-nodes", ts.cfg.EKSConfig.TotalNodes),
	)
	if readyPods < targetPods {
		return errors.New("not enough CNI pods ready")
	}

	return nil
}
