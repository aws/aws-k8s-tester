package eksconfig

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-k8s-tester/pkg/metrics"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnConfigMapsRemote defines parameters for EKS cluster
// add-on "ConfigMap" remote.
// It generates loads from the remote workers (Pod) in the cluster.
// Each worker writes serially with no concurrency.
// Configure "DeploymentReplicas" accordingly to increase the concurrency.
// The main use case is to write large objects to fill up etcd database.
type AddOnConfigMapsRemote struct {
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

// EnvironmentVariablePrefixAddOnConfigMapsRemote is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnConfigMapsRemote = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_CONFIG_MAPS_REMOTE_"

// IsEnabledAddOnConfigMapsRemote returns true if "AddOnConfigMapsRemote" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnConfigMapsRemote() bool {
	if cfg.AddOnConfigMapsRemote == nil {
		return false
	}
	if cfg.AddOnConfigMapsRemote.Enable {
		return true
	}
	cfg.AddOnConfigMapsRemote = nil
	return false
}

func getDefaultAddOnConfigMapsRemote() *AddOnConfigMapsRemote {
	return &AddOnConfigMapsRemote{
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

func (cfg *Config) validateAddOnConfigMapsRemote() error {
	if !cfg.IsEnabledAddOnConfigMapsRemote() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnConfigMapsRemote.Enable true but no node group is enabled")
	}

	if cfg.AddOnConfigMapsRemote.Namespace == "" {
		cfg.AddOnConfigMapsRemote.Namespace = cfg.Name + "-configmaps-remote"
	}

	if cfg.AddOnConfigMapsRemote.RepositoryAccountID == "" {
		return errors.New("AddOnConfigMapsRemote.RepositoryAccountID empty")
	}
	if cfg.AddOnConfigMapsRemote.RepositoryName == "" {
		return errors.New("AddOnConfigMapsRemote.RepositoryName empty")
	}
	if cfg.AddOnConfigMapsRemote.RepositoryImageTag == "" {
		return errors.New("AddOnConfigMapsRemote.RepositoryImageTag empty")
	}

	if cfg.AddOnConfigMapsRemote.DeploymentReplicas == 0 {
		cfg.AddOnConfigMapsRemote.DeploymentReplicas = 5
	}
	if cfg.AddOnConfigMapsRemote.Objects == 0 {
		cfg.AddOnConfigMapsRemote.Objects = 10
	}
	if cfg.AddOnConfigMapsRemote.ObjectSize == 0 {
		cfg.AddOnConfigMapsRemote.ObjectSize = 10 * 1024
	}
	if cfg.AddOnConfigMapsRemote.ObjectSize > 900000 {
		return fmt.Errorf("AddOnConfigMapsRemote.ObjectSize limit is 0.9 MB, got %d", cfg.AddOnConfigMapsRemote.ObjectSize)
	}

	if cfg.AddOnConfigMapsRemote.RequestsWritesJSONPath == "" {
		cfg.AddOnConfigMapsRemote.RequestsWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-remote-requests-writes.csv"
	}
	if cfg.AddOnConfigMapsRemote.RequestsWritesSummaryJSONPath == "" {
		cfg.AddOnConfigMapsRemote.RequestsWritesSummaryJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-remote-requests-writes-summary.json"
	}
	if cfg.AddOnConfigMapsRemote.RequestsWritesSummaryTablePath == "" {
		cfg.AddOnConfigMapsRemote.RequestsWritesSummaryTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-configmaps-remote-requests-writes-summary.txt"
	}

	if cfg.AddOnConfigMapsRemote.RequestsWritesSummaryOutputNamePrefix == "" {
		cfg.AddOnConfigMapsRemote.RequestsWritesSummaryOutputNamePrefix = "configmaps-writes" + randutil.String(10)
	}

	return nil
}
