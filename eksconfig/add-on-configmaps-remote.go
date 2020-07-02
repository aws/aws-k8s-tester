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

// AddOnConfigmapsRemote defines parameters for EKS cluster
// add-on "ConfigMap" remote.
// It generates loads from the remote workers (Pod) in the cluster.
// Each worker writes serially with no concurrency.
// Configure "DeploymentReplicas" accordingly to increase the concurrency.
// The main use case is to write large objects to fill up etcd database.
type AddOnConfigmapsRemote struct {
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
	// The total number of objects to be created is "Completes" * "Objects".
	Completes int `json:"completes"`
	// Parallels is the the maximum desired number of pods the
	// job should run at any given time.
	// Write QPS will be client QPS * replicas.
	// Read QPS will be client QPS * replicas.
	// The total number of objects to be created is "Completes" * "Objects".
	Parallels int `json:"parallels"`

	// Objects is the number of "ConfigMap" objects to create.
	Objects int `json:"objects"`
	// ObjectSize is the "ConfigMap" value size in bytes.
	ObjectSize int `json:"object-size"`

	// CreatedNames is the list of created "ConfigMap" object names.
	CreatedNames []string `json:"created-names" read-only:"true"`

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

// EnvironmentVariablePrefixAddOnConfigmapsRemote is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnConfigmapsRemote = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_CONFIGMAPS_REMOTE_"

// IsEnabledAddOnConfigmapsRemote returns true if "AddOnConfigmapsRemote" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnConfigmapsRemote() bool {
	if cfg.AddOnConfigmapsRemote == nil {
		return false
	}
	if cfg.AddOnConfigmapsRemote.Enable {
		return true
	}
	cfg.AddOnConfigmapsRemote = nil
	return false
}

func getDefaultAddOnConfigmapsRemote() *AddOnConfigmapsRemote {
	return &AddOnConfigmapsRemote{
		Enable:     false,
		Completes:  5,
		Parallels:  5,
		Objects:    10,
		ObjectSize: 10 * 1024, // 10 KB

		// writes total 300 MB data to etcd
		// Objects: 1000,
		// ObjectSize: 300000, // 0.3 MB

		RequestsSummaryWritesOutputNamePrefix: "configmaps-writes-" + randutil.String(10),
	}
}

func (cfg *Config) validateAddOnConfigmapsRemote() error {
	if !cfg.IsEnabledAddOnConfigmapsRemote() {
		return nil
	}

	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnConfigmapsRemote.Enable true but no node group is enabled")
	}

	if cfg.AddOnConfigmapsRemote.S3Dir == "" {
		cfg.AddOnConfigmapsRemote.S3Dir = path.Join(cfg.Name, "add-on-configmaps-remote")
	}

	if cfg.AddOnConfigmapsRemote.Namespace == "" {
		cfg.AddOnConfigmapsRemote.Namespace = cfg.Name + "-configmaps-remote"
	}

	if cfg.AddOnConfigmapsRemote.RepositoryAccountID == "" {
		return errors.New("AddOnConfigmapsRemote.RepositoryAccountID empty")
	}
	if cfg.AddOnConfigmapsRemote.RepositoryRegion == "" {
		cfg.AddOnConfigmapsRemote.RepositoryRegion = cfg.Region
	}
	if cfg.AddOnConfigmapsRemote.RepositoryName == "" {
		return errors.New("AddOnConfigmapsRemote.RepositoryName empty")
	}
	if cfg.AddOnConfigmapsRemote.RepositoryImageTag == "" {
		return errors.New("AddOnConfigmapsRemote.RepositoryImageTag empty")
	}

	if cfg.AddOnConfigmapsRemote.Objects == 0 {
		cfg.AddOnConfigmapsRemote.Objects = 10
	}
	if cfg.AddOnConfigmapsRemote.ObjectSize == 0 {
		cfg.AddOnConfigmapsRemote.ObjectSize = 10 * 1024
	}
	if cfg.AddOnConfigmapsRemote.ObjectSize > 900000 {
		return fmt.Errorf("AddOnConfigmapsRemote.ObjectSize limit is 0.9 MB, got %d", cfg.AddOnConfigmapsRemote.ObjectSize)
	}

	if cfg.AddOnConfigmapsRemote.RequestsSummaryWritesOutputNamePrefix == "" {
		cfg.AddOnConfigmapsRemote.RequestsSummaryWritesOutputNamePrefix = "configmaps-writes-" + randutil.String(10)
	}

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnConfigmapsRemote.RequestsRawWritesJSONPath == "" {
		cfg.AddOnConfigmapsRemote.RequestsRawWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-remote-requests-writes-raw.json"
	}
	if cfg.AddOnConfigmapsRemote.RequestsRawWritesJSONS3Key == "" {
		cfg.AddOnConfigmapsRemote.RequestsRawWritesJSONS3Key = path.Join(
			cfg.AddOnConfigmapsRemote.S3Dir,
			"requests-raw-writes",
			filepath.Base(cfg.AddOnConfigmapsRemote.RequestsRawWritesJSONPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnConfigmapsRemote.RequestsRawWritesCompareS3Dir == "" {
		cfg.AddOnConfigmapsRemote.RequestsRawWritesCompareS3Dir = path.Join("add-on-configmaps-remote", "requests-raw-writes-compare", cfg.Parameters.Version)
	}
	if cfg.AddOnConfigmapsRemote.RequestsRawWritesCompareAllJSONPath == "" {
		cfg.AddOnConfigmapsRemote.RequestsRawWritesCompareAllJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-remote-requests-raw-writes-compare-all.json"
	}
	if cfg.AddOnConfigmapsRemote.RequestsRawWritesCompareAllJSONS3Key == "" {
		cfg.AddOnConfigmapsRemote.RequestsRawWritesCompareAllJSONS3Key = path.Join(
			cfg.AddOnConfigmapsRemote.S3Dir,
			"requests-raw-writes-compare-all",
			filepath.Base(cfg.AddOnConfigmapsRemote.RequestsRawWritesCompareAllJSONPath),
		)
	}
	if cfg.AddOnConfigmapsRemote.RequestsRawWritesCompareAllCSVPath == "" {
		cfg.AddOnConfigmapsRemote.RequestsRawWritesCompareAllCSVPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-remote-requests-raw-writes-compare-all.csv"
	}
	if cfg.AddOnConfigmapsRemote.RequestsRawWritesCompareAllCSVS3Key == "" {
		cfg.AddOnConfigmapsRemote.RequestsRawWritesCompareAllCSVS3Key = path.Join(
			cfg.AddOnConfigmapsRemote.S3Dir,
			"requests-raw-writes-compare-all",
			filepath.Base(cfg.AddOnConfigmapsRemote.RequestsRawWritesCompareAllCSVPath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnConfigmapsRemote.RequestsSummaryWritesJSONPath == "" {
		cfg.AddOnConfigmapsRemote.RequestsSummaryWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-remote-requests-summary-writes.json"
	}
	if cfg.AddOnConfigmapsRemote.RequestsSummaryWritesJSONS3Key == "" {
		cfg.AddOnConfigmapsRemote.RequestsSummaryWritesJSONS3Key = path.Join(
			cfg.AddOnConfigmapsRemote.S3Dir,
			"requests-summary-writes",
			filepath.Base(cfg.AddOnConfigmapsRemote.RequestsSummaryWritesJSONPath),
		)
	}
	if cfg.AddOnConfigmapsRemote.RequestsSummaryWritesTablePath == "" {
		cfg.AddOnConfigmapsRemote.RequestsSummaryWritesTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-remote-requests-summary-writes.txt"
	}
	if cfg.AddOnConfigmapsRemote.RequestsSummaryWritesTableS3Key == "" {
		cfg.AddOnConfigmapsRemote.RequestsSummaryWritesTableS3Key = path.Join(
			cfg.AddOnConfigmapsRemote.S3Dir,
			"requests-summary-writes",
			filepath.Base(cfg.AddOnConfigmapsRemote.RequestsSummaryWritesTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	//////////////////////////////////////////////////////////////////////////////
	if cfg.AddOnConfigmapsRemote.RequestsSummaryWritesCompareS3Dir == "" {
		cfg.AddOnConfigmapsRemote.RequestsSummaryWritesCompareS3Dir = path.Join("add-on-configmaps-remote", "requests-summary-writes-compare", cfg.Parameters.Version)
	}
	if cfg.AddOnConfigmapsRemote.RequestsSummaryWritesCompareJSONPath == "" {
		cfg.AddOnConfigmapsRemote.RequestsSummaryWritesCompareJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-remote-requests-summary-writes-compare.json"
	}
	if cfg.AddOnConfigmapsRemote.RequestsSummaryWritesCompareJSONS3Key == "" {
		cfg.AddOnConfigmapsRemote.RequestsSummaryWritesCompareJSONS3Key = path.Join(
			cfg.AddOnConfigmapsRemote.S3Dir,
			"requests-summary-writes-compare",
			filepath.Base(cfg.AddOnConfigmapsRemote.RequestsSummaryWritesCompareJSONPath),
		)
	}
	if cfg.AddOnConfigmapsRemote.RequestsSummaryWritesCompareTablePath == "" {
		cfg.AddOnConfigmapsRemote.RequestsSummaryWritesCompareTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-remote-requests-summary-writes-compare.txt"
	}
	if cfg.AddOnConfigmapsRemote.RequestsSummaryWritesCompareTableS3Key == "" {
		cfg.AddOnConfigmapsRemote.RequestsSummaryWritesCompareTableS3Key = path.Join(
			cfg.AddOnConfigmapsRemote.S3Dir,
			"requests-summary-writes-compare",
			filepath.Base(cfg.AddOnConfigmapsRemote.RequestsSummaryWritesCompareTablePath),
		)
	}
	//////////////////////////////////////////////////////////////////////////////

	return nil
}
