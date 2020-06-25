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

// AddOnCSRsLocal defines parameters for EKS cluster
// add-on "CertificateSigningRequest" local.
// It generates loads from the local host machine.
// Every object is written serially with no concurrency.
// Use remote tester to write with concurrency.
// The main use case is to write a large number of objects to fill up etcd database.
type AddOnCSRsLocal struct {
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

	// Objects is the number of "CertificateSigningRequest" objects to create.
	Objects int `json:"objects"`

	// InitialRequestConditionType is the initial CSR condition type
	// to simulate CSR condition.
	//
	// Valid values are:
	//   "k8s.io/api/certificates/v1beta1.CertificateApproved" == "Approved"
	//   "k8s.io/api/certificates/v1beta1.CertificateDenied" == "Denied"
	//   "Random"
	//   "Pending"
	//   ""
	//
	InitialRequestConditionType string `json:"initial-request-condition-type"`

	// CreatedNames is the list of created "CertificateSigningRequest" object names.
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

// EnvironmentVariablePrefixAddOnCSRsLocal is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnCSRsLocal = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_CSRS_LOCAL_"

// IsEnabledAddOnCSRsLocal returns true if "AddOnCSRsLocal" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnCSRsLocal() bool {
	if cfg.AddOnCSRsLocal == nil {
		return false
	}
	if cfg.AddOnCSRsLocal.Enable {
		return true
	}
	cfg.AddOnCSRsLocal = nil
	return false
}

func getDefaultAddOnCSRsLocal() *AddOnCSRsLocal {
	return &AddOnCSRsLocal{
		Enable:                      false,
		Objects:                     10, // 1000 objects generates 5 MB data to etcd
		InitialRequestConditionType: "",
	}
}

func (cfg *Config) validateAddOnCSRsLocal() error {
	if !cfg.IsEnabledAddOnCSRsLocal() {
		return nil
	}

	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnCSRsLocal.Enable true but no node group is enabled")
	}

	if cfg.AddOnCSRsLocal.S3Dir == "" {
		cfg.AddOnCSRsLocal.S3Dir = path.Join(cfg.Name, "add-on-csrs-local")
	}

	if cfg.AddOnCSRsLocal.Objects == 0 {
		cfg.AddOnCSRsLocal.Objects = 10
	}

	switch cfg.AddOnCSRsLocal.InitialRequestConditionType {
	case "Approved":
	case "Denied":
	case "Pending", "":
	case "Random":
	default:
		return fmt.Errorf("unknown AddOnCSRsLocal.InitialRequestConditionType %q", cfg.AddOnCSRsLocal.InitialRequestConditionType)
	}

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnCSRsLocal.RequestsRawWritesJSONPath == "" {
		cfg.AddOnCSRsLocal.RequestsRawWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-local-requests-writes-raw.json"
	}
	if cfg.AddOnCSRsLocal.RequestsRawWritesJSONS3Key == "" {
		cfg.AddOnCSRsLocal.RequestsRawWritesJSONS3Key = path.Join(
			cfg.AddOnCSRsLocal.S3Dir,
			"requests-raw-writes",
			filepath.Base(cfg.AddOnCSRsLocal.RequestsRawWritesJSONPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnCSRsLocal.RequestsRawWritesCompareS3Dir == "" {
		cfg.AddOnCSRsLocal.RequestsRawWritesCompareS3Dir = path.Join("add-on-csrs-local", "requests-raw-writes-compare", cfg.Parameters.Version)
	}
	if cfg.AddOnCSRsLocal.RequestsRawWritesCompareAllJSONPath == "" {
		cfg.AddOnCSRsLocal.RequestsRawWritesCompareAllJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-local-requests-raw-writes-compare-all.json"
	}
	if cfg.AddOnCSRsLocal.RequestsRawWritesCompareAllJSONS3Key == "" {
		cfg.AddOnCSRsLocal.RequestsRawWritesCompareAllJSONS3Key = path.Join(
			cfg.AddOnCSRsLocal.S3Dir,
			"requests-raw-writes-compare-all",
			filepath.Base(cfg.AddOnCSRsLocal.RequestsRawWritesCompareAllJSONPath),
		)
	}
	if cfg.AddOnCSRsLocal.RequestsRawWritesCompareAllCSVPath == "" {
		cfg.AddOnCSRsLocal.RequestsRawWritesCompareAllCSVPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-local-requests-raw-writes-compare-all.csv"
	}
	if cfg.AddOnCSRsLocal.RequestsRawWritesCompareAllCSVS3Key == "" {
		cfg.AddOnCSRsLocal.RequestsRawWritesCompareAllCSVS3Key = path.Join(
			cfg.AddOnCSRsLocal.S3Dir,
			"requests-raw-writes-compare-all",
			filepath.Base(cfg.AddOnCSRsLocal.RequestsRawWritesCompareAllCSVPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnCSRsLocal.RequestsSummaryWritesJSONPath == "" {
		cfg.AddOnCSRsLocal.RequestsSummaryWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-local-requests-summary-writes.json"
	}
	if cfg.AddOnCSRsLocal.RequestsSummaryWritesJSONS3Key == "" {
		cfg.AddOnCSRsLocal.RequestsSummaryWritesJSONS3Key = path.Join(
			cfg.AddOnCSRsLocal.S3Dir,
			"requests-summary-writes",
			filepath.Base(cfg.AddOnCSRsLocal.RequestsSummaryWritesJSONPath),
		)
	}
	if cfg.AddOnCSRsLocal.RequestsSummaryWritesTablePath == "" {
		cfg.AddOnCSRsLocal.RequestsSummaryWritesTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-local-requests-summary-writes.txt"
	}
	if cfg.AddOnCSRsLocal.RequestsSummaryWritesTableS3Key == "" {
		cfg.AddOnCSRsLocal.RequestsSummaryWritesTableS3Key = path.Join(
			cfg.AddOnCSRsLocal.S3Dir,
			"requests-summary-writes",
			filepath.Base(cfg.AddOnCSRsLocal.RequestsSummaryWritesTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnCSRsLocal.RequestsSummaryWritesCompareS3Dir == "" {
		cfg.AddOnCSRsLocal.RequestsSummaryWritesCompareS3Dir = path.Join("add-on-csrs-local", "requests-summary-writes-compare", cfg.Parameters.Version)
	}
	if cfg.AddOnCSRsLocal.RequestsSummaryWritesCompareJSONPath == "" {
		cfg.AddOnCSRsLocal.RequestsSummaryWritesCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-local-requests-summary-writes-compare.json"
	}
	if cfg.AddOnCSRsLocal.RequestsSummaryWritesCompareJSONS3Key == "" {
		cfg.AddOnCSRsLocal.RequestsSummaryWritesCompareJSONS3Key = path.Join(
			cfg.AddOnCSRsLocal.S3Dir,
			"requests-summary-writes-compare",
			filepath.Base(cfg.AddOnCSRsLocal.RequestsSummaryWritesCompareJSONPath),
		)
	}
	if cfg.AddOnCSRsLocal.RequestsSummaryWritesCompareTablePath == "" {
		cfg.AddOnCSRsLocal.RequestsSummaryWritesCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-local-requests-summary-writes-compare.txt"
	}
	if cfg.AddOnCSRsLocal.RequestsSummaryWritesCompareTableS3Key == "" {
		cfg.AddOnCSRsLocal.RequestsSummaryWritesCompareTableS3Key = path.Join(
			cfg.AddOnCSRsLocal.S3Dir,
			"requests-summary-writes-compare",
			filepath.Base(cfg.AddOnCSRsLocal.RequestsSummaryWritesCompareTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	return nil
}
