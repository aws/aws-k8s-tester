package eksconfig

import (
	"errors"
	"path"
	"path/filepath"
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

	// S3Dir is the S3 directory to store all test results.
	// It is under the bucket "eksconfig.Config.S3BucketName".
	S3Dir string `json:"s3-dir"`

	// RequestsWritesRawJSONPath is the file path to store writes requests in JSON format.
	RequestsWritesRawJSONPath  string `json:"requests-writes-json-path" read-only:"true"`
	RequestsWritesRawJSONS3Key string `json:"requests-writes-json-s3-key" read-only:"true"`
	// RequestsWritesSummary is the writes results.
	RequestsWritesSummary metrics.RequestsSummary `json:"requests-writes-summary,omitempty" read-only:"true"`
	// RequestsWritesSummaryJSONPath is the file path to store writes requests summary in JSON format.
	RequestsWritesSummaryJSONPath  string `json:"requests-writes-summary-json-path" read-only:"true"`
	RequestsWritesSummaryJSONS3Key string `json:"requests-writes-summary-json-s3-key" read-only:"true"`
	// RequestsWritesSummaryTablePath is the file path to store writes requests summary in table format.
	RequestsWritesSummaryTablePath  string `json:"requests-writes-summary-table-path" read-only:"true"`
	RequestsWritesSummaryTableS3Key string `json:"requests-writes-summary-table-s3-path" read-only:"true"`
	// RequestsWritesCompareS3Dir is the S3 directory of previous/latest "RequestsWritesSummary".
	// Specify the S3 key in the same bucket of "eksconfig.Config.S3BucketName".
	// Use for regression tests. Specify the value not bound to the cluster directory.
	// Different runs from different clusters reads and writes in this directory.
	RequestsWritesCompareS3Dir string `json:"requests-writes-compare-s3-dir"`
	// RequestsWritesCompare is the comparision results.
	RequestsWritesCompare metrics.RequestsCompare `json:"requests-writes-compare" read-only:"true"`
	// RequestsWritesCompareJSONPath is the file path to store writes requests compare summary in JSON format.
	RequestsWritesCompareJSONPath  string `json:"requests-writes-compare-json-path" read-only:"true"`
	RequestsWritesCompareJSONS3Key string `json:"requests-writes-compare-json-s3-key" read-only:"true"`
	// RequestsWritesCompareTablePath is the file path to store writes requests compare summary in table format.
	RequestsWritesCompareTablePath  string `json:"requests-writes-compare-table-path" read-only:"true"`
	RequestsWritesCompareTableS3Key string `json:"requests-writes-compare-table-s3-path" read-only:"true"`

	// RequestsReadsRawJSONPath is the file path to store reads requests in JSON format.
	RequestsReadsRawJSONPath  string `json:"requests-reads-raw-json-path" read-only:"true"`
	RequestsReadsRawJSONS3Key string `json:"requests-reads-raw-json-s3-key" read-only:"true"`
	// RequestsReadsSummary is the reads results.
	RequestsReadsSummary metrics.RequestsSummary `json:"requests-reads-summary,omitempty" read-only:"true"`
	// RequestsReadsSummaryJSONPath is the file path to store reads requests summary in JSON format.
	RequestsReadsSummaryJSONPath  string `json:"requests-reads-summary-json-path" read-only:"true"`
	RequestsReadsSummaryJSONS3Key string `json:"requests-reads-summary-json-s3-key" read-only:"true"`
	// RequestsReadsSummaryTablePath is the file path to store reads requests summary in table format.
	RequestsReadsSummaryTablePath  string `json:"requests-reads-summary-table-path" read-only:"true"`
	RequestsReadsSummaryTableS3Key string `json:"requests-reads-summary-table-s3-path" read-only:"true"`
	// RequestsReadsCompareS3Dir is the S3 directory of previous/latest "RequestsReadsSummary".
	// Specify the S3 key in the same bucket of "eksconfig.Config.S3BucketName".
	// Use for regression tests. Specify the value not bound to the cluster directory.
	// Different runs from different clusters reads and writes in this directory.
	RequestsReadsCompareS3Dir string `json:"requests-reads-compare-s3-dir"`
	// RequestsReadsCompare is the comparision results.
	RequestsReadsCompare metrics.RequestsCompare `json:"requests-reads-compare" read-only:"true"`
	// RequestsReadsCompareJSONPath is the file path to store reads requests compare summary in JSON format.
	RequestsReadsCompareJSONPath  string `json:"requests-reads-compare-json-path" read-only:"true"`
	RequestsReadsCompareJSONS3Key string `json:"requests-reads-compare-json-s3-key" read-only:"true"`
	// RequestsReadsCompareTablePath is the file path to store reads requests compare summary in table format.
	RequestsReadsCompareTablePath  string `json:"requests-reads-compare-table-path" read-only:"true"`
	RequestsReadsCompareTableS3Key string `json:"requests-reads-compare-table-s3-path" read-only:"true"`
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

	if cfg.AddOnStresserLocal.S3Dir == "" {
		cfg.AddOnStresserLocal.S3Dir = path.Join(cfg.Name, "add-on-stresser-local")
	}

	if cfg.AddOnStresserLocal.RequestsWritesRawJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsWritesRawJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-writes-raw.json"
	}
	if cfg.AddOnStresserLocal.RequestsWritesRawJSONS3Key == "" {
		cfg.AddOnStresserLocal.RequestsWritesRawJSONS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"writes-raw",
			filepath.Base(cfg.AddOnStresserLocal.RequestsWritesRawJSONPath),
		)
	}
	if cfg.AddOnStresserLocal.RequestsWritesSummaryJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsWritesSummaryJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-writes-summary.json"
	}
	if cfg.AddOnStresserLocal.RequestsWritesSummaryJSONS3Key == "" {
		cfg.AddOnStresserLocal.RequestsWritesSummaryJSONS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"writes-summary",
			filepath.Base(cfg.AddOnStresserLocal.RequestsWritesSummaryJSONPath),
		)
	}
	if cfg.AddOnStresserLocal.RequestsWritesSummaryTablePath == "" {
		cfg.AddOnStresserLocal.RequestsWritesSummaryTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-writes-summary.txt"
	}
	if cfg.AddOnStresserLocal.RequestsWritesSummaryTableS3Key == "" {
		cfg.AddOnStresserLocal.RequestsWritesSummaryTableS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"writes-summary",
			filepath.Base(cfg.AddOnStresserLocal.RequestsWritesSummaryTablePath),
		)
	}
	if cfg.AddOnStresserLocal.RequestsWritesCompareS3Dir == "" {
		cfg.AddOnStresserLocal.RequestsWritesCompareS3Dir = path.Join("add-on-stresser-local", "writes-compare", cfg.Parameters.Version)
	}
	if cfg.AddOnStresserLocal.RequestsWritesCompareJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsWritesCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-writes-compare.json"
	}
	if cfg.AddOnStresserLocal.RequestsWritesCompareJSONS3Key == "" {
		cfg.AddOnStresserLocal.RequestsWritesCompareJSONS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"writes-compare",
			filepath.Base(cfg.AddOnStresserLocal.RequestsWritesCompareJSONPath),
		)
	}
	if cfg.AddOnStresserLocal.RequestsWritesCompareTablePath == "" {
		cfg.AddOnStresserLocal.RequestsWritesCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-writes-compare.txt"
	}
	if cfg.AddOnStresserLocal.RequestsWritesCompareTableS3Key == "" {
		cfg.AddOnStresserLocal.RequestsWritesCompareTableS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"writes-compare",
			filepath.Base(cfg.AddOnStresserLocal.RequestsWritesCompareTablePath),
		)
	}

	if cfg.AddOnStresserLocal.RequestsReadsRawJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsReadsRawJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-reads-raw.json"
	}
	if cfg.AddOnStresserLocal.RequestsReadsRawJSONS3Key == "" {
		cfg.AddOnStresserLocal.RequestsReadsRawJSONS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"reads-raw",
			filepath.Base(cfg.AddOnStresserLocal.RequestsReadsRawJSONPath),
		)
	}
	if cfg.AddOnStresserLocal.RequestsReadsSummaryJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsReadsSummaryJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-reads-summary.json"
	}
	if cfg.AddOnStresserLocal.RequestsReadsSummaryJSONS3Key == "" {
		cfg.AddOnStresserLocal.RequestsReadsSummaryJSONS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"reads-summary",
			filepath.Base(cfg.AddOnStresserLocal.RequestsReadsSummaryJSONPath),
		)
	}
	if cfg.AddOnStresserLocal.RequestsReadsSummaryTablePath == "" {
		cfg.AddOnStresserLocal.RequestsReadsSummaryTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-reads-summary.txt"
	}
	if cfg.AddOnStresserLocal.RequestsReadsSummaryTableS3Key == "" {
		cfg.AddOnStresserLocal.RequestsReadsSummaryTableS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"reads-summary",
			filepath.Base(cfg.AddOnStresserLocal.RequestsReadsSummaryTablePath),
		)
	}
	if cfg.AddOnStresserLocal.RequestsReadsCompareS3Dir == "" {
		cfg.AddOnStresserLocal.RequestsReadsCompareS3Dir = path.Join("add-on-stresser-local", "reads-compare", cfg.Parameters.Version)
	}
	if cfg.AddOnStresserLocal.RequestsReadsCompareJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsReadsCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-reads-compare.json"
	}
	if cfg.AddOnStresserLocal.RequestsReadsCompareJSONS3Key == "" {
		cfg.AddOnStresserLocal.RequestsReadsCompareJSONS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"reads-compare",
			filepath.Base(cfg.AddOnStresserLocal.RequestsReadsCompareJSONPath),
		)
	}
	if cfg.AddOnStresserLocal.RequestsReadsCompareTablePath == "" {
		cfg.AddOnStresserLocal.RequestsReadsCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-reads-compare.txt"
	}
	if cfg.AddOnStresserLocal.RequestsReadsCompareTableS3Key == "" {
		cfg.AddOnStresserLocal.RequestsReadsCompareTableS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"reads-compare",
			filepath.Base(cfg.AddOnStresserLocal.RequestsReadsCompareTablePath),
		)
	}

	return nil
}
