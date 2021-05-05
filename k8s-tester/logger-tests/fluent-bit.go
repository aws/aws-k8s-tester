package logger_tests

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/onsi/ginkgo"
	"go.uber.org/zap"
	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbac_v1 "k8s.io/api/rbac/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/exec"
)

const (
	appName                        = "fluent-bit"
	appServiceAccountName          = "fluentbit-service-account"
	appRBACRoleName                = "fluentbit-rbac-role"
	appRBACClusterRoleBindingName  = "fluentbit-rbac-role-binding"
	appConfigMapNameClusterInfo    = "fluentbit-configmap-cluster-info"
	appConfigMapNameConfig         = "fluentbit-configmap-config"
	appConfigMapFileNameFluentConf = "fluent-bit.conf"
	appDaemonSetName               = "fluentbit-cloudwatch"
	appContainerImage              = "fluent/fluent-bit:1.5"
	appHTTPClient                  = "127.0.0.1"
)

func (ts *tester) createServiceAccount() error {
	ts.cfg.Logger.Info("creating %s: %s", zap.String("ServiceAccount", appName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cli.
		CoreV1().ServiceAccounts(ts.cfg.Namespace).Create(
		ctx,
		&v1.ServiceAccount{
			TypeMeta: meta_v1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ServiceAccount",
			},
			ObjectMeta: meta_v1.ObjectMeta{
				Name:      appServiceAccountName,
				Namespace: ts.cfg.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name": appName,
				},
			},
		},
		meta_v1.CreateOptions{},
	)
	cancel()
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("resource already exists", zap.String("ServiceAccount", appName))
			return nil
		}
		return fmt.Errorf("failed to create %s: %s (%v)", "ServiceAccount", appName, err)
	}
	ts.cfg.Logger.Info("Create resource", zap.String("ServiceAccount", appName))
	return nil
}

func (ts *tester) createRBACClusterRole() error {
	ts.cfg.Logger.Info("creating %s: %s", zap.String("ClusterRole", appName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cli.
		RbacV1().
		ClusterRoles().
		Create(
			ctx,
			&rbac_v1.ClusterRole{
				TypeMeta: meta_v1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRole",
				},
				// "ClusterRole" is a non-namespaced resource.
				// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/#role-and-clusterrole
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      appRBACRoleName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": appName,
					},
				},
				Rules: []rbac_v1.PolicyRule{
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
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("resource already exists", zap.String("ClusterRole", appName))
			return nil
		}
		return fmt.Errorf("failed to create %s: %s (%v)", "ClusterRole", appName, err)
	}
	ts.cfg.Logger.Info("Create resource", zap.String("ClusterRole", appName))
	return nil
}

func (ts *tester) createRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("creating %s: %s", zap.String("ClusterRoleBinding", appName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cli.
		RbacV1().
		ClusterRoleBindings().
		Create(
			ctx,
			&rbac_v1.ClusterRoleBinding{
				TypeMeta: meta_v1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRoleBinding",
				},
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      appRBACClusterRoleBindingName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": appName,
					},
				},
				RoleRef: rbac_v1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     appRBACRoleName,
				},
				Subjects: []rbac_v1.Subject{
					{
						APIGroup:  "",
						Kind:      "ServiceAccount",
						Name:      appServiceAccountName,
						Namespace: ts.cfg.Namespace,
					},
				},
			},
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("resource already exists", zap.String("ClusterRoleBinding", appName))
			return nil
		}
		return fmt.Errorf("failed to create %s: %s (%v)", "ClusterRoleBinding", appName, err)
	}
	ts.cfg.Logger.Info("Create resource", zap.String("ClusterRoleBinding", appName))
	return nil
}

const FluentBitConf = `
[SERVICE]
	Flush         1
	Log_Level     info
	Daemon        off
	Parsers_File  parsers.conf
	HTTP_Server   On
	HTTP_Listen   0.0.0.0
	HTTP_Port     2020
@INCLUDE input-kubernetes.conf
@INCLUDE filter-kubernetes.conf
@INCLUDE output.conf
`

const FiltersConf = `
[FILTER]
	Name                kubernetes
	Match               kube.*
	Kube_URL            https://kubernetes.default.svc:443
	Kube_CA_File        /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
	Kube_Token_File     /var/run/secrets/kubernetes.io/serviceaccount/token
	Kube_Tag_Prefix     kube.var.log.containers.
	Merge_Log           On
	Merge_Log_Key       log_processed
	K8S-Logging.Parser  On
	K8S-Logging.Exclude Off
`

const InputConf = `
[INPUT]
	Name              tail
	Tag               kube.*
	Path              /var/log/containers/*.log
	Parser            docker
	DB                /var/log/flb_kube.db
	Mem_Buf_Limit     5MB
	Skip_Long_Lines   On
	Refresh_Interval  10
`

const OutputConf = `
[OUTPUT]
    Name  counter
    Match *
`

const ParsersConf = `
[PARSER]
	Name   nginx
	Format regex
	Regex ^(?<remote>[^ ]*) (?<host>[^ ]*) (?<user>[^ ]*) \[(?<time>[^\]]*)\] "(?<method>\S+)(?: +(?<path>[^\"]*?)(?: +\S*)?)?" (?<code>[^ ]*) (?<size>[^ ]*)(?: "(?<referer>[^\"]*)" "(?<agent>[^\"]*)")?$
	Time_Key time
	Time_Format %d/%b/%Y:%H:%M:%S %z

[PARSER]
	Name   json
	Format json
	Time_Key time
	Time_Format %d/%b/%Y:%H:%M:%S %z

[PARSER]
	Name        docker
	Format      json
	Time_Key    time
	Time_Format %Y-%m-%dT%H:%M:%S.%L
	Time_Keep   On
`

func (ts *tester) createAppConfigMap() error {
	ts.cfg.Logger.Info("creating %s: %s", zap.String("Configmap", appConfigMapNameConfig))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cli.
		CoreV1().
		ConfigMaps(ts.cfg.Namespace).
		Create(
			ctx,
			&v1.ConfigMap{
				TypeMeta: meta_v1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      appConfigMapNameConfig,
					Namespace: ts.cfg.Namespace,
					Labels: map[string]string{
						"name": appConfigMapNameConfig,
					},
				},
				Data: map[string]string{
					"fluent-bit.conf":        FluentBitConf,
					"filter-kubernetes.conf": FiltersConf,
					"input-kubernetes.conf":  InputConf,
					"parsers.conf":           ParsersConf,
					"output.conf":            OutputConf,
				},
			},
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("resource already exists", zap.String("Configmap", appConfigMapNameConfig))
			return nil
		}
		return fmt.Errorf("failed to create %s: %s (%v)", "Configmap", appConfigMapNameConfig, err)
	}
	ts.cfg.Logger.Info("Create resource", zap.String("Configmap", appConfigMapNameConfig))
	return nil
}

func (ts *tester) createDaemonSet() error {
	ts.cfg.Logger.Info("creating %s: %s", zap.String("Daemonset", appName))
	dirOrCreate := v1.HostPathDirectoryOrCreate
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cli.
		AppsV1().
		DaemonSets(ts.cfg.Namespace).
		Create(
			ctx,
			&apps_v1.DaemonSet{
				TypeMeta: meta_v1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "DaemonSet",
				},
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      appName,
					Namespace: ts.cfg.Namespace,
				},
				Spec: apps_v1.DaemonSetSpec{
					Selector: &meta_v1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": appName,
						},
					},
					Template: core_v1.PodTemplateSpec{
						ObjectMeta: meta_v1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": appName,
							},
						},
						Spec: core_v1.PodSpec{
							ServiceAccountName:            appServiceAccountName,
							TerminationGracePeriodSeconds: aws.Int64(10),
							// Unsupported value: "OnFailure": supported values: "Always"
							RestartPolicy: v1.RestartPolicyAlways,
							Containers: []v1.Container{
								{
									Name:            appName,
									Image:           appContainerImage,
									ImagePullPolicy: v1.PullAlways,
									Resources: v1.ResourceRequirements{
										Limits: v1.ResourceList{
											v1.ResourceMemory: resource.MustParse("500Mi"),
										},
										Requests: v1.ResourceList{
											v1.ResourceCPU:    resource.MustParse("500m"),
											v1.ResourceMemory: resource.MustParse("100Mi"),
										},
									},
									VolumeMounts: []v1.VolumeMount{
										{
											Name:      appConfigMapNameConfig,
											MountPath: "/fluent-bit/etc/",
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
									},
								},
							},
							// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
							Volumes: []v1.Volume{
								{
									Name: appConfigMapNameConfig,
									VolumeSource: v1.VolumeSource{
										ConfigMap: &v1.ConfigMapVolumeSource{
											LocalObjectReference: v1.LocalObjectReference{
												Name: appConfigMapNameConfig,
											},
											DefaultMode: aws.Int32(0777),
										},
									},
								},
								{
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
							},
						},
					},
				},
			},
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("resource already exists", zap.String("Daemonset", appName))
			return nil
		}
		return fmt.Errorf("failed to create %s: %s (%v)", "Daemonset", appName, err)
	}
	ts.cfg.Logger.Info("Create resource", zap.String("Daemonset", appName))
	return nil
}

func (ts *tester) checkDaemonSet() error {
	timeout := 1 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	_, err := client.WaitForDaemonSetCompletes(
		ctx,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.Stopc,
		ts.cli,
		10*time.Second,
		10*time.Second,
		ts.cfg.Namespace,
		appName,
		client.WithQueryFunc(func() {
			descArgs := []string{
				ts.cfg.ClientConfig.KubectlPath,
				"--kubeconfig=" + ts.cfg.ClientConfig.KubeConfigPath,
				"--namespace=" + ts.cfg.Namespace,
				"describe",
				"daemonset",
				appName,
			}
			descCmd := strings.Join(descArgs, " ")
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			output, err := exec.New().CommandContext(ctx, descArgs[0], descArgs[1:]...).CombinedOutput()
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe daemonset' failed", zap.Error(err))
			}
			out := string(output)
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"%s\" output:\n%s\n\n", descCmd, out)
		}),
	)
	cancel()
	return err
}

func (ts *tester) createService() error {
	ts.cfg.Logger.Info("creating %s: %s", zap.String("Service", appName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cli.
		CoreV1().
		Services(ts.cfg.Namespace).
		Create(
			ctx,
			&core_v1.Service{
				TypeMeta: meta_v1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      appName,
					Namespace: ts.cfg.Namespace,
				},
				Spec: core_v1.ServiceSpec{
					Selector: map[string]string{
						"app.kubernetes.io/name": appName,
					},
					Ports: []core_v1.ServicePort{
						{
							Protocol:   core_v1.ProtocolTCP,
							Port:       80,
							TargetPort: intstr.FromInt(2020),
						},
					},
				},
			},
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("resource already exists", zap.String("Service", appName))
			return nil
		}
		return fmt.Errorf("failed to create %s: %s (%v)", "Service", appName, err)
	}
	ts.cfg.Logger.Info("Create resource", zap.String("Service", appName))
	return nil
}

func (ts *tester) testHTTPClient() error {
	ts.cfg.Logger.Info("Testing HTTP Client for %s: %s", zap.String("Daemonset", appName))
	// var t *testing.T
	// g := gomega.NewGomegaWithT(t)
	// ginkgo.It("iT Should allow  pass logging tests", func() {
	podName := "alpine"
	action := fmt.Sprintf("Creating Pod %v to test HTTP Client", podName)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	ginkgo.By(action)
	clientPod := newAlpinePod(podName, "curl "+appName)
	_, err := ts.cli.
		CoreV1().
		Pods(ts.cfg.Namespace).
		Create(ctx, clientPod, meta_v1.CreateOptions{})
	cancel()
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("resource already exists", zap.String("Pod", podName))
		}
		ts.cfg.Logger.Info("failed to create %s: %s (%v)", zap.String("Pod", podName))
	}
	// Wait 15 seconds for the pod to be spawned
	time.Sleep(10 * time.Second)
	logs, err := client.CheckPodLogs(
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.Stopc,
		ts.cli,
		ts.cfg.Namespace,
		podName,
	)
	if err != nil {
		ts.cfg.Logger.Warn("'kubectl get pod logs' failed", zap.Error(err))
	}
	if strings.Contains(logs, `1.5.7`) {
		ts.cfg.Logger.Info(
			"HTTP CLIENT:",
			zap.String("TEST", "PASSED"),
		)
		return nil
	}
	// g.Expect(strings.Contains(logs, "fluent-bit")).To(gomega.BeTrue(), "HTTP Client must respond to curl at `http://$HOST/`")
	// })
	return nil
}

func newAlpinePod(name, command string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: name,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    name,
					Image:   "byrnedo/alpine-curl",
					Command: []string{"/bin/sh"},
					Args:    []string{"-c", command},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}
}

// curl fluent-bit/ "{"fluent-bit":{"version":"1.5.7","edition":"Community","flags":["FLB_HAVE_PARSER","FLB_HAVE_RECORD_ACCESSOR","FLB_HAVE_STREAM_PROCESSOR","FLB_HAVE_TLS","FLB_HAVE_AWS","FLB_HAVE_SIGNV4","FLB_HAVE_SQLDB","FLB_HAVE_METRICS","FLB_HAVE_HTTP_SERVER","FLB_HAVE_SYSTEMD","FLB_HAVE_FORK","FLB_HAVE_TIMESPEC_GET","FLB_HAVE_GMTOFF","FLB_HAVE_UNIX_SOCKET","FLB_H/"
// curl fluent-bit/api/v1/uptime "{"uptime_sec":1203,"uptime_hr":"Fluent Bit has been running:  0 day, 0 hour, 20 minutes and 3 seconds"}/"
// curl fluent-bit/api/v1/metrics "{"input":{"tail.0":{"records":581981,"bytes":112790650,"files_opened":50,"files_closed":0,"files_rotated":0}},"filter":{"kubernetes.0":{"drop_records":0,"add_records":0}},"output":{"counter.0":{"proc_records":581981,"proc_bytes":481692918,"errors":0,"retries":0,"retries_failed":0}}}/"
// curl fluent-bit/api/v1/storage "{"fluent-bit":{"version":"1.5.7","edition":"Community","flags":["FLB_HAVE_PARSER","FLB_HAVE_RECORD_ACCESSOR","FLB_HAVE_STREAM_PROCESSOR","FLB_HAVE_TLS","FLB_HAVE_AWS","FLB_HAVE_SIGNV4","FLB_HAVE_SQLDB","FLB_HAVE_METRICS","FLB_HAVE_HTTP_SERVER","FLB_HAVE_SYSTEMD","FLB_HAVE_FORK","FLB_HAVE_TIMESPEC_GET","FLB_HAVE_GMTOFF","FLB_HAVE_UNIX_SOCKET","FLB_HAVE_PROXY_GO","FLB_HAVE_SYSTEM_STRPTIME","FLB_HAVE_JEMALLOC","FLB_HAVE_LIBBACKTRACE","FLB_HAVE_REGEX","FLB_HAVE_UTF8_ENCODER","FLB_HAVE_LUAJIT","FLB_HAVE_C_TLS","FLB_HAVE_ACCEPT4","FLB_HAVE_INOTIFY"]}}/"
