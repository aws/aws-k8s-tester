package eksconfig

import (
	"errors"
	"fmt"
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

	// RequestsSummaryWrites is the writes results.
	RequestsSummaryWrites metrics.RequestsSummary `json:"requests-summary-writes,omitempty" read-only:"true"`
	// RequestsSummaryWritesJSONPath is the file path to store writes requests summary in JSON format.
	RequestsSummaryWritesJSONPath string `json:"requests-summary-writes-json-path" read-only:"true"`
	// RequestsSummaryWritesTablePath is the file path to store writes requests summary in table format.
	RequestsSummaryWritesTablePath string `json:"requests-summary-writes-table-path" read-only:"true"`

	// RequestsSummaryWritesOutputNamePrefix is the output path name in "/var/log" directory, used in remote worker.
	RequestsSummaryWritesOutputNamePrefix string `json:"requests-summary-writes-output-name-prefix"`
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
		DeploymentReplicas:                    5,
		Objects:                               10, // 1000 objects generates 5 MB data to etcd
		InitialRequestConditionType:           "",
		RequestsSummaryWritesOutputNamePrefix: "csrs-writes" + randutil.String(10),
	}
}

func (cfg *Config) validateAddOnCSRsRemote() error {
	if !cfg.IsEnabledAddOnCSRsRemote() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnCSRsRemote.Enable true but no node group is enabled")
	}

	if cfg.AddOnCSRsRemote.Namespace == "" {
		cfg.AddOnCSRsRemote.Namespace = cfg.Name + "-csrs-remote"
	}

	if cfg.AddOnCSRsRemote.RepositoryAccountID == "" {
		return errors.New("AddOnCSRsRemote.RepositoryAccountID empty")
	}
	if cfg.AddOnCSRsRemote.RepositoryName == "" {
		return errors.New("AddOnCSRsRemote.RepositoryName empty")
	}
	if cfg.AddOnCSRsRemote.RepositoryImageTag == "" {
		return errors.New("AddOnCSRsRemote.RepositoryImageTag empty")
	}

	if cfg.AddOnCSRsRemote.DeploymentReplicas == 0 {
		cfg.AddOnCSRsRemote.DeploymentReplicas = 5
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

	if cfg.AddOnCSRsRemote.RequestsSummaryWritesJSONPath == "" {
		cfg.AddOnCSRsRemote.RequestsSummaryWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-remote-requests-summary-writes.json"
	}
	if cfg.AddOnCSRsRemote.RequestsSummaryWritesTablePath == "" {
		cfg.AddOnCSRsRemote.RequestsSummaryWritesTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-csrs-remote-requests-summary-writes.txt"
	}

	if cfg.AddOnCSRsRemote.RequestsSummaryWritesOutputNamePrefix == "" {
		cfg.AddOnCSRsRemote.RequestsSummaryWritesOutputNamePrefix = "csrs-writes" + randutil.String(10)
	}

	return nil
}
