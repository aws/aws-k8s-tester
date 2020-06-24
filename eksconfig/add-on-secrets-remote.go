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

// AddOnSecretsRemote defines parameters for EKS cluster
// add-on "Secrets" remote.
// It generates loads from the remote workers (Pod) in the cluster.
// Each worker writes serially with no concurrency.
// Configure "DeploymentReplicas" accordingly to increase the concurrency.
// The main use case is to write a large number of objects to fill up etcd database.
// And measure latencies for secret encryption.
type AddOnSecretsRemote struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`

	// RepositoryAccountID is the account ID for tester ECR image.
	// e.g. "aws/aws-k8s-tester" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/aws/aws-k8s-tester"
	RepositoryAccountID string `json:"repository-account-id,omitempty"`
	// RepositoryName is the repositoryName for tester ECR image.
	// e.g. "aws/aws-k8s-tester" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/aws/aws-k8s-tester"
	RepositoryName string `json:"repository-name,omitempty"`
	// RepositoryImageTag is the image tag for tester ECR image.
	// e.g. "latest" for image URI "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/aws/aws-k8s-tester:latest"
	RepositoryImageTag string `json:"repository-image-tag,omitempty"`

	// DeploymentReplicas is the number of replicas to create for workers.
	// The total number of objects to be created is "DeploymentReplicas" * "Objects".
	DeploymentReplicas int32 `json:"deployment-replicas,omitempty"`
	// Objects is the number of "Secret" objects to write/read.
	Objects int `json:"objects"`
	// ObjectSize is the "Secret" value size in bytes.
	ObjectSize int `json:"object-size"`

	// NamePrefix is the prefix of Secret name.
	// If multiple Secret loader is running,
	// this must be unique per worker to avoid name conflicts.
	NamePrefix string `json:"name-prefix"`

	// RequestsSummaryWritesOutputNamePrefix is the output path name in "/var/log" directory, used in remote worker.
	RequestsSummaryWritesOutputNamePrefix string `json:"requests-summary-writes-output-name-prefix"`
	// RequestsSummaryReadsOutputNamePrefix is the output path name in "/var/log" directory, used in remote worker.
	RequestsSummaryReadsOutputNamePrefix string `json:"requests-summary-reads-output-name-prefix"`

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
	RequestsRawWritesCompareS3Dir        string `json:"requests-raw-writes-compare-s3-dir" read-only:"true"`
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
	RequestsSummaryWritesCompareS3Dir      string                  `json:"requests-summary-writes-compare-s3-dir" read-only:"true"`
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
	RequestsRawReadsCompareS3Dir        string `json:"requests-raw-reads-compare-s3-dir" read-only:"true"`
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
	RequestsSummaryReadsCompareS3Dir      string                  `json:"requests-summary-reads-compare-s3-dir" read-only:"true"`
	RequestsSummaryReadsCompare           metrics.RequestsCompare `json:"requests-summary-reads-compare" read-only:"true"`
	RequestsSummaryReadsCompareJSONPath   string                  `json:"requests-summary-reads-compare-json-path" read-only:"true"`
	RequestsSummaryReadsCompareJSONS3Key  string                  `json:"requests-summary-reads-compare-json-s3-key" read-only:"true"`
	RequestsSummaryReadsCompareTablePath  string                  `json:"requests-summary-reads-compare-table-path" read-only:"true"`
	RequestsSummaryReadsCompareTableS3Key string                  `json:"requests-summary-reads-compare-table-s3-path" read-only:"true"`

	//////////////////////////////////////////////////////////////////////////////
}

// EnvironmentVariablePrefixAddOnSecretsRemote is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnSecretsRemote = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_SECRETS_REMOTE_"

// IsEnabledAddOnSecretsRemote returns true if "AddOnSecretsRemote" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnSecretsRemote() bool {
	if cfg.AddOnSecretsRemote == nil {
		return false
	}
	if cfg.AddOnSecretsRemote.Enable {
		return true
	}
	cfg.AddOnSecretsRemote = nil
	return false
}

func getDefaultAddOnSecretsRemote() *AddOnSecretsRemote {
	return &AddOnSecretsRemote{
		Enable:             false,
		DeploymentReplicas: 5,
		Objects:            10,
		ObjectSize:         10 * 1024, // 10 KB

		// writes total 100 MB for "Secret" objects,
		// plus "Pod" objects, writes total 330 MB to etcd
		//
		// with 3 nodes, takes about 1.5 hour for all
		// these "Pod"s to complete
		//
		// Objects: 10000,
		// ObjectSize: 10 * 1024, // 10 KB

		NamePrefix: "secret" + randutil.String(5),

		RequestsSummaryWritesOutputNamePrefix: "secrets-writes-" + randutil.String(10),
		RequestsSummaryReadsOutputNamePrefix:  "secrets-reads-" + randutil.String(10),
	}
}

func (cfg *Config) validateAddOnSecretsRemote() error {
	if !cfg.IsEnabledAddOnSecretsRemote() {
		return nil
	}
	if cfg.S3BucketName == "" {
		return errors.New("AddOnSecretsRemote requires S3 bucket for collecting results but S3BucketName empty")
	}

	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnSecretsRemote.Enable true but no node group is enabled")
	}

	if cfg.AddOnSecretsRemote.Namespace == "" {
		cfg.AddOnSecretsRemote.Namespace = cfg.Name + "-secrets-remote"
	}

	if cfg.AddOnSecretsRemote.RepositoryAccountID == "" {
		return errors.New("AddOnSecretsRemote.RepositoryAccountID empty")
	}
	if cfg.AddOnSecretsRemote.RepositoryName == "" {
		return errors.New("AddOnSecretsRemote.RepositoryName empty")
	}
	if cfg.AddOnSecretsRemote.RepositoryImageTag == "" {
		return errors.New("AddOnSecretsRemote.RepositoryImageTag empty")
	}

	if cfg.AddOnSecretsRemote.DeploymentReplicas == 0 {
		cfg.AddOnSecretsRemote.DeploymentReplicas = 5
	}
	if cfg.AddOnSecretsRemote.Objects == 0 {
		cfg.AddOnSecretsRemote.Objects = 10
	}
	if cfg.AddOnSecretsRemote.ObjectSize == 0 {
		cfg.AddOnSecretsRemote.ObjectSize = 10 * 1024
	}

	if cfg.AddOnSecretsRemote.NamePrefix == "" {
		cfg.AddOnSecretsRemote.NamePrefix = "secret" + randutil.String(5)
	}

	if cfg.AddOnSecretsRemote.RequestsSummaryWritesOutputNamePrefix == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryWritesOutputNamePrefix = "secrets-writes-" + randutil.String(10)
	}
	if cfg.AddOnSecretsRemote.RequestsSummaryReadsOutputNamePrefix == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryReadsOutputNamePrefix = "secrets-reads-" + randutil.String(10)
	}

	if cfg.AddOnSecretsRemote.S3Dir == "" {
		cfg.AddOnSecretsRemote.S3Dir = path.Join(cfg.Name, "add-on-secrets-remote")
	}

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnSecretsRemote.RequestsRawWritesJSONPath == "" {
		cfg.AddOnSecretsRemote.RequestsRawWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-remote-requests-writes-raw.json"
	}
	if cfg.AddOnSecretsRemote.RequestsRawWritesJSONS3Key == "" {
		cfg.AddOnSecretsRemote.RequestsRawWritesJSONS3Key = path.Join(
			cfg.AddOnSecretsRemote.S3Dir,
			"requests-raw-writes",
			filepath.Base(cfg.AddOnSecretsRemote.RequestsRawWritesJSONPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnSecretsRemote.RequestsRawWritesCompareS3Dir == "" {
		cfg.AddOnSecretsRemote.RequestsRawWritesCompareS3Dir = path.Join("add-on-secrets-remote", "requests-raw-writes-compare", cfg.Parameters.Version)
	}
	if cfg.AddOnSecretsRemote.RequestsRawWritesCompareAllJSONPath == "" {
		cfg.AddOnSecretsRemote.RequestsRawWritesCompareAllJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-remote-requests-raw-writes-compare-all.json"
	}
	if cfg.AddOnSecretsRemote.RequestsRawWritesCompareAllJSONS3Key == "" {
		cfg.AddOnSecretsRemote.RequestsRawWritesCompareAllJSONS3Key = path.Join(
			cfg.AddOnSecretsRemote.S3Dir,
			"requests-raw-writes-compare-all",
			filepath.Base(cfg.AddOnSecretsRemote.RequestsRawWritesCompareAllJSONPath),
		)
	}
	if cfg.AddOnSecretsRemote.RequestsRawWritesCompareAllCSVPath == "" {
		cfg.AddOnSecretsRemote.RequestsRawWritesCompareAllCSVPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-remote-requests-raw-writes-compare-all.csv"
	}
	if cfg.AddOnSecretsRemote.RequestsRawWritesCompareAllCSVS3Key == "" {
		cfg.AddOnSecretsRemote.RequestsRawWritesCompareAllCSVS3Key = path.Join(
			cfg.AddOnSecretsRemote.S3Dir,
			"requests-raw-writes-compare-all",
			filepath.Base(cfg.AddOnSecretsRemote.RequestsRawWritesCompareAllCSVPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnSecretsRemote.RequestsSummaryWritesJSONPath == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-remote-requests-summary-writes.json"
	}
	if cfg.AddOnSecretsRemote.RequestsSummaryWritesJSONS3Key == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryWritesJSONS3Key = path.Join(
			cfg.AddOnSecretsRemote.S3Dir,
			"requests-summary-writes",
			filepath.Base(cfg.AddOnSecretsRemote.RequestsSummaryWritesJSONPath),
		)
	}
	if cfg.AddOnSecretsRemote.RequestsSummaryWritesTablePath == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryWritesTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-remote-requests-summary-writes.txt"
	}
	if cfg.AddOnSecretsRemote.RequestsSummaryWritesTableS3Key == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryWritesTableS3Key = path.Join(
			cfg.AddOnSecretsRemote.S3Dir,
			"requests-summary-writes",
			filepath.Base(cfg.AddOnSecretsRemote.RequestsSummaryWritesTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnSecretsRemote.RequestsSummaryWritesCompareS3Dir == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryWritesCompareS3Dir = path.Join("add-on-secrets-remote", "requests-summary-writes-compare", cfg.Parameters.Version)
	}
	if cfg.AddOnSecretsRemote.RequestsSummaryWritesCompareJSONPath == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryWritesCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-remote-requests-summary-writes-compare.json"
	}
	if cfg.AddOnSecretsRemote.RequestsSummaryWritesCompareJSONS3Key == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryWritesCompareJSONS3Key = path.Join(
			cfg.AddOnSecretsRemote.S3Dir,
			"requests-summary-writes-compare",
			filepath.Base(cfg.AddOnSecretsRemote.RequestsSummaryWritesCompareJSONPath),
		)
	}
	if cfg.AddOnSecretsRemote.RequestsSummaryWritesCompareTablePath == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryWritesCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-remote-requests-summary-writes-compare.txt"
	}
	if cfg.AddOnSecretsRemote.RequestsSummaryWritesCompareTableS3Key == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryWritesCompareTableS3Key = path.Join(
			cfg.AddOnSecretsRemote.S3Dir,
			"requests-summary-writes-compare",
			filepath.Base(cfg.AddOnSecretsRemote.RequestsSummaryWritesCompareTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnSecretsRemote.RequestsRawReadsJSONPath == "" {
		cfg.AddOnSecretsRemote.RequestsRawReadsJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-remote-requests-raw-reads.json"
	}
	if cfg.AddOnSecretsRemote.RequestsRawReadsJSONS3Key == "" {
		cfg.AddOnSecretsRemote.RequestsRawReadsJSONS3Key = path.Join(
			cfg.AddOnSecretsRemote.S3Dir,
			"requests-raw-reads",
			filepath.Base(cfg.AddOnSecretsRemote.RequestsRawReadsJSONPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnSecretsRemote.RequestsRawReadsCompareS3Dir == "" {
		cfg.AddOnSecretsRemote.RequestsRawReadsCompareS3Dir = path.Join("add-on-secrets-remote", "requests-raw-reads-compare", cfg.Parameters.Version)
	}
	if cfg.AddOnSecretsRemote.RequestsRawReadsCompareAllJSONPath == "" {
		cfg.AddOnSecretsRemote.RequestsRawReadsCompareAllJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-remote-requests-raw-reads-compare-all.json"
	}
	if cfg.AddOnSecretsRemote.RequestsRawReadsCompareAllJSONS3Key == "" {
		cfg.AddOnSecretsRemote.RequestsRawReadsCompareAllJSONS3Key = path.Join(
			cfg.AddOnSecretsRemote.S3Dir,
			"requests-raw-reads-compare-all",
			filepath.Base(cfg.AddOnSecretsRemote.RequestsRawReadsCompareAllJSONPath),
		)
	}
	if cfg.AddOnSecretsRemote.RequestsRawReadsCompareAllCSVPath == "" {
		cfg.AddOnSecretsRemote.RequestsRawReadsCompareAllCSVPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-remote-requests-raw-reads-compare-all.csv"
	}
	if cfg.AddOnSecretsRemote.RequestsRawReadsCompareAllCSVS3Key == "" {
		cfg.AddOnSecretsRemote.RequestsRawReadsCompareAllCSVS3Key = path.Join(
			cfg.AddOnSecretsRemote.S3Dir,
			"requests-raw-reads-compare-all",
			filepath.Base(cfg.AddOnSecretsRemote.RequestsRawReadsCompareAllCSVPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnSecretsRemote.RequestsSummaryReadsJSONPath == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryReadsJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-remote-requests-summary-reads.json"
	}
	if cfg.AddOnSecretsRemote.RequestsSummaryReadsJSONS3Key == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryReadsJSONS3Key = path.Join(
			cfg.AddOnSecretsRemote.S3Dir,
			"requests-summary-reads",
			filepath.Base(cfg.AddOnSecretsRemote.RequestsSummaryReadsJSONPath),
		)
	}
	if cfg.AddOnSecretsRemote.RequestsSummaryReadsTablePath == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryReadsTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-remote-requests-summary-reads.txt"
	}
	if cfg.AddOnSecretsRemote.RequestsSummaryReadsTableS3Key == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryReadsTableS3Key = path.Join(
			cfg.AddOnSecretsRemote.S3Dir,
			"requests-summary-reads",
			filepath.Base(cfg.AddOnSecretsRemote.RequestsSummaryReadsTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnSecretsRemote.RequestsSummaryReadsCompareS3Dir == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryReadsCompareS3Dir = path.Join("add-on-secrets-remote", "requests-summary-reads-compare", cfg.Parameters.Version)
	}
	if cfg.AddOnSecretsRemote.RequestsSummaryReadsCompareJSONPath == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryReadsCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-remote-requests-summary-reads-compare.json"
	}
	if cfg.AddOnSecretsRemote.RequestsSummaryReadsCompareJSONS3Key == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryReadsCompareJSONS3Key = path.Join(
			cfg.AddOnSecretsRemote.S3Dir,
			"requests-summary-reads-compare",
			filepath.Base(cfg.AddOnSecretsRemote.RequestsSummaryReadsCompareJSONPath),
		)
	}
	if cfg.AddOnSecretsRemote.RequestsSummaryReadsCompareTablePath == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryReadsCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-remote-requests-summary-reads-compare.txt"
	}
	if cfg.AddOnSecretsRemote.RequestsSummaryReadsCompareTableS3Key == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryReadsCompareTableS3Key = path.Join(
			cfg.AddOnSecretsRemote.S3Dir,
			"requests-summary-reads-compare",
			filepath.Base(cfg.AddOnSecretsRemote.RequestsSummaryReadsCompareTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	return nil
}
