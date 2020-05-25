package eksconfig

import (
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/metrics"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnStresserLocal defines parameters for EKS cluster
// add-on cluster loader local.
// It generates loads from the local host machine.
// Every request for write/read is sent serially with no concurrency.
// Use remote tester to write and read with concurrency.
type AddOnStresserLocal struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`

	// ObjectSize is the value size in bytes for write objects.
	ObjectSize int `json:"object-size"`
	// Duration is the duration to run load testing.
	// The cluster loader waits "one" "Duration" for hollow ones.
	// And other one for cluster loader.
	Duration       time.Duration `json:"duration,omitempty"`
	DurationString string        `json:"duration-string,omitempty" read-only:"true"`

	// RequestsSummaryWrites is the writes results.
	RequestsSummaryWrites metrics.RequestsSummary `json:"requests-summary-writes,omitempty" read-only:"true"`
	// RequestsSummaryWritesJSONPath is the file path to store writes requests summary in JSON format.
	RequestsSummaryWritesJSONPath string `json:"requests-summary-writes-json-path" read-only:"true"`
	// RequestsSummaryWritesTablePath is the file path to store writes requests summary in table format.
	RequestsSummaryWritesTablePath string `json:"requests-summary-writes-table-path" read-only:"true"`
	// RequestsSummaryReads is the reads results.
	RequestsSummaryReads metrics.RequestsSummary `json:"requests-summary-reads,omitempty" read-only:"true"`
	// RequestsSummaryReadsJSONPath is the file path to store reads requests summary in JSON format.
	RequestsSummaryReadsJSONPath string `json:"requests-summary-reads-json-path" read-only:"true"`
	// RequestsSummaryReadsTablePath is the file path to store reads requests summary in table format.
	RequestsSummaryReadsTablePath string `json:"requests-summary-reads-table-path" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnStresserLocal is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnStresserLocal = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_STRESSER_LOCAL_"

// IsEnabledAddOnStresserLocal returns true if "AddOnStresserLocal" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnStresserLocal() bool {
	if cfg.AddOnStresserLocal == nil {
		return false
	}
	if cfg.AddOnStresserLocal.Enable {
		return true
	}
	cfg.AddOnStresserLocal = nil
	return false
}

func getDefaultAddOnStresserLocal() *AddOnStresserLocal {
	return &AddOnStresserLocal{
		Enable:     false,
		ObjectSize: 0,
		Duration:   time.Minute,
	}
}

func (cfg *Config) validateAddOnStresserLocal() error {
	if !cfg.IsEnabledAddOnStresserLocal() {
		return nil
	}

	if cfg.AddOnStresserLocal.Namespace == "" {
		cfg.AddOnStresserLocal.Namespace = cfg.Name + "-stresser-local"
	}

	if cfg.AddOnStresserLocal.Duration == time.Duration(0) {
		cfg.AddOnStresserLocal.Duration = time.Minute
	}
	cfg.AddOnStresserLocal.DurationString = cfg.AddOnStresserLocal.Duration.String()

	if cfg.AddOnStresserLocal.RequestsSummaryWritesJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsSummaryWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-summary-writes.json"
	}
	if cfg.AddOnStresserLocal.RequestsSummaryWritesTablePath == "" {
		cfg.AddOnStresserLocal.RequestsSummaryWritesTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-summary-writes.txt"
	}
	if cfg.AddOnStresserLocal.RequestsSummaryReadsJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsSummaryReadsJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-summary-reads.json"
	}
	if cfg.AddOnStresserLocal.RequestsSummaryReadsTablePath == "" {
		cfg.AddOnStresserLocal.RequestsSummaryReadsTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-summary-reads.txt"
	}

	return nil
}
