package eksconfig

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-sdk-go/service/eks"
)

// AddOnJupyterHub defines parameters for EKS cluster
// add-on Jupyter Hub.
// ref. https://zero-to-jupyterhub.readthedocs.io/en/latest/index.html
type AddOnJupyterHub struct {
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

	// ProxySecretToken is 32-byte hexadecimal encoded secret token string.
	// e.g. "openssl rand -hex 32"
	ProxySecretToken string `json:"proxy-secret-token"`

	// NLBARN is the ARN of the NLB created from the service.
	NLBARN string `json:"nlb-arn" read-only:"true"`
	// NLBName is the name of the NLB created from the service.
	NLBName string `json:"nlb-name" read-only:"true"`
	// URL is the host name for Jupyter Hub service.
	URL string `json:"url" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnJupyterHub is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnJupyterHub = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_JUPYTER_HUB_"

// IsEnabledAddOnJupyterHub returns true if "AddOnJupyterHub" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnJupyterHub() bool {
	if cfg.AddOnJupyterHub == nil {
		return false
	}
	if cfg.AddOnJupyterHub.Enable {
		return true
	}
	cfg.AddOnJupyterHub = nil
	return false
}

func getDefaultAddOnJupyterHub() *AddOnJupyterHub {
	return &AddOnJupyterHub{
		Enable: false,
	}
}

func (cfg *Config) validateAddOnJupyterHub() error {
	if !cfg.IsEnabledAddOnJupyterHub() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnJupyterHub.Enable true but no node group is enabled")
	}

	gpuFound := false
	if cfg.IsEnabledAddOnNodeGroups() {
		for _, cur := range cfg.AddOnNodeGroups.ASGs {
			if cur.AMIType == ec2config.AMITypeAL2X8664GPU {
				gpuFound = true
				break
			}
		}
	}
	if !gpuFound && cfg.IsEnabledAddOnManagedNodeGroups() {
		for _, cur := range cfg.AddOnManagedNodeGroups.MNGs {
			if cur.AMIType == eks.AMITypesAl2X8664Gpu {
				gpuFound = true
				break
			}
		}
	}
	if !gpuFound {
		return errors.New("AddOnJupyterHub requires GPU AMI")
	}

	if cfg.AddOnJupyterHub.Namespace == "" {
		cfg.AddOnJupyterHub.Namespace = cfg.Name + "-jupyter-hub"
	}

	if cfg.AddOnJupyterHub.ProxySecretToken == "" {
		cfg.AddOnJupyterHub.ProxySecretToken = randHex(32)
	}
	_, err := hex.DecodeString(cfg.AddOnJupyterHub.ProxySecretToken)
	if err != nil {
		return fmt.Errorf("cannot hex decode AddOnJupyterHub.ProxySecretToken %q", err)
	}

	return nil
}
