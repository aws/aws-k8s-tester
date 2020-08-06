package eksconfig

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// ClusterAutoscalerSpec defines the spec for the Addon
type ClusterAutoscalerSpec struct {
	Image          string                      `json:"image,omitempty"`
	CloudProvider  CloudProvider               `json:"cloudProvider,omitempty"`
	MinNodes       int                         `json:"minNodes,omitempty"`
	MaxNodes       int                         `json:"maxNodes,omitempty"`
	Resources      corev1.ResourceRequirements `json:"resources,omitempty"`
	ScaleDownDelay string                      `json:"scaleDownDelay,omitempty"`
}

// CloudProvider enum for ClusterAutoscaler
type CloudProvider string

// CloudProvider options
const (
	CloudProviderKubemark CloudProvider = "kubemark"
	CloudProviderAWS      CloudProvider = "aws"
)

// ClusterAutoscalerStatus defines the status for the Addon
type ClusterAutoscalerStatus struct {
	AddonStatus `json:",inline"`
}

// Validate installs the addon
func (spec *ClusterAutoscalerSpec) Validate(cfg *Config) error {
	return nil
}

// Default installs the addon
func (spec *ClusterAutoscalerSpec) Default(cfg *Config) {
	if spec.CloudProvider == "" {
		spec.CloudProvider = CloudProviderKubemark
	}
	if spec.MinNodes == 0 {
		spec.MinNodes = 1
	}
	if spec.MaxNodes == 0 {
		spec.MaxNodes = 100
	}
	if spec.Resources.Requests == nil {
		spec.Resources.Requests = corev1.ResourceList{}
	}
	if spec.Resources.Limits == nil {
		spec.Resources.Limits = corev1.ResourceList{}
	}
	if spec.Resources.Limits.Cpu().IsZero() {
		spec.Resources.Limits[corev1.ResourceCPU] = resource.MustParse("200M")
	}
	if spec.Resources.Limits.Memory().IsZero() {
		spec.Resources.Limits[corev1.ResourceMemory] = resource.MustParse("1Gi")
	}
	if spec.Resources.Requests.Cpu().IsZero() {
		spec.Resources.Requests[corev1.ResourceCPU] = resource.MustParse("200M")
	}
	if spec.Resources.Requests.Memory().IsZero() {
		spec.Resources.Requests[corev1.ResourceMemory] = resource.MustParse("1Gi")
	}
	if spec.ScaleDownDelay == "" {
		spec.ScaleDownDelay = "30s"
	}
	spec.Image = spec.defaultImage()
}

func (spec *ClusterAutoscalerSpec) defaultImage() string {
	if spec.Image != "" {
		return spec.Image
	}
	if spec.CloudProvider == CloudProviderKubemark {
		return "197575167141.dkr.ecr.us-west-2.amazonaws.com/cluster-autoscaler-kubemark:latest"
	}
	return "k8s.gcr.io/cluster-autoscaler:v1.14.7"
}
