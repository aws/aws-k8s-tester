package eksconfig

import (
	"errors"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/metrics"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnStresserRemote defines parameters for EKS cluster
// add-on cluster loader remote.
// It generates loads from the remote workers (Pod) in the cluster.
// Each worker writes and reads serially with no concurrency.
// Configure "DeploymentReplicas" accordingly to increase the concurrency.
// ref. https://github.com/kubernetes/perf-tests
type AddOnStresserRemote struct {
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

	// ObjectSize is the value size in bytes for write objects.
	// If 0, do not write anything.
	ObjectSize int `json:"object-size"`
	// ListLimit is the maximum number of items in the list call.
	// Sets "metav1.ListOptions".Limit field.
	// 0 to list all.
	ListLimit int64 `json:"list-limit"`
	// Duration is the duration to run load testing.
	Duration       time.Duration `json:"duration,omitempty"`
	DurationString string        `json:"duration-string,omitempty" read-only:"true"`

	// RequestsSummaryWritesOutputNamePrefix is the output path name in "/var/log" directory, used in remote worker.
	RequestsSummaryWritesOutputNamePrefix string `json:"requests-summary-writes-output-name-prefix"`
	// RequestsSummaryReadsOutputNamePrefix is the output path name in "/var/log" directory, used in remote worker.
	RequestsSummaryReadsOutputNamePrefix string `json:"requests-summary-reads-output-name-prefix"`

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

// EnvironmentVariablePrefixAddOnStresserRemote is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnStresserRemote = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_STRESSER_REMOTE_"

// IsEnabledAddOnStresserRemote returns true if "AddOnStresserRemote" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnStresserRemote() bool {
	if cfg.AddOnStresserRemote == nil {
		return false
	}
	if cfg.AddOnStresserRemote.Enable {
		return true
	}
	cfg.AddOnStresserRemote = nil
	return false
}

func getDefaultAddOnStresserRemote() *AddOnStresserRemote {
	return &AddOnStresserRemote{
		Enable:                                false,
		Completes:                             5,
		Parallels:                             5,
		ObjectSize:                            0,
		ListLimit:                             0,
		Duration:                              time.Minute,
		RequestsSummaryWritesOutputNamePrefix: "stresser-writes-" + randutil.String(10),
		RequestsSummaryReadsOutputNamePrefix:  "stresser-reads-" + randutil.String(10),
	}
}

func (cfg *Config) GetAddOnStresserRemoteRepositoryRegion() string {
	if !cfg.IsEnabledAddOnStresserRemote() {
		return cfg.Region
	}
	return cfg.AddOnStresserRemote.RepositoryRegion
}

func (cfg *Config) validateAddOnStresserRemote() error {
	if !cfg.IsEnabledAddOnStresserRemote() {
		return nil
	}

	if cfg.AddOnStresserRemote.S3Dir == "" {
		cfg.AddOnStresserRemote.S3Dir = path.Join(cfg.Name, "add-on-stresser-remote")
	}

	if cfg.AddOnStresserRemote.Namespace == "" {
		cfg.AddOnStresserRemote.Namespace = cfg.Name + "-stresser-remote"
	}

	if cfg.AddOnStresserRemote.RepositoryAccountID == "" {
		return errors.New("AddOnStresserRemote.RepositoryAccountID empty")
	}
	if cfg.AddOnStresserRemote.RepositoryRegion == "" {
		cfg.AddOnStresserRemote.RepositoryRegion = cfg.Region
	}
	if cfg.AddOnStresserRemote.RepositoryName == "" {
		return errors.New("AddOnStresserRemote.RepositoryName empty")
	}
	if cfg.AddOnStresserRemote.RepositoryImageTag == "" {
		return errors.New("AddOnStresserRemote.RepositoryImageTag empty")
	}

	if cfg.AddOnStresserRemote.Duration == time.Duration(0) {
		cfg.AddOnStresserRemote.Duration = time.Minute
	}
	cfg.AddOnStresserRemote.DurationString = cfg.AddOnStresserRemote.Duration.String()

	if cfg.AddOnStresserRemote.RequestsSummaryWritesOutputNamePrefix == "" {
		cfg.AddOnStresserRemote.RequestsSummaryWritesOutputNamePrefix = "stresser-writes-" + randutil.String(10)
	}
	if cfg.AddOnStresserRemote.RequestsSummaryReadsOutputNamePrefix == "" {
		cfg.AddOnStresserRemote.RequestsSummaryReadsOutputNamePrefix = "stresser-reads-" + randutil.String(10)
	}

	if cfg.AddOnStresserRemote.S3Dir == "" {
		cfg.AddOnStresserRemote.S3Dir = path.Join(cfg.Name, "add-on-stresser-remote")
	}

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnStresserRemote.RequestsRawWritesJSONPath == "" {
		cfg.AddOnStresserRemote.RequestsRawWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-writes-raw.json"
	}
	if cfg.AddOnStresserRemote.RequestsRawWritesJSONS3Key == "" {
		cfg.AddOnStresserRemote.RequestsRawWritesJSONS3Key = path.Join(
			cfg.AddOnStresserRemote.S3Dir,
			"requests-raw-writes",
			filepath.Base(cfg.AddOnStresserRemote.RequestsRawWritesJSONPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnStresserRemote.RequestsRawWritesCompareS3Dir == "" {
		cfg.AddOnStresserRemote.RequestsRawWritesCompareS3Dir = path.Join("add-on-stresser-remote", "requests-raw-writes-compare", cfg.Version)
	}
	if cfg.AddOnStresserRemote.RequestsRawWritesCompareAllJSONPath == "" {
		cfg.AddOnStresserRemote.RequestsRawWritesCompareAllJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-raw-writes-compare-all.json"
	}
	if cfg.AddOnStresserRemote.RequestsRawWritesCompareAllJSONS3Key == "" {
		cfg.AddOnStresserRemote.RequestsRawWritesCompareAllJSONS3Key = path.Join(
			cfg.AddOnStresserRemote.S3Dir,
			"requests-raw-writes-compare-all",
			filepath.Base(cfg.AddOnStresserRemote.RequestsRawWritesCompareAllJSONPath),
		)
	}
	if cfg.AddOnStresserRemote.RequestsRawWritesCompareAllCSVPath == "" {
		cfg.AddOnStresserRemote.RequestsRawWritesCompareAllCSVPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-raw-writes-compare-all.csv"
	}
	if cfg.AddOnStresserRemote.RequestsRawWritesCompareAllCSVS3Key == "" {
		cfg.AddOnStresserRemote.RequestsRawWritesCompareAllCSVS3Key = path.Join(
			cfg.AddOnStresserRemote.S3Dir,
			"requests-raw-writes-compare-all",
			filepath.Base(cfg.AddOnStresserRemote.RequestsRawWritesCompareAllCSVPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnStresserRemote.RequestsSummaryWritesJSONPath == "" {
		cfg.AddOnStresserRemote.RequestsSummaryWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-summary-writes.json"
	}
	if cfg.AddOnStresserRemote.RequestsSummaryWritesJSONS3Key == "" {
		cfg.AddOnStresserRemote.RequestsSummaryWritesJSONS3Key = path.Join(
			cfg.AddOnStresserRemote.S3Dir,
			"requests-summary-writes",
			filepath.Base(cfg.AddOnStresserRemote.RequestsSummaryWritesJSONPath),
		)
	}
	if cfg.AddOnStresserRemote.RequestsSummaryWritesTablePath == "" {
		cfg.AddOnStresserRemote.RequestsSummaryWritesTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-summary-writes.txt"
	}
	if cfg.AddOnStresserRemote.RequestsSummaryWritesTableS3Key == "" {
		cfg.AddOnStresserRemote.RequestsSummaryWritesTableS3Key = path.Join(
			cfg.AddOnStresserRemote.S3Dir,
			"requests-summary-writes",
			filepath.Base(cfg.AddOnStresserRemote.RequestsSummaryWritesTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnStresserRemote.RequestsSummaryWritesCompareS3Dir == "" {
		cfg.AddOnStresserRemote.RequestsSummaryWritesCompareS3Dir = path.Join("add-on-stresser-remote", "requests-summary-writes-compare", cfg.Version)
	}
	if cfg.AddOnStresserRemote.RequestsSummaryWritesCompareJSONPath == "" {
		cfg.AddOnStresserRemote.RequestsSummaryWritesCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-summary-writes-compare.json"
	}
	if cfg.AddOnStresserRemote.RequestsSummaryWritesCompareJSONS3Key == "" {
		cfg.AddOnStresserRemote.RequestsSummaryWritesCompareJSONS3Key = path.Join(
			cfg.AddOnStresserRemote.S3Dir,
			"requests-summary-writes-compare",
			filepath.Base(cfg.AddOnStresserRemote.RequestsSummaryWritesCompareJSONPath),
		)
	}
	if cfg.AddOnStresserRemote.RequestsSummaryWritesCompareTablePath == "" {
		cfg.AddOnStresserRemote.RequestsSummaryWritesCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-summary-writes-compare.txt"
	}
	if cfg.AddOnStresserRemote.RequestsSummaryWritesCompareTableS3Key == "" {
		cfg.AddOnStresserRemote.RequestsSummaryWritesCompareTableS3Key = path.Join(
			cfg.AddOnStresserRemote.S3Dir,
			"requests-summary-writes-compare",
			filepath.Base(cfg.AddOnStresserRemote.RequestsSummaryWritesCompareTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnStresserRemote.RequestsRawReadsJSONPath == "" {
		cfg.AddOnStresserRemote.RequestsRawReadsJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-raw-reads.json"
	}
	if cfg.AddOnStresserRemote.RequestsRawReadsJSONS3Key == "" {
		cfg.AddOnStresserRemote.RequestsRawReadsJSONS3Key = path.Join(
			cfg.AddOnStresserRemote.S3Dir,
			"requests-raw-reads",
			filepath.Base(cfg.AddOnStresserRemote.RequestsRawReadsJSONPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnStresserRemote.RequestsRawReadsCompareS3Dir == "" {
		cfg.AddOnStresserRemote.RequestsRawReadsCompareS3Dir = path.Join("add-on-stresser-remote", "requests-raw-reads-compare", cfg.Version)
	}
	if cfg.AddOnStresserRemote.RequestsRawReadsCompareAllJSONPath == "" {
		cfg.AddOnStresserRemote.RequestsRawReadsCompareAllJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-raw-reads-compare-all.json"
	}
	if cfg.AddOnStresserRemote.RequestsRawReadsCompareAllJSONS3Key == "" {
		cfg.AddOnStresserRemote.RequestsRawReadsCompareAllJSONS3Key = path.Join(
			cfg.AddOnStresserRemote.S3Dir,
			"requests-raw-reads-compare-all",
			filepath.Base(cfg.AddOnStresserRemote.RequestsRawReadsCompareAllJSONPath),
		)
	}
	if cfg.AddOnStresserRemote.RequestsRawReadsCompareAllCSVPath == "" {
		cfg.AddOnStresserRemote.RequestsRawReadsCompareAllCSVPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-raw-reads-compare-all.csv"
	}
	if cfg.AddOnStresserRemote.RequestsRawReadsCompareAllCSVS3Key == "" {
		cfg.AddOnStresserRemote.RequestsRawReadsCompareAllCSVS3Key = path.Join(
			cfg.AddOnStresserRemote.S3Dir,
			"requests-raw-reads-compare-all",
			filepath.Base(cfg.AddOnStresserRemote.RequestsRawReadsCompareAllCSVPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnStresserRemote.RequestsSummaryReadsJSONPath == "" {
		cfg.AddOnStresserRemote.RequestsSummaryReadsJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-summary-reads.json"
	}
	if cfg.AddOnStresserRemote.RequestsSummaryReadsJSONS3Key == "" {
		cfg.AddOnStresserRemote.RequestsSummaryReadsJSONS3Key = path.Join(
			cfg.AddOnStresserRemote.S3Dir,
			"requests-summary-reads",
			filepath.Base(cfg.AddOnStresserRemote.RequestsSummaryReadsJSONPath),
		)
	}
	if cfg.AddOnStresserRemote.RequestsSummaryReadsTablePath == "" {
		cfg.AddOnStresserRemote.RequestsSummaryReadsTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-summary-reads.txt"
	}
	if cfg.AddOnStresserRemote.RequestsSummaryReadsTableS3Key == "" {
		cfg.AddOnStresserRemote.RequestsSummaryReadsTableS3Key = path.Join(
			cfg.AddOnStresserRemote.S3Dir,
			"requests-summary-reads",
			filepath.Base(cfg.AddOnStresserRemote.RequestsSummaryReadsTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnStresserRemote.RequestsSummaryReadsCompareS3Dir == "" {
		cfg.AddOnStresserRemote.RequestsSummaryReadsCompareS3Dir = path.Join("add-on-stresser-remote", "requests-summary-reads-compare", cfg.Version)
	}
	if cfg.AddOnStresserRemote.RequestsSummaryReadsCompareJSONPath == "" {
		cfg.AddOnStresserRemote.RequestsSummaryReadsCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-summary-reads-compare.json"
	}
	if cfg.AddOnStresserRemote.RequestsSummaryReadsCompareJSONS3Key == "" {
		cfg.AddOnStresserRemote.RequestsSummaryReadsCompareJSONS3Key = path.Join(
			cfg.AddOnStresserRemote.S3Dir,
			"requests-summary-reads-compare",
			filepath.Base(cfg.AddOnStresserRemote.RequestsSummaryReadsCompareJSONPath),
		)
	}
	if cfg.AddOnStresserRemote.RequestsSummaryReadsCompareTablePath == "" {
		cfg.AddOnStresserRemote.RequestsSummaryReadsCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-summary-reads-compare.txt"
	}
	if cfg.AddOnStresserRemote.RequestsSummaryReadsCompareTableS3Key == "" {
		cfg.AddOnStresserRemote.RequestsSummaryReadsCompareTableS3Key = path.Join(
			cfg.AddOnStresserRemote.S3Dir,
			"requests-summary-reads-compare",
			filepath.Base(cfg.AddOnStresserRemote.RequestsSummaryReadsCompareTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	return nil
}
