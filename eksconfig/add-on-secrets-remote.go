package eksconfig

import (
	"errors"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/metrics"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
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
	Created bool `json:"created" read-only:"true"`
	// CreateTook is the duration that took to create the resource.
	CreateTook time.Duration `json:"create-took,omitempty" read-only:"true"`
	// CreateTookString is the duration that took to create the resource.
	CreateTookString string `json:"create-took-string,omitempty" read-only:"true"`
	// DeleteTook is the duration that took to create the resource.
	DeleteTook time.Duration `json:"delete-took,omitempty" read-only:"true"`
	// DeleteTookString is the duration that took to create the resource.
	DeleteTookString string `json:"delete-took-string,omitempty" read-only:"true"`

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

	// RequestsSummaryWrites is the writes results.
	RequestsSummaryWrites metrics.RequestsSummary `json:"requests-summary-writes,omitempty" read-only:"true"`
	// RequestsSummaryWritesJSONPath is the file path to store writes requests summary in JSON format.
	RequestsSummaryWritesJSONPath string `json:"requests-summary-writes-json-path" read-only:"true"`
	// RequestsSummaryWritesTablePath is the file path to store writes requests summary in table format.
	RequestsSummaryWritesTablePath string `json:"requests-summary-writes-table-path" read-only:"true"`
	// RequestsSummaryReads is the reads results.
	// Reads happen inside "Pod" where it reads "Secret" objects from the mounted volume.
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

		RequestsSummaryWritesOutputNamePrefix: "secrets-writes" + randutil.String(10),
		RequestsSummaryReadsOutputNamePrefix:  "secrets-reads" + randutil.String(10),
	}
}

func (cfg *Config) validateAddOnSecretsRemote() error {
	if !cfg.IsEnabledAddOnSecretsRemote() {
		return nil
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

	if cfg.AddOnSecretsRemote.RequestsSummaryWritesJSONPath == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-remote-requests-summary-writes.json"
	}
	if cfg.AddOnSecretsRemote.RequestsSummaryWritesTablePath == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryWritesTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-remote-requests-summary-writes.txt"
	}
	if cfg.AddOnSecretsRemote.RequestsSummaryReadsJSONPath == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryReadsJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-remote-requests-summary-reads.json"
	}
	if cfg.AddOnSecretsRemote.RequestsSummaryReadsTablePath == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryReadsTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-remote-requests-summary-reads.txt"
	}

	if cfg.AddOnSecretsRemote.RequestsSummaryWritesOutputNamePrefix == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryWritesOutputNamePrefix = "secrets-writes" + randutil.String(10)
	}
	if cfg.AddOnSecretsRemote.RequestsSummaryReadsOutputNamePrefix == "" {
		cfg.AddOnSecretsRemote.RequestsSummaryReadsOutputNamePrefix = "secrets-reads" + randutil.String(10)
	}

	return nil
}
