package eksconfig

import (
	"errors"
	"fmt"

	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnHollowNodesRemote defines parameters for EKS cluster
// add-on hollow nodes remote.
// It generates loads from the remote workers (Pod) in the cluster.
// Each worker writes serially with no concurrency.
// Configure "NodeGroups" accordingly to increase the concurrency.
type AddOnHollowNodesRemote struct {
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
	// RepositoryRegion is the ECR repository region to pull from.
	RepositoryRegion string `json:"repository-region,omitempty"`
	// RepositoryName is the repositoryName for tester ECR image.
	// e.g. "aws/aws-k8s-tester" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/aws/aws-k8s-tester"
	RepositoryName string `json:"repository-name,omitempty"`
	// RepositoryImageTag is the image tag for tester ECR image.
	// e.g. "latest" for image URI "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/aws/aws-k8s-tester:latest"
	RepositoryImageTag string `json:"repository-image-tag,omitempty"`

	// NodeGroups is the number of replicas to create for workers.
	// The total number of objects to be created is "NodeGroups" * "Nodes".
	NodeGroups int `json:"node-groups,omitempty"`
	// Nodes is the number of hollow nodes to create.
	// The total number of objects to be created is "NodeGroups" * "Nodes".
	Nodes int `json:"nodes"`

	// NodeLabelPrefix is the node prefix.
	NodeLabelPrefix string `json:"node-label-prefix"`

	// MaxOpenFiles is number of files that can be opened by kubelet process.
	// "cmd/kubelet/app.rlimit.SetNumFiles(MaxOpenFiles)" sets this for the host.
	MaxOpenFiles int64 `json:"max-open-files"`

	// CreatedNodeNames is the list of created "Node" object names.
	CreatedNodeNames []string `json:"created-node-names" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnHollowNodesRemote is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnHollowNodesRemote = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_HOLLOW_NODES_REMOTE_"

// IsEnabledAddOnHollowNodesRemote returns true if "AddOnHollowNodesRemote" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnHollowNodesRemote() bool {
	if cfg.AddOnHollowNodesRemote == nil {
		return false
	}
	if cfg.AddOnHollowNodesRemote.Enable {
		return true
	}
	cfg.AddOnHollowNodesRemote = nil
	return false
}

func getDefaultAddOnHollowNodesRemote() *AddOnHollowNodesRemote {
	return &AddOnHollowNodesRemote{
		Enable:          false,
		NodeGroups:      5,
		Nodes:           2,
		NodeLabelPrefix: "hollow" + randutil.String(5),
		MaxOpenFiles:    1000000,
	}
}

func (cfg *Config) GetAddOnHollowNodesRemoteRepositoryRegion() string {
	if !cfg.IsEnabledAddOnHollowNodesRemote() {
		return cfg.Region
	}
	return cfg.AddOnHollowNodesRemote.RepositoryRegion
}

func (cfg *Config) validateAddOnHollowNodesRemote() error {
	if !cfg.IsEnabledAddOnHollowNodesRemote() {
		return nil
	}

	if cfg.AddOnHollowNodesRemote.Namespace == "" {
		cfg.AddOnHollowNodesRemote.Namespace = "kubemark"
	}

	if cfg.AddOnHollowNodesRemote.RepositoryAccountID == "" {
		return errors.New("AddOnHollowNodesRemote.RepositoryAccountID empty")
	}
	if cfg.AddOnHollowNodesRemote.RepositoryRegion == "" {
		cfg.AddOnHollowNodesRemote.RepositoryRegion = cfg.Region
	}
	if cfg.AddOnHollowNodesRemote.RepositoryName == "" {
		return errors.New("AddOnHollowNodesRemote.RepositoryName empty")
	}
	if cfg.AddOnHollowNodesRemote.RepositoryImageTag == "" {
		return errors.New("AddOnHollowNodesRemote.RepositoryImageTag empty")
	}

	if cfg.AddOnHollowNodesRemote.NodeGroups == 0 {
		cfg.AddOnHollowNodesRemote.NodeGroups = 5
	}
	if cfg.AddOnHollowNodesRemote.Nodes == 0 {
		cfg.AddOnHollowNodesRemote.Nodes = 1
	}

	// e.g. Unable to register node "fake-node-000004-evere" with API server: Node "fake-node-000004-evere" is invalid: [metadata.labels: Invalid value: "...-hollow-nodes-remote-fake-ami-type-duneg": must be no more than 63 characters, metadata.labels: Invalid value: "...-hollow-nodes-remote-fake-ng-name-duneg": must be no more than 63 characters, metadata.labels: Invalid value: "...-hollow-nodes-remote-fake-ng-type-duneg": must be no more than 63 characters]
	if cfg.AddOnHollowNodesRemote.NodeLabelPrefix == "" {
		cfg.AddOnHollowNodesRemote.NodeLabelPrefix = "hollow" + randutil.String(5)
	}
	if len(cfg.AddOnHollowNodesRemote.NodeLabelPrefix) > 55 {
		return fmt.Errorf("invalid node label prefix %q (%d characters, label value can not be more than 63 characters)", cfg.AddOnHollowNodesRemote.NodeLabelPrefix, len(cfg.AddOnHollowNodesRemote.NodeLabelPrefix))
	}

	if cfg.AddOnHollowNodesRemote.MaxOpenFiles == 0 {
		cfg.AddOnHollowNodesRemote.MaxOpenFiles = 1000000
	}

	return nil
}
