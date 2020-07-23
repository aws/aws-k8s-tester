package eksconfig

// ClusterAutoscalerSpec defines the spec for the Addon
type ClusterAutoscalerSpec struct {
	Image          string        `json:"image,omitempty"`
	CloudProvider  CloudProvider `json:"cloudProvider,omitempty"`
	MinNodes       int           `json:"minNodes,omitempty"`
	MaxNodes       int           `json:"maxNodes,omitempty"`
	ScaleDownDelay string        `json:"scaleDownDelay,omitempty"`
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
		return "767520670908.dkr.ecr.us-west-2.amazonaws.com/cluster-autoscaler-kubemark:custom-build-20200727"
	}
	return "k8s.gcr.io/cluster-autoscaler:v1.14.7"
}
