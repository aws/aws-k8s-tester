// Package cloudwatch_agent installs an Amazon CloudWatch Agent.
// Replace https://github.com/aws/aws-k8s-tester/tree/v1.5.9/eks/cw-agent.
// See https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-setup-logs.html.
// See https://hub.docker.com/r/amazon/cloudwatch-agent/tags?page=1&ordering=last_updated.
package cloudwatch_agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"reflect"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester/tester"
	"github.com/aws/aws-k8s-tester/utils/rand"
	utils_time "github.com/aws/aws-k8s-tester/utils/time"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	rbac_v1 "k8s.io/api/rbac/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	api_resource "k8s.io/apimachinery/pkg/api/resource"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_client "k8s.io/client-go/kubernetes"
	"k8s.io/utils/exec"
)

type Config struct {
	Prompt bool `json:"-"`

	Logger    *zap.Logger   `json:"-"`
	LogWriter io.Writer     `json:"-"`
	Stopc     chan struct{} `json:"-"`

	ClientConfig *client.Config `json:"-"`

	Region      string `json:"region"`
	ClusterName string `json:"cluster-name"`

	// MinimumNodes is the minimum number of Kubernetes nodes required for installing this addon.
	MinimumNodes int `json:"minimum-nodes"`
	// Namespace to create test resources.
	Namespace string `json:"namespace"`
}

const DefaultMinimumNodes int = 1

func NewDefault() *Config {
	return &Config{
		MinimumNodes: DefaultMinimumNodes,
		Namespace:    pkgName + "-" + rand.String(10) + "-" + utils_time.GetTS(10),
	}
}

func New(cfg *Config) k8s_tester.Tester {
	ccfg, err := client.CreateConfig(cfg.ClientConfig)
	if err != nil {
		cfg.Logger.Panic("failed to create client config", zap.Error(err))
	}
	cli, err := k8s_client.NewForConfig(ccfg)
	if err != nil {
		cfg.Logger.Panic("failed to create client", zap.Error(err))
	}

	return &tester{
		cfg: cfg,
		cli: cli,
	}
}

type tester struct {
	cfg *Config
	cli k8s_client.Interface
}

var pkgName = path.Base(reflect.TypeOf(tester{}).PkgPath())

func Env() string {
	return "ADD_ON_" + strings.ToUpper(strings.Replace(pkgName, "-", "_", -1))
}

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Apply() error {
	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}

	if nodes, err := client.ListNodes(ts.cli); len(nodes) < ts.cfg.MinimumNodes || err != nil {
		return fmt.Errorf("failed to validate minimum nodes requirement %d (nodes %v, error %v)", ts.cfg.MinimumNodes, len(nodes), err)
	}

	if err := ts.createServiceAccount(); err != nil {
		return err
	}

	if err := ts.createRBACClusterRole(); err != nil {
		return err
	}

	if err := ts.createRBACClusterRoleBinding(); err != nil {
		return err
	}

	if err := ts.createConfigMapConfig(); err != nil {
		return err
	}

	if err := ts.createDaemonSet(); err != nil {
		return err
	}

	if err := ts.checkPods(); err != nil {
		return err
	}

	return nil
}

func (ts *tester) Delete() error {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}

	var errs []string

	if err := ts.deleteDaemonSet(); err != nil {
		errs = append(errs, err.Error())
	}
	time.Sleep(time.Minute)

	if err := ts.deleteConfigMapConfig(); err != nil {
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

	if err := client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cli,
		ts.cfg.Namespace,
		client.DefaultNamespaceDeletionInterval,
		client.DefaultNamespaceDeletionTimeout,
		client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	return nil
}

func (ts *tester) runPrompt(action string) (ok bool) {
	if ts.cfg.Prompt {
		msg := fmt.Sprintf("Ready to %q resources for the namespace %q, should we continue?", action, ts.cfg.Namespace)
		prompt := promptui.Select{
			Label: msg,
			Items: []string{
				"No, cancel it!",
				fmt.Sprintf("Yes, let's %q!", action),
			},
		}
		idx, answer, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx != 1 {
			fmt.Printf("cancelled %q [index %d, answer %q]\n", action, idx, answer)
			return false
		}
	}
	return true
}

const (
	cwAgentServiceAccountName         = "amazon-cloudwatch-agent-service-account"
	cwAgentRBACRoleName               = "amazon-cloudwatch-agent-rbac-role"
	cwAgentRBACClusterRoleBindingName = "amazon-cloudwatch-agent-rbac-role-binding"
	cwAgentConfigMapNameConfig        = "amazon-cloudwatch-configmap-config"
	cwAgentConfigMapFileNameConfig    = "cwagentconfig.json"
	cwAgentAppName                    = "amazon-cloudwatch"
	cwAgentDaemonSetName              = "amazon-cloudwatch"
)

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createServiceAccount() error {
	ts.cfg.Logger.Info("creating cw agent ServiceAccount")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cli.
		CoreV1().
		ServiceAccounts(ts.cfg.Namespace).
		Create(
			ctx,
			&core_v1.ServiceAccount{
				TypeMeta: meta_v1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
				},
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      cwAgentServiceAccountName,
					Namespace: ts.cfg.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": cwAgentAppName,
					},
				},
			},
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create cw agent ServiceAccount (%v)", err)
	}

	ts.cfg.Logger.Info("created cw agent ServiceAccount")
	return nil
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteServiceAccount() error {
	ts.cfg.Logger.Info("deleting cw agent ServiceAccount")
	foreground := meta_v1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cli.
		CoreV1().
		ServiceAccounts(ts.cfg.Namespace).
		Delete(
			ctx,
			cwAgentServiceAccountName,
			meta_v1.DeleteOptions{
				GracePeriodSeconds: int64Ref(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !k8s_errors.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete cw agent ServiceAccount (%v)", err)
	}
	ts.cfg.Logger.Info("deleted cw agent ServiceAccount", zap.Error(err))

	return nil
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createRBACClusterRole() error {
	ts.cfg.Logger.Info("creating cw agent RBAC ClusterRole")
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
					Name:      cwAgentRBACRoleName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": cwAgentAppName,
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
							"pods",
							"nodes",
							"endpoints",
						},
						Verbs: []string{
							"list",
							"watch",
						},
					},
					{
						APIGroups: []string{
							"apps",
						},
						Resources: []string{
							"replicasets",
						},
						Verbs: []string{
							"list",
							"watch",
						},
					},
					{
						APIGroups: []string{
							"batch",
						},
						Resources: []string{
							"jobs",
						},
						Verbs: []string{
							"list",
							"watch",
						},
					},
					{
						// "" indicates the core API group
						// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/#role-and-clusterrole
						APIGroups: []string{
							"",
						},
						Resources: []string{
							"nodes/stats",
							"configmaps",
							"events",
						},
						Verbs: []string{
							"create",
						},
					},
					{
						// "" indicates the core API group
						// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/#role-and-clusterrole
						APIGroups: []string{
							"",
						},
						Resources: []string{
							"configmaps",
						},
						ResourceNames: []string{
							"cwagent-clusterleader",
						},
						Verbs: []string{
							"get",
							"update",
						},
					},
				},
			},
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create cw agent RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("created cw agent RBAC ClusterRole")
	return nil
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteRBACClusterRole() error {
	ts.cfg.Logger.Info("deleting cw agent RBAC ClusterRole")
	foreground := meta_v1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cli.
		RbacV1().
		ClusterRoles().
		Delete(
			ctx,
			cwAgentRBACRoleName,
			meta_v1.DeleteOptions{
				GracePeriodSeconds: int64Ref(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !k8s_errors.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete cw agent RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("deleted cw agent RBAC ClusterRole", zap.Error(err))
	return nil
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("creating cw agent RBAC ClusterRoleBinding")
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
					Name:      cwAgentRBACClusterRoleBindingName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": cwAgentAppName,
					},
				},
				RoleRef: rbac_v1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     cwAgentRBACRoleName,
				},
				Subjects: []rbac_v1.Subject{
					{
						APIGroup:  "",
						Kind:      "ServiceAccount",
						Name:      cwAgentServiceAccountName,
						Namespace: ts.cfg.Namespace,
					},
				},
			},
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create cw agent RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("created cw agent RBAC ClusterRoleBinding")
	return nil
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("deleting cw agent RBAC ClusterRoleBinding")
	foreground := meta_v1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cli.
		RbacV1().
		ClusterRoleBindings().
		Delete(
			ctx,
			cwAgentRBACClusterRoleBindingName,
			meta_v1.DeleteOptions{
				GracePeriodSeconds: int64Ref(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !k8s_errors.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete cw agent RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("deleted cw agent RBAC ClusterRoleBinding", zap.Error(err))
	return nil
}

// https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-setup-metrics.html
const TemplateCWAgentConf = `{
  "agent": {
    "region": "{{.RegionName}}"
  },
  "logs": {
    "metrics_collected": {
      "kubernetes": {
        "cluster_name": "{{.ClusterName}}",
        "metrics_collection_interval": 60
      }
    },
    "force_flush_interval": 5
  }
}
`

type templateCWAgentConf struct {
	RegionName  string
	ClusterName string
}

func (ts *tester) createConfigMapConfig() (err error) {
	ts.cfg.Logger.Info("creating cw agent ConfigMap config")

	buf := bytes.NewBuffer(nil)
	cwConf := templateCWAgentConf{
		RegionName:  ts.cfg.Region,
		ClusterName: ts.cfg.ClusterName,
	}
	cwConfTmpl := template.Must(template.New("TemplateCWAgentConf").Parse(TemplateCWAgentConf))
	if err := cwConfTmpl.Execute(buf, cwConf); err != nil {
		return err
	}
	cwConfBody := buf.String()
	buf.Reset()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cli.
		CoreV1().
		ConfigMaps(ts.cfg.Namespace).
		Create(
			ctx,
			&core_v1.ConfigMap{
				TypeMeta: meta_v1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      cwAgentConfigMapNameConfig,
					Namespace: ts.cfg.Namespace,
					Labels: map[string]string{
						"name": cwAgentConfigMapNameConfig,
					},
				},
				Data: map[string]string{
					cwAgentConfigMapFileNameConfig: cwConfBody,
				},
			},
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("created cw agent ConfigMap config")
	return nil
}

func (ts *tester) deleteConfigMapConfig() error {
	ts.cfg.Logger.Info("deleting cw agent ConfigMap config")
	foreground := meta_v1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cli.
		CoreV1().
		ConfigMaps(ts.cfg.Namespace).
		Delete(
			ctx,
			cwAgentConfigMapNameConfig,
			meta_v1.DeleteOptions{
				GracePeriodSeconds: int64Ref(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("deleted cw agent ConfigMap config")
	return nil
}

// CWAgentImageName is the image name of CloudWatch agent daemon set.
// ref. https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-setup-logs.html
// ref. https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-setup-EKS-quickstart.html
// ref. https://hub.docker.com/r/amazon/cloudwatch-agent
const CWAgentImageName = "amazon/cloudwatch-agent:1.247347.6b250880"

func (ts *tester) createDaemonSet() (err error) {
	podSpec := core_v1.PodTemplateSpec{
		ObjectMeta: meta_v1.ObjectMeta{
			Labels: map[string]string{
				"app.kubernetes.io/name": cwAgentAppName,
			},
		},
		Spec: core_v1.PodSpec{
			ServiceAccountName:            cwAgentServiceAccountName,
			TerminationGracePeriodSeconds: int64Ref(60),
			// Unsupported value: "OnFailure": supported values: "Always"
			RestartPolicy: core_v1.RestartPolicyAlways,

			// https://www.eksworkshop.com/intermediate/230_logging/deploy/
			Containers: []core_v1.Container{
				{
					Name:            cwAgentAppName,
					Image:           CWAgentImageName,
					ImagePullPolicy: core_v1.PullAlways,

					Resources: core_v1.ResourceRequirements{
						Limits: core_v1.ResourceList{
							core_v1.ResourceCPU:    api_resource.MustParse("200m"),
							core_v1.ResourceMemory: api_resource.MustParse("200Mi"),
						},
						Requests: core_v1.ResourceList{
							core_v1.ResourceCPU:    api_resource.MustParse("200m"),
							core_v1.ResourceMemory: api_resource.MustParse("200Mi"),
						},
					},

					Env: []core_v1.EnvVar{
						{
							Name: "HOST_IP",
							ValueFrom: &core_v1.EnvVarSource{
								FieldRef: &core_v1.ObjectFieldSelector{
									FieldPath: "status.hostIP",
								},
							},
						},
						{
							Name: "HOST_NAME",
							ValueFrom: &core_v1.EnvVarSource{
								FieldRef: &core_v1.ObjectFieldSelector{
									FieldPath: "spec.nodeName",
								},
							},
						},
						{
							Name: "K8S_NAMESPACE",
							ValueFrom: &core_v1.EnvVarSource{
								FieldRef: &core_v1.ObjectFieldSelector{
									FieldPath: "metadata.namespace",
								},
							},
						},
						{
							Name:  "CI_VERSION",
							Value: "k8s/1.1.1",
						},
					},

					// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
					VolumeMounts: []core_v1.VolumeMount{
						{
							Name:      cwAgentConfigMapNameConfig,
							MountPath: "/etc/cwagentconfig",
						},
						{
							Name:      "rootfs",
							MountPath: "/rootfs",
							ReadOnly:  true,
						},
						{
							Name:      "dockersock",
							MountPath: "/var/run/docker.sock",
							ReadOnly:  true,
						},
						{
							Name:      "varlibdocker",
							MountPath: "/var/lib/docker",
							ReadOnly:  true,
						},
						{
							Name:      "sys",
							MountPath: "/sys",
							ReadOnly:  true,
						},
						{
							Name:      "devdisk",
							MountPath: "/dev/disk",
							ReadOnly:  true,
						},
					},
				},
			},

			// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
			Volumes: []core_v1.Volume{
				{
					Name: cwAgentConfigMapNameConfig,
					VolumeSource: core_v1.VolumeSource{
						ConfigMap: &core_v1.ConfigMapVolumeSource{
							LocalObjectReference: core_v1.LocalObjectReference{
								Name: cwAgentConfigMapNameConfig,
							},
							DefaultMode: int32Ref(0666),
						},
					},
				},
				{
					Name: "rootfs",
					VolumeSource: core_v1.VolumeSource{
						HostPath: &core_v1.HostPathVolumeSource{
							Path: "/",
						},
					},
				},
				{
					Name: "dockersock",
					VolumeSource: core_v1.VolumeSource{
						HostPath: &core_v1.HostPathVolumeSource{
							Path: "/var/run/docker.sock",
						},
					},
				},
				{
					Name: "varlibdocker",
					VolumeSource: core_v1.VolumeSource{
						HostPath: &core_v1.HostPathVolumeSource{
							Path: "/var/lib/docker",
						},
					},
				},
				{
					Name: "sys",
					VolumeSource: core_v1.VolumeSource{
						HostPath: &core_v1.HostPathVolumeSource{
							Path: "/sys",
						},
					},
				},
				{
					Name: "devdisk",
					VolumeSource: core_v1.VolumeSource{
						HostPath: &core_v1.HostPathVolumeSource{
							Path: "/dev/disk/",
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

	dsObj := apps_v1.DaemonSet{
		TypeMeta: meta_v1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "DaemonSet",
		},
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      cwAgentDaemonSetName,
			Namespace: ts.cfg.Namespace,
		},
		Spec: apps_v1.DaemonSetSpec{
			Selector: &meta_v1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": cwAgentAppName,
				},
			},

			Template: podSpec,
		},
	}

	ts.cfg.Logger.Info("creating cw agent DaemonSet", zap.String("name", cwAgentDaemonSetName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cli.
		AppsV1().
		DaemonSets(ts.cfg.Namespace).
		Create(ctx, &dsObj, meta_v1.CreateOptions{})
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create cw agent DaemonSet (%v)", err)
	}

	ts.cfg.Logger.Info("created cw agent DaemonSet")
	return nil
}

func (ts *tester) deleteDaemonSet() (err error) {
	foreground := meta_v1.DeletePropagationForeground
	ts.cfg.Logger.Info("deleting cw agent DaemonSet", zap.String("name", cwAgentDaemonSetName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err = ts.cli.
		AppsV1().
		DaemonSets(ts.cfg.Namespace).
		Delete(
			ctx,
			cwAgentDaemonSetName,
			meta_v1.DeleteOptions{
				GracePeriodSeconds: int64Ref(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !k8s_errors.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete cw agent DaemonSet", zap.Error(err))
		return fmt.Errorf("failed to delete cw agent DaemonSet (%v)", err)
	}
	return nil
}

func (ts *tester) checkPods() (err error) {
	waitDur := 10 * time.Minute
	retryStart := time.Now()
	for time.Since(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("check aborted")
		case <-time.After(15 * time.Second):
		}
		if err = ts._checkPods(); err == nil {
			break
		}
		ts.cfg.Logger.Info("failed to check cw agent pods; retrying", zap.Error(err))
	}
	return err
}

func (ts *tester) _checkPods() error {
	pods, err := client.ListPods(ts.cfg.Logger, ts.cli, ts.cfg.Namespace, 1000, 5*time.Second)
	if err != nil {
		ts.cfg.Logger.Warn("listing pods failed", zap.Error(err))
		return err
	}
	if len(pods) > 0 {
		ts.cfg.Logger.Info("pods found", zap.Int("pods", len(pods)))
		fmt.Fprintf(ts.cfg.LogWriter, "\n")
		for _, pod := range pods {
			fmt.Fprintf(ts.cfg.LogWriter, "%q Pod using client-go: %q\n", ts.cfg.Namespace, pod.Name)
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n")
	} else {
		ts.cfg.Logger.Info("no pod found", zap.String("namespace", ts.cfg.Namespace))
		return errors.New("no pod found in " + ts.cfg.Namespace)
	}

	nodes, err := client.ListNodes(ts.cli)
	if err != nil {
		return fmt.Errorf("failed to list nodes %v", err)
	}

	totalNodes := int64(len(nodes))
	targetPods := int64(1)
	if totalNodes > 1 {
		targetPods = totalNodes / int64(2)
	}
	ts.cfg.Logger.Info("checking cw agent pods",
		zap.Int64("target-ready-pods", targetPods),
		zap.Int64("total-nodes", totalNodes),
	)
	readyPods := int64(0)
	for _, pod := range pods {
		appName, ok := pod.Labels["app.kubernetes.io/name"]
		if !ok || appName != cwAgentAppName {
			ts.cfg.Logger.Info("skipping pod, not cw agent", zap.String("labels", fmt.Sprintf("%+v", pod.Labels)))
			continue
		}

		descArgsPods := []string{
			ts.cfg.ClientConfig.KubectlPath,
			"--kubeconfig=" + ts.cfg.ClientConfig.KubeconfigPath,
			"--namespace=" + ts.cfg.Namespace,
			"describe",
			"pods/" + pod.Name,
		}
		descCmdPods := strings.Join(descArgsPods, " ")

		logArgs := []string{
			ts.cfg.ClientConfig.KubectlPath,
			"--kubeconfig=" + ts.cfg.ClientConfig.KubeconfigPath,
			"--namespace=" + ts.cfg.Namespace,
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
			if cond.Status != core_v1.ConditionTrue {
				continue
			}
			statusType = fmt.Sprintf("%s", cond.Type)
			status = fmt.Sprintf("%s", cond.Status)
			if cond.Type == core_v1.PodInitialized || cond.Type == core_v1.PodReady {
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
				zap.Int64("total-nodes", totalNodes),
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
				zap.Int64("total-nodes", totalNodes),
				zap.String("pod-name", pod.Name),
				zap.String("app-name", appName),
				zap.String("status-type", statusType),
				zap.String("status", status),
			)
		}
	}
	ts.cfg.Logger.Info("checking cw agent pods",
		zap.Int64("current-ready-pods", readyPods),
		zap.Int64("target-ready-pods", targetPods),
		zap.Int64("total-nodes", totalNodes),
	)
	if readyPods < targetPods {
		return errors.New("not enough cw agent pods ready")
	}

	return nil
}

func int32Ref(v int32) *int32 {
	return &v
}

func int64Ref(v int64) *int64 {
	return &v
}
