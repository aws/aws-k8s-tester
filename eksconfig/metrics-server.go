package eksconfig

// MetricsServerSpec defines the spec for the Addon
type MetricsServerSpec struct {
}

// MetricsServerStatus defines the status for the Addon
type MetricsServerStatus struct {
	AddonStatus `json:",inline"`
}

// Validate installs the addon
func (spec *MetricsServerSpec) Validate(cfg *Config) error {
	return nil
}

// Default installs the addon
func (spec *MetricsServerSpec) Default(cfg *Config) {}
