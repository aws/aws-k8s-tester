package eksconfig

import (
	"errors"
	"fmt"
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
	// Objects is the number of "ConfigMap" objects to create.
	Objects int `json:"objects"`
	// ObjectSize is the "ConfigMap" value size in bytes.
	ObjectSize int `json:"object-size"`

	// CreatedNames is the list of created "ConfigMap" object names.
	CreatedNames []string `json:"created-names" read-only:"true"`

	// RequestsWritesJSONPath is the file path to store writes requests in JSON format.
	RequestsWritesJSONPath string `json:"requests-writes-json-path" read-only:"true"`
	// RequestsWritesSummary is the writes results.
	RequestsWritesSummary metrics.RequestsSummary `json:"requests-writes-summary,omitempty" read-only:"true"`
	// RequestsWritesSummaryJSONPath is the file path to store writes requests summary in JSON format.
	RequestsWritesSummaryJSONPath string `json:"requests-writes-summary-json-path" read-only:"true"`
	// RequestsWritesSummaryTablePath is the file path to store writes requests summary in table format.
	RequestsWritesSummaryTablePath string `json:"requests-writes-summary-table-path" read-only:"true"`

	// RequestsWritesSummaryOutputNamePrefix is the output path name in "/var/log" directory, used in remote worker.
	RequestsWritesSummaryOutputNamePrefix string `json:"requests-writes-summary-output-name-prefix"`
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
		Enable:             false,
		DeploymentReplicas: 5,
		Objects:            10,
		ObjectSize:         10 * 1024, // 10 KB

		// writes total 300 MB data to etcd
		// Objects: 1000,
		// ObjectSize: 300000, // 0.3 MB

		RequestsWritesSummaryOutputNamePrefix: "configmaps-writes" + randutil.String(10),
	}
}

func (cfg *Config) validateAddOnConfigmapsRemote() error {
	if !cfg.IsEnabledAddOnConfigmapsRemote() {
		return nil
	}
	if cfg.S3BucketName == "" {
		return errors.New("AddOnConfigmapsRemote requires S3 bucket for collecting results but S3BucketName empty")
	}

	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnConfigmapsRemote.Enable true but no node group is enabled")
	}

	if cfg.AddOnConfigmapsRemote.Namespace == "" {
		cfg.AddOnConfigmapsRemote.Namespace = cfg.Name + "-configmaps-remote"
	}

	if cfg.AddOnConfigmapsRemote.RepositoryAccountID == "" {
		return errors.New("AddOnConfigmapsRemote.RepositoryAccountID empty")
	}
	if cfg.AddOnConfigmapsRemote.RepositoryName == "" {
		return errors.New("AddOnConfigmapsRemote.RepositoryName empty")
	}
	if cfg.AddOnConfigmapsRemote.RepositoryImageTag == "" {
		return errors.New("AddOnConfigmapsRemote.RepositoryImageTag empty")
	}

	if cfg.AddOnConfigmapsRemote.DeploymentReplicas == 0 {
		cfg.AddOnConfigmapsRemote.DeploymentReplicas = 5
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

	if cfg.AddOnConfigmapsRemote.RequestsWritesJSONPath == "" {
		cfg.AddOnConfigmapsRemote.RequestsWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-remote-requests-writes.csv"
	}
	if cfg.AddOnConfigmapsRemote.RequestsWritesSummaryJSONPath == "" {
		cfg.AddOnConfigmapsRemote.RequestsWritesSummaryJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-remote-requests-writes-summary.json"
	}
	if cfg.AddOnConfigmapsRemote.RequestsWritesSummaryTablePath == "" {
		cfg.AddOnConfigmapsRemote.RequestsWritesSummaryTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-remote-requests-writes-summary.txt"
	}

	if cfg.AddOnConfigmapsRemote.RequestsWritesSummaryOutputNamePrefix == "" {
		cfg.AddOnConfigmapsRemote.RequestsWritesSummaryOutputNamePrefix = "configmaps-writes" + randutil.String(10)
	}

	return nil
}
