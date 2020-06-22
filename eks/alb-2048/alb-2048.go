// Package alb2048 implements ALB plugin that installs 2048.
package alb2048

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/aws/elb"
	"github.com/aws/aws-k8s-tester/pkg/httputil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/exec"
)

// Config defines ALB configuration.
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	CFNAPI    cloudformationiface.CloudFormationAPI
	K8SClient k8s_client.EKS
	ELB2API   elbv2iface.ELBV2API
}

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

// New creates a new Job tester.
func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg              Config
	policyCFNStackID string // TODO: persist
}

const (
	albIngressControllerName                       = "alb-ingress-controller"
	albIngressControllerServiceAccountName         = "alb-ingress-controller-service-account"
	albIngressControllerRBACRoleName               = "alb-ingress-controller-rbac-cluster-role"
	albIngressControllerRBACClusterRoleBindingName = "alb-ingress-controller-rbac-cluster-role-binding"
	albIngressControllerDeploymentName             = "alb-ingress-controller-deployment"

	alb2048DeploymentName = "alb-2048-deployment"
	alb2048AppName        = "alb-2048"
	alb2048AppImageName   = "alexwhen/docker-2048"
	alb2048SvcName        = "alb-2048-service"
	alb2048IngressName    = "alb-2048-ingress"
)

// ALBImageName is the image name of ALB Ingress Controller.
// ref. https://github.com/kubernetes-sigs/aws-alb-ingress-controller/releases
const ALBImageName = "docker.io/amazon/aws-alb-ingress-controller:v1.1.8"

// https://docs.aws.amazon.com/eks/latest/userguide/alb-ingress.html
func (ts *tester) Create() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnALB2048() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnALB2048.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnALB2048.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnALB2048.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnALB2048.Namespace,
	); err != nil {
		return err
	}
	if err := ts.createALBServiceAccount(); err != nil {
		return err
	}
	if err := ts.createALBRBACClusterRole(); err != nil {
		return err
	}
	if err := ts.createALBRBACClusterRoleBinding(); err != nil {
		return err
	}
	if err := ts.createALBDeployment(); err != nil {
		return err
	}
	if err := ts.waitDeploymentALB(); err != nil {
		return err
	}
	if err := ts.create2048Deployment(); err != nil {
		return err
	}
	if err := ts.waitDeployment2048(); err != nil {
		return err
	}
	if err := ts.create2048Service(); err != nil {
		return err
	}
	if err := ts.create2048Ingress(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnALB2048() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnALB2048.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnALB2048.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string
	if err := ts.delete2048Ingress(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete ALB 2048 Ingress (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting 2048 Ingress")
	time.Sleep(time.Minute)

	if err := ts.delete2048Service(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete ALB 2048 Service (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting 2048 Service")
	time.Sleep(time.Minute)

	if err := ts.delete2048Deployment(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete ALB 2048 Deployment (%v)", err))
	}
	time.Sleep(30 * time.Second)

	if err := ts.deleteALBDeployment(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete ALB Ingress Controller Deployment (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting ALB Deployment")
	time.Sleep(time.Minute)

	if err := ts.deleteALBRBACClusterRoleBinding(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete ALB Ingress Controller RBAC (%v)", err))
	}
	time.Sleep(10 * time.Second)

	if err := ts.deleteALBRBACClusterRole(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete ALB Ingress Controller RBAC (%v)", err))
	}
	time.Sleep(10 * time.Second)

	if err := ts.deleteALBServiceAccount(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete ALB Ingress Controller ServiceAccount (%v)", err))
	}
	time.Sleep(10 * time.Second)

	/*
	   # ALB tags
	   ingress.k8s.aws/stack
	   leegyuho-test-prod-alb-2048/alb-2048-ingress

	   kubernetes.io/ingress-name
	   alb-2048-ingress

	   ingress.k8s.aws/cluster
	   leegyuho-test-prod

	   ingress.k8s.aws/resource
	   LoadBalancer

	   kubernetes.io/cluster/leegyuho-test-prod
	   owned

	   kubernetes.io/namespace
	   leegyuho-test-prod-alb-2048
	*/
	if err := elb.DeleteELBv2(
		ts.cfg.Logger,
		ts.cfg.ELB2API,
		ts.cfg.EKSConfig.AddOnALB2048.ALBARN,
		ts.cfg.EKSConfig.Parameters.VPCID,
		map[string]string{
			"kubernetes.io/cluster/" + ts.cfg.EKSConfig.Name: "owned",
			"kubernetes.io/namespace":                        ts.cfg.EKSConfig.AddOnALB2048.Namespace,
		},
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete ALB (%v)", err))
	}

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnALB2048.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete ALB namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnALB2048.Created = false
	return ts.cfg.EKSConfig.Sync()
}

// https://docs.aws.amazon.com/eks/latest/userguide/alb-ingress.html
// https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/rbac-role.yaml
func (ts *tester) createALBServiceAccount() error {
	ts.cfg.Logger.Info("creating ALB Ingress Controller ServiceAccount")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts("kube-system").
		Create(
			ctx,
			&v1.ServiceAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      albIngressControllerServiceAccountName,
					Namespace: "kube-system",
					Labels: map[string]string{
						"app.kubernetes.io/name": albIngressControllerName,
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create ALB Ingress Controller ServiceAccount (%v)", err)
	}

	ts.cfg.Logger.Info("created ALB Ingress Controller ServiceAccount")
	return ts.cfg.EKSConfig.Sync()
}

// https://docs.aws.amazon.com/eks/latest/userguide/alb-ingress.html
// https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/rbac-role.yaml
func (ts *tester) deleteALBServiceAccount() error {
	ts.cfg.Logger.Info("deleting ALB Ingress Controller ServiceAccount")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts("kube-system").
		Delete(
			ctx,
			albIngressControllerServiceAccountName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete ALB Ingress Controller ServiceAccount (%v)", err)
	}
	ts.cfg.Logger.Info("deleted ALB Ingress Controller ServiceAccount", zap.Error(err))

	return ts.cfg.EKSConfig.Sync()
}

// https://docs.aws.amazon.com/eks/latest/userguide/alb-ingress.html
// https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/rbac-role.yaml
func (ts *tester) createALBRBACClusterRole() error {
	ts.cfg.Logger.Info("creating ALB Ingress Controller RBAC ClusterRole")
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
					Name:      albIngressControllerRBACRoleName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": albIngressControllerName,
					},
				},
				Rules: []rbacv1.PolicyRule{
					{ // read/write
						APIGroups: []string{
							"",
							"extensions",
						},
						Resources: []string{
							"configmaps",
							"endpoints",
							"events",
							"ingresses",
							"ingresses/status",
							"services",
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
					{ // read-only
						APIGroups: []string{
							"",
							"extensions",
						},
						Resources: []string{
							"nodes",
							"pods",
							"secrets",
							"services",
							"namespaces",
						},
						Verbs: []string{
							"get",
							"list",
							"watch",
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create ALB Ingress Controller RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("created ALB Ingress Controller RBAC ClusterRole")
	return ts.cfg.EKSConfig.Sync()
}

// https://docs.aws.amazon.com/eks/latest/userguide/alb-ingress.html
// https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/rbac-role.yaml
func (ts *tester) deleteALBRBACClusterRole() error {
	ts.cfg.Logger.Info("deleting ALB Ingress Controller RBAC ClusterRole")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoles().
		Delete(
			ctx,
			albIngressControllerRBACRoleName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete ALB Ingress Controller RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("deleted ALB Ingress Controller RBAC ClusterRole", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

// https://docs.aws.amazon.com/eks/latest/userguide/alb-ingress.html
// https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/rbac-role.yaml
func (ts *tester) createALBRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("creating ALB Ingress Controller RBAC ClusterRoleBinding")
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
					Name:      albIngressControllerRBACClusterRoleBindingName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": albIngressControllerName,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     albIngressControllerRBACRoleName,
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup:  "",
						Kind:      "ServiceAccount",
						Name:      albIngressControllerServiceAccountName,
						Namespace: "kube-system",
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create ALB Ingress Controller RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("created ALB Ingress Controller RBAC ClusterRoleBinding")
	return ts.cfg.EKSConfig.Sync()
}

// https://docs.aws.amazon.com/eks/latest/userguide/alb-ingress.html
// https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/rbac-role.yaml
func (ts *tester) deleteALBRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("deleting ALB Ingress Controller RBAC ClusterRoleBinding")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoleBindings().
		Delete(
			ctx,
			albIngressControllerRBACClusterRoleBindingName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete ALB Ingress Controller RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("deleted ALB Ingress Controller RBAC ClusterRoleBinding", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

// https://docs.aws.amazon.com/eks/latest/userguide/alb-ingress.html
// https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/alb-ingress-controller.yaml
func (ts *tester) createALBDeployment() error {
	ngType := "managed"
	if ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() {
		// TODO: test in MNG
		ngType = "custom"
	}
	nodeSelector := map[string]string{
		// do not deploy in bottlerocket; PVC not working
		// do not mix with MNG
		// controller "msg"="Reconciler error" "error"="no object matching key \"eks-2020042119-bluee7qmz7kb-alb-2048/alb-2048-ingress\" in local store"  "controller"="alb-ingress-controller" "request"={"Namespace":"eks-2020042119-bluee7qmz7kb-alb-2048","Name":"alb-2048-ingress"}
		"AMIType": ec2config.AMITypeAL2X8664,
		"NGType":  ngType,
	}
	if len(ts.cfg.EKSConfig.AddOnALB2048.DeploymentNodeSelector2048) > 0 {
		nodeSelector = ts.cfg.EKSConfig.AddOnALB2048.DeploymentNodeSelector2048
	}

	ts.cfg.Logger.Info("creating ALB Ingress Controller Deployment", zap.Any("node-selector", nodeSelector))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments("kube-system").
		Create(
			ctx,
			&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      albIngressControllerDeploymentName,
					Namespace: "kube-system",
					Labels: map[string]string{
						"app.kubernetes.io/name": albIngressControllerName,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: aws.Int32(ts.cfg.EKSConfig.AddOnALB2048.DeploymentReplicasALB),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": albIngressControllerName,
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": albIngressControllerName,
							},
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:            albIngressControllerDeploymentName,
									Image:           ALBImageName,
									ImagePullPolicy: v1.PullAlways,
									Args: []string{
										"--ingress-class=alb",
										fmt.Sprintf("--cluster-name=%s", ts.cfg.EKSConfig.Name),
										fmt.Sprintf("--aws-vpc-id=%s", ts.cfg.EKSConfig.Parameters.VPCID),
										fmt.Sprintf("--aws-region=%s", ts.cfg.EKSConfig.Region),
										"-v=2", // for debugging
									},
								},
							},
							ServiceAccountName: albIngressControllerServiceAccountName,
							NodeSelector:       nodeSelector,
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create ALB Ingress Controller Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("created ALB Ingress Controller Deployment")
	return ts.cfg.EKSConfig.Sync()
}

// https://docs.aws.amazon.com/eks/latest/userguide/alb-ingress.html
// https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/alb-ingress-controller.yaml
func (ts *tester) deleteALBDeployment() error {
	ts.cfg.Logger.Info("deleting ALB Ingress Controller Deployment")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments("kube-system").
		Delete(
			ctx,
			albIngressControllerDeploymentName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete ALB Ingress Controller Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("deleted ALB Ingress Controller Deployment", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitDeploymentALB() error {
	ts.cfg.Logger.Info("waiting for ALB Deployment")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(
		ctx,
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=kube-system",
		"describe",
		"deployment",
		albIngressControllerDeploymentName,
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl describe deployment' failed %v", err)
	}
	out := string(output)
	fmt.Printf("\n\n\"kubectl describe deployment\" output:\n%s\n\n", out)

	ready := false
	waitDur := 7*time.Minute + time.Duration(ts.cfg.EKSConfig.AddOnALB2048.DeploymentReplicasALB)*time.Minute
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
			Deployments("kube-system").
			Get(ctx, albIngressControllerDeploymentName, metav1.GetOptions{})
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
		if available && dresp.Status.AvailableReplicas >= ts.cfg.EKSConfig.AddOnALB2048.DeploymentReplicasALB {
			ready = true
			break
		}
	}
	if !ready {
		return errors.New("deployment not ready")
	}

	ts.cfg.Logger.Info("waited for ALB Deployment")
	return ts.cfg.EKSConfig.Sync()
}

// https://docs.aws.amazon.com/eks/latest/userguide/alb-ingress.html
// https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/2048/2048-deployment.yaml
func (ts *tester) create2048Deployment() error {
	ts.cfg.Logger.Info("creating ALB 2048 Deployment")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnALB2048.Namespace).
		Create(
			ctx,
			&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      alb2048DeploymentName,
					Namespace: ts.cfg.EKSConfig.AddOnALB2048.Namespace,
					Labels: map[string]string{
						"app": alb2048AppName,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: aws.Int32(ts.cfg.EKSConfig.AddOnALB2048.DeploymentReplicas2048),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": alb2048AppName,
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": alb2048AppName,
							},
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:            alb2048AppName,
									Image:           alb2048AppImageName,
									ImagePullPolicy: v1.PullAlways,
									Ports: []v1.ContainerPort{
										{
											Protocol:      v1.ProtocolTCP,
											ContainerPort: 80,
										},
									},
								},
							},
							NodeSelector: map[string]string{
								// do not deploy in bottlerocket; PVC not working
								"AMIType": ec2config.AMITypeAL2X8664,
							},
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create ALB 2048 Deployment (%v)", err)
	}
	ts.cfg.Logger.Info("created ALB 2048 Deployment")

	return ts.cfg.EKSConfig.Sync()
}

// https://docs.aws.amazon.com/eks/latest/userguide/alb-ingress.html
// https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/2048/2048-deployment.yaml
func (ts *tester) delete2048Deployment() error {
	ts.cfg.Logger.Info("deleting ALB 2048 Deployment")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnALB2048.Namespace).
		Delete(
			ctx,
			alb2048DeploymentName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete ALB 2048 Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("deleted ALB 2048 Deployment", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitDeployment2048() error {
	ts.cfg.Logger.Info("waiting for 2048 Deployment")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(
		ctx,
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace="+ts.cfg.EKSConfig.AddOnALB2048.Namespace,
		"describe",
		"deployment",
		alb2048DeploymentName,
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl describe deployment' failed %v", err)
	}
	out := string(output)
	fmt.Printf("\n\n\"kubectl describe deployment\" output:\n%s\n\n", out)

	ready := false
	waitDur := 7*time.Minute + time.Duration(ts.cfg.EKSConfig.AddOnALB2048.DeploymentReplicas2048)*time.Minute
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
			Deployments(ts.cfg.EKSConfig.AddOnALB2048.Namespace).
			Get(ctx, alb2048DeploymentName, metav1.GetOptions{})
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
		if available && dresp.Status.AvailableReplicas >= ts.cfg.EKSConfig.AddOnALB2048.DeploymentReplicas2048 {
			ready = true
			break
		}
	}
	if !ready {
		return errors.New("deployment not ready")
	}

	ts.cfg.Logger.Info("waited for 2048 Deployment")
	return ts.cfg.EKSConfig.Sync()
}

// https://docs.aws.amazon.com/eks/latest/userguide/alb-ingress.html
// https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/2048/2048-service.yaml
func (ts *tester) create2048Service() error {
	ts.cfg.Logger.Info("creating ALB 2048 Service")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Services(ts.cfg.EKSConfig.AddOnALB2048.Namespace).
		Create(
			ctx,
			&v1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      alb2048SvcName,
					Namespace: ts.cfg.EKSConfig.AddOnALB2048.Namespace,
				},
				Spec: v1.ServiceSpec{
					Selector: map[string]string{
						"app": alb2048AppName,
					},
					Type: v1.ServiceTypeNodePort,
					Ports: []v1.ServicePort{
						{
							Protocol:   v1.ProtocolTCP,
							Port:       80,
							TargetPort: intstr.FromInt(80),
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create ALB 2048 Service (%v)", err)
	}

	ts.cfg.Logger.Info("created ALB 2048 Service")
	return ts.cfg.EKSConfig.Sync()
}

// https://docs.aws.amazon.com/eks/latest/userguide/alb-ingress.html
// https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/2048/2048-service.yaml
func (ts *tester) delete2048Service() error {
	ts.cfg.Logger.Info("deleting ALB 2048 Service")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Services(ts.cfg.EKSConfig.AddOnALB2048.Namespace).
		Delete(
			ctx,
			alb2048SvcName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete ALB 2048 Service (%v)", err)
	}

	ts.cfg.Logger.Info("deleted ALB 2048 Service", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

// https://docs.aws.amazon.com/eks/latest/userguide/alb-ingress.html
// https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/2048/2048-ingress.yaml
func (ts *tester) create2048Ingress() error {
	ts.cfg.Logger.Info("creating ALB 2048 Ingress")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		ExtensionsV1beta1().
		Ingresses(ts.cfg.EKSConfig.AddOnALB2048.Namespace).
		Create(
			ctx,
			&v1beta1.Ingress{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "extensions/v1beta1",
					Kind:       "Ingress",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      alb2048IngressName,
					Namespace: ts.cfg.EKSConfig.AddOnALB2048.Namespace,
					Annotations: map[string]string{
						"kubernetes.io/ingress.class":      "alb",
						"alb.ingress.kubernetes.io/scheme": "internet-facing",
					},
					Labels: map[string]string{
						"app": alb2048AppName,
					},
				},
				Spec: v1beta1.IngressSpec{
					Rules: []v1beta1.IngressRule{
						{
							IngressRuleValue: v1beta1.IngressRuleValue{
								HTTP: &v1beta1.HTTPIngressRuleValue{
									Paths: []v1beta1.HTTPIngressPath{
										{
											Path: "/*",
											Backend: v1beta1.IngressBackend{
												ServiceName: alb2048SvcName,
												ServicePort: intstr.FromInt(80),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create ALB 2048 Ingress (%v)", err)
	}
	ts.cfg.Logger.Info("created ALB 2048 Ingress")

	waitDur := 3 * time.Minute
	ts.cfg.Logger.Info("waiting for ALB 2048 Ingress", zap.Duration("wait", waitDur))
	select {
	case <-ts.cfg.Stopc:
		return errors.New("ALB 2048 Ingress creation aborted")
	case <-time.After(waitDur):
	}

	logsArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=kube-system",
		"logs",
		"--selector=app.kubernetes.io/name=" + albIngressControllerName,
	}
	logsCmd := strings.Join(logsArgs, " ")

	descArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnALB2048.Namespace,
		"describe",
		"svc",
		alb2048SvcName,
	}
	descCmd := strings.Join(descArgs, " ")

	hostName := ""
	waitDur = 4 * time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("ALB 2048 Ingress creation aborted")
		case <-time.After(5 * time.Second):
		}

		ts.cfg.Logger.Info("fetching ALB pod logs", zap.String("logs-command", logsCmd))
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		logsOutput, err := exec.New().CommandContext(ctx, logsArgs[0], logsArgs[1:]...).CombinedOutput()
		cancel()
		out := string(logsOutput)
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl logs alb' failed", zap.Error(err))
		}
		fmt.Printf("\n\n\n\"%s\" output:\n\n%s\n\n", logsCmd, out)

		ts.cfg.Logger.Info("describing ALB service", zap.String("describe-command", descCmd))
		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		descOutput, err := exec.New().CommandContext(ctx, descArgs[0], descArgs[1:]...).CombinedOutput()
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl describe svc' failed", zap.Error(err))
		}
		out = string(descOutput)
		fmt.Printf("\n\n\n\"%s\" output:\n\n%s\n\n", descCmd, out)

		ts.cfg.Logger.Info("querying ALB 2048 Ingress for HTTP endpoint")
		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		so, err := ts.cfg.K8SClient.KubernetesClientSet().
			ExtensionsV1beta1().
			Ingresses(ts.cfg.EKSConfig.AddOnALB2048.Namespace).
			Get(ctx, alb2048IngressName, metav1.GetOptions{})
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to get ALB 2048 Ingress; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		ts.cfg.Logger.Info(
			"ALB 2048 Ingress has been linked to LoadBalancer",
			zap.String("load-balancer", fmt.Sprintf("%+v", so.Status.LoadBalancer)),
		)
		for _, ing := range so.Status.LoadBalancer.Ingress {
			ts.cfg.Logger.Info(
				"ALB 2048 Ingress has been linked to LoadBalancer.Ingress",
				zap.String("ingress", fmt.Sprintf("%+v", ing)),
			)
			hostName = ing.Hostname
			break
		}
		if hostName != "" {
			ts.cfg.Logger.Info("found ALB Ingress host name", zap.String("host-name", hostName))
			break
		}
	}
	if hostName == "" {
		return errors.New("failed to find ALB host name")
	}

	fields := strings.Split(hostName, "-")
	if len(fields) >= 3 {
		ts.cfg.EKSConfig.AddOnALB2048.ALBName = strings.Join(fields[:3], "-")
	}
	ts.cfg.EKSConfig.AddOnALB2048.URL = "http://" + hostName
	ts.cfg.EKSConfig.Sync()

	if ts.cfg.EKSConfig.AddOnALB2048.ALBName == "" {
		return errors.New("failed to create 2048 Ingress; got empty ALB name")
	}
	ts.cfg.Logger.Info("describing LB to get ARN",
		zap.String("name", ts.cfg.EKSConfig.AddOnALB2048.ALBName),
		zap.String("url", ts.cfg.EKSConfig.AddOnALB2048.URL),
	)
	do, err := ts.cfg.ELB2API.DescribeLoadBalancers(&elbv2.DescribeLoadBalancersInput{
		Names: aws.StringSlice([]string{ts.cfg.EKSConfig.AddOnALB2048.ALBName}),
	})
	if err != nil {
		// it may fail
		// 8ad2aa58-cnitest91alb2048--13d6-797589654.*********.elb.amazonaws.com
		// ValidationError: The load balancer name '8ad2aa58-cnitest91alb2048-' cannot end with a hyphen(-)\n\tstatus code: 400
		ts.cfg.Logger.Info("failed to describe LB",
			zap.String("name", ts.cfg.EKSConfig.AddOnALB2048.ALBName),
			zap.Error(err),
		)
		return err
	}
	for _, lb := range do.LoadBalancers {
		ts.cfg.EKSConfig.AddOnALB2048.ALBARN = aws.StringValue(lb.LoadBalancerArn)
		break
	}

	fmt.Printf("\nALB 2048 ARN: %s\n", ts.cfg.EKSConfig.AddOnALB2048.ALBARN)
	fmt.Printf("ALB 2048 Name: %s\n", ts.cfg.EKSConfig.AddOnALB2048.ALBName)
	fmt.Printf("ALB 2048 URL: %s\n\n", ts.cfg.EKSConfig.AddOnALB2048.URL)

	ts.cfg.Logger.Info("waiting before testing ALB 2048 Ingress")
	time.Sleep(10 * time.Second)

	htmlChecked := false
	retryStart = time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("ALB 2048 Ingress creation aborted")
		case <-time.After(5 * time.Second):
		}

		out, err := httputil.ReadInsecure(ts.cfg.Logger, ioutil.Discard, ts.cfg.EKSConfig.AddOnALB2048.URL)
		if err != nil {
			ts.cfg.Logger.Warn("failed to read ALB 2048 Service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
		httpOutput := string(out)
		fmt.Printf("\nALB 2048 Ingress output:\n%s\n", httpOutput)

		if strings.Contains(httpOutput, `2048 tile!`) {
			ts.cfg.Logger.Info("read ALB 2048 Service; exiting", zap.String("host-name", hostName))
			htmlChecked = true
			break
		}

		ts.cfg.Logger.Warn("unexpected ALB 2048 Ingress output; retrying")
	}

	fmt.Printf("\nALB 2048 ARN: %s\n", ts.cfg.EKSConfig.AddOnALB2048.ALBARN)
	fmt.Printf("ALB 2048 Name: %s\n", ts.cfg.EKSConfig.AddOnALB2048.ALBName)
	fmt.Printf("ALB 2048 URL: %s\n\n", ts.cfg.EKSConfig.AddOnALB2048.URL)

	if !htmlChecked {
		return fmt.Errorf("ALB 2048 %q did not return expected HTML output", ts.cfg.EKSConfig.AddOnALB2048.URL)
	}
	return ts.cfg.EKSConfig.Sync()
}

// https://docs.aws.amazon.com/eks/latest/userguide/alb-ingress.html
// https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/2048/2048-ingress.yaml
func (ts *tester) delete2048Ingress() error {
	ts.cfg.Logger.Info("deleting ALB 2048 Ingress")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		ExtensionsV1beta1().
		Ingresses(ts.cfg.EKSConfig.AddOnALB2048.Namespace).
		Delete(
			ctx,
			alb2048IngressName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete ALB 2048 Ingress (%v)", err)
	}
	ts.cfg.Logger.Info("deleted ALB 2048 Ingress", zap.Error(err))

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) AggregateResults() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnALB2048() {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnALB2048.Created {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.AggregateResults", zap.String("tester", pkgName))
	return nil
}
