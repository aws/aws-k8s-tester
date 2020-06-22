package eksconfig

import (
	"errors"
	"fmt"
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

	// RequestsWritesJSONPath is the file path to store writes requests in JSON format.
	RequestsWritesJSONPath string `json:"requests-writes-json-path" read-only:"true"`
	// RequestsWritesSummary is the writes results.
	RequestsWritesSummary metrics.RequestsSummary `json:"requests-writes-summary,omitempty" read-only:"true"`
	// RequestsWritesSummaryJSONPath is the file path to store writes requests summary in JSON format.
	RequestsWritesSummaryJSONPath string `json:"requests-writes-summary-json-path" read-only:"true"`
	// RequestsWritesSummaryTablePath is the file path to store writes requests summary in table format.
	RequestsWritesSummaryTablePath string `json:"requests-writes-summary-table-path" read-only:"true"`
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

	if cfg.AddOnCSRsLocal.RequestsWritesJSONPath == "" {
		cfg.AddOnCSRsLocal.RequestsWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-local-requests-writes.csv"
	}
	if cfg.AddOnCSRsLocal.RequestsWritesSummaryJSONPath == "" {
		cfg.AddOnCSRsLocal.RequestsWritesSummaryJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-local-requests-writes-summary.json"
	}
	if cfg.AddOnCSRsLocal.RequestsWritesSummaryTablePath == "" {
		cfg.AddOnCSRsLocal.RequestsWritesSummaryTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-local-requests-writes-summary.txt"
	}

	return nil
}
