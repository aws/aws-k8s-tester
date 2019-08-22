package resources

import (
	"fmt"
	"os"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// NewTestpodResources creates new Kubernetes resources with image exported as env TESTPOD_IMAGE_URI.
// Build the image at https://github.com/aws/amazon-vpc-cni-k8s/ with `make build-docker-testpod`
func NewTestpodResources(ns, serviceAccountName, nodeName string, replicas int32) *Resources {
	app := "testpod"
	maxUnavailable := intstr.FromInt(1)
	maxSurge := intstr.FromInt(5)

	labels := map[string]string{
		"app": "testpod",
	}

	annotations := map[string]string{
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   "8080",
	}

	affinity := &corev1.Affinity{}
	if nodeName != "" {
		affinity = &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "kubernetes.io/hostname",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{nodeName},
								},
							},
						},
					},
				},
			},
		}
		nodeName = "-" + nodeName
	}

	dp := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("testpod%s", nodeName),
			Namespace: ns,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
					Affinity:           affinity,
					Containers: []corev1.Container{
						{
							Name:  app,
							Image: os.Getenv("TESTPOD_IMAGE_URI"),
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
								},
							},
							ImagePullPolicy: corev1.PullAlways,
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt(8080),
									},
								},
							},
						},
					},
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &maxUnavailable,
					MaxSurge:       &maxSurge,
				},
			},
		},
	}

	svcs := []*corev1.Service{}

	svcClusterIP := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testpod-clusterip",
			Namespace: ns,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": app,
			},
			Ports: []corev1.ServicePort{
				{
					Port: 8080,
				},
			},
		},
	}

	svcs = append(svcs, svcClusterIP)

	svcPodIP := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testpod-pod-ip",
			Namespace: ns,
		},
		Spec: corev1.ServiceSpec{
			// Type: svcType,
			Selector: map[string]string{
				"app": app,
			},
			ClusterIP: "None",
			Ports: []corev1.ServicePort{
				{
					Port: 8080,
				},
			},
		},
	}

	svcs = append(svcs, svcPodIP)

	return &Resources{
		Deployment: dp,
		Services:   svcs,
	}
}
