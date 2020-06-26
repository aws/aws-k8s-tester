package eksconfig

import (
	"errors"

	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnJobsPi defines parameters for EKS cluster
// add-on Job with Perl.
type AddOnJobsPi struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`

	// Completes is the desired number of successfully finished pods.
	Completes int `json:"completes"`
	// Parallels is the the maximum desired number of pods the
	// job should run at any given time.
	Parallels int `json:"parallels"`
}

// EnvironmentVariablePrefixAddOnJobsPi is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnJobsPi = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_JOBS_PI_"

// IsEnabledAddOnJobsPi returns true if "AddOnJobsPi" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnJobsPi() bool {
	if cfg.AddOnJobsPi == nil {
		return false
	}
	if cfg.AddOnJobsPi.Enable {
		return true
	}
	cfg.AddOnJobsPi = nil
	return false
}

func getDefaultAddOnJobsPi() *AddOnJobsPi {
	return &AddOnJobsPi{
		Enable:    false,
		Completes: 10,
		Parallels: 10,
	}
}

func (cfg *Config) validateAddOnJobsPi() error {
	if !cfg.IsEnabledAddOnJobsPi() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnJobsPi.Enable true but no node group is enabled")
	}
	if cfg.AddOnJobsPi.Namespace == "" {
		cfg.AddOnJobsPi.Namespace = cfg.Name + "-jobs-pi"
	}
	return nil
}
