package eksconfig

// ClusterLoaderSpec defines the spec for the Addon
type ClusterLoaderSpec struct {
	Image          string   `json:"image,omitempty"`
	TestConfigUris []string `json:"testConfigUris,omitempty"`
	TestParams     []string `json:"testParams,omitempty"`
	S3Uri          string   `json:"s3Uri,omitempty"`
}

// MetricsServerStatus defines the status for the Addon
type ClusterLoaderStatus struct {
	AddonStatus `json:",inline"`
}

// Validate installs the addon
func (spec *ClusterLoaderSpec) Validate(cfg *Config) error {
	return nil
}

// Default installs the addon
func (spec *ClusterLoaderSpec) Default(cfg *Config) {
}
