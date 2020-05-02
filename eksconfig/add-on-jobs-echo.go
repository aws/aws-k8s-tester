package eksconfig

import (
	"errors"
	"fmt"
	"time"
)

// AddOnJobsEcho defines parameters for EKS cluster
// add-on Job with echo.
type AddOnJobsEcho struct {
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

	// Completes is the desired number of successfully finished pods.
	Completes int `json:"completes"`
	// Parallels is the the maximum desired number of pods the
	// job should run at any given time.
	Parallels int `json:"parallels"`
	// EchoSize is the job object size in bytes.
	// "Request entity too large: limit is 3145728" (3.1 MB).
	// "The Job "echo" is invalid: metadata.annotations:
	// Too long: must have at most 262144 characters". (0.26 MB)
	EchoSize int `json:"echo-size"`
}

// EnvironmentVariablePrefixAddOnJobsEcho is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnJobsEcho = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_JOBS_ECHO_"

// IsEnabledAddOnJobsEcho returns true if "AddOnJobsEcho" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnJobsEcho() bool {
	if cfg.AddOnJobsEcho == nil {
		return false
	}
	if cfg.AddOnJobsEcho.Enable {
		return true
	}
	cfg.AddOnJobsEcho = nil
	return false
}

func (cfg *Config) validateAddOnJobsEcho() error {
	if !cfg.IsEnabledAddOnJobsEcho() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnJobsEcho.Enable true but no node group is enabled")
	}
	if cfg.AddOnJobsEcho.Namespace == "" {
		cfg.AddOnJobsEcho.Namespace = cfg.Name + "-job-echo"
	}
	if cfg.AddOnJobsEcho.EchoSize > 250000 {
		return fmt.Errorf("echo size limit is 0.25 MB, got %d", cfg.AddOnJobsEcho.EchoSize)
	}
	return nil
}
