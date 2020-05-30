// Package remote implements tester for CertificateSigningRequest.
package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"sort"
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

// Config defines csrs configuration.
// ref. https://github.com/kubernetes/perf-tests
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	ECRAPI    ecriface.ECRAPI
}

// Tester defines csrs tester.
// ref. https://github.com/kubernetes/perf-tests
type Tester interface {
	// Create installs csrs tester.
	Create() error
	// Delete deletes csrs tester.
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
	if ts.cfg.EKSConfig.AddOnCSRsRemote.Created {
		ts.cfg.Logger.Info("skipping create AddOnCSRsRemote")
		return nil
	}

	ts.cfg.Logger.Info("starting configmap testing")
	ts.cfg.EKSConfig.AddOnCSRsRemote.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnCSRsRemote.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if ts.ecrImage, err = aws_ecr.Check(
		ts.cfg.Logger,
		ts.cfg.ECRAPI,
		ts.cfg.EKSConfig.AddOnCSRsRemote.RepositoryAccountID,
		ts.cfg.EKSConfig.AddOnCSRsRemote.RepositoryName,
		ts.cfg.EKSConfig.AddOnCSRsRemote.RepositoryImageTag,
	); err != nil {
		return err
	}
	if err = k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnCSRsRemote.Namespace,
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
		ts.cfg.Logger.Warn("csrs tester aborted")
		return errors.New("csrs tester aborted")
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
	if !ts.cfg.EKSConfig.AddOnCSRsRemote.Created {
		ts.cfg.Logger.Info("skipping delete AddOnCSRsRemote")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnCSRsRemote.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
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
		ts.cfg.EKSConfig.AddOnCSRsRemote.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete csrs namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnCSRsRemote.Created = false
	return ts.cfg.EKSConfig.Sync()
}

const (
	csrsServiceAccountName          = "csrs-remote-service-account"
	csrsRBACRoleName                = "csrs-remote-rbac-role"
	csrsRBACClusterRoleBindingName  = "csrs-remote-rbac-role-binding"
	csrsKubeConfigConfigMapName     = "csrs-remote-kubeconfig-config-map"
	csrsKubeConfigConfigMapFileName = "csrs-remote-kubeconfig-config-map.yaml"
	csrsDeploymentName              = "csrs-remote-deployment"
	csrsAppName                     = "csrs-remote-app"
)

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createServiceAccount() error {
	ts.cfg.Logger.Info("creating csrs ServiceAccount")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnCSRsRemote.Namespace).
		Create(
			ctx,
			&v1.ServiceAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      csrsServiceAccountName,
					Namespace: ts.cfg.EKSConfig.AddOnCSRsRemote.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": csrsAppName,
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create csrs ServiceAccount (%v)", err)
	}

	ts.cfg.Logger.Info("created csrs ServiceAccount")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteServiceAccount() error {
	ts.cfg.Logger.Info("deleting csrs ServiceAccount")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnCSRsRemote.Namespace).
		Delete(
			ctx,
			csrsServiceAccountName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete csrs ServiceAccount (%v)", err)
	}
	ts.cfg.Logger.Info("deleted csrs ServiceAccount", zap.Error(err))

	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createALBRBACClusterRole() error {
	ts.cfg.Logger.Info("creating csrs RBAC ClusterRole")
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
					Name:      csrsRBACRoleName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": csrsAppName,
					},
				},
				Rules: []rbacv1.PolicyRule{
					// https://kubernetes.io/docs/reference/access-authn-authz/certificate-signing-requests/
					{
						APIGroups: []string{
							"*",
						},
						Resources: []string{
							"certificatesigningrequests",
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
		return fmt.Errorf("failed to create csrs RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("created csrs RBAC ClusterRole")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteALBRBACClusterRole() error {
	ts.cfg.Logger.Info("deleting csrs RBAC ClusterRole")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoles().
		Delete(
			ctx,
			csrsRBACRoleName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete csrs RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("deleted csrs RBAC ClusterRole", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createALBRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("creating csrs RBAC ClusterRoleBinding")
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
					Name:      csrsRBACClusterRoleBindingName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": csrsAppName,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     csrsRBACRoleName,
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup:  "",
						Kind:      "ServiceAccount",
						Name:      csrsServiceAccountName,
						Namespace: ts.cfg.EKSConfig.AddOnCSRsRemote.Namespace,
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
		return fmt.Errorf("failed to create csrs RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("created csrs RBAC ClusterRoleBinding")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteALBRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("deleting csrs RBAC ClusterRoleBinding")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoleBindings().
		Delete(
			ctx,
			csrsRBACClusterRoleBindingName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete csrs RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("deleted csrs RBAC ClusterRoleBinding", zap.Error(err))
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
		ConfigMaps(ts.cfg.EKSConfig.AddOnCSRsRemote.Namespace).
		Create(
			ctx,
			&v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      csrsKubeConfigConfigMapName,
					Namespace: ts.cfg.EKSConfig.AddOnCSRsRemote.Namespace,
					Labels: map[string]string{
						"name": csrsKubeConfigConfigMapName,
					},
				},
				Data: map[string]string{
					csrsKubeConfigConfigMapFileName: string(b),
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
		ConfigMaps(ts.cfg.EKSConfig.AddOnCSRsRemote.Namespace).
		Delete(
			ctx,
			csrsKubeConfigConfigMapName,
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
	// "/opt/"+csrsKubeConfigConfigMapFileName,
	// do not specify "kubeconfig", and use in-cluster config via "pkg/k8s-client"
	// otherwise, error "namespaces is forbidden: User "system:node:ip-192-168-84..."
	// ref. https://github.com/kubernetes/client-go/blob/master/examples/in-cluster-client-configuration/main.go
	testerCmd := fmt.Sprintf("/aws-k8s-tester eks create csrs --clients=%d --client-qps=%f --client-burst=%d --client-timeout=%s --objects=%d --initial-request-condition-type=%q --writes-output-name-prefix=%s --block=true",
		ts.cfg.EKSConfig.Clients,
		ts.cfg.EKSConfig.ClientQPS,
		ts.cfg.EKSConfig.ClientBurst,
		ts.cfg.EKSConfig.ClientTimeout,
		ts.cfg.EKSConfig.AddOnCSRsRemote.Objects,
		ts.cfg.EKSConfig.AddOnCSRsRemote.InitialRequestConditionType,
		ts.cfg.EKSConfig.AddOnCSRsRemote.RequestsWritesSummaryOutputNamePrefix,
	)

	ts.cfg.Logger.Info("creating csrs Deployment", zap.String("image", ts.ecrImage), zap.String("tester-command", testerCmd))
	dirOrCreate := v1.HostPathDirectoryOrCreate
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnCSRsRemote.Namespace).
		Create(
			ctx,
			&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      csrsDeploymentName,
					Namespace: ts.cfg.EKSConfig.AddOnCSRsRemote.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": csrsAppName,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: aws.Int32(ts.cfg.EKSConfig.AddOnCSRsRemote.DeploymentReplicas),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": csrsAppName,
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": csrsAppName,
							},
						},
						Spec: v1.PodSpec{
							ServiceAccountName: csrsServiceAccountName,

							// TODO: set resource limits
							Containers: []v1.Container{
								{
									Name:            csrsAppName,
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
											Name:      csrsKubeConfigConfigMapName,
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
									Name: csrsKubeConfigConfigMapName,
									VolumeSource: v1.VolumeSource{
										ConfigMap: &v1.ConfigMapVolumeSource{
											LocalObjectReference: v1.LocalObjectReference{
												Name: csrsKubeConfigConfigMapName,
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
		return fmt.Errorf("failed to create csrs Deployment (%v)", err)
	}
	return nil
}

func (ts *tester) deleteDeployment() error {
	ts.cfg.Logger.Info("deleting deployment")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnCSRsRemote.Namespace).
		Delete(
			ctx,
			csrsDeploymentName,
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
	ts.cfg.Logger.Info("waiting for csrs Deployment")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(
		ctx,
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace="+ts.cfg.EKSConfig.AddOnCSRsRemote.Namespace,
		"describe",
		"deployment",
		csrsDeploymentName,
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl describe deployment' failed %v", err)
	}
	out := string(output)
	fmt.Printf("\n\n\"kubectl describe deployment\" output:\n%s\n\n", out)

	ready := false
	waitDur := 5*time.Minute + time.Duration(ts.cfg.EKSConfig.AddOnCSRsRemote.DeploymentReplicas)*time.Minute
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
			Deployments(ts.cfg.EKSConfig.AddOnCSRsRemote.Namespace).
			Get(ctx, csrsDeploymentName, metav1.GetOptions{})
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
		if available && dresp.Status.AvailableReplicas >= ts.cfg.EKSConfig.AddOnCSRsRemote.DeploymentReplicas {
			ready = true
			break
		}
	}
	if !ready {
		// TODO: return error...
		// return errors.New("Deployment not ready")
		ts.cfg.Logger.Warn("Deployment not ready")
	}

	ts.cfg.Logger.Info("waited for csrs Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) AggregateResults() (err error) {
	if !ts.cfg.EKSConfig.AddOnCSRsRemote.Created {
		ts.cfg.Logger.Info("skipping aggregating AddOnCSRsRemote")
		return nil
	}

	ts.cfg.Logger.Info("aggregating results from Pods")
	writes := metrics.RequestsSummary{}
	writeLatencies := make(metrics.Durations, 0, 20000)
	if ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() && ts.cfg.EKSConfig.AddOnNodeGroups.FetchLogs {
		ts.cfg.Logger.Info("fetching logs from ngs")
		for _, v := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
			for _, fpaths := range v.Logs {
				for _, fpath := range fpaths {
					if strings.Contains(fpath, ts.cfg.EKSConfig.AddOnCSRsRemote.RequestsWritesSummaryOutputNamePrefix) {
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
					if strings.Contains(fpath, ts.cfg.EKSConfig.AddOnCSRsRemote.RequestsWritesSummaryOutputNamePrefix) {
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

	sortStart := time.Now()
	ts.cfg.Logger.Info("sorting write latencies")
	sort.Sort(writeLatencies)
	ts.cfg.Logger.Info("sorted write latencies", zap.String("took", time.Since(sortStart).String()))
	writes.LantencyP50 = writeLatencies.PickLantencyP50()
	writes.LantencyP90 = writeLatencies.PickLantencyP90()
	writes.LantencyP99 = writeLatencies.PickLantencyP99()
	writes.LantencyP999 = writeLatencies.PickLantencyP999()
	writes.LantencyP9999 = writeLatencies.PickLantencyP9999()
	ts.cfg.EKSConfig.AddOnCSRsRemote.RequestsWritesSummary = writes
	ts.cfg.EKSConfig.Sync()

	wb, err := json.Marshal(writeLatencies)
	if err != nil {
		ts.cfg.Logger.Warn("failed to encode JSON", zap.Error(err))
		return err
	}
	if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnCSRsRemote.RequestsWritesJSONPath, wb, 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return err
	}
	if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnCSRsRemote.RequestsWritesSummaryJSONPath, []byte(writes.JSON()), 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return err
	}
	if err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnCSRsRemote.RequestsWritesSummaryTablePath, []byte(writes.Table()), 0600); err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return err
	}
	fmt.Printf("\n\nAddOnCSRsRemote.RequestsWritesSummary:\n%s\n", writes.Table())

	ts.cfg.Logger.Info("aggregated results from Pods")
	return ts.cfg.EKSConfig.Sync()
}
