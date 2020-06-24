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

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnSecretsLocal.RequestsRawWritesJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsRawWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-writes-raw.json"
	}
	if cfg.AddOnSecretsLocal.RequestsRawWritesJSONS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsRawWritesJSONS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"requests-raw-writes",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsRawWritesJSONPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnSecretsLocal.RequestsRawWritesCompareS3Dir == "" {
		cfg.AddOnSecretsLocal.RequestsRawWritesCompareS3Dir = path.Join("add-on-secrets-local", "requests-raw-writes-compare", cfg.Parameters.Version)
	}
	if cfg.AddOnSecretsLocal.RequestsRawWritesCompareAllJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsRawWritesCompareAllJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-raw-writes-compare-all.json"
	}
	if cfg.AddOnSecretsLocal.RequestsRawWritesCompareAllJSONS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsRawWritesCompareAllJSONS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"requests-raw-writes-compare-all",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsRawWritesCompareAllJSONPath),
		)
	}
	if cfg.AddOnSecretsLocal.RequestsRawWritesCompareAllCSVPath == "" {
		cfg.AddOnSecretsLocal.RequestsRawWritesCompareAllCSVPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-raw-writes-compare-all.csv"
	}
	if cfg.AddOnSecretsLocal.RequestsRawWritesCompareAllCSVS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsRawWritesCompareAllCSVS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"requests-raw-writes-compare-all",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsRawWritesCompareAllCSVPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnSecretsLocal.RequestsSummaryWritesJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-summary-writes.json"
	}
	if cfg.AddOnSecretsLocal.RequestsSummaryWritesJSONS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryWritesJSONS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"requests-summary-writes",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsSummaryWritesJSONPath),
		)
	}
	if cfg.AddOnSecretsLocal.RequestsSummaryWritesTablePath == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryWritesTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-summary-writes.txt"
	}
	if cfg.AddOnSecretsLocal.RequestsSummaryWritesTableS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryWritesTableS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"requests-summary-writes",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsSummaryWritesTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnSecretsLocal.RequestsSummaryWritesCompareS3Dir == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryWritesCompareS3Dir = path.Join("add-on-secrets-local", "requests-summary-writes-compare", cfg.Parameters.Version)
	}
	if cfg.AddOnSecretsLocal.RequestsSummaryWritesCompareJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryWritesCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-summary-writes-compare.json"
	}
	if cfg.AddOnSecretsLocal.RequestsSummaryWritesCompareJSONS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryWritesCompareJSONS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"requests-summary-writes-compare",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsSummaryWritesCompareJSONPath),
		)
	}
	if cfg.AddOnSecretsLocal.RequestsSummaryWritesCompareTablePath == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryWritesCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-summary-writes-compare.txt"
	}
	if cfg.AddOnSecretsLocal.RequestsSummaryWritesCompareTableS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryWritesCompareTableS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"requests-summary-writes-compare",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsSummaryWritesCompareTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnSecretsLocal.RequestsRawReadsJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsRawReadsJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-raw-reads.json"
	}
	if cfg.AddOnSecretsLocal.RequestsRawReadsJSONS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsRawReadsJSONS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"requests-raw-reads",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsRawReadsJSONPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnSecretsLocal.RequestsRawReadsCompareS3Dir == "" {
		cfg.AddOnSecretsLocal.RequestsRawReadsCompareS3Dir = path.Join("add-on-secrets-local", "requests-raw-reads-compare", cfg.Parameters.Version)
	}
	if cfg.AddOnSecretsLocal.RequestsRawReadsCompareAllJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsRawReadsCompareAllJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-raw-reads-compare-all.json"
	}
	if cfg.AddOnSecretsLocal.RequestsRawReadsCompareAllJSONS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsRawReadsCompareAllJSONS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"requests-raw-reads-compare-all",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsRawReadsCompareAllJSONPath),
		)
	}
	if cfg.AddOnSecretsLocal.RequestsRawReadsCompareAllCSVPath == "" {
		cfg.AddOnSecretsLocal.RequestsRawReadsCompareAllCSVPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-raw-reads-compare-all.csv"
	}
	if cfg.AddOnSecretsLocal.RequestsRawReadsCompareAllCSVS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsRawReadsCompareAllCSVS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"requests-raw-reads-compare-all",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsRawReadsCompareAllCSVPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnSecretsLocal.RequestsSummaryReadsJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryReadsJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-summary-reads.json"
	}
	if cfg.AddOnSecretsLocal.RequestsSummaryReadsJSONS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryReadsJSONS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"requests-summary-reads",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsSummaryReadsJSONPath),
		)
	}
	if cfg.AddOnSecretsLocal.RequestsSummaryReadsTablePath == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryReadsTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-summary-reads.txt"
	}
	if cfg.AddOnSecretsLocal.RequestsSummaryReadsTableS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryReadsTableS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"requests-summary-reads",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsSummaryReadsTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnSecretsLocal.RequestsSummaryReadsCompareS3Dir == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryReadsCompareS3Dir = path.Join("add-on-secrets-local", "requests-summary-reads-compare", cfg.Parameters.Version)
	}
	if cfg.AddOnSecretsLocal.RequestsSummaryReadsCompareJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryReadsCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-summary-reads-compare.json"
	}
	if cfg.AddOnSecretsLocal.RequestsSummaryReadsCompareJSONS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryReadsCompareJSONS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"requests-summary-reads-compare",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsSummaryReadsCompareJSONPath),
		)
	}
	if cfg.AddOnSecretsLocal.RequestsSummaryReadsCompareTablePath == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryReadsCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-summary-reads-compare.txt"
	}
	if cfg.AddOnSecretsLocal.RequestsSummaryReadsCompareTableS3Key == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryReadsCompareTableS3Key = path.Join(
			cfg.AddOnSecretsLocal.S3Dir,
			"requests-summary-reads-compare",
			filepath.Base(cfg.AddOnSecretsLocal.RequestsSummaryReadsCompareTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	return nil
}
