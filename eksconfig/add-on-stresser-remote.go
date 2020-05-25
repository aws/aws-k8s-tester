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
	// The cluster loader waits "one" "Duration" for hollow ones.
	// And other one for cluster loader.
	Duration       time.Duration `json:"duration,omitempty"`
	DurationString string        `json:"duration-string,omitempty" read-only:"true"`

	// RequestsSummaryWrites is the writes results.
	RequestsSummaryWrites metrics.RequestsSummary `json:"requests-summary-writes,omitempty" read-only:"true"`
	// RequestsSummaryWritesJSONPath is the file path to store writes requests summary in JSON format.
	RequestsSummaryWritesJSONPath string `json:"requests-summary-writes-json-path" read-only:"true"`
	// RequestsSummaryWritesTablePath is the file path to store writes requests summary in table format.
	RequestsSummaryWritesTablePath string `json:"requests-summary-writes-table-path" read-only:"true"`
	// RequestsSummaryReads is the reads results.
	RequestsSummaryReads metrics.RequestsSummary `json:"requests-summary-reads,omitempty" read-only:"true"`
	// RequestsSummaryReadsJSONPath is the file path to store reads requests summary in JSON format.
	RequestsSummaryReadsJSONPath string `json:"requests-summary-reads-json-path" read-only:"true"`
	// RequestsSummaryReadsTablePath is the file path to store reads requests summary in table format.
	RequestsSummaryReadsTablePath string `json:"requests-summary-reads-table-path" read-only:"true"`

	// RequestsSummaryWritesOutputNamePrefix is the output path name in "/var/log" directory, used in remote worker.
	RequestsSummaryWritesOutputNamePrefix string `json:"requests-summary-writes-output-name-prefix"`
	// RequestsSummaryReadsOutputNamePrefix is the output path name in "/var/log" directory, used in remote worker.
	RequestsSummaryReadsOutputNamePrefix string `json:"requests-summary-reads-output-name-prefix"`
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
		RequestsSummaryWritesOutputNamePrefix: "stresser-writes" + randutil.String(10),
		RequestsSummaryReadsOutputNamePrefix:  "stresser-reads" + randutil.String(10),
	}
}

func (cfg *Config) validateAddOnStresserRemote() error {
	if !cfg.IsEnabledAddOnStresserRemote() {
		return nil
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

	if cfg.AddOnStresserRemote.RequestsSummaryWritesJSONPath == "" {
		cfg.AddOnStresserRemote.RequestsSummaryWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-summary-writes.json"
	}
	if cfg.AddOnStresserRemote.RequestsSummaryWritesTablePath == "" {
		cfg.AddOnStresserRemote.RequestsSummaryWritesTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-summary-writes.txt"
	}
	if cfg.AddOnStresserRemote.RequestsSummaryReadsJSONPath == "" {
		cfg.AddOnStresserRemote.RequestsSummaryReadsJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-summary-reads.json"
	}
	if cfg.AddOnStresserRemote.RequestsSummaryReadsTablePath == "" {
		cfg.AddOnStresserRemote.RequestsSummaryReadsTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-stresser-remote-requests-summary-reads.txt"
	}

	if cfg.AddOnStresserRemote.RequestsSummaryWritesOutputNamePrefix == "" {
		cfg.AddOnStresserRemote.RequestsSummaryWritesOutputNamePrefix = "stresser-writes" + randutil.String(10)
	}
	if cfg.AddOnStresserRemote.RequestsSummaryReadsOutputNamePrefix == "" {
		cfg.AddOnStresserRemote.RequestsSummaryReadsOutputNamePrefix = "stresser-reads" + randutil.String(10)
	}

	return nil
}
