// Package remote implements remote cluster loader.
// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2
// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2/testing/overrides
package remote

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
	"time"

	cluster_loader "github.com/aws/aws-k8s-tester/eks/cluster-loader"
	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/aws/cw"
	aws_ecr "github.com/aws/aws-k8s-tester/pkg/aws/ecr"
	aws_s3 "github.com/aws/aws-k8s-tester/pkg/aws/s3"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Config defines cluster loader configuration.
type Config struct {
	Logger    *zap.Logger
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
	if !ts.cfg.EKSConfig.IsEnabledAddOnClusterLoaderRemote() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
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

	if err = ts.createJob(); err != nil {
		return err
	}
	var pods []v1.Pod
	_, pods, err = k8s_client.WaitForJobCompletes(
		ts.cfg.Logger,
		ts.cfg.Stopc,
		ts.cfg.K8SClient,
		2*time.Minute,
		10*time.Second,
		3*time.Minute+time.Duration(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Runs)*2*time.Minute,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace,
		clusterLoaderJobName,
		1,
	)
	if err != nil {
		return err
	}
	println()
	for _, item := range pods {
		fmt.Printf("Job Pod %q: %q\n", item.Name, item.Status.Phase)
	}
	println()

	if err = ts.checkResults(); err == nil {
		return err
	}
	if err = ts.publishResults(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnClusterLoaderRemote() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
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
		k8s_client.WithForceDelete(true),
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
	clusterLoaderAppName                     = "cluster-loader-remote-app"
	clusterLoaderJobName                     = "cluster-loader-remote-job"
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

func (ts *tester) createJob() (err error) {
	obj, b, err := ts.createObject()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("creating Job",
		zap.String("name", clusterLoaderJobName),
		zap.String("object-size", humanize.Bytes(uint64(len(b)))),
	)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		BatchV1().
		Jobs(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace).
		Create(ctx, &obj, metav1.CreateOptions{})
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create Job (%v)", err)
	}

	ts.cfg.Logger.Info("created Job")
	return nil
}

func (ts *tester) createObject() (batchv1.Job, string, error) {
	// "/opt/"+clusterLoaderKubeConfigConfigMapFileName,
	// do not specify "kubeconfig", and use in-cluster config via "pkg/k8s-client"
	// ref. https://github.com/kubernetes/client-go/blob/master/examples/in-cluster-client-configuration/main.go

	// ref. https://github.com/kubernetes/perf-tests/pull/1295
	testerCmd := fmt.Sprintf("/aws-k8s-tester eks create cluster-loader --partition=%s --region=%s --s3-bucket-name=%s --cluster-loader-path=/clusterloader2 --test-config-path=/clusterloader2-test-config.yaml --report-dir=/var/log/cluster-loader-remote --report-tar-gz-path=/var/log/cluster-loader-remote.tar.gz --report-tar-gz-s3-path=%s --log-path=/var/log/cluster-loader-remote.log --log-s3-path=%s --pod-startup-latency-path=/var/log/cluster-loader-remote.pod-startup-latency-output.json --pod-startup-latency-s3-path=%s --runs=%d --timeout=%v --nodes=%d --nodes-per-namespace=%d --pods-per-node=%d --big-group-size=%d --medium-group-size=%d --small-group-size=%d --small-stateful-sets-per-namespace=%d --medium-stateful-sets-per-namespace=%d --cl2-use-host-network-pods=%v --cl2-load-test-throughput=%d --cl2-enable-pvs=%v --prometheus-scrape-kube-proxy=%v --enable-system-pod-metrics=%v",
		ts.cfg.EKSConfig.Partition,
		ts.cfg.EKSConfig.Region,
		ts.cfg.EKSConfig.S3BucketName,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.ReportTarGzS3Key,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.LogS3Key,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.PodStartupLatencyS3Key,
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
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.CL2UseHostNetworkPods,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.CL2LoadTestThroughput,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.CL2EnablePVS,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.PrometheusScrapeKubeProxy,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.EnableSystemPodMetrics,
	)

	dirOrCreate := v1.HostPathDirectoryOrCreate
	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app.kubernetes.io/name": clusterLoaderAppName,
			},
		},
		Spec: v1.PodSpec{
			ServiceAccountName: clusterLoaderServiceAccountName,

			// spec.template.spec.restartPolicy: Unsupported value: "Always": supported values: "OnFailure", "Never"
			RestartPolicy: v1.RestartPolicyOnFailure,
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
	}

	jobObj := batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterLoaderJobName,
			Namespace: ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace,
		},
		Spec: batchv1.JobSpec{
			Completions: aws.Int32(1),
			Parallelism: aws.Int32(1),
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
	ts.cfg.Logger.Info("deleting Job", zap.String("name", clusterLoaderJobName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err = ts.cfg.
		K8SClient.KubernetesClientSet().
		BatchV1().
		Jobs(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Namespace).
		Delete(
			ctx,
			clusterLoaderJobName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err == nil {
		ts.cfg.Logger.Info("deleted Job", zap.String("name", clusterLoaderJobName))
	} else {
		ts.cfg.Logger.Warn("failed to delete Job", zap.Error(err))
	}
	return err
}

func (ts *tester) checkResults() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), ts.cfg.EKSConfig.AddOnClusterLoaderRemote.Timeout)
	ch := aws_s3.PollUntilExist(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.PodStartupLatencyS3Key,
		2*time.Minute,
		15*time.Second,
	)
	for cur := range ch {
		err = cur.Error
	}
	cancel()
	if err != nil {
		return err
	}
	if err = aws_s3.Download(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.PodStartupLatencyS3Key,
		ts.cfg.EKSConfig.AddOnClusterLoaderRemote.PodStartupLatencyPath,
	); err != nil {
		return err
	}

	ts.cfg.EKSConfig.AddOnClusterLoaderRemote.PodStartupLatency, err = cluster_loader.ParsePodStartupLatency(ts.cfg.EKSConfig.AddOnClusterLoaderRemote.PodStartupLatencyPath)
	if err != nil {
		return fmt.Errorf("failed to read PodStartupLatency %q (%v)", "", err)
	}
	return ts.cfg.EKSConfig.Sync()
}

/*
e.g.

  pod-startup-latency:
    dataItems:
    - data:
        Perc50: 0
        Perc90: 0
        Perc99: 250
      labels:
        Metric: create_to_schedule
      unit: ms
    - data:
        Perc50: 1000
        Perc90: 1062.5
        Perc99: 2062.5
      labels:
        Metric: schedule_to_run
      unit: ms
    - data:
        Perc50: 1200.219827375
        Perc90: 1741.2740193125
        Perc99: 1992.869426375
      labels:
        Metric: run_to_watch
      unit: ms
    - data:
        Perc50: 2289.84256025
        Perc90: 2848.8128514375
        Perc99: 3143.05340125
      labels:
        Metric: schedule_to_watch
      unit: ms
    - data:
        Perc50: 2297.02802625
        Perc90: 2861.429472625
        Perc99: 3156.2773068125
      labels:
        Metric: pod_startup
      unit: ms
    version: "1.0"
*/

func (ts *tester) publishResults() (err error) {
	tv := aws.Time(time.Now().UTC())
	datums := make([]*cloudwatch.MetricDatum, 0)
	for _, item := range ts.cfg.EKSConfig.AddOnClusterLoaderLocal.PodStartupLatency.DataItems {
		name := "add-on-cluster-loader-remote-pod-startup-latency"
		if mv, ok := item.Labels["Metric"]; ok {
			name += "-" + mv
		}
		for k, fv := range item.Data {
			key := name + "-" + k
			ts.cfg.Logger.Info("adding", zap.String("key", key), zap.Float64("value", fv))
			datums = append(datums, &cloudwatch.MetricDatum{
				Timestamp:  tv,
				MetricName: aws.String(key),
				Unit:       toUnit(item.Unit),
				Value:      aws.Float64(fv),
			})
		}
	}
	return cw.PutData(ts.cfg.Logger, ts.cfg.CWAPI, ts.cfg.EKSConfig.CWNamespace, 20, datums...)
}

func toUnit(k string) *string {
	switch k {
	case "ms":
		return aws.String(cloudwatch.StandardUnitMilliseconds)
	default:
		return nil
	}
}
