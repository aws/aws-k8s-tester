package eksconfig

import (
	"errors"
	"path"
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
	// ListLimit is the maximum number of items in the list call.
	// Sets "metav1.ListOptions".Limit field.
	// 0 to list all.
	ListLimit int64 `json:"list-limit"`
	// Duration is the duration to run load testing.
	Duration       time.Duration `json:"duration,omitempty"`
	DurationString string        `json:"duration-string,omitempty" read-only:"true"`

	// RequestsWritesJSONPath is the file path to store writes requests in JSON format.
	RequestsWritesJSONPath string `json:"requests-writes-json-path" read-only:"true"`
	// RequestsWritesSummary is the writes results.
	RequestsWritesSummary metrics.RequestsSummary `json:"requests-writes-summary,omitempty" read-only:"true"`
	// RequestsWritesSummaryJSONPath is the file path to store writes requests summary in JSON format.
	RequestsWritesSummaryJSONPath string `json:"requests-writes-summary-json-path" read-only:"true"`
	// RequestsWritesSummaryTablePath is the file path to store writes requests summary in table format.
	RequestsWritesSummaryTablePath string `json:"requests-writes-summary-table-path" read-only:"true"`
	// RequestsWritesSummaryS3Dir is the S3 directory of previous/latest "RequestsWritesSummary".
	// Specify the S3 key in the same bucket of "eksconfig.Config.S3BucketName".
	// Use for regression tests.
	RequestsWritesSummaryS3Dir string `json:"requests-writes-summary-s3-dir"`
	// RequestsWritesSummaryCompare is the comparision results.
	RequestsWritesSummaryCompare metrics.RequestsSummaryCompare `json:"requests-writes-summary-compare" read-only:"true"`
	// RequestsWritesSummaryCompareJSONPath is the file path to store writes requests compare summary in JSON format.
	RequestsWritesSummaryCompareJSONPath string `json:"requests-writes-summary-compare-json-path" read-only:"true"`
	// RequestsWritesSummaryCompareTablePath is the file path to store writes requests compare summary in table format.
	RequestsWritesSummaryCompareTablePath string `json:"requests-writes-summary-compare-table-path" read-only:"true"`

	// RequestsReadsJSONPath is the file path to store reads requests in JSON format.
	RequestsReadsJSONPath string `json:"requests-reads-json-path" read-only:"true"`
	// RequestsReadsSummary is the reads results.
	RequestsReadsSummary metrics.RequestsSummary `json:"requests-reads-summary,omitempty" read-only:"true"`
	// RequestsReadsSummaryJSONPath is the file path to store reads requests summary in JSON format.
	RequestsReadsSummaryJSONPath string `json:"requests-reads-summary-json-path" read-only:"true"`
	// RequestsReadsSummaryTablePath is the file path to store reads requests summary in table format.
	RequestsReadsSummaryTablePath string `json:"requests-reads-summary-table-path" read-only:"true"`
	// RequestsReadsSummaryS3Dir is the S3 directory of previous/latest "RequestsReadsSummary".
	// Specify the S3 key in the same bucket of "eksconfig.Config.S3BucketName".
	// Use for regression tests.
	RequestsReadsSummaryS3Dir string `json:"requests-reads-summary-s3-dir"`
	// RequestsReadsSummaryCompare is the comparision results.
	RequestsReadsSummaryCompare metrics.RequestsSummaryCompare `json:"requests-reads-summary-compare" read-only:"true"`
	// RequestsReadsSummaryCompareJSONPath is the file path to store reads requests compare summary in JSON format.
	RequestsReadsSummaryCompareJSONPath string `json:"requests-reads-summary-compare-json-path" read-only:"true"`
	// RequestsReadsSummaryCompareTablePath is the file path to store reads requests compare summary in table format.
	RequestsReadsSummaryCompareTablePath string `json:"requests-reads-summary-compare-table-path" read-only:"true"`
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
		ListLimit:  0,
		Duration:   time.Minute,
	}
}

func (cfg *Config) validateAddOnStresserLocal() error {
	if !cfg.IsEnabledAddOnStresserLocal() {
		return nil
	}
	if cfg.S3BucketName == "" {
		return errors.New("AddOnStresserLocal requires S3 bucket for collecting results but S3BucketName empty")
	}

	if cfg.AddOnStresserLocal.Namespace == "" {
		cfg.AddOnStresserLocal.Namespace = cfg.Name + "-stresser-local"
	}

	if cfg.AddOnStresserLocal.Duration == time.Duration(0) {
		cfg.AddOnStresserLocal.Duration = time.Minute
	}
	cfg.AddOnStresserLocal.DurationString = cfg.AddOnStresserLocal.Duration.String()

	if cfg.AddOnStresserLocal.RequestsWritesJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-writes.json"
	}
	if cfg.AddOnStresserLocal.RequestsWritesSummaryJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsWritesSummaryJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-writes-summary.json"
	}
	if cfg.AddOnStresserLocal.RequestsWritesSummaryTablePath == "" {
		cfg.AddOnStresserLocal.RequestsWritesSummaryTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-writes-summary.txt"
	}
	if cfg.AddOnStresserLocal.RequestsWritesSummaryS3Dir == "" {
		cfg.AddOnStresserLocal.RequestsWritesSummaryS3Dir = path.Join("add-on-stresser-local", "writes-summary", cfg.Parameters.Version)
	}
	if cfg.AddOnStresserLocal.RequestsWritesSummaryCompareJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsWritesSummaryCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-writes-summary-compare.json"
	}
	if cfg.AddOnStresserLocal.RequestsWritesSummaryCompareTablePath == "" {
		cfg.AddOnStresserLocal.RequestsWritesSummaryCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-writes-summary-compare.txt"
	}

	if cfg.AddOnStresserLocal.RequestsReadsJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsReadsJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-reads.json"
	}
	if cfg.AddOnStresserLocal.RequestsReadsSummaryJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsReadsSummaryJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-reads-summary.json"
	}
	if cfg.AddOnStresserLocal.RequestsReadsSummaryTablePath == "" {
		cfg.AddOnStresserLocal.RequestsReadsSummaryTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-reads-summary.txt"
	}
	if cfg.AddOnStresserLocal.RequestsReadsSummaryS3Dir == "" {
		cfg.AddOnStresserLocal.RequestsReadsSummaryS3Dir = path.Join("add-on-stresser-local", "reads-summary", cfg.Parameters.Version)
	}
	if cfg.AddOnStresserLocal.RequestsReadsSummaryCompareJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsReadsSummaryCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-reads-summary-compare.json"
	}
	if cfg.AddOnStresserLocal.RequestsReadsSummaryCompareTablePath == "" {
		cfg.AddOnStresserLocal.RequestsReadsSummaryCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-reads-summary-compare.txt"
	}

	return nil
}
