package eksconfig

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/randutil"
)

// AddOnHollowNodesRemote defines parameters for EKS cluster
// add-on local Hollow Nodes.
type AddOnHollowNodesRemote struct {
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

	// NodeLabelPrefix is the node prefix.
	NodeLabelPrefix string `json:"node-label-prefix"`
	// NodeLabels is the node labels to attach when creating hollow nodes.
	NodeLabels map[string]string `json:"node-labels" read-only:"true"`

	// Nodes is the number of hollow nodes to create.
	// If "Local" equals to "false", the number of nodes deployed
	// will be multiplied by "DeploymentReplicas".
	// e.g. each Pod creates 5 hollow nodes, while deployment replicas are 10.
	// The deployment will create total 50 nodes (= 5 times 10).
	Nodes int `json:"nodes"`
	// DeploymentReplicas is the number of replicas to deploy when hollow nodes are deployed via Pod.
	DeploymentReplicas int32 `json:"deployment-replicas,omitempty"`

	// MaxOpenFiles is number of files that can be opened by kubelet process.
	// "cmd/kubelet/app.rlimit.SetNumFiles(MaxOpenFiles)" sets this for the host.
	MaxOpenFiles int64 `json:"max-open-files"`


	// RepositoryAccountID is the account ID for tester ECR image.
	// e.g. "aws/aws-k8s-tester" for "[ACCOUNT_ID].dkr.ecr.us-west-2.amazonaws.com/aws/aws-k8s-tester"
	RepositoryAccountID string `json:"repository-account-id,omitempty"`
	// RepositoryName is the repositoryName for tester ECR image.
	// e.g. "aws/aws-k8s-tester" for "[ACCOUNT_ID].dkr.ecr.us-west-2.amazonaws.com/aws/aws-k8s-tester"
	RepositoryName string `json:"repository-name,omitempty"`
	// RepositoryURI is the repositoryUri for tester ECR image.
	// e.g. "[ACCOUNT_ID].dkr.ecr.us-west-2.amazonaws.com/aws/aws-k8s-tester"
	RepositoryURI string `json:"repository-uri,omitempty"`
	// RepositoryImageTag is the image tag for tester ECR image.
	// e.g. "latest" for image URI "[ACCOUNT_ID].dkr.ecr.us-west-2.amazonaws.com/aws/aws-k8s-tester:latest"
	RepositoryImageTag string `json:"repository-image-tag,omitempty"`

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
		Enable:             false,
		Nodes:              2,
		DeploymentReplicas: 5,
		MaxOpenFiles:       1000000,
	}
}

func (cfg *Config) validateAddOnHollowNodesRemote() error {
	if !cfg.IsEnabledAddOnHollowNodesRemote() {
		return nil
	}

	if cfg.AddOnHollowNodesRemote.Namespace == "" {
		cfg.AddOnHollowNodesRemote.Namespace = cfg.Name + "-hollow-nodes-remote"
	}

	if cfg.AddOnHollowNodesRemote.Nodes == 0 {
		cfg.AddOnHollowNodesRemote.Nodes = 2
	}
	if cfg.AddOnHollowNodesRemote.DeploymentReplicas == 0 {
		cfg.AddOnHollowNodesRemote.DeploymentReplicas = 5
	}

	// e.g. Unable to register node "fake-node-000004-evere" with API server: Node "fake-node-000004-evere" is invalid: [metadata.labels: Invalid value: "...-hollow-nodes-remote-fake-ami-type-duneg": must be no more than 63 characters, metadata.labels: Invalid value: "...-hollow-nodes-remote-fake-ng-name-duneg": must be no more than 63 characters, metadata.labels: Invalid value: "...-hollow-nodes-remote-fake-ng-type-duneg": must be no more than 63 characters]
	if cfg.AddOnHollowNodesRemote.NodeLabelPrefix == "" {
		cfg.AddOnHollowNodesRemote.NodeLabelPrefix = "fake" + randutil.String(10)
	}
	if len(cfg.AddOnHollowNodesRemote.NodeLabelPrefix) > 55 {
		return fmt.Errorf("invalid node label prefix %q (%d characters, label value can not be more than 63 characters)", cfg.AddOnHollowNodesRemote.NodeLabelPrefix, len(cfg.AddOnHollowNodesRemote.NodeLabelPrefix))
	}
	cfg.AddOnHollowNodesRemote.NodeLabels = map[string]string{
		"AMIType": cfg.AddOnHollowNodesRemote.NodeLabelPrefix + "-ami-type",
		"NGType":  cfg.AddOnHollowNodesRemote.NodeLabelPrefix + "-ng-type",
		"NGName":  cfg.AddOnHollowNodesRemote.NodeLabelPrefix + "-ng-name",
	}

	if cfg.AddOnHollowNodesRemote.MaxOpenFiles == 0 {
		cfg.AddOnHollowNodesRemote.MaxOpenFiles = 1000000
	}

	if cfg.AddOnHollowNodesRemote.RepositoryAccountID == "" {
		return errors.New("AddOnHollowNodesRemote.RepositoryAccountID empty")
	}
	if cfg.AddOnHollowNodesRemote.RepositoryName == "" {
		return errors.New("AddOnHollowNodesRemote.RepositoryName empty")
	}
	if cfg.AddOnHollowNodesRemote.RepositoryURI == "" {
		return errors.New("AddOnHollowNodesRemote.RepositoryURI empty")
	}
	if strings.Contains(cfg.AddOnHollowNodesRemote.RepositoryURI, cfg.AddOnHollowNodesRemote.RepositoryAccountID) {
		return fmt.Errorf("AddOnHollowNodesRemote.RepositoryURI %q does not have AddOnHollowNodesRemote.RepositoryAccountID %q", cfg.AddOnHollowNodesRemote.RepositoryURI, cfg.AddOnHollowNodesRemote.RepositoryAccountID)
	}
	if cfg.AddOnHollowNodesRemote.RepositoryImageTag == "" {
		return errors.New("AddOnHollowNodesRemote.RepositoryImageTag empty")
	}

	return nil
}
