package eksconfig

import (
	"errors"
	"time"
)

// AddOnHollowNodes defines parameters for EKS cluster
// add-on Hollow Nodes.
type AddOnHollowNodes struct {
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

	// Nodes is the number of hollow nodes to create.
	// If "Local" equals to "false", the number of nodes deployed
	// will be multiplied by "DeploymentReplicas".
	// e.g. each Pod creates 5 hollow nodes, while deployment replicas are 10.
	// The deployment will create total 50 nodes (= 5 times 10).
	Nodes int `json:"nodes"`
	// NodeLabelPrefix is the node prefix.
	NodeLabelPrefix string `json:"node-label-prefix"`
	// NodeLabels is the node labels to attach when creating hollow nodes.
	NodeLabels map[string]string `json:"node-labels" read-only:"true"`

	// MaxOpenFiles is number of files that can be opened by hollow node kubelet process.
	// "cmd/kubelet/app.rlimit.SetNumFiles(MaxOpenFiles)" sets this for the host.
	MaxOpenFiles int64 `json:"max-open-files"`

	// Remote is true to create hollow nodes in remote Pods.
	// If false, it run hollow node kubelets in the local host system.
	// More than >100 node may exceed local system file descriptor limits.
	// e.g. too many open files
	// If true, create hollow nodes using Kubernetes Pod.
	Remote bool `json:"remote"`

	// RepositoryName is the repositoryName for hollow node tester.
	// e.g. "aws/aws-k8s-tester" for "[ACCOUNT_ID].dkr.ecr.us-west-2.amazonaws.com/aws/aws-k8s-tester"
	RepositoryName string `json:"repository-name,omitempty"`
	// RepositoryURI is the repositoryUri for hollow node tester.
	// e.g. "[ACCOUNT_ID].dkr.ecr.us-west-2.amazonaws.com/aws/aws-k8s-tester"
	RepositoryURI string `json:"repository-uri,omitempty"`
	// RepositoryImageTag is the image tag for hollow node tester.
	// e.g. "latest" for image URI "[ACCOUNT_ID].dkr.ecr.us-west-2.amazonaws.com/aws/aws-k8s-tester:latest"
	RepositoryImageTag string `json:"repository-image-tag,omitempty"`
	// DeploymentReplicas is the number of replicas to deploy when hollow nodes are deployed via Pod.
	DeploymentReplicas int32 `json:"deployment-replicas,omitempty"`

	// CreatedNodeNames is the list of created "Node" object names.
	CreatedNodeNames []string `json:"created-node-names" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnHollowNodes is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnHollowNodes = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_HOLLOW_NODES_"

// IsEnabledAddOnHollowNodes returns true if "AddOnHollowNodes" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnHollowNodes() bool {
	if cfg.AddOnHollowNodes == nil {
		return false
	}
	if cfg.AddOnHollowNodes.Enable {
		return true
	}
	cfg.AddOnHollowNodes = nil
	return false
}

func getDefaultAddOnHollowNodes() *AddOnHollowNodes {
	return &AddOnHollowNodes{
		Enable:             false,
		Nodes:              2,
		MaxOpenFiles:       1000000,
		Remote:             false,
		DeploymentReplicas: 5,
	}
}

func (cfg *Config) validateAddOnHollowNodes() error {
	if !cfg.IsEnabledAddOnHollowNodes() {
		return nil
	}

	if cfg.AddOnHollowNodes.Namespace == "" {
		cfg.AddOnHollowNodes.Namespace = cfg.Name + "-hollow-nodes"
	}

	if cfg.AddOnHollowNodes.Nodes == 0 {
		cfg.AddOnHollowNodes.Nodes = 2
	}

	if cfg.AddOnHollowNodes.NodeLabelPrefix == "" {
		cfg.AddOnHollowNodes.NodeLabelPrefix = cfg.Name + "-hollow-nodes-fake"
	}

	cfg.AddOnHollowNodes.NodeLabels = map[string]string{
		"NGType":  cfg.AddOnHollowNodes.NodeLabelPrefix + "-ng-type",
		"AMIType": cfg.AddOnHollowNodes.NodeLabelPrefix + "-ami-type",
		"NGName":  cfg.AddOnHollowNodes.NodeLabelPrefix + "-ng-name",
	}

	if cfg.AddOnHollowNodes.MaxOpenFiles == 0 {
		cfg.AddOnHollowNodes.MaxOpenFiles = 1000000
	}

	if cfg.AddOnHollowNodes.Remote {
		if cfg.AddOnHollowNodes.RepositoryName == "" {
			return errors.New("AddOnHollowNodes.Local empty, expected non-empty RepositoryName")
		}
		if cfg.AddOnHollowNodes.RepositoryURI == "" {
			return errors.New("AddOnHollowNodes.Local empty, expected non-empty RepositoryURI")
		}
		if cfg.AddOnHollowNodes.RepositoryImageTag == "" {
			return errors.New("AddOnHollowNodes.Local empty, expected non-empty RepositoryImageTag")
		}
		if cfg.AddOnHollowNodes.DeploymentReplicas == 0 {
			cfg.AddOnHollowNodes.DeploymentReplicas = 5
		}
	}

	return nil
}
