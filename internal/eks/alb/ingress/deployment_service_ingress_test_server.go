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

// ConfigDeploymentServiceIngressTestServer defines ingress-test-server deployment and service configuration.
type ConfigDeploymentServiceIngressTestServer struct {
	// Name is used for metadata name and for service selector.
	Name string
	// ServiceName is the service name to route to.
	ServiceName string
	// Namespace is the name space to deploy ingress-test to.
	// If empty, defaults to the "default" namespace.
	Namespace string
	// Image is the ingress-test docker image.
	Image string
	// Replicas is the number of pods to deploy.
	Replicas int
	// Routes is the number of routes to generate.
	Routes int
	// ResponseSize is the server response size.
	ResponseSize int
}

// CreateDeploymentServiceIngressTestServer generates deployment and service for ALB Ingress Controller.
// Reference https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#ObjectMeta.
func CreateDeploymentServiceIngressTestServer(cfg ConfigDeploymentServiceIngressTestServer) (string, error) {
	if cfg.Name == "" {
		return "", errors.New("empty Name")
	}
	if cfg.ServiceName == "" {
		return "", errors.New("empty ServiceName")
	}
	if cfg.Namespace == "" {
		return "", errors.New("empty Namespace")
	}
	if cfg.Image == "" {
		return "", errors.New("empty Image")
	}
	if cfg.Replicas == 0 {
		return "", errors.New("zero Replicas")
	}
	if cfg.Routes == 0 {
		return "", errors.New("zero Routes")
	}

	oneV := intstr.FromInt(1)
	dp := v1beta1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions/v1beta1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Name,
			Namespace: cfg.Namespace,
			Labels: map[string]string{
				"app": cfg.Name,
			},
		},
		Spec: v1beta1.DeploymentSpec{
			Replicas: newInt32(cfg.Replicas),

			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": cfg.Name,
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
						"app": cfg.Name,
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:            cfg.Name,
							Image:           cfg.Image,
							ImagePullPolicy: v1.PullAlways,
							Args: []string{
								"awstester",
								"eks",
								"ingress",
								"server",
								"--port=:32030",
								fmt.Sprintf("--routes=%d", cfg.Routes),
								fmt.Sprintf("--response-size=%d", cfg.ResponseSize),
							},
							Ports: []v1.ContainerPort{
								{
									ContainerPort: 32030, // use default
									Protocol:      v1.ProtocolTCP,
								},
							},
							ReadinessProbe: &v1.Probe{
								Handler: v1.Handler{
									TCPSocket: &v1.TCPSocketAction{
										Port: intstr.FromInt(32030),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       20,
								TimeoutSeconds:      30,
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
			Name:      cfg.ServiceName,
			Namespace: cfg.Namespace,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:       "ingress-test-server-web",
					Port:       80,
					TargetPort: intstr.FromInt(32030),
					Protocol:   v1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": cfg.Name,
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
