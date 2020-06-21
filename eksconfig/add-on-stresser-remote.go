package eksconfig

import (
	"errors"
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
	// Write QPS will be client QPS * replicas.
	// Read QPS will be client QPS * replicas.
	DeploymentReplicas int32 `json:"deployment-replicas,omitempty"`
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

	// RequestsWritesJSONPath is the file path to store writes requests in JSON format.
	RequestsWritesJSONPath string `json:"requests-writes-json-path" read-only:"true"`
	// RequestsWritesSummary is the writes results.
	RequestsWritesSummary metrics.RequestsSummary `json:"requests-writes-summary,omitempty" read-only:"true"`
	// RequestsWritesSummaryJSONPath is the file path to store writes requests summary in JSON format.
	RequestsWritesSummaryJSONPath string `json:"requests-writes-summary-json-path" read-only:"true"`
	// RequestsWritesSummaryTablePath is the file path to store writes requests summary in table format.
	RequestsWritesSummaryTablePath string `json:"requests-writes-summary-table-path" read-only:"true"`

	// RequestsReadsJSONPath is the file path to store reads requests in JSON format.
	RequestsReadsJSONPath string `json:"requests-reads-json-path" read-only:"true"`
	// RequestsReadsSummary is the reads results.
	RequestsReadsSummary metrics.RequestsSummary `json:"requests-reads-summary,omitempty" read-only:"true"`
	// RequestsReadsSummaryJSONPath is the file path to store reads requests summary in JSON format.
	RequestsReadsSummaryJSONPath string `json:"requests-reads-summary-json-path" read-only:"true"`
	// RequestsReadsSummaryTablePath is the file path to store reads requests summary in table format.
	RequestsReadsSummaryTablePath string `json:"requests-reads-summary-table-path" read-only:"true"`

	// RequestsWritesSummaryOutputNamePrefix is the output path name in "/var/log" directory, used in remote worker.
	RequestsWritesSummaryOutputNamePrefix string `json:"requests-writes-summary-output-name-prefix"`
	// RequestsReadsSummaryOutputNamePrefix is the output path name in "/var/log" directory, used in remote worker.
	RequestsReadsSummaryOutputNamePrefix string `json:"requests-reads-summary-output-name-prefix"`
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
		DeploymentReplicas:                    5,
		ObjectSize:                            0,
		ListLimit:                             0,
		Duration:                              time.Minute,
		RequestsWritesSummaryOutputNamePrefix: "stresser-writes" + randutil.String(10),
		RequestsReadsSummaryOutputNamePrefix:  "stresser-reads" + randutil.String(10),
	}
}

func (cfg *Config) validateAddOnStresserRemote() error {
	if !cfg.IsEnabledAddOnStresserRemote() {
		return nil
	}
	if cfg.S3BucketName == "" {
		return errors.New("AddOnStresserRemote requires S3 bucket but S3BucketName empty")
	}

	if cfg.AddOnStresserRemote.Namespace == "" {
		cfg.AddOnStresserRemote.Namespace = cfg.Name + "-stresser-remote"
	}

	if cfg.AddOnStresserRemote.RepositoryAccountID == "" {
		return errors.New("AddOnStresserRemote.RepositoryAccountID empty")
	}
	if cfg.AddOnStresserRemote.RepositoryName == "" {
		return errors.New("AddOnStresserRemote.RepositoryName empty")
	}
	if cfg.AddOnStresserRemote.RepositoryImageTag == "" {
		return errors.New("AddOnStresserRemote.RepositoryImageTag empty")
	}

	if cfg.AddOnStresserRemote.DeploymentReplicas == 0 {
		cfg.AddOnStresserRemote.DeploymentReplicas = 5
	}
	if cfg.AddOnStresserRemote.Duration == time.Duration(0) {
		cfg.AddOnStresserRemote.Duration = time.Minute
	}
	cfg.AddOnStresserRemote.DurationString = cfg.AddOnStresserRemote.Duration.String()

	if cfg.AddOnStresserRemote.RequestsWritesJSONPath == "" {
		cfg.AddOnStresserRemote.RequestsWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-writes.json"
	}
	if cfg.AddOnStresserRemote.RequestsWritesSummaryJSONPath == "" {
		cfg.AddOnStresserRemote.RequestsWritesSummaryJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-writes-summary.json"
	}
	if cfg.AddOnStresserRemote.RequestsWritesSummaryTablePath == "" {
		cfg.AddOnStresserRemote.RequestsWritesSummaryTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-writes-summary.txt"
	}

	if cfg.AddOnStresserRemote.RequestsReadsJSONPath == "" {
		cfg.AddOnStresserRemote.RequestsReadsJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-reads.json"
	}
	if cfg.AddOnStresserRemote.RequestsReadsSummaryJSONPath == "" {
		cfg.AddOnStresserRemote.RequestsReadsSummaryJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-reads-summary.json"
	}
	if cfg.AddOnStresserRemote.RequestsReadsSummaryTablePath == "" {
		cfg.AddOnStresserRemote.RequestsReadsSummaryTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-reads-summary.txt"
	}

	if cfg.AddOnStresserRemote.RequestsWritesSummaryOutputNamePrefix == "" {
		cfg.AddOnStresserRemote.RequestsWritesSummaryOutputNamePrefix = "stresser-writes" + randutil.String(10)
	}
	if cfg.AddOnStresserRemote.RequestsReadsSummaryOutputNamePrefix == "" {
		cfg.AddOnStresserRemote.RequestsReadsSummaryOutputNamePrefix = "stresser-reads" + randutil.String(10)
	}

	return nil
}
