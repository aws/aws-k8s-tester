package eksconfig

import (
	"errors"
	"fmt"
	"path"
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

	if cfg.AddOnSecretsLocal.RequestsWritesJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-writes.csv"
	}
	if cfg.AddOnSecretsLocal.RequestsWritesSummaryJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsWritesSummaryJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-writes-summary.json"
	}
	if cfg.AddOnSecretsLocal.RequestsWritesSummaryTablePath == "" {
		cfg.AddOnSecretsLocal.RequestsWritesSummaryTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-writes-summary.txt"
	}
	if cfg.AddOnSecretsLocal.RequestsWritesSummaryS3Dir == "" {
		cfg.AddOnSecretsLocal.RequestsWritesSummaryS3Dir = path.Join("add-on-secrets-local", fmt.Sprintf("writes-summary-%s", cfg.Parameters.Version))
	}
	if cfg.AddOnSecretsLocal.RequestsWritesSummaryCompareJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsWritesSummaryCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-writes-summary-compare.json"
	}
	if cfg.AddOnSecretsLocal.RequestsWritesSummaryCompareTablePath == "" {
		cfg.AddOnSecretsLocal.RequestsWritesSummaryCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-writes-summary-compare.txt"
	}

	if cfg.AddOnSecretsLocal.RequestsReadsJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsReadsJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-reads.csv"
	}
	if cfg.AddOnSecretsLocal.RequestsReadsSummaryJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsReadsSummaryJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-reads-summary.json"
	}
	if cfg.AddOnSecretsLocal.RequestsReadsSummaryTablePath == "" {
		cfg.AddOnSecretsLocal.RequestsReadsSummaryTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-reads-summary.txt"
	}
	if cfg.AddOnSecretsLocal.RequestsReadsSummaryS3Dir == "" {
		cfg.AddOnSecretsLocal.RequestsReadsSummaryS3Dir = path.Join("add-on-secrets-local", fmt.Sprintf("reads-summary-%s", cfg.Parameters.Version))
	}
	if cfg.AddOnSecretsLocal.RequestsReadsSummaryCompareJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsReadsSummaryCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-reads-summary-compare.json"
	}
	if cfg.AddOnSecretsLocal.RequestsReadsSummaryCompareTablePath == "" {
		cfg.AddOnSecretsLocal.RequestsReadsSummaryCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-reads-summary-compare.txt"
	}

	return nil
}
