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

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`

	// Objects is the number of "ConfigMap" objects to create.
	Objects int `json:"objects"`
	// ObjectSize is the "ConfigMap" value size in bytes.
	ObjectSize int `json:"object-size"`

	// CreatedNames is the list of created "ConfigMap" object names.
	CreatedNames []string `json:"created-names" read-only:"true"`

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
	// RequestsWritesSummaryS3Dir is the S3 directory of previous/latest "RequestsWritesSummary".
	// Specify the S3 key in the same bucket of "eksconfig.Config.S3BucketName".
	// Use for regression tests. Specify the value not bound to the cluster directory.
	// Different runs from different clusters reads and writes in this directory.
	RequestsWritesSummaryS3Dir string `json:"requests-writes-summary-s3-dir"`
	// RequestsWritesSummaryCompare is the comparision results.
	RequestsWritesSummaryCompare metrics.RequestsSummaryCompare `json:"requests-writes-summary-compare" read-only:"true"`
	// RequestsWritesSummaryCompareJSONPath is the file path to store writes requests compare summary in JSON format.
	RequestsWritesSummaryCompareJSONPath  string `json:"requests-writes-summary-compare-json-path" read-only:"true"`
	RequestsWritesSummaryCompareJSONS3Key string `json:"requests-writes-summary-compare-json-s3-key" read-only:"true"`
	// RequestsWritesSummaryCompareTablePath is the file path to store writes requests compare summary in table format.
	RequestsWritesSummaryCompareTablePath  string `json:"requests-writes-summary-compare-table-path" read-only:"true"`
	RequestsWritesSummaryCompareTableS3Key string `json:"requests-writes-summary-compare-table-s3-path" read-only:"true"`
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
	if cfg.S3BucketName == "" {
		return errors.New("AddOnConfigmapsLocal requires S3 bucket for collecting results but S3BucketName empty")
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

	if cfg.AddOnConfigmapsLocal.S3Dir == "" {
		cfg.AddOnConfigmapsLocal.S3Dir = path.Join(cfg.Name, "add-on-configmaps-local")
	}

	if cfg.AddOnConfigmapsLocal.RequestsWritesRawJSONPath == "" {
		cfg.AddOnConfigmapsLocal.RequestsWritesRawJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-local-requests-writes-raw.json"
	}
	if cfg.AddOnConfigmapsLocal.RequestsWritesRawJSONS3Key == "" {
		cfg.AddOnConfigmapsLocal.RequestsWritesRawJSONS3Key = path.Join(
			cfg.AddOnConfigmapsLocal.S3Dir,
			"writes-raw",
			filepath.Base(cfg.AddOnConfigmapsLocal.RequestsWritesRawJSONPath),
		)
	}
	if cfg.AddOnConfigmapsLocal.RequestsWritesSummaryJSONPath == "" {
		cfg.AddOnConfigmapsLocal.RequestsWritesSummaryJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-local-requests-writes-summary.json"
	}
	if cfg.AddOnConfigmapsLocal.RequestsWritesSummaryJSONS3Key == "" {
		cfg.AddOnConfigmapsLocal.RequestsWritesSummaryJSONS3Key = path.Join(
			cfg.AddOnConfigmapsLocal.S3Dir,
			"writes-summary",
			filepath.Base(cfg.AddOnConfigmapsLocal.RequestsWritesSummaryJSONPath),
		)
	}
	if cfg.AddOnConfigmapsLocal.RequestsWritesSummaryTablePath == "" {
		cfg.AddOnConfigmapsLocal.RequestsWritesSummaryTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-local-requests-writes-summary.txt"
	}
	if cfg.AddOnConfigmapsLocal.RequestsWritesSummaryTableS3Key == "" {
		cfg.AddOnConfigmapsLocal.RequestsWritesSummaryTableS3Key = path.Join(
			cfg.AddOnConfigmapsLocal.S3Dir,
			"writes-summary",
			filepath.Base(cfg.AddOnConfigmapsLocal.RequestsWritesSummaryTablePath),
		)
	}
	if cfg.AddOnConfigmapsLocal.RequestsWritesSummaryS3Dir == "" {
		cfg.AddOnConfigmapsLocal.RequestsWritesSummaryS3Dir = path.Join("add-on-configmaps-local", "writes-summary", cfg.Parameters.Version)
	}
	if cfg.AddOnConfigmapsLocal.RequestsWritesSummaryCompareJSONPath == "" {
		cfg.AddOnConfigmapsLocal.RequestsWritesSummaryCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-local-requests-writes-summary-compare.json"
	}
	if cfg.AddOnConfigmapsLocal.RequestsWritesSummaryCompareJSONS3Key == "" {
		cfg.AddOnConfigmapsLocal.RequestsWritesSummaryCompareJSONS3Key = path.Join(
			cfg.AddOnConfigmapsLocal.S3Dir,
			"writes-compare",
			filepath.Base(cfg.AddOnConfigmapsLocal.RequestsWritesSummaryCompareJSONPath),
		)
	}
	if cfg.AddOnConfigmapsLocal.RequestsWritesSummaryCompareTablePath == "" {
		cfg.AddOnConfigmapsLocal.RequestsWritesSummaryCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-local-requests-writes-summary-compare.txt"
	}
	if cfg.AddOnConfigmapsLocal.RequestsWritesSummaryCompareTableS3Key == "" {
		cfg.AddOnConfigmapsLocal.RequestsWritesSummaryCompareTableS3Key = path.Join(
			cfg.AddOnConfigmapsLocal.S3Dir,
			"writes-compare",
			filepath.Base(cfg.AddOnConfigmapsLocal.RequestsWritesSummaryCompareTablePath),
		)
	}

	return nil
}
