package eksconfig

import (
	"errors"
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-k8s-tester/pkg/metrics"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnSecretsLocal defines parameters for EKS cluster
// add-on "Secrets" local.
// It generates loads from the local host machine.
// Every object is written serially with no concurrency.
// Use remote tester to write with concurrency.
// The main use case is to write a large number of objects to fill up etcd database.
// And measure latencies for secret encryption.
type AddOnSecretsLocal struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`

	// Objects is the number of "Secret" objects to write/read.
	Objects int `json:"objects"`
	// ObjectSize is the "Secret" value size in bytes.
	ObjectSize int `json:"object-size"`

	// NamePrefix is the prefix of Secret name.
	// If multiple Secret loader is running,
	// this must be unique per worker to avoid name conflicts.
	NamePrefix string `json:"name-prefix"`

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

// EnvironmentVariablePrefixAddOnSecretsLocal is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnSecretsLocal = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_SECRETS_LOCAL_"

// IsEnabledAddOnSecretsLocal returns true if "AddOnSecretsLocal" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnSecretsLocal() bool {
	if cfg.AddOnSecretsLocal == nil {
		return false
	}
	if cfg.AddOnSecretsLocal.Enable {
		return true
	}
	cfg.AddOnSecretsLocal = nil
	return false
}

func getDefaultAddOnSecretsLocal() *AddOnSecretsLocal {
	return &AddOnSecretsLocal{
		Enable:     false,
		Objects:    10,
		ObjectSize: 10 * 1024, // 10 KB

		// writes total 100 MB for "Secret" objects,
		// plus "Pod" objects, writes total 330 MB to etcd
		//
		// with 3 nodes, takes about 1.5 hour for all
		// these "Pod"s to complete
		//
		// Objects: 10000,
		// ObjectSize: 10 * 1024, // 10 KB

		NamePrefix: "secret" + randutil.String(5),
	}
}

func (cfg *Config) validateAddOnSecretsLocal() error {
	if !cfg.IsEnabledAddOnSecretsLocal() {
		return nil
	}
	if cfg.S3BucketName == "" {
		return errors.New("AddOnSecretsLocal requires S3 bucket for collecting results but S3BucketName empty")
	}

	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnSecretsLocal.Enable true but no node group is enabled")
	}
	if cfg.AddOnSecretsLocal.Namespace == "" {
		cfg.AddOnSecretsLocal.Namespace = cfg.Name + "-secrets-local"
	}

	if cfg.AddOnSecretsLocal.Objects == 0 {
		cfg.AddOnSecretsLocal.Objects = 10
	}
	if cfg.AddOnSecretsLocal.ObjectSize == 0 {
		cfg.AddOnSecretsLocal.ObjectSize = 10 * 1024
	}

	if cfg.AddOnSecretsLocal.NamePrefix == "" {
		cfg.AddOnSecretsLocal.NamePrefix = "secret" + randutil.String(5)
	}

	if cfg.AddOnSecretsLocal.S3Dir == "" {
		cfg.AddOnSecretsLocal.S3Dir = path.Join(cfg.Name, "add-on-secrets-local")
	}

	if cfg.AddOnSecretsLocal.RequestsWritesRawJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsWritesRawJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-writes-raw.json"
	}
	if cfg.AddOnSecretsLocal.RequestsWritesRawJSONS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsWritesRawJSONS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"writes-raw",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsWritesRawJSONPath),
		)
	}
	if cfg.AddOnSecretsLocal.RequestsWritesSummaryJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsWritesSummaryJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-writes-summary.json"
	}
	if cfg.AddOnSecretsLocal.RequestsWritesSummaryJSONS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsWritesSummaryJSONS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"writes-summary",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsWritesSummaryJSONPath),
		)
	}
	if cfg.AddOnSecretsLocal.RequestsWritesSummaryTablePath == "" {
		cfg.AddOnSecretsLocal.RequestsWritesSummaryTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-writes-summary.txt"
	}
	if cfg.AddOnSecretsLocal.RequestsWritesSummaryTableS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsWritesSummaryTableS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"writes-summary",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsWritesSummaryTablePath),
		)
	}
	if cfg.AddOnSecretsLocal.RequestsWritesCompareS3Dir == "" {
		cfg.AddOnSecretsLocal.RequestsWritesCompareS3Dir = path.Join("add-on-secrets-local", "writes-compare", cfg.Parameters.Version)
	}
	if cfg.AddOnSecretsLocal.RequestsWritesCompareJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsWritesCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-writes-compare.json"
	}
	if cfg.AddOnSecretsLocal.RequestsWritesCompareJSONS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsWritesCompareJSONS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"writes-compare",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsWritesCompareJSONPath),
		)
	}
	if cfg.AddOnSecretsLocal.RequestsWritesCompareTablePath == "" {
		cfg.AddOnSecretsLocal.RequestsWritesCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-writes-compare.txt"
	}
	if cfg.AddOnSecretsLocal.RequestsWritesCompareTableS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsWritesCompareTableS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"writes-compare",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsWritesCompareTablePath),
		)
	}

	if cfg.AddOnSecretsLocal.RequestsReadsRawJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsReadsRawJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-reads-raw.json"
	}
	if cfg.AddOnSecretsLocal.RequestsReadsRawJSONS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsReadsRawJSONS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"reads-raw",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsReadsRawJSONPath),
		)
	}
	if cfg.AddOnSecretsLocal.RequestsReadsSummaryJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsReadsSummaryJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-reads-summary.json"
	}
	if cfg.AddOnSecretsLocal.RequestsReadsSummaryJSONS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsReadsSummaryJSONS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"reads-summary",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsReadsSummaryJSONPath),
		)
	}
	if cfg.AddOnSecretsLocal.RequestsReadsSummaryTablePath == "" {
		cfg.AddOnSecretsLocal.RequestsReadsSummaryTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-reads-summary.txt"
	}
	if cfg.AddOnSecretsLocal.RequestsReadsSummaryTableS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsReadsSummaryTableS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"reads-summary",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsReadsSummaryTablePath),
		)
	}
	if cfg.AddOnSecretsLocal.RequestsReadsCompareS3Dir == "" {
		cfg.AddOnSecretsLocal.RequestsReadsCompareS3Dir = path.Join("add-on-secrets-local", "reads-compare", cfg.Parameters.Version)
	}
	if cfg.AddOnSecretsLocal.RequestsReadsCompareJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsReadsCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-reads-compare.json"
	}
	if cfg.AddOnSecretsLocal.RequestsReadsCompareJSONS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsReadsCompareJSONS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"reads-compare",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsReadsCompareJSONPath),
		)
	}
	if cfg.AddOnSecretsLocal.RequestsReadsCompareTablePath == "" {
		cfg.AddOnSecretsLocal.RequestsReadsCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-reads-compare.txt"
	}
	if cfg.AddOnSecretsLocal.RequestsReadsCompareTableS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsReadsCompareTableS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"reads-compare",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsReadsCompareTablePath),
		)
	}

	return nil
}
