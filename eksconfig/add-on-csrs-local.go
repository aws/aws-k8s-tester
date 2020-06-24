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
	if cfg.S3BucketName == "" {
		return errors.New("AddOnCSRsLocal requires S3 bucket for collecting results but S3BucketName empty")
	}

	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnCSRsLocal.Enable true but no node group is enabled")
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

	if cfg.AddOnCSRsLocal.S3Dir == "" {
		cfg.AddOnCSRsLocal.S3Dir = path.Join(cfg.Name, "add-on-csrs-local")
	}

	if cfg.AddOnCSRsLocal.RequestsWritesRawJSONPath == "" {
		cfg.AddOnCSRsLocal.RequestsWritesRawJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-local-requests-writes-raw.json"
	}
	if cfg.AddOnCSRsLocal.RequestsWritesRawJSONS3Key == "" {
		cfg.AddOnCSRsLocal.RequestsWritesRawJSONS3Key = path.Join(
			cfg.AddOnCSRsLocal.S3Dir,
			"writes-raw",
			filepath.Base(cfg.AddOnCSRsLocal.RequestsWritesRawJSONPath),
		)
	}
	if cfg.AddOnCSRsLocal.RequestsWritesSummaryJSONPath == "" {
		cfg.AddOnCSRsLocal.RequestsWritesSummaryJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-local-requests-writes-summary.json"
	}
	if cfg.AddOnCSRsLocal.RequestsWritesSummaryJSONS3Key == "" {
		cfg.AddOnCSRsLocal.RequestsWritesSummaryJSONS3Key = path.Join(
			cfg.AddOnCSRsLocal.S3Dir,
			"writes-summary",
			filepath.Base(cfg.AddOnCSRsLocal.RequestsWritesSummaryJSONPath),
		)
	}
	if cfg.AddOnCSRsLocal.RequestsWritesSummaryTablePath == "" {
		cfg.AddOnCSRsLocal.RequestsWritesSummaryTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-local-requests-writes-summary.txt"
	}
	if cfg.AddOnCSRsLocal.RequestsWritesSummaryTableS3Key == "" {
		cfg.AddOnCSRsLocal.RequestsWritesSummaryTableS3Key = path.Join(
			cfg.AddOnCSRsLocal.S3Dir,
			"writes-summary",
			filepath.Base(cfg.AddOnCSRsLocal.RequestsWritesSummaryTablePath),
		)
	}
	if cfg.AddOnCSRsLocal.RequestsWritesCompareS3Dir == "" {
		cfg.AddOnCSRsLocal.RequestsWritesCompareS3Dir = path.Join("add-on-csrs-local", "writes-compare", cfg.Parameters.Version)
	}
	if cfg.AddOnCSRsLocal.RequestsWritesCompareJSONPath == "" {
		cfg.AddOnCSRsLocal.RequestsWritesCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-local-requests-writes-compare.json"
	}
	if cfg.AddOnCSRsLocal.RequestsWritesCompareJSONS3Key == "" {
		cfg.AddOnCSRsLocal.RequestsWritesCompareJSONS3Key = path.Join(
			cfg.AddOnCSRsLocal.S3Dir,
			"writes-compare",
			filepath.Base(cfg.AddOnCSRsLocal.RequestsWritesCompareJSONPath),
		)
	}
	if cfg.AddOnCSRsLocal.RequestsWritesCompareTablePath == "" {
		cfg.AddOnCSRsLocal.RequestsWritesCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-local-requests-writes-compare.txt"
	}
	if cfg.AddOnCSRsLocal.RequestsWritesCompareTableS3Key == "" {
		cfg.AddOnCSRsLocal.RequestsWritesCompareTableS3Key = path.Join(
			cfg.AddOnCSRsLocal.S3Dir,
			"writes-compare",
			filepath.Base(cfg.AddOnCSRsLocal.RequestsWritesCompareTablePath),
		)
	}

	return nil
}
