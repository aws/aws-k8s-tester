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

// ConfigDeploymentServiceALBIngressController defines ALB Ingress Controller deployment and service configuration.
type ConfigDeploymentServiceALBIngressController struct {
	// AWSRegion is the AWS region.
	AWSRegion string
	// Name is used for metadata name and for service selector.
	Name string
	// ServiceName is the service name to route to.
	ServiceName string
	// Namespace is the name space to deploy ALB Ingress Controller to.
	// If empty, defaults to the "default" namespace.
	Namespace string
	// Image is the ALB Ingress Controller docker image.
	Image string
	// ClusterName is the EKS cluster name.
	ClusterName string
}

// CreateDeploymentServiceALBIngressController generates deployment and service for ALB Ingress Controller.
// Reference https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#ObjectMeta.
func CreateDeploymentServiceALBIngressController(cfg ConfigDeploymentServiceALBIngressController) (string, error) {
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
	if cfg.ClusterName == "" {
		return "", errors.New("empty ClusterName")
	}
	if cfg.AWSRegion == "" {
		return "", errors.New("empty Region")
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
			Replicas: newInt32(1),

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
					Volumes: []v1.Volume{
						{
							Name: "aws-cred-aws-k8s-tester",
							// VolumeSource represents the location and type of the mounted volume.
							VolumeSource: v1.VolumeSource{
								Secret: &v1.SecretVolumeSource{
									SecretName: "aws-cred-aws-k8s-tester",
								},
							},
						},
					},
					Containers: []v1.Container{
						{
							Name:            cfg.Name,
							Image:           cfg.Image,
							ImagePullPolicy: v1.PullAlways,
							Args: []string{
								"/server",
								"--ingress-class=alb",
								fmt.Sprintf("--cluster-name=%s", cfg.ClusterName),
								"--aws-max-retries=20",
								"--healthz-port=10254",
							},
							Ports: []v1.ContainerPort{
								{
									ContainerPort: 10254, // use ALB Ingress Controller's default
									Protocol:      v1.ProtocolTCP,
								},
							},
							ReadinessProbe: &v1.Probe{
								Handler: v1.Handler{
									HTTPGet: &v1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromInt(10254),
										Scheme: v1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       60,
								TimeoutSeconds:      30,
							},
							LivenessProbe: &v1.Probe{
								Handler: v1.Handler{
									HTTPGet: &v1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromInt(10254),
										Scheme: v1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       60,
								TimeoutSeconds:      30,
							},
							Env: []v1.EnvVar{
								{Name: "AWS_REGION", Value: cfg.AWSRegion},
								{Name: "AWS_DEBUG", Value: "false"},
								{
									Name: "POD_NAME",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.name",
										},
									},
								},
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.namespace",
										},
									},
								},
								{
									Name:  "AWS_SHARED_CREDENTIALS_FILE",
									Value: "/etc/aws-cred-aws-k8s-tester/aws-cred-aws-k8s-tester",
								},
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "aws-cred-aws-k8s-tester",
									MountPath: "/etc/aws-cred-aws-k8s-tester",
									ReadOnly:  true,
								},
							},
							TerminationMessagePath: "/dev/termination-log",
						},
					},
					DNSPolicy:                     v1.DNSClusterFirst,
					RestartPolicy:                 v1.RestartPolicyAlways,
					TerminationGracePeriodSeconds: newInt64(30),
					ServiceAccountName:            "alb-ingress",
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
					Name:       "metrics-alb-ingress-controller",
					Port:       80,
					TargetPort: intstr.FromInt(10254),
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

func newInt32(v int) *int32 {
	vv := int32(v)
	return &vv
}

func newInt64(v int) *int64 {
	vv := int64(v)
	return &vv
}
