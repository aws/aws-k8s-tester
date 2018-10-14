package ingress

import (
	"errors"
	"fmt"

	gyaml "github.com/ghodss/yaml"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ConfigNginx defines Nginx deployment and service configuration.
type ConfigNginx struct {
	// Replicas is the number of pods to deploy.
	Replicas int
	// Namespace is the name space to deploy ingress-test to.
	// If empty, defaults to the "default" namespace.
	Namespace string
}

// CreateDeploymentServiceNginx generates deployment and service for Nginx.
func CreateDeploymentServiceNginx(cfg ConfigNginx) (string, error) {
	if cfg.Replicas == 0 {
		return "", errors.New("zero Replicas")
	}

	oneV := intstr.FromInt(1)
	dp := v1beta1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions/v1beta1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-deployment",
			Namespace: cfg.Namespace,
			Labels: map[string]string{
				"app": "nginx",
			},
		},
		Spec: v1beta1.DeploymentSpec{
			Replicas: newInt32(cfg.Replicas),

			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx",
				},
			},

			Strategy: v1beta1.DeploymentStrategy{
				Type: v1beta1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &v1beta1.RollingUpdateDeployment{
					MaxSurge:       &oneV,
					MaxUnavailable: &oneV,
				},
			},

			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "nginx",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:            "nginx",
							Image:           "nginx",
							ImagePullPolicy: v1.PullAlways,
							Ports: []v1.ContainerPort{
								{
									ContainerPort: 80,
									Protocol:      v1.ProtocolTCP,
								},
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "nginx-default-conf-volume",
									MountPath: "/etc/nginx/conf.d",
									ReadOnly:  true,
								},
								{
									Name:      "nginx-index-html-volume",
									MountPath: "/usr/share/nginx/html",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "nginx-default-conf-volume",
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: "nginx-default-conf",
									},
								},
							},
						},
						{
							Name: "nginx-index-html-volume",
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: "nginx-index-html",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	svc := v1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-service",
			Namespace: cfg.Namespace,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(80),
					Protocol:   v1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": "nginx",
			},
			// builds on ClusterIP and allocates a port on every node
			Type: v1.ServiceTypeNodePort,
		},
	}

	d1, err := gyaml.Marshal(dp)
	if err != nil {
		return "", err
	}
	d2, err := gyaml.Marshal(svc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`---
%s


---
%s


`, string(d1), string(d2)), nil
}
