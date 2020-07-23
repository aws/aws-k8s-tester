package eksconfig

// AddonStatus contains shared status fields for all addons
type AddonStatus struct {
	// Installed is true after the configuration has been applied
	Installed bool `json:"installed"`
	// Ready is true after the addon's health check has passed
	Ready bool `json:"ready"`
}
