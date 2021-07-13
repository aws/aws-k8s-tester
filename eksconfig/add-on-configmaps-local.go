package eksconfig

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
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

	// S3Dir is the S3 directory to store all test results.
	// It is under the bucket "eksconfig.Config.S3BucketName".
	S3Dir string `json:"s3-dir"`

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`

	// Objects is the number of "ConfigMap" objects to create.
	Objects int `json:"objects"`
	// ObjectSize is the "ConfigMap" value size in bytes.
	ObjectSize int `json:"object-size"`

	// CreatedNames is the list of created "ConfigMap" object names.
	CreatedNames []string `json:"created-names" read-only:"true"`

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

	if cfg.AddOnConfigmapsLocal.S3Dir == "" {
		cfg.AddOnConfigmapsLocal.S3Dir = path.Join(cfg.Name, "add-on-configmaps-local")
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

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnConfigmapsLocal.RequestsRawWritesJSONPath == "" {
		cfg.AddOnConfigmapsLocal.RequestsRawWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-local-requests-writes-raw.json"
	}
	if cfg.AddOnConfigmapsLocal.RequestsRawWritesJSONS3Key == "" {
		cfg.AddOnConfigmapsLocal.RequestsRawWritesJSONS3Key = path.Join(
			cfg.AddOnConfigmapsLocal.S3Dir,
			"requests-raw-writes",
			filepath.Base(cfg.AddOnConfigmapsLocal.RequestsRawWritesJSONPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnConfigmapsLocal.RequestsRawWritesCompareS3Dir == "" {
		cfg.AddOnConfigmapsLocal.RequestsRawWritesCompareS3Dir = path.Join("add-on-configmaps-local", "requests-raw-writes-compare", cfg.Version)
	}
	if cfg.AddOnConfigmapsLocal.RequestsRawWritesCompareAllJSONPath == "" {
		cfg.AddOnConfigmapsLocal.RequestsRawWritesCompareAllJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-local-requests-raw-writes-compare-all.json"
	}
	if cfg.AddOnConfigmapsLocal.RequestsRawWritesCompareAllJSONS3Key == "" {
		cfg.AddOnConfigmapsLocal.RequestsRawWritesCompareAllJSONS3Key = path.Join(
			cfg.AddOnConfigmapsLocal.S3Dir,
			"requests-raw-writes-compare-all",
			filepath.Base(cfg.AddOnConfigmapsLocal.RequestsRawWritesCompareAllJSONPath),
		)
	}
	if cfg.AddOnConfigmapsLocal.RequestsRawWritesCompareAllCSVPath == "" {
		cfg.AddOnConfigmapsLocal.RequestsRawWritesCompareAllCSVPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-local-requests-raw-writes-compare-all.csv"
	}
	if cfg.AddOnConfigmapsLocal.RequestsRawWritesCompareAllCSVS3Key == "" {
		cfg.AddOnConfigmapsLocal.RequestsRawWritesCompareAllCSVS3Key = path.Join(
			cfg.AddOnConfigmapsLocal.S3Dir,
			"requests-raw-writes-compare-all",
			filepath.Base(cfg.AddOnConfigmapsLocal.RequestsRawWritesCompareAllCSVPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnConfigmapsLocal.RequestsSummaryWritesJSONPath == "" {
		cfg.AddOnConfigmapsLocal.RequestsSummaryWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-local-requests-summary-writes.json"
	}
	if cfg.AddOnConfigmapsLocal.RequestsSummaryWritesJSONS3Key == "" {
		cfg.AddOnConfigmapsLocal.RequestsSummaryWritesJSONS3Key = path.Join(
			cfg.AddOnConfigmapsLocal.S3Dir,
			"requests-summary-writes",
			filepath.Base(cfg.AddOnConfigmapsLocal.RequestsSummaryWritesJSONPath),
		)
	}
	if cfg.AddOnConfigmapsLocal.RequestsSummaryWritesTablePath == "" {
		cfg.AddOnConfigmapsLocal.RequestsSummaryWritesTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-local-requests-summary-writes.txt"
	}
	if cfg.AddOnConfigmapsLocal.RequestsSummaryWritesTableS3Key == "" {
		cfg.AddOnConfigmapsLocal.RequestsSummaryWritesTableS3Key = path.Join(
			cfg.AddOnConfigmapsLocal.S3Dir,
			"requests-summary-writes",
			filepath.Base(cfg.AddOnConfigmapsLocal.RequestsSummaryWritesTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnConfigmapsLocal.RequestsSummaryWritesCompareS3Dir == "" {
		cfg.AddOnConfigmapsLocal.RequestsSummaryWritesCompareS3Dir = path.Join("add-on-configmaps-local", "requests-summary-writes-compare", cfg.Version)
	}
	if cfg.AddOnConfigmapsLocal.RequestsSummaryWritesCompareJSONPath == "" {
		cfg.AddOnConfigmapsLocal.RequestsSummaryWritesCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-local-requests-summary-writes-compare.json"
	}
	if cfg.AddOnConfigmapsLocal.RequestsSummaryWritesCompareJSONS3Key == "" {
		cfg.AddOnConfigmapsLocal.RequestsSummaryWritesCompareJSONS3Key = path.Join(
			cfg.AddOnConfigmapsLocal.S3Dir,
			"requests-summary-writes-compare",
			filepath.Base(cfg.AddOnConfigmapsLocal.RequestsSummaryWritesCompareJSONPath),
		)
	}
	if cfg.AddOnConfigmapsLocal.RequestsSummaryWritesCompareTablePath == "" {
		cfg.AddOnConfigmapsLocal.RequestsSummaryWritesCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-local-requests-summary-writes-compare.txt"
	}
	if cfg.AddOnConfigmapsLocal.RequestsSummaryWritesCompareTableS3Key == "" {
		cfg.AddOnConfigmapsLocal.RequestsSummaryWritesCompareTableS3Key = path.Join(
			cfg.AddOnConfigmapsLocal.S3Dir,
			"requests-summary-writes-compare",
			filepath.Base(cfg.AddOnConfigmapsLocal.RequestsSummaryWritesCompareTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	return nil
}
