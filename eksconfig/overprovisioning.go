package eksconfig

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// OverprovisioningSpec defines the spec for the Addon
type OverprovisioningSpec struct {
	Namespace       string                      `json:"namespace,omitempty"`
	Image           string                      `json:"image,omitempty"`
	Replicas        int                         `json:"replicas,omitempty"`
	Resources       corev1.ResourceRequirements `json:"resources,omitempty"`
	KubemarkEnabled bool                        `json:"kubemarkEnabled,omitempty"`
}

// OverprovisioningStatus defines the status for the Addon
type OverprovisioningStatus struct {
	AddonStatus `json:",inline"`
}

// Validate installs the addon
func (spec *OverprovisioningSpec) Validate(cfg *Config) error {
	return nil
}

// Default installs the addon
func (spec *OverprovisioningSpec) Default(cfg *Config) {
	if cfg.AddOnHollowNodesRemote.Enable {
		spec.KubemarkEnabled = true
	}
	if spec.Namespace == "" {
		spec.Namespace = "overprovisioning"
	}
	if spec.Image == "" {
		spec.Image = "k8s.gcr.io/pause"
	}
	if spec.Resources.Requests.Cpu() == nil {
		spec.Resources = corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("400m")}}
	}
	if spec.Resources.Requests.Memory() == nil {
		spec.Resources = corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")}}
	}
}
