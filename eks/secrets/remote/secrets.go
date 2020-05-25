// Package remote implements Secrets plugin.
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

// Config defines secrets configuration.
// ref. https://github.com/kubernetes/perf-tests
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	ECRAPI    ecriface.ECRAPI
}

// Tester defines secrets tester.
// ref. https://github.com/kubernetes/perf-tests
type Tester interface {
	// Create installs secrets tester.
	Create() error
	// Delete deletes secrets tester.
	Delete() error
	// AggregateResults aggregates all test results from remote nodes.
	AggregateResults() error
}

func New(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg      Config
	ecrImage string
}

func (ts *tester) Create() (err error) {
	if ts.cfg.EKSConfig.AddOnSecretsRemote.Created {
		ts.cfg.Logger.Info("skipping create AddOnSecretsRemote")
		return nil
	}

	ts.cfg.Logger.Info("starting configmap testing")
	ts.cfg.EKSConfig.AddOnSecretsRemote.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnSecretsRemote.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if ts.ecrImage, err = aws_ecr.Check(
		ts.cfg.Logger,
		ts.cfg.ECRAPI,
		ts.cfg.EKSConfig.AddOnSecretsRemote.RepositoryAccountID,
		ts.cfg.EKSConfig.AddOnSecretsRemote.RepositoryName,
		ts.cfg.EKSConfig.AddOnSecretsRemote.RepositoryImageTag,
	); err != nil {
		return err
	}
	if err = k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnSecretsRemote.Namespace,
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
		ts.cfg.Logger.Warn("secrets tester aborted")
		return errors.New("secrets tester aborted")
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
	if !ts.cfg.EKSConfig.AddOnSecretsRemote.Created {
		ts.cfg.Logger.Info("skipping delete AddOnSecretsRemote")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnSecretsRemote.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
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
		ts.cfg.EKSConfig.AddOnSecretsRemote.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete secrets namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnSecretsRemote.Created = false
	return ts.cfg.EKSConfig.Sync()
}

const (
	secretsServiceAccountName          = "secrets-remote-service-account"
	secretsRBACRoleName                = "secrets-remote-rbac-role"
	secretsRBACClusterRoleBindingName  = "secrets-remote-rbac-role-binding"
	secretsKubeConfigConfigMapName     = "secrets-remote-kubeconfig-config-map"
	secretsKubeConfigConfigMapFileName = "secrets-remote-kubeconfig-config-map.yaml"
	secretsDeploymentName              = "secrets-remote-deployment"
	secretsAppName                     = "secrets-remote-app"
)

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createServiceAccount() error {
	ts.cfg.Logger.Info("creating secrets ServiceAccount")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnSecretsRemote.Namespace).
		Create(
			ctx,
			&v1.ServiceAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretsServiceAccountName,
					Namespace: ts.cfg.EKSConfig.AddOnSecretsRemote.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": secretsAppName,
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create secrets ServiceAccount (%v)", err)
	}

	ts.cfg.Logger.Info("created secrets ServiceAccount")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteServiceAccount() error {
	ts.cfg.Logger.Info("deleting secrets ServiceAccount")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnSecretsRemote.Namespace).
		Delete(
			ctx,
			secretsServiceAccountName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete secrets ServiceAccount (%v)", err)
	}
	ts.cfg.Logger.Info("deleted secrets ServiceAccount", zap.Error(err))

	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createALBRBACClusterRole() error {
	ts.cfg.Logger.Info("creating secrets RBAC ClusterRole")
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
					Name:      secretsRBACRoleName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": secretsAppName,
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{
							"*",
						},
						Resources: []string{
							"secrets",
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
		return fmt.Errorf("failed to create secrets RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("created secrets RBAC ClusterRole")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteALBRBACClusterRole() error {
	ts.cfg.Logger.Info("deleting secrets RBAC ClusterRole")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoles().
		Delete(
			ctx,
			secretsRBACRoleName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete secrets RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("deleted secrets RBAC ClusterRole", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createALBRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("creating secrets RBAC ClusterRoleBinding")
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
					Name:      secretsRBACClusterRoleBindingName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": secretsAppName,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     secretsRBACRoleName,
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup:  "",
						Kind:      "ServiceAccount",
						Name:      secretsServiceAccountName,
						Namespace: ts.cfg.EKSConfig.AddOnSecretsRemote.Namespace,
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
		return fmt.Errorf("failed to create secrets RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("created secrets RBAC ClusterRoleBinding")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteALBRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("deleting secrets RBAC ClusterRoleBinding")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoleBindings().
		Delete(
			ctx,
			secretsRBACClusterRoleBindingName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete secrets RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("deleted secrets RBAC ClusterRoleBinding", zap.Error(err))
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
		ConfigMaps(ts.cfg.EKSConfig.AddOnSecretsRemote.Namespace).
		Create(
			ctx,
			&v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretsKubeConfigConfigMapName,
					Namespace: ts.cfg.EKSConfig.AddOnSecretsRemote.Namespace,
					Labels: map[string]string{
						"name": secretsKubeConfigConfigMapName,
					},
				},
				Data: map[string]string{
					secretsKubeConfigConfigMapFileName: string(b),
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
		ConfigMaps(ts.cfg.EKSConfig.AddOnSecretsRemote.Namespace).
		Delete(
			ctx,
			secretsKubeConfigConfigMapName,
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
	// "/opt/"+secretsKubeConfigConfigMapFileName,
	// do not specify "kubeconfig", and use in-cluster config via "pkg/k8s-client"
	// otherwise, error "namespaces is forbidden: User "system:node:ip-192-168-84..."
	// ref. https://github.com/kubernetes/client-go/blob/master/examples/in-cluster-client-configuration/main.go
	testerCmd := fmt.Sprintf("/aws-k8s-tester eks create secrets --clients=%d --client-qps=%f --client-burst=%d --client-timeout=%s --namespace=%s --name-prefix=%s --objects=%d --object-size=%d --writes-output-name-prefix=%s --reads-output-name-prefix=%s --block=true",
		ts.cfg.EKSConfig.Clients,
		ts.cfg.EKSConfig.ClientQPS,
		ts.cfg.EKSConfig.ClientBurst,
		ts.cfg.EKSConfig.ClientTimeout,
		ts.cfg.EKSConfig.AddOnSecretsRemote.Namespace,
		ts.cfg.EKSConfig.AddOnSecretsRemote.NamePrefix,
		ts.cfg.EKSConfig.AddOnSecretsRemote.Objects,
		ts.cfg.EKSConfig.AddOnSecretsRemote.ObjectSize,
		ts.cfg.EKSConfig.AddOnSecretsRemote.RequestsSummaryWritesOutputNamePrefix,
		ts.cfg.EKSConfig.AddOnSecretsRemote.RequestsSummaryReadsOutputNamePrefix,
	)

	ts.cfg.Logger.Info("creating secrets Deployment", zap.String("image", ts.ecrImage), zap.String("tester-command", testerCmd))
	dirOrCreate := v1.HostPathDirectoryOrCreate
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnSecretsRemote.Namespace).
		Create(
			ctx,
			&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretsDeploymentName,
					Namespace: ts.cfg.EKSConfig.AddOnSecretsRemote.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": secretsAppName,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: aws.Int32(ts.cfg.EKSConfig.AddOnSecretsRemote.DeploymentReplicas),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": secretsAppName,
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": secretsAppName,
							},
						},
						Spec: v1.PodSpec{
							ServiceAccountName: secretsServiceAccountName,

							// TODO: set resource limits
							Containers: []v1.Container{
								{
									Name:            secretsAppName,
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
											Name:      secretsKubeConfigConfigMapName,
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
									Name: secretsKubeConfigConfigMapName,
									VolumeSource: v1.VolumeSource{
										ConfigMap: &v1.ConfigMapVolumeSource{
											LocalObjectReference: v1.LocalObjectReference{
												Name: secretsKubeConfigConfigMapName,
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
		return fmt.Errorf("failed to create secrets Deployment (%v)", err)
	}
	return nil
}

func (ts *tester) deleteDeployment() error {
	ts.cfg.Logger.Info("deleting deployment")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnSecretsRemote.Namespace).
		Delete(
			ctx,
			secretsDeploymentName,
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
	ts.cfg.Logger.Info("waiting for secrets Deployment")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(
		ctx,
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace="+ts.cfg.EKSConfig.AddOnSecretsRemote.Namespace,
		"describe",
		"deployment",
		secretsDeploymentName,
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl describe deployment' failed %v", err)
	}
	out := string(output)
	fmt.Printf("\n\n\"kubectl describe deployment\" output:\n%s\n\n", out)

	ready := false
	waitDur := 5*time.Minute + time.Duration(ts.cfg.EKSConfig.AddOnSecretsRemote.DeploymentReplicas)*time.Minute
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
			Deployments(ts.cfg.EKSConfig.AddOnSecretsRemote.Namespace).
			Get(ctx, secretsDeploymentName, metav1.GetOptions{})
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
		if available && dresp.Status.AvailableReplicas >= ts.cfg.EKSConfig.AddOnSecretsRemote.DeploymentReplicas {
			ready = true
			break
		}
	}
	if !ready {
		// TODO: return error...
		// return errors.New("Deployment not ready")
		ts.cfg.Logger.Warn("Deployment not ready")
	}

	ts.cfg.Logger.Info("waited for secrets Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) AggregateResults() (err error) {
	if !ts.cfg.EKSConfig.AddOnSecretsRemote.Created {
		ts.cfg.Logger.Info("skipping aggregating AddOnSecretsRemote")
		return nil
	}

	ts.cfg.Logger.Info("aggregating results from Pods")
	writes, reads := metrics.RequestsSummary{}, metrics.RequestsSummary{}
	if ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() && ts.cfg.EKSConfig.AddOnNodeGroups.FetchLogs {
		ts.cfg.Logger.Info("fetching logs from ngs")
		for _, v := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
			for _, fpaths := range v.Logs {
				for _, fpath := range fpaths {
					if strings.Contains(fpath, ts.cfg.EKSConfig.AddOnSecretsRemote.RequestsSummaryWritesOutputNamePrefix) && strings.HasSuffix(fpath, "-writes.json") {
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
					if strings.Contains(fpath, ts.cfg.EKSConfig.AddOnSecretsRemote.RequestsSummaryReadsOutputNamePrefix) && strings.HasSuffix(fpath, "-reads.json") {
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
					if strings.Contains(fpath, ts.cfg.EKSConfig.AddOnSecretsRemote.RequestsSummaryWritesOutputNamePrefix) && strings.HasSuffix(fpath, "-writes.json") {
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
					if strings.Contains(fpath, ts.cfg.EKSConfig.AddOnSecretsRemote.RequestsSummaryReadsOutputNamePrefix) && strings.HasSuffix(fpath, "-reads.json") {
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

	ts.cfg.EKSConfig.AddOnSecretsRemote.RequestsSummaryWrites = writes
	ts.cfg.EKSConfig.AddOnSecretsRemote.RequestsSummaryReads = reads
	ts.cfg.EKSConfig.Sync()

	if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnSecretsRemote.RequestsSummaryWritesJSONPath, []byte(writes.JSON()), 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return err
	}
	if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnSecretsRemote.RequestsSummaryWritesTablePath, []byte(writes.Table()), 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return err
	}
	fmt.Printf("\n\nAddOnSecretsRemote.RequestsSummaryWritesTable:\n%s\n", writes.Table())

	if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnSecretsRemote.RequestsSummaryReadsJSONPath, []byte(reads.JSON()), 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return err
	}
	if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnSecretsRemote.RequestsSummaryReadsTablePath, []byte(reads.Table()), 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return err
	}
	fmt.Printf("\n\nAddOnSecretsRemote.RequestsSummaryReadsTable:\n%s\n", reads.Table())

	ts.cfg.Logger.Info("aggregated results from Pods")
	return ts.cfg.EKSConfig.Sync()
}
