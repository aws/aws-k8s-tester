// Package fluentd implements Fluentd plugin.
// ref. https://www.eksworkshop.com/intermediate/230_logging/deploy/
package fluentd

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
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines fluentd configuration.
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg Config
}

// FluentdImageName is the image name of Fluentd daemon set.
// ref. https://github.com/fluent/fluentd-kubernetes-daemonset
const FluentdImageName = "fluent/fluentd-kubernetes-daemonset:v1.11.0-debian-cloudwatch-1.0"

func (ts *tester) Create() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnFluentd() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnFluentd.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnFluentd.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnFluentd.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err = k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		"kube-system",
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
	if err = ts.createDaemonSet(); err != nil {
		return err
	}
	if err = ts.checkPods(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnFluentd() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnFluentd.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnFluentd.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteDaemonSet(); err != nil {
		errs = append(errs, err.Error())
	}
	time.Sleep(2 * time.Minute)

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

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnFluentd.Created = false
	return ts.cfg.EKSConfig.Sync()
}

const (
	fluentdServiceAccountName              = "fluentd-service-account"
	fluentdRBACRoleName                    = "fluentd-rbac-role"
	fluentdRBACClusterRoleBindingName      = "fluentd-rbac-role-binding"
	fluentdConfigMapName                   = "fluentd-configmap"
	fluentdConfigMapFileNameFluentConf     = "fluent.conf"
	fluentdConfigMapFileNameContainersConf = "containers.conf"
	fluentdConfigMapFileNameSystemdConf    = "systemd.conf"
	fluentdConfigMapFileNameHostConf       = "host.conf"
	fluentdAppName                         = "fluentd-cloudwatch"
	fluentdDaemonSetName                   = "fluentd-cloudwatch"
)

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createServiceAccount() error {
	ts.cfg.Logger.Info("creating fluentd ServiceAccount")
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
					Name:      fluentdServiceAccountName,
					Namespace: "kube-system",
					Labels: map[string]string{
						"app.kubernetes.io/name": fluentdAppName,
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create fluentd ServiceAccount (%v)", err)
	}

	ts.cfg.Logger.Info("created fluentd ServiceAccount")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteServiceAccount() error {
	ts.cfg.Logger.Info("deleting fluentd ServiceAccount")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts("kube-system").
		Delete(
			ctx,
			fluentdServiceAccountName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete fluentd ServiceAccount (%v)", err)
	}
	ts.cfg.Logger.Info("deleted fluentd ServiceAccount", zap.Error(err))

	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createRBACClusterRole() error {
	ts.cfg.Logger.Info("creating fluentd RBAC ClusterRole")
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
				// "ClusterRole" is a non-namespaced resource.
				// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/#role-and-clusterrole
				ObjectMeta: metav1.ObjectMeta{
					Name:      fluentdRBACRoleName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": fluentdAppName,
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						// "" indicates the core API group
						// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/#role-and-clusterrole
						APIGroups: []string{
							"",
						},
						Resources: []string{
							"namespaces",
							"pods",
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
		return fmt.Errorf("failed to create fluentd RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("created fluentd RBAC ClusterRole")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteRBACClusterRole() error {
	ts.cfg.Logger.Info("deleting fluentd RBAC ClusterRole")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoles().
		Delete(
			ctx,
			fluentdRBACRoleName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete fluentd RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("deleted fluentd RBAC ClusterRole", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("creating fluentd RBAC ClusterRoleBinding")
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
					Name:      fluentdRBACClusterRoleBindingName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": fluentdAppName,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     fluentdRBACRoleName,
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup:  "",
						Kind:      "ServiceAccount",
						Name:      fluentdServiceAccountName,
						Namespace: "kube-system",
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create fluentd RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("created fluentd RBAC ClusterRoleBinding")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("deleting fluentd RBAC ClusterRoleBinding")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoleBindings().
		Delete(
			ctx,
			fluentdRBACClusterRoleBindingName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete fluentd RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("deleted fluentd RBAC ClusterRoleBinding", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

const templateFluentConf = `
@include containers.conf
@include systemd.conf

<match fluent.**>
  @type null
</match>
`

const templateContainersConf = `
<source>
  @type tail
  @id in_tail_container_logs
  @label @containers
  path /var/log/containers/*.log
  pos_file /var/log/fluentd-containers.log.pos
  tag *
  read_from_head true
  <parse>
    @type json
    time_format %Y-%m-%dT%H:%M:%S.%NZ
  </parse>
</source>

<label @containers>
  <filter **>
    @type kubernetes_metadata
    @id filter_kube_metadata
  </filter>

  <filter **>
    @type record_transformer
    @id filter_containers_stream_transformer
    <record>
      stream_name ${tag_parts[3]}
    </record>
  </filter>

  <match **>
    @type cloudwatch_logs
    @id out_cloudwatch_logs_containers
    region "#{ENV.fetch('REGION')}"
    log_group_name "/eks/#{ENV.fetch('CLUSTER_NAME')}/containers"
    log_stream_name_key stream_name
    remove_log_stream_name_key true
    auto_create_stream true
    <buffer>
        flush_interval 5
        chunk_limit_size 2m
        queued_chunks_limit_size 32
        retry_forever true
    </buffer>
  </match>
</label>
`

const templateSystemdConf = `
<source>
  @type systemd
  @id in_systemd_kubelet
  @label @systemd
  filters [{ "_SYSTEMD_UNIT": "kubelet.service" }]
  <entry>
    field_map {"MESSAGE": "message", "_HOSTNAME": "hostname", "_SYSTEMD_UNIT": "systemd_unit"}
    field_map_strict true
  </entry>
  path /run/log/journal
  pos_file /var/log/fluentd-journald-kubelet.pos
  read_from_head true
  tag kubelet.service
</source>

<source>
  @type systemd
  @id in_systemd_kubeproxy
  @label @systemd
  filters [{ "_SYSTEMD_UNIT": "kubeproxy.service" }]
  <entry>
    field_map {"MESSAGE": "message", "_HOSTNAME": "hostname", "_SYSTEMD_UNIT": "systemd_unit"}
    field_map_strict true
  </entry>
  path /run/log/journal
  pos_file /var/log/fluentd-journald-kubeproxy.pos
  read_from_head true
  tag kubeproxy.service
</source>

<source>
  @type systemd
  @id in_systemd_docker
  @label @systemd
  filters [{ "_SYSTEMD_UNIT": "docker.service" }]
  <entry>
    field_map {"MESSAGE": "message", "_HOSTNAME": "hostname", "_SYSTEMD_UNIT": "systemd_unit"}
    field_map_strict true
  </entry>
  path /run/log/journal
  pos_file /var/log/fluentd-journald-docker.pos
  read_from_head true
  tag docker.service
</source>

<label @systemd>
  <filter **>
    @type record_transformer
    @id filter_systemd_stream_transformer
    <record>
      stream_name ${tag}-${record["hostname"]}
    </record>
  </filter>

  <match **>
    @type cloudwatch_logs
    @id out_cloudwatch_logs_systemd
    region "#{ENV.fetch('REGION')}"
    log_group_name "/eks/#{ENV.fetch('CLUSTER_NAME')}/systemd"
    log_stream_name_key stream_name
    auto_create_stream true
    remove_log_stream_name_key true
    <buffer>
      flush_interval 5
      chunk_limit_size 2m
      queued_chunks_limit_size 32
      retry_forever true
    </buffer>
  </match>
</label>
`

func (ts *tester) createConfigMap() (err error) {
	ts.cfg.Logger.Info("creating config map")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ConfigMaps("kube-system").
		Create(
			ctx,
			&v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      fluentdConfigMapName,
					Namespace: "kube-system",
					Labels: map[string]string{
						"name": fluentdConfigMapName,
					},
				},
				Data: map[string]string{
					fluentdConfigMapFileNameFluentConf:     templateFluentConf,
					fluentdConfigMapFileNameContainersConf: templateContainersConf,
					fluentdConfigMapFileNameSystemdConf:    templateSystemdConf,
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
		ConfigMaps("kube-system").
		Delete(
			ctx,
			fluentdConfigMapName,
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

func (ts *tester) createDaemonSet() (err error) {
	dirOrCreate := v1.HostPathDirectoryOrCreate
	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app.kubernetes.io/name": fluentdAppName,
			},
		},
		Spec: v1.PodSpec{
			ServiceAccountName:            fluentdServiceAccountName,
			TerminationGracePeriodSeconds: aws.Int64(30),
			// Unsupported value: "OnFailure": supported values: "Always"
			RestartPolicy: v1.RestartPolicyAlways,

			// image's entrypoint requires to write on /fluentd/etc
			// but we mount configmap there as read-only
			// this init container copy workaround is required
			// https://github.com/fluent/fluentd-kubernetes-daemonset/issues/90
			InitContainers: []v1.Container{
				{
					Name:  "copy-fluentd-config",
					Image: "busybox",
					Command: []string{
						"sh",
						"-c",
						"cp /config-volume/..data/* /fluentd/etc",
					},
					// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
					VolumeMounts: []v1.VolumeMount{
						{ // to execute
							Name:      fluentdConfigMapName,
							MountPath: "/config-volume",
						},
						{
							Name:      "fluentdconf",
							MountPath: "/fluentd/etc",
						},
					},
				},
			},

			// https://www.eksworkshop.com/intermediate/230_logging/deploy/
			Containers: []v1.Container{
				{
					Name:            fluentdAppName,
					Image:           FluentdImageName,
					ImagePullPolicy: v1.PullAlways,

					Env: []v1.EnvVar{
						{
							Name:  "REGION",
							Value: ts.cfg.EKSConfig.Region,
						},
						{
							Name:  "CLUSTER_NAME",
							Value: ts.cfg.EKSConfig.Name,
						},
					},

					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("400Mi"),
						},
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("100m"),
							v1.ResourceMemory: resource.MustParse("200Mi"),
						},
					},

					// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
					VolumeMounts: []v1.VolumeMount{
						{ // to execute
							Name:      fluentdConfigMapName,
							MountPath: "/config-volume",
						},
						{
							Name:      "fluentdconf",
							MountPath: "/fluentd/etc",
						},
						{
							Name:      "varlibdockercontainers",
							MountPath: "/var/lib/docker/containers",
							ReadOnly:  true,
						},
						{
							Name:      "runlogjournal",
							MountPath: "/run/log/journal",
							ReadOnly:  true,
						},
						{
							Name:      "dmesg",
							MountPath: "/var/log/dmesg",
							ReadOnly:  true,
						},
					},
				},
			},

			// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
			Volumes: []v1.Volume{
				{ // to execute
					Name: fluentdConfigMapName,
					VolumeSource: v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: fluentdConfigMapName,
							},
							DefaultMode: aws.Int32(0777),
						},
					},
				},
				{
					Name: "fluentdconf",
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{},
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
				{
					Name: "varlibdockercontainers",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/var/lib/docker/containers",
							Type: &dirOrCreate,
						},
					},
				},
				{
					Name: "runlogjournal",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/run/log/journal",
						},
					},
				},
				{
					Name: "dmesg",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/var/log/dmesg",
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

	dsObj := appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "DaemonSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fluentdDaemonSetName,
			Namespace: "kube-system",
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": fluentdAppName,
				},
			},

			Template: podSpec,
		},
	}

	ts.cfg.Logger.Info("creating DaemonSet", zap.String("name", fluentdDaemonSetName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		DaemonSets("kube-system").
		Create(ctx, &dsObj, metav1.CreateOptions{})
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create DaemonSet (%v)", err)
	}

	ts.cfg.Logger.Info("created DaemonSet")
	return nil
}

func (ts *tester) deleteDaemonSet() (err error) {
	foreground := metav1.DeletePropagationForeground
	ts.cfg.Logger.Info("deleting DaemonSet", zap.String("name", fluentdDaemonSetName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err = ts.cfg.
		K8SClient.KubernetesClientSet().
		AppsV1().
		DaemonSets("kube-system").
		Delete(
			ctx,
			fluentdDaemonSetName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete DaemonSet (%v)", err)
	}
	return nil
}

func (ts *tester) checkPods() (err error) {
	waitDur := 10 * time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("check aborted")
		case <-time.After(15 * time.Second):
		}
		if err = ts._checkPods(); err == nil {
			break
		}
		ts.cfg.Logger.Info("failed to check fluentd pods; retrying", zap.Error(err))
	}
	return err
}

func (ts *tester) _checkPods() error {
	pods, err := ts.cfg.K8SClient.ListPods("kube-system", 150, 5*time.Second)
	if err != nil {
		ts.cfg.Logger.Warn("listing pods failed", zap.Error(err))
		return err
	}
	if len(pods) > 0 {
		ts.cfg.Logger.Info("pods found", zap.Int("pods", len(pods)))
		fmt.Fprintf(ts.cfg.LogWriter, "\n")
		for _, pod := range pods {
			fmt.Fprintf(ts.cfg.LogWriter, "'kube-system' Pod using client-go: %q\n", pod.Name)
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n")
	} else {
		ts.cfg.Logger.Info("no pod found")
		return errors.New("no pod found in kube-system")
	}

	ready := false
	for _, pod := range pods {
		appName, ok := pod.Labels["app.kubernetes.io/name"]
		if !ok || appName != fluentdAppName {
			ts.cfg.Logger.Info("skipping pod, not fluentd", zap.String("labels", fmt.Sprintf("%+v", pod.Labels)))
			continue
		}

		descArgsPods := []string{
			ts.cfg.EKSConfig.KubectlPath,
			"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
			"--namespace=kube-system",
			"describe",
			"pods/" + pod.Name,
		}
		descCmdPods := strings.Join(descArgsPods, " ")

		logArgs := []string{
			ts.cfg.EKSConfig.KubectlPath,
			"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
			"--namespace=kube-system",
			"logs",
			"pods/" + pod.Name,
			"--all-containers=true",
			"--timestamps",
		}
		logsCmd := strings.Join(logArgs, " ")

		ts.cfg.Logger.Info("checking Pod",
			zap.String("pod-name", pod.Name),
			zap.String("app-name", appName),
			zap.String("command-describe", descCmdPods),
			zap.String("command-logs", logsCmd),
		)

		for _, cond := range pod.Status.Conditions {
			if cond.Status != v1.ConditionTrue {
				continue
			}
			ts.cfg.Logger.Info("pod",
				zap.String("name", pod.GetName()),
				zap.String("status-type", fmt.Sprintf("%s", cond.Type)),
				zap.String("status", fmt.Sprintf("%s", cond.Status)),
			)
			ready = cond.Type == v1.PodInitialized || cond.Type == v1.PodReady
			break
		}
		if !ready {
			ts.cfg.Logger.Warn("pod is not ready yet", zap.String("pod-name", pod.Name))
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		output, err := exec.New().CommandContext(ctx, descArgsPods[0], descArgsPods[1:]...).CombinedOutput()
		cancel()
		out := string(output)
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl describe' failed", zap.Error(err))
			continue
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n'%s' output:\n\n%s\n\n", descCmdPods, out)

		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		output, err = exec.New().CommandContext(ctx, logArgs[0], logArgs[1:]...).CombinedOutput()
		cancel()
		out = string(output)
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl logs' failed", zap.Error(err))
			continue
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n'%s' output:\n\n%s\n\n", logsCmd, out)

		ts.cfg.Logger.Info("succcessfully checked pod; skipping next pod", zap.String("pod-name", pod.Name))
		break
	}
	if !ready {
		return errors.New("no fluentd pod ready")
	}

	return nil
}
