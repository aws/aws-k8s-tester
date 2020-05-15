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
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/metrics"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines Cluster Loader configuration.
// ref. https://github.com/kubernetes/perf-tests
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}

	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS

	ECRAPI ecriface.ECRAPI
}

// Tester defines Cluster Loader tester.
// ref. https://github.com/kubernetes/perf-tests
type Tester interface {
	// Create installs cluster Loader.
	Create() error
	// Delete deletes cluster Loader.
	Delete() error
	// AggregateResults aggregates all test results from remote nodes.
	AggregateResults() error
}

// TODO: use kubemark
// nodelease.NewController, kubemark.GetHollowKubeletConfig

func NewTester(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg Config
}

func (ts *tester) Create() (err error) {
	if ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Created {
		ts.cfg.Logger.Info("skipping create AddOnClusterLoaderRemote")
		return nil
	}

	ts.cfg.Logger.Info("starting load testing", zap.Duration("duration", ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Duration))
	ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()

	defer func() {
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.CreateTookString = ts.cfg.EKSConfig.AddOnClusterLoaderRemote.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err = ts.checkECR(); err != nil {
		return err
	}
	if err := k8s_client.CreateNamespace(ts.cfg.Logger, ts.cfg.K8SClient.KubernetesClientSet(), ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace); err != nil {
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
		ts.cfg.Logger.Warn("cluster loader aborted")
		return nil

	case <-time.After(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Duration):
		ts.cfg.Logger.Info("completing load testing", zap.Duration("duration", ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Duration))

		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("cluster loader aborted")
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
	if !ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Created {
		ts.cfg.Logger.Info("skipping delete AddOnClusterLoaderRemote")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.DeleteTookString = ts.cfg.EKSConfig.AddOnClusterLoaderRemote.DeleteTook.String()
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

	if err := k8s_client.DeleteNamespaceAndWait(ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete cluster loader namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) checkECR() error {
	ts.cfg.Logger.Info("describing ECR repositories")
	out, err := ts.cfg.ECRAPI.DescribeRepositories(&ecr.DescribeRepositoriesInput{
		RepositoryNames: aws.StringSlice([]string{ts.cfg.EKSConfig.AddOnClusterLoaderRemote.RepositoryName}),
	})
	if err != nil {
		return err
	}
	if len(out.Repositories) != 1 {
		return fmt.Errorf("%q expected 1 ECR repository, got %d", ts.cfg.EKSConfig.AddOnClusterLoaderRemote.RepositoryName, len(out.Repositories))
	}
	repo := out.Repositories[0]
	arn := aws.StringValue(repo.RepositoryArn)
	name := aws.StringValue(repo.RepositoryName)
	uri := aws.StringValue(repo.RepositoryUri)
	ts.cfg.Logger.Info(
		"described ECR repository",
		zap.String("arn", arn),
		zap.String("name", name),
		zap.String("uri", uri),
	)

	if name != ts.cfg.EKSConfig.AddOnClusterLoaderRemote.RepositoryName {
		return fmt.Errorf("unexpected ECR repository name %q", name)
	}
	if uri != ts.cfg.EKSConfig.AddOnClusterLoaderRemote.RepositoryURI {
		return fmt.Errorf("unexpected ECR repository uri %q", uri)
	}

	ts.cfg.Logger.Info("describing images")
	imgOut, err := ts.cfg.ECRAPI.DescribeImages(&ecr.DescribeImagesInput{
		RepositoryName: aws.String(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.RepositoryName),
		ImageIds: []*ecr.ImageIdentifier{
			{
				ImageTag: aws.String(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.RepositoryImageTag),
			},
		},
	})
	if err != nil {
		return err
	}
	if len(imgOut.ImageDetails) == 0 {
		return fmt.Errorf("image tag %q not found", ts.cfg.EKSConfig.AddOnClusterLoaderRemote.RepositoryImageTag)
	}
	ts.cfg.Logger.Info("described images", zap.Int("images", len(imgOut.ImageDetails)))
	for i, img := range imgOut.ImageDetails {
		ts.cfg.Logger.Info("found an image",
			zap.Int("index", i),
			zap.String("requested-tag", ts.cfg.EKSConfig.AddOnClusterLoaderRemote.RepositoryImageTag),
			zap.Strings("returned-tags", aws.StringValueSlice(img.ImageTags)),
			zap.String("digest", aws.StringValue(img.ImageDigest)),
			zap.String("pushed-at", humanize.Time(aws.TimeValue(img.ImagePushedAt))),
			zap.String("size", humanize.Bytes(uint64(aws.Int64Value(img.ImageSizeInBytes)))),
		)
	}
	return nil
}

const (
	clusterLoaderServiceAccountName          = "cluster-loader-remote-service-account"
	clusterLoaderRBACRoleName                = "cluster-loader-remote-rbac-role"
	clusterLoaderRBACClusterRoleBindingName  = "cluster-loader-remote-rbac-role-binding"
	clusterLoaderKubeConfigConfigMapName     = "cluster-loader-remote-kubeconfig-config-map"
	clusterLoaderKubeConfigConfigMapFileName = "cluster-loader-remote-kubeconfig-config-map.yaml"
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
	if err != nil {
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
		return fmt.Errorf("failed to delete cluster loader ServiceAccount (%v)", err)
	}
	ts.cfg.Logger.Info("deleted cluster loader ServiceAccount", zap.Error(err))

	return ts.cfg.EKSConfig.Sync()
}

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
					{
						APIGroups: []string{
							"*",
						},
						Resources: []string{
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

	ts.cfg.Logger.Info("created config map")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteConfigMap() error {
	ts.cfg.Logger.Info("deleting config map")
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
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("deleted config map")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createDeployment() error {
	ngType := "custom"
	if ts.cfg.EKSConfig.IsEnabledAddOnManagedNodeGroups() {
		ngType = "managed"
	}

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

	// "/opt/"+clusterLoaderKubeConfigConfigMapFileName,
	// do not specify "kubeconfig", and use in-cluster config via "pkg/k8s-client"
	// otherwise, error "namespaces is forbidden: User "system:node:ip-192-168-84..."
	// ref. https://github.com/kubernetes/client-go/blob/master/examples/in-cluster-client-configuration/main.go
	testerCmd := fmt.Sprintf("/aws-k8s-tester eks create cluster-loader --clients=%d --client-qps=%f --client-burst=%d --namespaces=%s --duration=%s --output-path-prefix=/var/log/%s --block=true",
		ts.cfg.EKSConfig.Clients,
		ts.cfg.EKSConfig.ClientQPS,
		ts.cfg.EKSConfig.ClientBurst,
		strings.Join(ns, ","),
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Duration,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.OutputPathPrefix,
	)

	image := ts.cfg.EKSConfig.AddOnClusterLoaderRemote.RepositoryURI + ":" + ts.cfg.EKSConfig.AddOnClusterLoaderRemote.RepositoryImageTag
	ts.cfg.Logger.Info("creating cluster loader Deployment", zap.String("image", image), zap.String("tester-command", testerCmd))
	dirOrCreate := v1.HostPathDirectoryOrCreate
	ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
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
									Image:           image,
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
								"NGType": ngType,
							},
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create cluster loader Deployment (%v)", err)
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
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("deleted deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitDeployment() error {
	ts.cfg.Logger.Info("waiting for cluster loader Deployment")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(
		ctx,
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace="+ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace,
		"describe",
		"deployment",
		clusterLoaderDeploymentName,
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl describe deployment' failed %v", err)
	}
	out := string(output)
	fmt.Printf("\n\n\"kubectl describe deployment\" output:\n%s\n\n", out)

	ready := false
	waitDur := 5*time.Minute + time.Duration(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.DeploymentReplicas)*time.Minute
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
	}
	if !ready {
		// TODO: return error...
		// return errors.New("Deployment not ready")
		ts.cfg.Logger.Warn("Deployment not ready")
	}

	ts.cfg.Logger.Info("waited for cluster loader Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) AggregateResults() error {
	if !ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Created {
		ts.cfg.Logger.Info("skipping aggregating AddOnClusterLoaderRemote")
		return nil
	}

	ts.cfg.Logger.Info("aggregating results from Pods")

	var rs eksconfig.RequestsSummary

	if ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() && ts.cfg.EKSConfig.AddOnNodeGroups.FetchLogs {
		ts.cfg.Logger.Info("fetching logs from ngs")
		for _, v := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
			for _, fpaths := range v.Logs {
				for _, fpath := range fpaths {
					if !strings.Contains(fpath, ts.cfg.EKSConfig.AddOnClusterLoaderRemote.OutputPathPrefix) {
						continue
					}
					b, err := ioutil.ReadFile(fpath)
					if err != nil {
						return fmt.Errorf("failed to open %q (%v)", fpath, err)
					}
					var r eksconfig.RequestsSummary
					if err = json.Unmarshal(b, &r); err != nil {
						return fmt.Errorf("failed to unmarshal %q (%s, %v)", fpath, string(b), err)
					}
					rs.SuccessTotal += r.SuccessTotal
					rs.FailureTotal += r.FailureTotal
					if rs.LatencyHistogram == nil || len(rs.LatencyHistogram) == 0 {
						rs.LatencyHistogram = r.LatencyHistogram
					} else {
						rs.LatencyHistogram, err = metrics.MergeHistograms(rs.LatencyHistogram, r.LatencyHistogram)
						if err != nil {
							return fmt.Errorf("failed to merge histograms (%v)", err)
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
					if !strings.Contains(fpath, ts.cfg.EKSConfig.AddOnClusterLoaderRemote.OutputPathPrefix) {
						continue
					}
					b, err := ioutil.ReadFile(fpath)
					if err != nil {
						return fmt.Errorf("failed to open %q (%v)", fpath, err)
					}
					var r eksconfig.RequestsSummary
					if err = json.Unmarshal(b, &r); err != nil {
						return fmt.Errorf("failed to unmarshal %q (%s, %v)", fpath, string(b), err)
					}
					rs.SuccessTotal += r.SuccessTotal
					rs.FailureTotal += r.FailureTotal
					if rs.LatencyHistogram == nil || len(rs.LatencyHistogram) == 0 {
						rs.LatencyHistogram = r.LatencyHistogram
					} else {
						rs.LatencyHistogram, err = metrics.MergeHistograms(rs.LatencyHistogram, r.LatencyHistogram)
						if err != nil {
							return fmt.Errorf("failed to merge histograms (%v)", err)
						}
					}
				}
			}
		}
	}

	ts.cfg.EKSConfig.AddOnClusterLoaderRemote.RequestsSummary = rs
	ts.cfg.EKSConfig.Sync()

	b, err := json.Marshal(rs)
	if err != nil {
		ts.cfg.Logger.Warn("failed to marshal JSON", zap.Error(err))
		return err
	}
	err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.RequestsSummaryJSONPath, b, 0600)
	if err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return err
	}

	tableBody := ts.cfg.EKSConfig.AddOnClusterLoaderRemote.RequestsSummary.LatencyHistogram.Table()
	tableBody = fmt.Sprintf(`

SUCCESS TOTAL: %.2f
FAILURE TOTAL: %.2f

`,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.RequestsSummary.SuccessTotal,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.RequestsSummary.FailureTotal,
	) + tableBody
	err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.RequestsSummaryTablePath, []byte(tableBody), 0600)
	if err != nil {
		ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
		return err
	}

	ts.cfg.Logger.Info("aggregated results from Pods")
	return ts.cfg.EKSConfig.Sync()
}
