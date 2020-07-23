package eksconfig

// Addon is an interface for configuration initialization
type Addon interface {
	// Validate the addon against the config.
	Validate(cfg *Config) error
	// Default the addon with the config.
	Default(cfg *Config)
}
