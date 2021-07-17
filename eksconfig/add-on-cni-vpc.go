package eksconfig

import (
	"errors"
	"fmt"

	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnCNIVPC defines parameters for https://github.com/aws/amazon-vpc-cni-k8s.
// If not enabled, uses the default one.
// The version must be at least "v1.7".
// ref. https://github.com/aws/amazon-vpc-cni-k8s/blob/release-1.7/config/v1.7/aws-k8s-cni.yaml
type AddOnCNIVPC struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// Version defines the release version for https://github.com/aws/amazon-vpc-cni-k8s.
	// Must be at least "v1.7".
	// "v1.6" is not supported.
	// ref. https://github.com/aws/amazon-vpc-cni-k8s/releases
	Version string `json:"version"`

	// RepositoryInitAccountID is the account ID for tester ECR image.
	// e.g. "602401143452" for "602401143452.dkr.ecr.[REGION].amazonaws.com/amazon-k8s-cni-init"
	RepositoryInitAccountID string `json:"repository-init-account-id,omitempty"`
	// RepositoryInitRegion is the ECR repository region to pull from.
	RepositoryInitRegion string `json:"repository-init-region,omitempty"`
	// RepositoryInitName is the repositoryName for tester ECR image.
	// e.g. "amazon-k8s-cni-init" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/amazon-k8s-cni-init"
	RepositoryInitName string `json:"repository-init-name,omitempty"`
	// RepositoryInitImageTag is the image tag for tester ECR image.
	// e.g. "v1.7.0-rc1" for image URI "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/amazon-k8s-cni-init:v1.7.0-rc1"
	RepositoryInitImageTag string `json:"repository-init-image-tag,omitempty"`

	// RepositoryAccountID is the account ID for tester ECR image.
	// e.g. "602401143452" for "602401143452.dkr.ecr.[REGION].amazonaws.com/amazon-k8s-cni"
	RepositoryAccountID string `json:"repository-account-id,omitempty"`
	// RepositoryRegion is the ECR repository region to pull from.
	RepositoryRegion string `json:"repository-region,omitempty"`
	// RepositoryName is the repositoryName for tester ECR image.
	// e.g. "amazon-k8s-cni" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/amazon-k8s-cni"
	RepositoryName string `json:"repository-name,omitempty"`
	// RepositoryImageTag is the image tag for tester ECR image.
	// e.g. "v1.7.0-rc1" for image URI "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/amazon-k8s-cni:v1.7.0-rc1"
	RepositoryImageTag string `json:"repository-image-tag,omitempty"`

	// MinimumIPTarget configures "MINIMUM_IP_TARGET" for VPC CNI plugin daemon set.
	// ref. https://github.com/aws/amazon-vpc-cni-k8s#cni-configuration-variables
	MinimumIPTarget int `json:"minimum-ip-target"`
	// WarmIPTarget configures "WARM_IP_TARGET" for VPC CNI plugin daemon set.
	// ref. https://github.com/aws/amazon-vpc-cni-k8s#cni-configuration-variables
	WarmIPTarget int `json:"warm-ip-target"`

	// NodeSelector is configured to overwrite existing node selector
	// for amazon-vpc-cni-k8s DaemonSet.
	NodeSelector map[string]string `json:"node-selector"`
}

// AWS_K8S_TESTER_EKS_ADD_ON_CNI_VPC_PREFIX is the environment variable prefix used for "eksconfig".
const AWS_K8S_TESTER_EKS_ADD_ON_CNI_VPC_PREFIX = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_CNI_VPC_"

// IsEnabledAddOnCNIVPC returns true if "AddOnCNIVPC" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnCNIVPC() bool {
	if cfg.AddOnCNIVPC == nil {
		return false
	}
	if cfg.AddOnCNIVPC.Enable {
		return true
	}
	cfg.AddOnCNIVPC = nil
	return false
}

func getDefaultAddOnCNIVPC() *AddOnCNIVPC {
	return &AddOnCNIVPC{
		Enable: false,

		// https://github.com/aws/amazon-vpc-cni-k8s/releases
		Version: "v1.7",

		NodeSelector: map[string]string{
			// do not deploy in fake nodes, obviously
			"NodeType": "regular",
		},
	}
}

func (cfg *Config) GetAddOnCNIVPCRepositoryRegion() string {
	if !cfg.IsEnabledAddOnCNIVPC() {
		return cfg.Region
	}
	return cfg.AddOnCNIVPC.RepositoryRegion
}

func (cfg *Config) validateAddOnCNIVPC() error {
	if !cfg.IsEnabledAddOnCNIVPC() {
		return nil
	}

	switch cfg.AddOnCNIVPC.Version {
	case "v1.7":
	default:
		return fmt.Errorf("unknown AddOnCNIVPC.Version %q", cfg.AddOnCNIVPC.Version)
	}

	if cfg.AddOnCNIVPC.RepositoryInitAccountID == "" {
		return errors.New("AddOnCNIVPC.RepositoryInitAccountID empty")
	}
	if cfg.AddOnCNIVPC.RepositoryInitRegion == "" {
		cfg.AddOnCNIVPC.RepositoryInitRegion = cfg.Region
	}
	if cfg.AddOnCNIVPC.RepositoryInitName == "" {
		return errors.New("AddOnCNIVPC.RepositoryInitName empty")
	}
	if cfg.AddOnCNIVPC.RepositoryInitImageTag == "" {
		return errors.New("AddOnCNIVPC.RepositoryInitImageTag empty")
	}

	if cfg.AddOnCNIVPC.RepositoryAccountID == "" {
		return errors.New("AddOnCNIVPC.RepositoryAccountID empty")
	}
	if cfg.AddOnCNIVPC.RepositoryRegion == "" {
		cfg.AddOnCNIVPC.RepositoryRegion = cfg.Region
	}
	if cfg.AddOnCNIVPC.RepositoryName == "" {
		return errors.New("AddOnCNIVPC.RepositoryName empty")
	}
	if cfg.AddOnCNIVPC.RepositoryImageTag == "" {
		return errors.New("AddOnCNIVPC.RepositoryImageTag empty")
	}

	return nil
}
