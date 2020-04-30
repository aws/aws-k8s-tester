package eksconfig

import "time"

// IsEnabledAddOnSecrets returns true if "AddOnSecrets" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnSecrets() bool {
	if cfg.AddOnSecrets == nil {
		return false
	}
	if cfg.AddOnSecrets.Enable {
		return true
	}
	cfg.AddOnSecrets = nil
	return false
}

// AddOnSecrets defines parameters for EKS cluster
// add-on "Secrets".
type AddOnSecrets struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created bool `json:"created" read-only:"true"`

	// CreateTook is the duration that took to create the resource.
	CreateTook time.Duration `json:"create-took,omitempty" read-only:"true"`
	// CreateTookString is the duration that took to create the resource.
	CreateTookString string `json:"create-took-string,omitempty" read-only:"true"`
	// DeleteTook is the duration that took to create the resource.
	DeleteTook time.Duration `json:"delete-took,omitempty" read-only:"true"`
	// DeleteTookString is the duration that took to create the resource.
	DeleteTookString string `json:"delete-took-string,omitempty" read-only:"true"`

	// Namespace is the namespace to create "Secret" and "Pod" objects in.
	Namespace string `json:"namespace"`

	// Objects is the number of "Secret" objects to write/read.
	Objects int `json:"objects"`
	// Size is the "Secret" value size in bytes.
	Size int `json:"size"`

	// CreatedSecretsNames is the list of created "Secret" object names.
	CreatedSecretsNames []string `json:"created-secrets-names" read-only:"true"`
	// CreatedPodNames is the list of created "Pod" object names.
	CreatedPodNames []string `json:"created-pod-names" read-only:"true"`

	// WritesResultPath is the CSV file path to output Secret writes test results.
	WritesResultPath string `json:"writes-result-path"`
	// ReadsResultPath is the CSV file path to output Secret reads test results.
	ReadsResultPath string `json:"reads-result-path"`
}
