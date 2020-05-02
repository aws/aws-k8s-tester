package eksconfig

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"
)

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

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`

	// Objects is the number of "Secret" objects to write/read.
	Objects int `json:"objects"`
	// Size is the "Secret" value size in bytes.
	Size int `json:"size"`

	// FailThreshold is the number of write failures to allow.
	FailThreshold int `json:"fail-threshold"`

	// CreatedSecretsNames is the list of created "Secret" object names.
	CreatedSecretsNames []string `json:"created-secrets-names" read-only:"true"`
	// CreatedPodNames is the list of created "Pod" object names.
	CreatedPodNames []string `json:"created-pod-names" read-only:"true"`

	// WritesResultPath is the CSV file path to output Secret writes test results.
	WritesResultPath string `json:"writes-result-path"`
	// ReadsResultPath is the CSV file path to output Secret reads test results.
	ReadsResultPath string `json:"reads-result-path"`
}

// EnvironmentVariablePrefixAddOnSecrets is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnSecrets = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_SECRETS_"

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

func (cfg *Config) validateAddOnSecrets() error {
	if !cfg.IsEnabledAddOnSecrets() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnSecrets.Enable true but no node group is enabled")
	}
	if cfg.AddOnSecrets.Namespace == "" {
		cfg.AddOnSecrets.Namespace = cfg.Name + "-secrets"
	}
	if cfg.AddOnSecrets.WritesResultPath == "" {
		cfg.AddOnSecrets.WritesResultPath = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-secret-writes.csv")
	}
	if filepath.Ext(cfg.AddOnSecrets.WritesResultPath) != ".csv" {
		return fmt.Errorf("expected .csv extension for WritesResultPath, got %q", cfg.AddOnSecrets.WritesResultPath)
	}
	if cfg.AddOnSecrets.ReadsResultPath == "" {
		cfg.AddOnSecrets.ReadsResultPath = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-secret-reads.csv")
	}
	if filepath.Ext(cfg.AddOnSecrets.ReadsResultPath) != ".csv" {
		return fmt.Errorf("expected .csv extension for ReadsResultPath, got %q", cfg.AddOnSecrets.ReadsResultPath)
	}
	return nil
}
