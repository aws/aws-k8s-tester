package eksconfig

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-k8s-tester/pkg/metrics"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnConfigmapsLocal defines parameters for EKS cluster
// add-on "ConfigMap" local.
// It generates loads from the local host machine.
// Every object is written serially with no concurrency.
// Use remote tester to write with concurrency.
// The main use case is to write large objects to fill up etcd database.
type AddOnConfigmapsLocal struct {
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

	// RequestsWritesJSONPath is the file path to store writes requests in JSON format.
	RequestsWritesJSONPath string `json:"requests-writes-json-path" read-only:"true"`
	// RequestsWritesSummary is the writes results.
	RequestsWritesSummary metrics.RequestsSummary `json:"requests-writes-summary,omitempty" read-only:"true"`
	// RequestsWritesSummaryJSONPath is the file path to store writes requests summary in JSON format.
	RequestsWritesSummaryJSONPath string `json:"requests-writes-summary-json-path" read-only:"true"`
	// RequestsWritesSummaryTablePath is the file path to store writes requests summary in table format.
	RequestsWritesSummaryTablePath string `json:"requests-writes-summary-table-path" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnConfigmapsLocal is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnConfigmapsLocal = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_CONFIGMAPS_LOCAL_"

// IsEnabledAddOnConfigmapsLocal returns true if "AddOnConfigmapsLocal" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnConfigmapsLocal() bool {
	if cfg.AddOnConfigmapsLocal == nil {
		return false
	}
	if cfg.AddOnConfigmapsLocal.Enable {
		return true
	}
	cfg.AddOnConfigmapsLocal = nil
	return false
}

func getDefaultAddOnConfigmapsLocal() *AddOnConfigmapsLocal {
	return &AddOnConfigmapsLocal{
		Enable:     false,
		Objects:    10,
		ObjectSize: 10 * 1024, // 10 KB

		// writes total 300 MB data to etcd
		// Objects: 1000,
		// ObjectSize: 300000, // 0.3 MB
	}
}

func (cfg *Config) validateAddOnConfigmapsLocal() error {
	if !cfg.IsEnabledAddOnConfigmapsLocal() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnConfigmapsLocal.Enable true but no node group is enabled")
	}
	if cfg.AddOnConfigmapsLocal.Namespace == "" {
		cfg.AddOnConfigmapsLocal.Namespace = cfg.Name + "-configmaps-local"
	}

	if cfg.AddOnConfigmapsLocal.Objects == 0 {
		cfg.AddOnConfigmapsLocal.Objects = 10
	}
	if cfg.AddOnConfigmapsLocal.ObjectSize == 0 {
		cfg.AddOnConfigmapsLocal.ObjectSize = 10 * 1024
	}
	if cfg.AddOnConfigmapsLocal.ObjectSize > 900000 {
		return fmt.Errorf("AddOnConfigmapsLocal.ObjectSize limit is 0.9 MB, got %d", cfg.AddOnConfigmapsLocal.ObjectSize)
	}

	if cfg.AddOnConfigmapsLocal.RequestsWritesJSONPath == "" {
		cfg.AddOnConfigmapsLocal.RequestsWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-local-requests-writes.csv"
	}
	if cfg.AddOnConfigmapsLocal.RequestsWritesSummaryJSONPath == "" {
		cfg.AddOnConfigmapsLocal.RequestsWritesSummaryJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-local-requests-writes-summary.json"
	}
	if cfg.AddOnConfigmapsLocal.RequestsWritesSummaryTablePath == "" {
		cfg.AddOnConfigmapsLocal.RequestsWritesSummaryTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-local-requests-writes-summary.txt"
	}

	return nil
}
