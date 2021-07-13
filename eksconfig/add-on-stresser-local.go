package eksconfig

import (
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

	// S3Dir is the S3 directory to store all test results.
	// It is under the bucket "eksconfig.Config.S3BucketName".
	S3Dir string `json:"s3-dir"`

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

	//////////////////////////////////////////////////////////////////////////////

	RequestsRawWritesJSONPath  string `json:"requests-raw-writes-json-path" read-only:"true"`
	RequestsRawWritesJSONS3Key string `json:"requests-raw-writes-json-s3-key" read-only:"true"`

	//////////////////////////////////////////////////////////////////////////////

	// RequestsRawWritesCompareS3Dir is the s3 directory to store raw data points.
	// Used to comparison results.
	// ref. https://en.wikipedia.org/wiki/Kolmogorov%E2%80%93Smirnov_test
	RequestsRawWritesCompareS3Dir        string `json:"requests-raw-writes-compare-s3-dir"`
	RequestsRawWritesCompareAllJSONPath  string `json:"requests-raw-writes-compare-all-json-path" read-only:"true"`
	RequestsRawWritesCompareAllJSONS3Key string `json:"requests-raw-writes-compare-all-json-s3-key" read-only:"true"`
	RequestsRawWritesCompareAllCSVPath   string `json:"requests-raw-writes-compare-all-csv-path" read-only:"true"`
	RequestsRawWritesCompareAllCSVS3Key  string `json:"requests-raw-writes-compare-all-csv-s3-key" read-only:"true"`

	//////////////////////////////////////////////////////////////////////////////

	// RequestsSummaryWrites is the writes results.
	RequestsSummaryWrites           metrics.RequestsSummary `json:"requests-summary-writes,omitempty" read-only:"true"`
	RequestsSummaryWritesJSONPath   string                  `json:"requests-summary-writes-json-path" read-only:"true"`
	RequestsSummaryWritesJSONS3Key  string                  `json:"requests-summary-writes-json-s3-key" read-only:"true"`
	RequestsSummaryWritesTablePath  string                  `json:"requests-summary-writes-table-path" read-only:"true"`
	RequestsSummaryWritesTableS3Key string                  `json:"requests-summary-writes-table-s3-path" read-only:"true"`

	//////////////////////////////////////////////////////////////////////////////

	// RequestsSummaryWritesCompareS3Dir is the S3 directory of previous/latest "RequestsSummary".
	// Specify the S3 key in the same bucket of "eksconfig.Config.S3BucketName".
	// Use for regression tests. Specify the value not bound to the cluster directory.
	// Different runs from different clusters reads and writes in this directory.
	RequestsSummaryWritesCompareS3Dir      string                  `json:"requests-summary-writes-compare-s3-dir"`
	RequestsSummaryWritesCompare           metrics.RequestsCompare `json:"requests-summary-writes-compare" read-only:"true"`
	RequestsSummaryWritesCompareJSONPath   string                  `json:"requests-summary-writes-compare-json-path" read-only:"true"`
	RequestsSummaryWritesCompareJSONS3Key  string                  `json:"requests-summary-writes-compare-json-s3-key" read-only:"true"`
	RequestsSummaryWritesCompareTablePath  string                  `json:"requests-summary-writes-compare-table-path" read-only:"true"`
	RequestsSummaryWritesCompareTableS3Key string                  `json:"requests-summary-writes-compare-table-s3-path" read-only:"true"`

	//////////////////////////////////////////////////////////////////////////////

	RequestsRawReadsJSONPath  string `json:"requests-raw-reads-json-path" read-only:"true"`
	RequestsRawReadsJSONS3Key string `json:"requests-raw-reads-json-s3-key" read-only:"true"`

	//////////////////////////////////////////////////////////////////////////////

	// RequestsRawReadsCompareS3Dir is the s3 directory to store raw data points.
	// Used to comparison results.
	// ref. https://en.wikipedia.org/wiki/Kolmogorov%E2%80%93Smirnov_test
	RequestsRawReadsCompareS3Dir        string `json:"requests-raw-reads-compare-s3-dir"`
	RequestsRawReadsCompareAllJSONPath  string `json:"requests-raw-reads-compare-all-json-path" read-only:"true"`
	RequestsRawReadsCompareAllJSONS3Key string `json:"requests-raw-reads-compare-all-json-s3-key" read-only:"true"`
	RequestsRawReadsCompareAllCSVPath   string `json:"requests-raw-reads-compare-all-csv-path" read-only:"true"`
	RequestsRawReadsCompareAllCSVS3Key  string `json:"requests-raw-reads-compare-all-csv-s3-key" read-only:"true"`

	//////////////////////////////////////////////////////////////////////////////

	// RequestsSummaryReads is the reads results.
	RequestsSummaryReads           metrics.RequestsSummary `json:"requests-summary-reads,omitempty" read-only:"true"`
	RequestsSummaryReadsJSONPath   string                  `json:"requests-summary-reads-json-path" read-only:"true"`
	RequestsSummaryReadsJSONS3Key  string                  `json:"requests-summary-reads-json-s3-key" read-only:"true"`
	RequestsSummaryReadsTablePath  string                  `json:"requests-summary-reads-table-path" read-only:"true"`
	RequestsSummaryReadsTableS3Key string                  `json:"requests-summary-reads-table-s3-path" read-only:"true"`

	//////////////////////////////////////////////////////////////////////////////

	// RequestsSummaryReadsCompareS3Dir is the S3 directory of previous/latest "RequestsSummary".
	// Specify the S3 key in the same bucket of "eksconfig.Config.S3BucketName".
	// Use for regression tests. Specify the value not bound to the cluster directory.
	// Different runs from different clusters reads and writes in this directory.
	RequestsSummaryReadsCompareS3Dir      string                  `json:"requests-summary-reads-compare-s3-dir"`
	RequestsSummaryReadsCompare           metrics.RequestsCompare `json:"requests-summary-reads-compare" read-only:"true"`
	RequestsSummaryReadsCompareJSONPath   string                  `json:"requests-summary-reads-compare-json-path" read-only:"true"`
	RequestsSummaryReadsCompareJSONS3Key  string                  `json:"requests-summary-reads-compare-json-s3-key" read-only:"true"`
	RequestsSummaryReadsCompareTablePath  string                  `json:"requests-summary-reads-compare-table-path" read-only:"true"`
	RequestsSummaryReadsCompareTableS3Key string                  `json:"requests-summary-reads-compare-table-s3-path" read-only:"true"`

	//////////////////////////////////////////////////////////////////////////////
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

	if cfg.AddOnStresserLocal.S3Dir == "" {
		cfg.AddOnStresserLocal.S3Dir = path.Join(cfg.Name, "add-on-stresser-local")
	}

	if cfg.AddOnStresserLocal.Namespace == "" {
		cfg.AddOnStresserLocal.Namespace = cfg.Name + "-stresser-local"
	}

	if cfg.AddOnStresserLocal.Duration == time.Duration(0) {
		cfg.AddOnStresserLocal.Duration = time.Minute
	}
	cfg.AddOnStresserLocal.DurationString = cfg.AddOnStresserLocal.Duration.String()

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnStresserLocal.RequestsRawWritesJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsRawWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-writes-raw.json"
	}
	if cfg.AddOnStresserLocal.RequestsRawWritesJSONS3Key == "" {
		cfg.AddOnStresserLocal.RequestsRawWritesJSONS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"requests-raw-writes",
			filepath.Base(cfg.AddOnStresserLocal.RequestsRawWritesJSONPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnStresserLocal.RequestsRawWritesCompareS3Dir == "" {
		cfg.AddOnStresserLocal.RequestsRawWritesCompareS3Dir = path.Join("add-on-stresser-local", "requests-raw-writes-compare", cfg.Version)
	}
	if cfg.AddOnStresserLocal.RequestsRawWritesCompareAllJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsRawWritesCompareAllJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-raw-writes-compare-all.json"
	}
	if cfg.AddOnStresserLocal.RequestsRawWritesCompareAllJSONS3Key == "" {
		cfg.AddOnStresserLocal.RequestsRawWritesCompareAllJSONS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"requests-raw-writes-compare-all",
			filepath.Base(cfg.AddOnStresserLocal.RequestsRawWritesCompareAllJSONPath),
		)
	}
	if cfg.AddOnStresserLocal.RequestsRawWritesCompareAllCSVPath == "" {
		cfg.AddOnStresserLocal.RequestsRawWritesCompareAllCSVPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-raw-writes-compare-all.csv"
	}
	if cfg.AddOnStresserLocal.RequestsRawWritesCompareAllCSVS3Key == "" {
		cfg.AddOnStresserLocal.RequestsRawWritesCompareAllCSVS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"requests-raw-writes-compare-all",
			filepath.Base(cfg.AddOnStresserLocal.RequestsRawWritesCompareAllCSVPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnStresserLocal.RequestsSummaryWritesJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsSummaryWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-summary-writes.json"
	}
	if cfg.AddOnStresserLocal.RequestsSummaryWritesJSONS3Key == "" {
		cfg.AddOnStresserLocal.RequestsSummaryWritesJSONS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"requests-summary-writes",
			filepath.Base(cfg.AddOnStresserLocal.RequestsSummaryWritesJSONPath),
		)
	}
	if cfg.AddOnStresserLocal.RequestsSummaryWritesTablePath == "" {
		cfg.AddOnStresserLocal.RequestsSummaryWritesTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-summary-writes.txt"
	}
	if cfg.AddOnStresserLocal.RequestsSummaryWritesTableS3Key == "" {
		cfg.AddOnStresserLocal.RequestsSummaryWritesTableS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"requests-summary-writes",
			filepath.Base(cfg.AddOnStresserLocal.RequestsSummaryWritesTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnStresserLocal.RequestsSummaryWritesCompareS3Dir == "" {
		cfg.AddOnStresserLocal.RequestsSummaryWritesCompareS3Dir = path.Join("add-on-stresser-local", "requests-summary-writes-compare", cfg.Version)
	}
	if cfg.AddOnStresserLocal.RequestsSummaryWritesCompareJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsSummaryWritesCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-summary-writes-compare.json"
	}
	if cfg.AddOnStresserLocal.RequestsSummaryWritesCompareJSONS3Key == "" {
		cfg.AddOnStresserLocal.RequestsSummaryWritesCompareJSONS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"requests-summary-writes-compare",
			filepath.Base(cfg.AddOnStresserLocal.RequestsSummaryWritesCompareJSONPath),
		)
	}
	if cfg.AddOnStresserLocal.RequestsSummaryWritesCompareTablePath == "" {
		cfg.AddOnStresserLocal.RequestsSummaryWritesCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-summary-writes-compare.txt"
	}
	if cfg.AddOnStresserLocal.RequestsSummaryWritesCompareTableS3Key == "" {
		cfg.AddOnStresserLocal.RequestsSummaryWritesCompareTableS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"requests-summary-writes-compare",
			filepath.Base(cfg.AddOnStresserLocal.RequestsSummaryWritesCompareTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnStresserLocal.RequestsRawReadsJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsRawReadsJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-raw-reads.json"
	}
	if cfg.AddOnStresserLocal.RequestsRawReadsJSONS3Key == "" {
		cfg.AddOnStresserLocal.RequestsRawReadsJSONS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"requests-raw-reads",
			filepath.Base(cfg.AddOnStresserLocal.RequestsRawReadsJSONPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnStresserLocal.RequestsRawReadsCompareS3Dir == "" {
		cfg.AddOnStresserLocal.RequestsRawReadsCompareS3Dir = path.Join("add-on-stresser-local", "requests-raw-reads-compare", cfg.Version)
	}
	if cfg.AddOnStresserLocal.RequestsRawReadsCompareAllJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsRawReadsCompareAllJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-raw-reads-compare-all.json"
	}
	if cfg.AddOnStresserLocal.RequestsRawReadsCompareAllJSONS3Key == "" {
		cfg.AddOnStresserLocal.RequestsRawReadsCompareAllJSONS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"requests-raw-reads-compare-all",
			filepath.Base(cfg.AddOnStresserLocal.RequestsRawReadsCompareAllJSONPath),
		)
	}
	if cfg.AddOnStresserLocal.RequestsRawReadsCompareAllCSVPath == "" {
		cfg.AddOnStresserLocal.RequestsRawReadsCompareAllCSVPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-raw-reads-compare-all.csv"
	}
	if cfg.AddOnStresserLocal.RequestsRawReadsCompareAllCSVS3Key == "" {
		cfg.AddOnStresserLocal.RequestsRawReadsCompareAllCSVS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"requests-raw-reads-compare-all",
			filepath.Base(cfg.AddOnStresserLocal.RequestsRawReadsCompareAllCSVPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnStresserLocal.RequestsSummaryReadsJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsSummaryReadsJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-summary-reads.json"
	}
	if cfg.AddOnStresserLocal.RequestsSummaryReadsJSONS3Key == "" {
		cfg.AddOnStresserLocal.RequestsSummaryReadsJSONS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"requests-summary-reads",
			filepath.Base(cfg.AddOnStresserLocal.RequestsSummaryReadsJSONPath),
		)
	}
	if cfg.AddOnStresserLocal.RequestsSummaryReadsTablePath == "" {
		cfg.AddOnStresserLocal.RequestsSummaryReadsTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-summary-reads.txt"
	}
	if cfg.AddOnStresserLocal.RequestsSummaryReadsTableS3Key == "" {
		cfg.AddOnStresserLocal.RequestsSummaryReadsTableS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"requests-summary-reads",
			filepath.Base(cfg.AddOnStresserLocal.RequestsSummaryReadsTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnStresserLocal.RequestsSummaryReadsCompareS3Dir == "" {
		cfg.AddOnStresserLocal.RequestsSummaryReadsCompareS3Dir = path.Join("add-on-stresser-local", "requests-summary-reads-compare", cfg.Version)
	}
	if cfg.AddOnStresserLocal.RequestsSummaryReadsCompareJSONPath == "" {
		cfg.AddOnStresserLocal.RequestsSummaryReadsCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-summary-reads-compare.json"
	}
	if cfg.AddOnStresserLocal.RequestsSummaryReadsCompareJSONS3Key == "" {
		cfg.AddOnStresserLocal.RequestsSummaryReadsCompareJSONS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"requests-summary-reads-compare",
			filepath.Base(cfg.AddOnStresserLocal.RequestsSummaryReadsCompareJSONPath),
		)
	}
	if cfg.AddOnStresserLocal.RequestsSummaryReadsCompareTablePath == "" {
		cfg.AddOnStresserLocal.RequestsSummaryReadsCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-local-requests-summary-reads-compare.txt"
	}
	if cfg.AddOnStresserLocal.RequestsSummaryReadsCompareTableS3Key == "" {
		cfg.AddOnStresserLocal.RequestsSummaryReadsCompareTableS3Key = path.Join(
			cfg.AddOnStresserLocal.S3Dir,
			"requests-summary-reads-compare",
			filepath.Base(cfg.AddOnStresserLocal.RequestsSummaryReadsCompareTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	return nil
}
