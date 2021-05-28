package fluentd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"strings"
	"time"

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

const (
	fluentdServiceAccountName              = "fluentd-service-account"
	fluentdRBACRoleName                    = "fluentd-rbac-role"
	fluentdRBACClusterRoleBindingName      = "fluentd-rbac-role-binding"
	fluentdConfigMapNameClusterInfo        = "fluentd-configmap-cluster-info"
	fluentdConfigMapNameConfig             = "fluentd-configmap-config"
	fluentdConfigMapFileNameFluentConf     = "fluent.conf"
	fluentdConfigMapFileNameContainersConf = "containers.conf"
	fluentdConfigMapFileNameSystemdConf    = "systemd.conf"
	fluentdConfigMapFileNameHostConf       = "host.conf"
	fluentdAppName                         = "fluentd-cloudwatch"
	fluentdDaemonSetName                   = "fluentd-cloudwatch"
)

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createFluentdServiceAccount() error {
	ts.cfg.Logger.Info("creating fluentd ServiceAccount")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnFluentd.Namespace).
		Create(
			ctx,
			&v1.ServiceAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      fluentdServiceAccountName,
					Namespace: ts.cfg.EKSConfig.AddOnFluentd.Namespace,
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
	ts.cfg.EKSConfig.Sync()
	return nil
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteFluentdServiceAccount() error {
	ts.cfg.Logger.Info("deleting fluentd ServiceAccount")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnFluentd.Namespace).
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

	ts.cfg.EKSConfig.Sync()
	return nil
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createFluentdRBACClusterRole() error {
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
							"pods/logs",
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
	ts.cfg.EKSConfig.Sync()
	return nil
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteFluentdRBACClusterRole() error {
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
	ts.cfg.EKSConfig.Sync()
	return nil
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createFluentdRBACClusterRoleBinding() error {
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
						Namespace: ts.cfg.EKSConfig.AddOnFluentd.Namespace,
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
	ts.cfg.EKSConfig.Sync()
	return nil
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteFluentdRBACClusterRoleBinding() error {
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
	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) createFluentdConfigMapClusterInfo() (err error) {
	ts.cfg.Logger.Info("creating fluentd ConfigMap cluster-info")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ConfigMaps(ts.cfg.EKSConfig.AddOnFluentd.Namespace).
		Create(
			ctx,
			&v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      fluentdConfigMapNameClusterInfo,
					Namespace: ts.cfg.EKSConfig.AddOnFluentd.Namespace,
					Labels: map[string]string{
						"name": fluentdConfigMapNameClusterInfo,
					},
				},
				Data: map[string]string{
					"cluster.name": ts.cfg.EKSConfig.Name,
					"logs.region":  ts.cfg.EKSConfig.Region,
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("created fluentd ConfigMap cluster-info")
	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) deleteFluentdConfigMapClusterInfo() error {
	ts.cfg.Logger.Info("deleting fluentd ConfigMap cluster-info")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ConfigMaps(ts.cfg.EKSConfig.AddOnFluentd.Namespace).
		Delete(
			ctx,
			fluentdConfigMapNameClusterInfo,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("deleted fluentd ConfigMap cluster-info")
	ts.cfg.EKSConfig.Sync()
	return nil
}

const TemplateFluentdConf = `
@include containers.conf
@include systemd.conf
@include host.conf

<filter kubernetes.var.log.containers.**.log>
  @type kubernetes_metadata
  @id filter_kube_metadata_application
  @log_level "{{.MetadataLogLevel}}"
  skip_labels {{.MetadataSkipLabels}}
  skip_container_metadata {{.MetadataSkipContainerMetadata}}
  skip_master_url {{.MetadataSkipMasterURL}}
  skip_namespace_metadata {{.MetadataSkipNamespaceMetadata}}
  cache_size {{.MetadataCacheSize}}
  watch {{.MetadataWatch}}
  de_dot false
</filter>

<match fluent.**>
  @type null
  num_threads {{.Threads}}
</match>
`

type templateFluentdConf struct {
	Threads                       int
	MetadataLogLevel              string
	MetadataCacheSize             int
	MetadataWatch                 bool
	MetadataSkipLabels            bool
	MetadataSkipMasterURL         bool
	MetadataSkipContainerMetadata bool
	MetadataSkipNamespaceMetadata bool
}

const TemplateContainersConf = `
<source>
  @type tail
  @id in_tail_container_logs
  @label @containers
  path /var/log/containers/*.log
  exclude_path ["/var/log/containers/cloudwatch-agent*", "/var/log/containers/fluentd*"]
  pos_file /var/log/fluentd-containers.log.pos
  tag *
  read_from_head true
  <parse>
    @type json
    time_format %Y-%m-%dT%H:%M:%S.%NZ
  </parse>
</source>

<source>
  @type tail
  @id in_tail_cwagent_logs
  @label @cwagentlogs
  path /var/log/containers/cloudwatch-agent*
  pos_file /var/log/cloudwatch-agent.log.pos
  tag *
  read_from_head true
  <parse>
    @type json
    time_format %Y-%m-%dT%H:%M:%S.%NZ
  </parse>
</source>

<source>
  @type tail
  @id in_tail_fluentd_logs
  @label @fluentdlogs
  path /var/log/containers/fluentd*
  pos_file /var/log/fluentd.log.pos
  tag *
  read_from_head true
  <parse>
    @type json
    time_format %Y-%m-%dT%H:%M:%S.%NZ
  </parse>
</source>

<label @fluentdlogs>
  <filter **>
    @type kubernetes_metadata
    @id filter_kube_metadata_fluentd
  </filter>

  <filter **>
    @type record_transformer
    @id filter_fluentd_stream_transformer
    <record>
      stream_name ${tag_parts[3]}
    </record>
  </filter>

  <match **>
    @type relabel
    @label @NORMAL
  </match>
</label>

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

  <filter **>
    @type concat
    key log
    multiline_start_regexp /^\S/
    separator ""
    flush_interval 5
    timeout_label @NORMAL
  </filter>

  <match **>
    @type relabel
    @label @NORMAL
  </match>
</label>

<label @cwagentlogs>
  <filter **>
    @type kubernetes_metadata
    @id filter_kube_metadata_cwagent
  </filter>

  <filter **>
    @type record_transformer
    @id filter_cwagent_stream_transformer
    <record>
      stream_name ${tag_parts[3]}
    </record>
  </filter>

  <filter **>
    @type concat
    key log
    multiline_start_regexp /^\d{4}[-/]\d{1,2}[-/]\d{1,2}/
    separator ""
    flush_interval 5
    timeout_label @NORMAL
  </filter>

  <match **>
    @type relabel
    @label @NORMAL
  </match>
</label>

<label @NORMAL>
  <match **>
    @type cloudwatch_logs
    @id out_cloudwatch_logs_containers
    region "#{ENV.fetch('REGION')}"
    log_group_name "/aws/containerinsights/#{ENV.fetch('CLUSTER_NAME')}/application"
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

const TemplateSystemdConf = `
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
  <storage>
    @type local
    persistent true
    path /var/log/fluentd-journald-kubelet-pos.json
  </storage>
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
  <storage>
    @type local
    persistent true
    path /var/log/fluentd-journald-kubeproxy-pos.json
  </storage>
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
  <storage>
    @type local
    persistent true
    path /var/log/fluentd-journald-docker-pos.json
  </storage>
  read_from_head true
  tag docker.service
</source>

<label @systemd>
  <filter **>
    @type kubernetes_metadata
    @id filter_kube_metadata_systemd
  </filter>

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
    log_group_name "/aws/containerinsights/#{ENV.fetch('CLUSTER_NAME')}/dataplane"
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

const TemplateHostConf = `
<source>
  @type tail
  @id in_tail_dmesg
  @label @hostlogs
  path /var/log/dmesg
  pos_file /var/log/dmesg.log.pos
  tag host.dmesg
  read_from_head true
  <parse>
    @type syslog
  </parse>
</source>

<source>
  @type tail
  @id in_tail_secure
  @label @hostlogs
  path /var/log/secure
  pos_file /var/log/secure.log.pos
  tag host.secure
  read_from_head true
  <parse>
    @type syslog
  </parse>
</source>

<source>
  @type tail
  @id in_tail_messages
  @label @hostlogs
  path /var/log/messages
  pos_file /var/log/messages.log.pos
  tag host.messages
  read_from_head true
  <parse>
    @type syslog
  </parse>
</source>

<label @hostlogs>
  <filter **>
    @type kubernetes_metadata
    @id filter_kube_metadata_host
  </filter>

  <filter **>
    @type record_transformer
    @id filter_containers_stream_transformer_host
    <record>
      stream_name ${tag}-${record["host"]}
    </record>
  </filter>

  <match host.**>
    @type cloudwatch_logs
    @id out_cloudwatch_logs_host_logs
    region "#{ENV.fetch('REGION')}"
    log_group_name "/aws/containerinsights/#{ENV.fetch('CLUSTER_NAME')}/host"
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

func (ts *tester) createFluentdConfigMapConfig() (err error) {
	ts.cfg.Logger.Info("creating fluentd ConfigMap config")

	buf := bytes.NewBuffer(nil)
	fdConf := templateFluentdConf{
		Threads:                       ts.cfg.EKSConfig.AddOnFluentd.Threads,
		MetadataLogLevel:              ts.cfg.EKSConfig.AddOnFluentd.MetadataLogLevel,
		MetadataCacheSize:             ts.cfg.EKSConfig.AddOnFluentd.MetadataCacheSize,
		MetadataWatch:                 ts.cfg.EKSConfig.AddOnFluentd.MetadataWatch,
		MetadataSkipLabels:            ts.cfg.EKSConfig.AddOnFluentd.MetadataSkipLabels,
		MetadataSkipMasterURL:         ts.cfg.EKSConfig.AddOnFluentd.MetadataSkipMasterURL,
		MetadataSkipContainerMetadata: ts.cfg.EKSConfig.AddOnFluentd.MetadataSkipContainerMetadata,
		MetadataSkipNamespaceMetadata: ts.cfg.EKSConfig.AddOnFluentd.MetadataSkipNamespaceMetadata,
	}
	fdConfTmpl := template.Must(template.New("TemplateFluentdConf").Parse(TemplateFluentdConf))
	if err := fdConfTmpl.Execute(buf, fdConf); err != nil {
		return err
	}
	fdConfBody := buf.String()
	buf.Reset()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ConfigMaps(ts.cfg.EKSConfig.AddOnFluentd.Namespace).
		Create(
			ctx,
			&v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      fluentdConfigMapNameConfig,
					Namespace: ts.cfg.EKSConfig.AddOnFluentd.Namespace,
					Labels: map[string]string{
						"name": fluentdConfigMapNameConfig,
					},
				},
				Data: map[string]string{
					fluentdConfigMapFileNameFluentConf:     fdConfBody,
					fluentdConfigMapFileNameContainersConf: TemplateContainersConf,
					fluentdConfigMapFileNameSystemdConf:    TemplateSystemdConf,
					fluentdConfigMapFileNameHostConf:       TemplateHostConf,
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("created fluentd ConfigMap config")
	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) deleteFluentdConfigMapConfig() error {
	ts.cfg.Logger.Info("deleting fluentd ConfigMap config")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ConfigMaps(ts.cfg.EKSConfig.AddOnFluentd.Namespace).
		Delete(
			ctx,
			fluentdConfigMapNameConfig,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("deleted fluentd ConfigMap config")
	ts.cfg.EKSConfig.Sync()
	return nil
}

// FluentdImageName is the image name of Fluentd daemon set.
// ref. https://github.com/fluent/fluentd-kubernetes-daemonset
const FluentdImageName = "fluent/fluentd-kubernetes-daemonset:v1.7.3-debian-cloudwatch-1.0"

func (ts *tester) createFluentdDaemonSet() (err error) {
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
					Image: ts.busyboxImg,
					Command: []string{
						"sh",
						"-c",
						"cp /config-volume/..data/* /fluentd/etc",
					},
					// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
					VolumeMounts: []v1.VolumeMount{
						{ // to execute
							Name:      fluentdConfigMapNameConfig,
							MountPath: "/config-volume",
						},
						{
							Name:      "fluentdconf",
							MountPath: "/fluentd/etc",
						},
					},
				},

				// TODO: do we need this?
				// ref. https://github.com/aws-samples/amazon-cloudwatch-container-insights/tree/master/k8s-deployment-manifest-templates/deployment-mode/daemonset/container-insights-monitoring/quickstart
				// {
				// 	Name:    "update-log-driver",
				// 	Image:   ts.busyboxImg,
				// 	Command: []string{"sh", "-c", "''"},
				// },
			},

			// https://www.eksworkshop.com/intermediate/230_logging/deploy/
			Containers: []v1.Container{
				{
					Name:            fluentdAppName,
					Image:           FluentdImageName,
					ImagePullPolicy: v1.PullAlways,

					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("400Mi"),
						},
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("100m"),
							v1.ResourceMemory: resource.MustParse("200Mi"),
						},
					},

					Env: []v1.EnvVar{
						{
							Name: "REGION",
							ValueFrom: &v1.EnvVarSource{
								ConfigMapKeyRef: &v1.ConfigMapKeySelector{
									LocalObjectReference: v1.LocalObjectReference{
										Name: fluentdConfigMapNameClusterInfo,
									},
									Key: "logs.region",
								},
							},
						},
						{
							Name: "CLUSTER_NAME",
							ValueFrom: &v1.EnvVarSource{
								ConfigMapKeyRef: &v1.ConfigMapKeySelector{
									LocalObjectReference: v1.LocalObjectReference{
										Name: fluentdConfigMapNameClusterInfo,
									},
									Key: "cluster.name",
								},
							},
						},
						{
							Name:  "CI_VERSION",
							Value: "k8s/1.1.1",
						},
					},

					// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      fluentdConfigMapNameConfig,
							MountPath: "/config-volume",
						},
						{
							Name:      "fluentdconf",
							MountPath: "/fluentd/etc",
						},
						{
							Name:      "varlog",
							MountPath: "/var/log",
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
				{
					Name: fluentdConfigMapNameConfig,
					VolumeSource: v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: fluentdConfigMapNameConfig,
							},
							DefaultMode: aws.Int32(0666),
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
							Type: &dirOrCreate,
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
			Namespace: ts.cfg.EKSConfig.AddOnFluentd.Namespace,
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

	ts.cfg.Logger.Info("creating fluentd DaemonSet", zap.String("name", fluentdDaemonSetName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		DaemonSets(ts.cfg.EKSConfig.AddOnFluentd.Namespace).
		Create(ctx, &dsObj, metav1.CreateOptions{})
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create fluentd DaemonSet (%v)", err)
	}

	ts.cfg.Logger.Info("created fluentd DaemonSet")
	return nil
}

func (ts *tester) deleteFluentdDaemonSet() (err error) {
	foreground := metav1.DeletePropagationForeground
	ts.cfg.Logger.Info("deleting fluentd DaemonSet", zap.String("name", fluentdDaemonSetName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err = ts.cfg.
		K8SClient.KubernetesClientSet().
		AppsV1().
		DaemonSets(ts.cfg.EKSConfig.AddOnFluentd.Namespace).
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
		ts.cfg.Logger.Warn("failed to delete fluentd DaemonSet", zap.Error(err))
		return fmt.Errorf("failed to delete fluentd DaemonSet (%v)", err)
	}
	return nil
}

func (ts *tester) checkFluentdPods() (err error) {
	waitDur := 10 * time.Minute
	retryStart := time.Now()
	for time.Since(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("check aborted")
		case <-time.After(15 * time.Second):
		}
		if err = ts._checkFluentdPods(); err == nil {
			break
		}
		ts.cfg.Logger.Info("failed to check fluentd pods; retrying", zap.Error(err))
	}
	return err
}

func (ts *tester) _checkFluentdPods() error {
	pods, err := ts.cfg.K8SClient.ListPods(ts.cfg.EKSConfig.AddOnFluentd.Namespace, 1000, 5*time.Second)
	if err != nil {
		ts.cfg.Logger.Warn("listing pods failed", zap.Error(err))
		return err
	}
	if len(pods) > 0 {
		ts.cfg.Logger.Info("pods found", zap.Int("pods", len(pods)))
		fmt.Fprintf(ts.cfg.LogWriter, "\n")
		for _, pod := range pods {
			fmt.Fprintf(ts.cfg.LogWriter, "%q Pod using client-go: %q\n", ts.cfg.EKSConfig.AddOnFluentd.Namespace, pod.Name)
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n")
	} else {
		ts.cfg.Logger.Info("no pod found", zap.String("namespace", ts.cfg.EKSConfig.AddOnFluentd.Namespace))
		return errors.New("no pod found in " + ts.cfg.EKSConfig.AddOnFluentd.Namespace)
	}

	targetPods := int64(1)
	if ts.cfg.EKSConfig.TotalNodes > 1 {
		targetPods = ts.cfg.EKSConfig.TotalNodes / int64(2)
	}
	ts.cfg.Logger.Info("checking fluentd pods",
		zap.Int64("target-ready-pods", targetPods),
		zap.Int64("total-nodes", ts.cfg.EKSConfig.TotalNodes),
	)
	readyPods := int64(0)
	for _, pod := range pods {
		appName, ok := pod.Labels["app.kubernetes.io/name"]
		if !ok || appName != fluentdAppName {
			ts.cfg.Logger.Info("skipping pod, not fluentd", zap.String("labels", fmt.Sprintf("%+v", pod.Labels)))
			continue
		}

		descArgsPods := []string{
			ts.cfg.EKSConfig.KubectlPath,
			"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
			"--namespace=" + ts.cfg.EKSConfig.AddOnFluentd.Namespace,
			"describe",
			"pods/" + pod.Name,
		}
		descCmdPods := strings.Join(descArgsPods, " ")

		logArgs := []string{
			ts.cfg.EKSConfig.KubectlPath,
			"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
			"--namespace=" + ts.cfg.EKSConfig.AddOnFluentd.Namespace,
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
	ts.cfg.Logger.Info("checking fluentd pods",
		zap.Int64("current-ready-pods", readyPods),
		zap.Int64("target-ready-pods", targetPods),
		zap.Int64("total-nodes", ts.cfg.EKSConfig.TotalNodes),
	)
	if readyPods < targetPods {
		return errors.New("not enough fluentd pods ready")
	}

	return nil
}
