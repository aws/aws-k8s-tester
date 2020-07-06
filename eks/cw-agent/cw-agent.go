package cwagent

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
func (ts *tester) createCWAgentServiceAccount() error {
	ts.cfg.Logger.Info("creating cw agent ServiceAccount")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnCWAgent.Namespace).
		Create(
			ctx,
			&v1.ServiceAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      cwAgentServiceAccountName,
					Namespace: ts.cfg.EKSConfig.AddOnCWAgent.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": cwAgentAppName,
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create cw agent ServiceAccount (%v)", err)
	}

	ts.cfg.Logger.Info("created cw agent ServiceAccount")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteCWAgentServiceAccount() error {
	ts.cfg.Logger.Info("deleting cw agent ServiceAccount")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnCWAgent.Namespace).
		Delete(
			ctx,
			cwAgentServiceAccountName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete cw agent ServiceAccount (%v)", err)
	}
	ts.cfg.Logger.Info("deleted cw agent ServiceAccount", zap.Error(err))

	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createCWAgentRBACClusterRole() error {
	ts.cfg.Logger.Info("creating cw agent RBAC ClusterRole")
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
					Name:      cwAgentRBACRoleName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": cwAgentAppName,
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
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create cw agent RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("created cw agent RBAC ClusterRole")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteCWAgentRBACClusterRole() error {
	ts.cfg.Logger.Info("deleting cw agent RBAC ClusterRole")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoles().
		Delete(
			ctx,
			cwAgentRBACRoleName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete cw agent RBAC ClusterRole (%v)", err)
	}

	ts.cfg.Logger.Info("deleted cw agent RBAC ClusterRole", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) createCWAgentRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("creating cw agent RBAC ClusterRoleBinding")
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
					Name:      cwAgentRBACClusterRoleBindingName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/name": cwAgentAppName,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     cwAgentRBACRoleName,
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup:  "",
						Kind:      "ServiceAccount",
						Name:      cwAgentServiceAccountName,
						Namespace: ts.cfg.EKSConfig.AddOnCWAgent.Namespace,
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create cw agent RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("created cw agent RBAC ClusterRoleBinding")
	return ts.cfg.EKSConfig.Sync()
}

// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func (ts *tester) deleteCWAgentRBACClusterRoleBinding() error {
	ts.cfg.Logger.Info("deleting cw agent RBAC ClusterRoleBinding")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		RbacV1().
		ClusterRoleBindings().
		Delete(
			ctx,
			cwAgentRBACClusterRoleBindingName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete cw agent RBAC ClusterRoleBinding (%v)", err)
	}

	ts.cfg.Logger.Info("deleted cw agent RBAC ClusterRoleBinding", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
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

func (ts *tester) createCWAgentConfigMapConfig() (err error) {
	ts.cfg.Logger.Info("creating cw agent ConfigMap config")

	buf := bytes.NewBuffer(nil)
	cwConf := templateCWAgentConf{
		RegionName:  ts.cfg.EKSConfig.Region,
		ClusterName: ts.cfg.EKSConfig.Name,
	}
	cwConfTmpl := template.Must(template.New("TemplateCWAgentConf").Parse(TemplateCWAgentConf))
	if err := cwConfTmpl.Execute(buf, cwConf); err != nil {
		return err
	}
	cwConfBody := buf.String()
	buf.Reset()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ConfigMaps(ts.cfg.EKSConfig.AddOnCWAgent.Namespace).
		Create(
			ctx,
			&v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      cwAgentConfigMapNameConfig,
					Namespace: ts.cfg.EKSConfig.AddOnCWAgent.Namespace,
					Labels: map[string]string{
						"name": cwAgentConfigMapNameConfig,
					},
				},
				Data: map[string]string{
					cwAgentConfigMapFileNameConfig: cwConfBody,
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("created cw agent ConfigMap config")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteCWAgentConfigMapConfig() error {
	ts.cfg.Logger.Info("deleting cw agent ConfigMap config")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ConfigMaps(ts.cfg.EKSConfig.AddOnCWAgent.Namespace).
		Delete(
			ctx,
			cwAgentConfigMapNameConfig,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("deleted cw agent ConfigMap config")
	return ts.cfg.EKSConfig.Sync()
}

// CWAgentImageName is the image name of CloudWatch agent daemon set.
// ref. https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-setup-logs.html
// ref. https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-setup-EKS-quickstart.html
// ref. https://hub.docker.com/r/amazon/cloudwatch-agent
const CWAgentImageName = "amazon/cloudwatch-agent:1.245315.0"

func (ts *tester) createCWAgentDaemonSet() (err error) {
	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app.kubernetes.io/name": cwAgentAppName,
			},
		},
		Spec: v1.PodSpec{
			ServiceAccountName:            cwAgentServiceAccountName,
			TerminationGracePeriodSeconds: aws.Int64(60),
			// Unsupported value: "OnFailure": supported values: "Always"
			RestartPolicy: v1.RestartPolicyAlways,

			// https://www.eksworkshop.com/intermediate/230_logging/deploy/
			Containers: []v1.Container{
				{
					Name:            cwAgentAppName,
					Image:           CWAgentImageName,
					ImagePullPolicy: v1.PullAlways,

					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("200m"),
							v1.ResourceMemory: resource.MustParse("200Mi"),
						},
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("200m"),
							v1.ResourceMemory: resource.MustParse("200Mi"),
						},
					},

					Env: []v1.EnvVar{
						{
							Name: "HOST_IP",
							ValueFrom: &v1.EnvVarSource{
								FieldRef: &v1.ObjectFieldSelector{
									FieldPath: "status.hostIP",
								},
							},
						},
						{
							Name: "HOST_NAME",
							ValueFrom: &v1.EnvVarSource{
								FieldRef: &v1.ObjectFieldSelector{
									FieldPath: "spec.nodeName",
								},
							},
						},
						{
							Name: "K8S_NAMESPACE",
							ValueFrom: &v1.EnvVarSource{
								FieldRef: &v1.ObjectFieldSelector{
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
					VolumeMounts: []v1.VolumeMount{
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
			Volumes: []v1.Volume{
				{
					Name: cwAgentConfigMapNameConfig,
					VolumeSource: v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: cwAgentConfigMapNameConfig,
							},
							DefaultMode: aws.Int32(0666),
						},
					},
				},
				{
					Name: "rootfs",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/",
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
					Name: "varlibdocker",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/var/lib/docker",
						},
					},
				},
				{
					Name: "sys",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/sys",
						},
					},
				},
				{
					Name: "devdisk",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
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

	dsObj := appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "DaemonSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cwAgentDaemonSetName,
			Namespace: ts.cfg.EKSConfig.AddOnCWAgent.Namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": cwAgentAppName,
				},
			},

			Template: podSpec,
		},
	}

	ts.cfg.Logger.Info("creating cw agent DaemonSet", zap.String("name", cwAgentDaemonSetName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		DaemonSets(ts.cfg.EKSConfig.AddOnCWAgent.Namespace).
		Create(ctx, &dsObj, metav1.CreateOptions{})
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create cw agent DaemonSet (%v)", err)
	}

	ts.cfg.Logger.Info("created cw agent DaemonSet")
	return nil
}

func (ts *tester) deleteCWAgentDaemonSet() (err error) {
	foreground := metav1.DeletePropagationForeground
	ts.cfg.Logger.Info("deleting cw agent DaemonSet", zap.String("name", cwAgentDaemonSetName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err = ts.cfg.
		K8SClient.KubernetesClientSet().
		AppsV1().
		DaemonSets(ts.cfg.EKSConfig.AddOnCWAgent.Namespace).
		Delete(
			ctx,
			cwAgentDaemonSetName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete cw agent DaemonSet", zap.Error(err))
		return fmt.Errorf("failed to delete cw agent DaemonSet (%v)", err)
	}
	return nil
}

func (ts *tester) checkCWAgentPods() (err error) {
	waitDur := 10 * time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("check aborted")
		case <-time.After(15 * time.Second):
		}
		if err = ts._checkCWAgentPods(); err == nil {
			break
		}
		ts.cfg.Logger.Info("failed to check cw agent pods; retrying", zap.Error(err))
	}
	return err
}

func (ts *tester) _checkCWAgentPods() error {
	pods, err := ts.cfg.K8SClient.ListPods(ts.cfg.EKSConfig.AddOnCWAgent.Namespace, 150, 5*time.Second)
	if err != nil {
		ts.cfg.Logger.Warn("listing pods failed", zap.Error(err))
		return err
	}
	if len(pods) > 0 {
		ts.cfg.Logger.Info("pods found", zap.Int("pods", len(pods)))
		fmt.Fprintf(ts.cfg.LogWriter, "\n")
		for _, pod := range pods {
			fmt.Fprintf(ts.cfg.LogWriter, "%q Pod using client-go: %q\n", ts.cfg.EKSConfig.AddOnCWAgent.Namespace, pod.Name)
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n")
	} else {
		ts.cfg.Logger.Info("no pod found", zap.String("namespace", ts.cfg.EKSConfig.AddOnCWAgent.Namespace))
		return errors.New("no pod found in " + ts.cfg.EKSConfig.AddOnCWAgent.Namespace)
	}

	targetNodes := int64(1)
	if ts.cfg.EKSConfig.TotalNodes > 1 {
		targetNodes = ts.cfg.EKSConfig.TotalNodes / int64(2)
	}
	ts.cfg.Logger.Info("checking cw agent pods",
		zap.Int64("total-nodes", ts.cfg.EKSConfig.TotalNodes),
		zap.Int64("target-nodes", targetNodes),
	)
	readyNodes := int64(0)
	for _, pod := range pods {
		appName, ok := pod.Labels["app.kubernetes.io/name"]
		if !ok || appName != cwAgentAppName {
			ts.cfg.Logger.Info("skipping pod, not cw agent", zap.String("labels", fmt.Sprintf("%+v", pod.Labels)))
			continue
		}

		descArgsPods := []string{
			ts.cfg.EKSConfig.KubectlPath,
			"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
			"--namespace=" + ts.cfg.EKSConfig.AddOnCWAgent.Namespace,
			"describe",
			"pods/" + pod.Name,
		}
		descCmdPods := strings.Join(descArgsPods, " ")

		logArgs := []string{
			ts.cfg.EKSConfig.KubectlPath,
			"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
			"--namespace=" + ts.cfg.EKSConfig.AddOnCWAgent.Namespace,
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

		ready := false
		for _, cond := range pod.Status.Conditions {
			if cond.Status != v1.ConditionTrue {
				continue
			}
			ts.cfg.Logger.Info("pod",
				zap.String("name", pod.GetName()),
				zap.String("status-type", fmt.Sprintf("%s", cond.Type)),
				zap.String("status", fmt.Sprintf("%s", cond.Status)),
			)
			if cond.Type == v1.PodInitialized || cond.Type == v1.PodReady {
				ready = true
				readyNodes++
			}
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
	ts.cfg.Logger.Info("checking cw agent pods",
		zap.Int64("total-nodes", ts.cfg.EKSConfig.TotalNodes),
		zap.Int64("target-nodes", targetNodes),
		zap.Int64("ready-nodes", readyNodes),
	)
	if readyNodes < targetNodes {
		return errors.New("not enough cw agent pods ready")
	}

	return nil
}
