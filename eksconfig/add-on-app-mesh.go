package eksconfig

import (
	"errors"
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnAppMesh defines parameters for EKS cluster
// add-on "EKS App Mesh Integration".
type AddOnAppMesh struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// S3Dir is the S3 directory to store all test results.
	// It is under the bucket "eksconfig.Config.S3BucketName".
	S3Dir string `json:"s3-dir"`

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`

	// ControllerImage is the image of appMesh controller
	ControllerImage string `json:"controller-image"`
	// InjectorImage is the image of appMesh injector
	InjectorImage string `json:"injector-image"`

	// PolicyCFNStackID is the CFN stack ID for policy.
	PolicyCFNStackID        string `json:"policy-cfn-stack-id,omitempty" read-only:"true"`
	PolicyCFNStackYAMLPath  string `json:"policy-cfn-stack-yaml-path" read-only:"true"`
	PolicyCFNStackYAMLS3Key string `json:"policy-cfn-stack-yaml-s3-key" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnAppMesh is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnAppMesh = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_APP_MESH_"

// IsEnabledAddOnAppMesh returns true if "AddOnAppMesh" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnAppMesh() bool {
	if cfg.AddOnAppMesh == nil {
		return false
	}
	if cfg.AddOnAppMesh.Enable {
		return true
	}
	cfg.AddOnAppMesh = nil
	return false
}

func getDefaultAddOnAppMesh() *AddOnAppMesh {
	return &AddOnAppMesh{
		Enable: false,
	}
}

func (cfg *Config) validateAddOnAppMesh() error {
	if !cfg.IsEnabledAddOnAppMesh() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnAppMesh.Enable true but no node group is enabled")
	}

	if cfg.AddOnAppMesh.S3Dir == "" {
		cfg.AddOnAppMesh.S3Dir = path.Join(cfg.Name, "add-on-app-mesh")
	}

	if cfg.AddOnAppMesh.Namespace == "" {
		cfg.AddOnAppMesh.Namespace = cfg.Name + "-appmesh"
	}

	if cfg.AddOnAppMesh.PolicyCFNStackYAMLPath == "" {
		cfg.AddOnAppMesh.PolicyCFNStackYAMLPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".add-on-app-mesh.policy.cfn.yaml"
	}
	if cfg.AddOnAppMesh.PolicyCFNStackYAMLS3Key == "" {
		cfg.AddOnAppMesh.PolicyCFNStackYAMLS3Key = path.Join(
			cfg.AddOnAppMesh.S3Dir,
			filepath.Base(cfg.AddOnAppMesh.PolicyCFNStackYAMLPath),
		)
	}

	return nil
}
