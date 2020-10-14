package eksconfig

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnClusterVersionUpgrade defines parameters
// for EKS cluster version upgrade add-on.
type AddOnClusterVersionUpgrade struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`

	// WaitBeforeUpgrade is the wait duration before it starts cluster version upgrade.
	WaitBeforeUpgrade       time.Duration `json:"wait-before-upgrade"`
	WaitBeforeUpgradeString string        `json:"wait-before-upgrade-string" read-only:"true"`

	// Version is the target version of EKS Kubernetes "cluster".
	// If empty, set default version.
	Version      string  `json:"version"`
	VersionValue float64 `json:"version-value" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnClusterVersionUpgrade is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnClusterVersionUpgrade = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_CLUSTER_VERSION_UPGRADE_"

// IsEnabledAddOnClusterVersionUpgrade returns true if "AddOnClusterVersionUpgrade" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnClusterVersionUpgrade() bool {
	if cfg.AddOnClusterVersionUpgrade == nil {
		return false
	}
	if cfg.AddOnClusterVersionUpgrade.Enable {
		return true
	}
	cfg.AddOnClusterVersionUpgrade = nil
	return false
}

func getDefaultAddOnClusterVersionUpgrade() *AddOnClusterVersionUpgrade {
	return &AddOnClusterVersionUpgrade{
		Enable:            false,
		Version:           "1.18",
		WaitBeforeUpgrade: 3 * time.Minute,
	}
}

func (cfg *Config) validateAddOnClusterVersionUpgrade() error {
	if !cfg.IsEnabledAddOnClusterVersionUpgrade() {
		return nil
	}

	cfg.AddOnClusterVersionUpgrade.WaitBeforeUpgradeString = cfg.AddOnClusterVersionUpgrade.WaitBeforeUpgrade.String()

	if cfg.AddOnClusterVersionUpgrade.Version == "" {
		return errors.New("empty AddOnClusterVersionUpgrade.Version")
	}
	var err error
	cfg.AddOnClusterVersionUpgrade.VersionValue, err = strconv.ParseFloat(cfg.AddOnClusterVersionUpgrade.Version, 64)
	if err != nil {
		return fmt.Errorf("cannot parse AddOnClusterVersionUpgrade.Version %q (%v)", cfg.Parameters.Version, err)
	}
	delta := cfg.AddOnClusterVersionUpgrade.VersionValue - cfg.Parameters.VersionValue
	if fmt.Sprintf("%.2f", delta) != "0.01" {
		return fmt.Errorf("AddOnClusterVersionUpgrade only supports one minor version upgrade but got %.2f [invalid: %q -> %q]", delta, cfg.Parameters.Version, cfg.AddOnClusterVersionUpgrade.Version)
	}

	return nil
}
