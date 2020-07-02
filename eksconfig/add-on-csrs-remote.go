package eksconfig

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-k8s-tester/pkg/metrics"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnCSRsRemote defines parameters for EKS cluster
// add-on "CertificateSigningRequest" remote.
// It generates loads from the remote workers (Pod) in the cluster.
// Each worker writes serially with no concurrency.
// Configure "DeploymentReplicas" accordingly to increase the concurrency.
// The main use case is to write a large number of objects to fill up etcd database.
type AddOnCSRsRemote struct {
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

	// RepositoryAccountID is the account ID for tester ECR image.
	// e.g. "aws/aws-k8s-tester" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/aws/aws-k8s-tester"
	RepositoryAccountID string `json:"repository-account-id,omitempty"`
	// RepositoryRegion is the ECR repository region to pull from.
	RepositoryRegion string `json:"repository-region,omitempty"`
	// RepositoryName is the repositoryName for tester ECR image.
	// e.g. "aws/aws-k8s-tester" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/aws/aws-k8s-tester"
	RepositoryName string `json:"repository-name,omitempty"`
	// RepositoryImageTag is the image tag for tester ECR image.
	// e.g. "latest" for image URI "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/aws/aws-k8s-tester:latest"
	RepositoryImageTag string `json:"repository-image-tag,omitempty"`

	// Completes is the desired number of successfully finished pods.
	// Write QPS will be client QPS * replicas.
	// Read QPS will be client QPS * replicas.
	Completes int `json:"completes"`
	// Parallels is the the maximum desired number of pods the
	// job should run at any given time.
	// Write QPS will be client QPS * replicas.
	// Read QPS will be client QPS * replicas.
	Parallels int `json:"parallels"`

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

	// RequestsSummaryWritesOutputNamePrefix is the output path name in "/var/log" directory, used in remote worker.
	RequestsSummaryWritesOutputNamePrefix string `json:"requests-summary-writes-output-name-prefix"`

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

// EnvironmentVariablePrefixAddOnCSRsRemote is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnCSRsRemote = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_CSRS_REMOTE_"

// IsEnabledAddOnCSRsRemote returns true if "AddOnCSRsRemote" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnCSRsRemote() bool {
	if cfg.AddOnCSRsRemote == nil {
		return false
	}
	if cfg.AddOnCSRsRemote.Enable {
		return true
	}
	cfg.AddOnCSRsRemote = nil
	return false
}

func getDefaultAddOnCSRsRemote() *AddOnCSRsRemote {
	return &AddOnCSRsRemote{
		Enable:                                false,
		Completes:                             5,
		Parallels:                             5,
		Objects:                               10, // 1000 objects generates 5 MB data to etcd
		InitialRequestConditionType:           "",
		RequestsSummaryWritesOutputNamePrefix: "csrs-writes-" + randutil.String(10),
	}
}

func (cfg *Config) validateAddOnCSRsRemote() error {
	if !cfg.IsEnabledAddOnCSRsRemote() {
		return nil
	}

	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnCSRsRemote.Enable true but no node group is enabled")
	}

	if cfg.AddOnCSRsRemote.S3Dir == "" {
		cfg.AddOnCSRsRemote.S3Dir = path.Join(cfg.Name, "add-on-csrs-remote")
	}

	if cfg.AddOnCSRsRemote.Namespace == "" {
		cfg.AddOnCSRsRemote.Namespace = cfg.Name + "-csrs-remote"
	}

	if cfg.AddOnCSRsRemote.RepositoryAccountID == "" {
		return errors.New("AddOnCSRsRemote.RepositoryAccountID empty")
	}
	if cfg.AddOnCSRsRemote.RepositoryRegion == "" {
		cfg.AddOnCSRsRemote.RepositoryRegion = cfg.Region
	}
	if cfg.AddOnCSRsRemote.RepositoryName == "" {
		return errors.New("AddOnCSRsRemote.RepositoryName empty")
	}
	if cfg.AddOnCSRsRemote.RepositoryImageTag == "" {
		return errors.New("AddOnCSRsRemote.RepositoryImageTag empty")
	}

	if cfg.AddOnCSRsRemote.Objects == 0 {
		cfg.AddOnCSRsRemote.Objects = 10
	}

	switch cfg.AddOnCSRsRemote.InitialRequestConditionType {
	case "Approved":
	case "Denied":
	case "Pending", "":
	case "Random":
	default:
		return fmt.Errorf("unknown AddOnCSRsRemote.InitialRequestConditionType %q", cfg.AddOnCSRsRemote.InitialRequestConditionType)
	}

	if cfg.AddOnCSRsRemote.RequestsSummaryWritesOutputNamePrefix == "" {
		cfg.AddOnCSRsRemote.RequestsSummaryWritesOutputNamePrefix = "csrs-writes-" + randutil.String(10)
	}

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnCSRsRemote.RequestsRawWritesJSONPath == "" {
		cfg.AddOnCSRsRemote.RequestsRawWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-remote-requests-writes-raw.json"
	}
	if cfg.AddOnCSRsRemote.RequestsRawWritesJSONS3Key == "" {
		cfg.AddOnCSRsRemote.RequestsRawWritesJSONS3Key = path.Join(
			cfg.AddOnCSRsRemote.S3Dir,
			"requests-raw-writes",
			filepath.Base(cfg.AddOnCSRsRemote.RequestsRawWritesJSONPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnCSRsRemote.RequestsRawWritesCompareS3Dir == "" {
		cfg.AddOnCSRsRemote.RequestsRawWritesCompareS3Dir = path.Join("add-on-csrs-remote", "requests-raw-writes-compare", cfg.Parameters.Version)
	}
	if cfg.AddOnCSRsRemote.RequestsRawWritesCompareAllJSONPath == "" {
		cfg.AddOnCSRsRemote.RequestsRawWritesCompareAllJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-remote-requests-raw-writes-compare-all.json"
	}
	if cfg.AddOnCSRsRemote.RequestsRawWritesCompareAllJSONS3Key == "" {
		cfg.AddOnCSRsRemote.RequestsRawWritesCompareAllJSONS3Key = path.Join(
			cfg.AddOnCSRsRemote.S3Dir,
			"requests-raw-writes-compare-all",
			filepath.Base(cfg.AddOnCSRsRemote.RequestsRawWritesCompareAllJSONPath),
		)
	}
	if cfg.AddOnCSRsRemote.RequestsRawWritesCompareAllCSVPath == "" {
		cfg.AddOnCSRsRemote.RequestsRawWritesCompareAllCSVPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-remote-requests-raw-writes-compare-all.csv"
	}
	if cfg.AddOnCSRsRemote.RequestsRawWritesCompareAllCSVS3Key == "" {
		cfg.AddOnCSRsRemote.RequestsRawWritesCompareAllCSVS3Key = path.Join(
			cfg.AddOnCSRsRemote.S3Dir,
			"requests-raw-writes-compare-all",
			filepath.Base(cfg.AddOnCSRsRemote.RequestsRawWritesCompareAllCSVPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnCSRsRemote.RequestsSummaryWritesJSONPath == "" {
		cfg.AddOnCSRsRemote.RequestsSummaryWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-remote-requests-summary-writes.json"
	}
	if cfg.AddOnCSRsRemote.RequestsSummaryWritesJSONS3Key == "" {
		cfg.AddOnCSRsRemote.RequestsSummaryWritesJSONS3Key = path.Join(
			cfg.AddOnCSRsRemote.S3Dir,
			"requests-summary-writes",
			filepath.Base(cfg.AddOnCSRsRemote.RequestsSummaryWritesJSONPath),
		)
	}
	if cfg.AddOnCSRsRemote.RequestsSummaryWritesTablePath == "" {
		cfg.AddOnCSRsRemote.RequestsSummaryWritesTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-remote-requests-summary-writes.txt"
	}
	if cfg.AddOnCSRsRemote.RequestsSummaryWritesTableS3Key == "" {
		cfg.AddOnCSRsRemote.RequestsSummaryWritesTableS3Key = path.Join(
			cfg.AddOnCSRsRemote.S3Dir,
			"requests-summary-writes",
			filepath.Base(cfg.AddOnCSRsRemote.RequestsSummaryWritesTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnCSRsRemote.RequestsSummaryWritesCompareS3Dir == "" {
		cfg.AddOnCSRsRemote.RequestsSummaryWritesCompareS3Dir = path.Join("add-on-csrs-remote", "requests-summary-writes-compare", cfg.Parameters.Version)
	}
	if cfg.AddOnCSRsRemote.RequestsSummaryWritesCompareJSONPath == "" {
		cfg.AddOnCSRsRemote.RequestsSummaryWritesCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-remote-requests-summary-writes-compare.json"
	}
	if cfg.AddOnCSRsRemote.RequestsSummaryWritesCompareJSONS3Key == "" {
		cfg.AddOnCSRsRemote.RequestsSummaryWritesCompareJSONS3Key = path.Join(
			cfg.AddOnCSRsRemote.S3Dir,
			"requests-summary-writes-compare",
			filepath.Base(cfg.AddOnCSRsRemote.RequestsSummaryWritesCompareJSONPath),
		)
	}
	if cfg.AddOnCSRsRemote.RequestsSummaryWritesCompareTablePath == "" {
		cfg.AddOnCSRsRemote.RequestsSummaryWritesCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-remote-requests-summary-writes-compare.txt"
	}
	if cfg.AddOnCSRsRemote.RequestsSummaryWritesCompareTableS3Key == "" {
		cfg.AddOnCSRsRemote.RequestsSummaryWritesCompareTableS3Key = path.Join(
			cfg.AddOnCSRsRemote.S3Dir,
			"requests-summary-writes-compare",
			filepath.Base(cfg.AddOnCSRsRemote.RequestsSummaryWritesCompareTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	return nil
}
