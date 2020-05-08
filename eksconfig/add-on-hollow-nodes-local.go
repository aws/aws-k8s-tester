package eksconfig

import (
	"time"
)

// AddOnHollowNodesLocal defines parameters for EKS cluster
// add-on local Hollow Nodes.
type AddOnHollowNodesLocal struct {
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

	// MaxOpenFiles is number of files that can be opened by hollow node kubelet process.
	// "cmd/kubelet/app.rlimit.SetNumFiles(MaxOpenFiles)" sets this for the host.
	// More than >100 node may exceed local system file descriptor limits.
	// e.g. too many open files
	MaxOpenFiles int64 `json:"max-open-files"`

	// CreatedNodeNames is the list of created "Node" object names.
	CreatedNodeNames []string `json:"created-node-names" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnHollowNodesLocal is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnHollowNodesLocal = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_HOLLOW_NODES_LOCAL_"

// IsEnabledAddOnHollowNodesLocal returns true if "AddOnHollowNodesLocal" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnHollowNodesLocal() bool {
	if cfg.AddOnHollowNodesLocal == nil {
		return false
	}
	if cfg.AddOnHollowNodesLocal.Enable {
		return true
	}
	cfg.AddOnHollowNodesLocal = nil
	return false
}

func getDefaultAddOnHollowNodesLocal() *AddOnHollowNodesLocal {
	return &AddOnHollowNodesLocal{
		Enable:       false,
		Nodes:        2,
		MaxOpenFiles: 1000000,
	}
}

func (cfg *Config) validateAddOnHollowNodesLocal() error {
	if !cfg.IsEnabledAddOnHollowNodesLocal() {
		return nil
	}

	if cfg.AddOnHollowNodesLocal.Nodes == 0 {
		cfg.AddOnHollowNodesLocal.Nodes = 2
	}

	if cfg.AddOnHollowNodesLocal.NodeLabelPrefix == "" {
		cfg.AddOnHollowNodesLocal.NodeLabelPrefix = cfg.Name + "-hollow-nodes-local-fake"
	}
	cfg.AddOnHollowNodesLocal.NodeLabels = map[string]string{
		"AMIType": cfg.AddOnHollowNodesLocal.NodeLabelPrefix + "-ami-type",
		"NGType":  cfg.AddOnHollowNodesLocal.NodeLabelPrefix + "-ng-type",
		"NGName":  cfg.AddOnHollowNodesLocal.NodeLabelPrefix + "-ng-name",
	}

	if cfg.AddOnHollowNodesLocal.MaxOpenFiles == 0 {
		cfg.AddOnHollowNodesLocal.MaxOpenFiles = 1000000
	}

	return nil
}
