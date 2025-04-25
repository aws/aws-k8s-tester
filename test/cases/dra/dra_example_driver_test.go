//go:build e2e

package dra

import (
	"context"
	_ "embed"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/internal/e2e"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

// see: https://github.com/kubernetes-sigs/dra-example-driver
func TestDraExampleDriver(t *testing.T) {
	draDriverResources := draDriverResources()
	deviceClass, resourceClaimTemplate, pod := testResources()

	exampleDraDriver := features.New("dra-example-driver").
		WithLabel("feature", "dra").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			for _, obj := range draDriverResources {
				assert.NoError(t, cfg.Client().Resources().Create(ctx, obj))
			}
			assert.NoError(t, cfg.Client().Resources().Create(ctx, &deviceClass))
			assert.NoError(t, cfg.Client().Resources().Create(ctx, &resourceClaimTemplate))
			return ctx
		}).
		Assess("device driver present", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			assert.NoError(t, cfg.Client().Resources().Create(ctx, &pod))
			defer func() {
				assert.NoError(t, cfg.Client().Resources().Delete(ctx, &pod))
				assert.NoError(t, wait.For(conditions.New(cfg.Client().Resources()).ResourceDeleted(&pod),
					wait.WithTimeout(time.Minute),
					wait.WithContext(ctx),
				))
			}()

			assert.NoError(t, wait.For(conditions.New(cfg.Client().Resources()).PodRunning(&pod),
				wait.WithTimeout(time.Minute),
				wait.WithContext(ctx),
			))

			podLogs, err := e2e.ReadPodLogs(ctx, cfg.Client().RESTConfig(), pod.Namespace, pod.Name, pod.Spec.Containers[0].Name)
			if assert.NoErrorf(t, err, "skipping error getting pod logs %q: %v", pod.Name, err) {
				t.Logf("Logs for %q\n%s", pod.Name, podLogs)
				assert.Contains(t, podLogs, fmt.Sprintf(`DRA_RESOURCE_DRIVER_NAME="%s"`, deviceClass.Name))
			}

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			assert.NoError(t, cfg.Client().Resources().Delete(ctx, &deviceClass))
			assert.NoError(t, cfg.Client().Resources().Delete(ctx, &resourceClaimTemplate))
			for _, obj := range draDriverResources {
				assert.NoError(t, cfg.Client().Resources().Delete(ctx, obj))
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, exampleDraDriver)
}

func testResources() (v1beta1.DeviceClass, v1beta1.ResourceClaimTemplate, corev1.Pod) {
	deviceClass := v1beta1.DeviceClass{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "resource.k8s.io/v1beta1",
			Kind:       "DeviceClass",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "gpu.example.com",
		},
		Spec: v1beta1.DeviceClassSpec{
			Selectors: []v1beta1.DeviceSelector{
				{
					CEL: &v1beta1.CELDeviceSelector{
						Expression: "device.driver == 'gpu.example.com'",
					},
				},
			},
		},
	}

	deviceRequest := v1beta1.DeviceRequest{
		Name:            "gpu",
		DeviceClassName: deviceClass.Name,
	}

	resourceClaimTemplate := v1beta1.ResourceClaimTemplate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "resource.k8s.io/v1beta1",
			Kind:       "ResourceClaimTemplate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "single-gpu",
			Namespace: corev1.NamespaceDefault,
		},
		Spec: v1beta1.ResourceClaimTemplateSpec{
			Spec: v1beta1.ResourceClaimSpec{
				Devices: v1beta1.DeviceClaim{
					Requests: []v1beta1.DeviceRequest{deviceRequest},
				},
			},
		},
	}

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod0",
			Namespace: corev1.NamespaceDefault,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "ctr0",
					Image:   "public.ecr.aws/amazonlinux/amazonlinux:latest",
					Command: []string{"bash", "-c"},
					Args:    []string{"export; trap 'exit 0' TERM; sleep infinity & wait"},
					Resources: corev1.ResourceRequirements{
						Claims: []corev1.ResourceClaim{
							{
								Name: deviceRequest.Name,
							},
						},
					},
				},
			},
			ResourceClaims: []corev1.PodResourceClaim{
				{
					Name:                      deviceRequest.Name,
					ResourceClaimTemplateName: &resourceClaimTemplate.Name,
				},
			},
		},
	}

	return deviceClass, resourceClaimTemplate, pod
}

func draDriverResources() []k8s.Object {
	serviceAccount := corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dra-service-account",
			Namespace: corev1.NamespaceDefault,
		},
	}

	clusterRole := rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dra-example-driver-role",
			Namespace: corev1.NamespaceDefault,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"resource.k8s.io"},
				Resources: []string{"resourceclaims"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"nodes"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{"resource.k8s.io"},
				Resources: []string{"resourceslices"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
		},
	}

	clusterRoleBinding := rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dra-example-driver-role-binding",
			Namespace: corev1.NamespaceDefault,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      serviceAccount.Kind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Name:     clusterRole.Name,
			Kind:     clusterRole.Kind,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	driverDaemonset := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dra-example-driver-kubeletplugin",
			Namespace: corev1.NamespaceDefault,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "dra-example-driver",
				"app.kubernetes.io/instance":  "dra-example-driver",
				"app.kubernetes.io/component": "kubeletplugin",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":      "dra-example-driver",
					"app.kubernetes.io/instance":  "dra-example-driver",
					"app.kubernetes.io/component": "kubeletplugin",
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":      "dra-example-driver",
						"app.kubernetes.io/instance":  "dra-example-driver",
						"app.kubernetes.io/component": "kubeletplugin",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccount.Name,
					PriorityClassName:  "system-node-critical",
					Containers: []corev1.Container{
						{
							Name:            "plugin",
							SecurityContext: &corev1.SecurityContext{Privileged: &[]bool{true}[0]},
							Image:           "registry.k8s.io/dra-example-driver/dra-example-driver:v0.1.0",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"dra-example-kubeletplugin"},
							Env: []corev1.EnvVar{
								{Name: "CDI_ROOT", Value: "/var/run/cdi"},
								{Name: "NODE_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
								{Name: "NAMESPACE", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
								// NOTE: this is what arbitrarily decides the
								// number of GPUs being mocked on the node.
								{Name: "NUM_DEVICES", Value: "8"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "plugins-registry", MountPath: "/var/lib/kubelet/plugins_registry"},
								{Name: "plugins", MountPath: "/var/lib/kubelet/plugins"},
								{Name: "cdi", MountPath: "/var/run/cdi"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{Name: "plugins-registry", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins_registry"}}},
						{Name: "plugins", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins"}}},
						{Name: "cdi", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/run/cdi"}}},
					},
				},
			},
		},
	}

	return []k8s.Object{
		&clusterRoleBinding,
		&clusterRole,
		&serviceAccount,
		&driverDaemonset,
	}
}
