package eksconfig

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-k8s-tester/pkg/metrics"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnConfigMapsLocal defines parameters for EKS cluster
// add-on "ConfigMap" local.
// It generates loads from the local host machine.
// Every object is written serially with no concurrency.
// Use remote tester to write with concurrency.
// The main use case is to write large objects to fill up etcd database.
type AddOnConfigMapsLocal struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`

	// Objects is the number of "ConfigMap" objects to create.
	Objects int `json:"objects"`
	// ObjectSize is the "ConfigMap" value size in bytes.
	ObjectSize int `json:"object-size"`

	// CreatedNames is the list of created "ConfigMap" object names.
	CreatedNames []string `json:"created-names" read-only:"true"`

	// RequestsSummaryWrites is the writes results.
	RequestsSummaryWrites metrics.RequestsSummary `json:"requests-summary-writes,omitempty" read-only:"true"`
	// RequestsSummaryWritesJSONPath is the file path to store writes requests summary in JSON format.
	RequestsSummaryWritesJSONPath string `json:"requests-summary-writes-json-path" read-only:"true"`
	// RequestsSummaryWritesTablePath is the file path to store writes requests summary in table format.
	RequestsSummaryWritesTablePath string `json:"requests-summary-writes-table-path" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnConfigMapsLocal is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnConfigMapsLocal = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_CONFIG_MAPS_LOCAL_"

// IsEnabledAddOnConfigMapsLocal returns true if "AddOnConfigMapsLocal" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnConfigMapsLocal() bool {
	if cfg.AddOnConfigMapsLocal == nil {
		return false
	}
	if cfg.AddOnConfigMapsLocal.Enable {
		return true
	}
	cfg.AddOnConfigMapsLocal = nil
	return false
}

func getDefaultAddOnConfigMapsLocal() *AddOnConfigMapsLocal {
	return &AddOnConfigMapsLocal{
		Enable:     false,
		Objects:    10,
		ObjectSize: 10 * 1024, // 10 KB

		// writes total 300 MB data to etcd
		// Objects: 1000,
		// ObjectSize: 300000, // 0.3 MB
	}
}

func (cfg *Config) validateAddOnConfigMapsLocal() error {
	if !cfg.IsEnabledAddOnConfigMapsLocal() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnConfigMapsLocal.Enable true but no node group is enabled")
	}
	if cfg.AddOnConfigMapsLocal.Namespace == "" {
		cfg.AddOnConfigMapsLocal.Namespace = cfg.Name + "-config-maps-local"
	}

	if cfg.AddOnConfigMapsLocal.Objects == 0 {
		cfg.AddOnConfigMapsLocal.Objects = 10
	}
	if cfg.AddOnConfigMapsLocal.ObjectSize == 0 {
		cfg.AddOnConfigMapsLocal.ObjectSize = 10 * 1024
	}
	if cfg.AddOnConfigMapsLocal.ObjectSize > 900000 {
		return fmt.Errorf("AddOnConfigMapsLocal.ObjectSize limit is 0.9 MB, got %d", cfg.AddOnConfigMapsLocal.ObjectSize)
	}

	if cfg.AddOnConfigMapsLocal.RequestsSummaryWritesJSONPath == "" {
		cfg.AddOnConfigMapsLocal.RequestsSummaryWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-config-maps-local-requests-summary-writes.json"
	}
	if cfg.AddOnConfigMapsLocal.RequestsSummaryWritesTablePath == "" {
		cfg.AddOnConfigMapsLocal.RequestsSummaryWritesTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-config-maps-local-requests-summary-writes.txt"
	}

	return nil
}
