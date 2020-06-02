package eksconfig

import (
	"errors"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/service/eks"
)

type AddOnCudaVectorAdd struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`
}

// EnvironmentVariablePrefixAddOnCudaVectorAdd is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnCudaVectorAdd = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_CUDA_VECTOR_ADD_"

// IsEnabledAddOnCudaVectorAdd returns true if "AddOnCudaVectorAdd" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnCudaVectorAdd() bool {
	if cfg.AddOnCudaVectorAdd == nil {
		return false
	}
	if cfg.AddOnCudaVectorAdd.Enable {
		return true
	}
	cfg.AddOnCudaVectorAdd = nil
	return false
}

// TBD !!!!!!!!!!!
func getDefaultAddOnCudaVectorAdd() *AddOnCudaVectorAdd {
	return &AddOnCudaVectorAdd{
		Enable: false,
	}
}

func (cfg *Config) validateAddOnCudaVectorAdd() error {
	if !cfg.IsEnabledAddOnCudaVectorAdd() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnCudaVectorAdd.Enable true but no node group is enabled")
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
		return errors.New("AddOnCudaVectorAdd requires GPU AMI")
	}

	if cfg.AddOnCudaVectorAdd.Namespace == "" {
		cfg.AddOnCudaVectorAdd.Namespace = cfg.Name + "-cuda-vector-add"
	}

	return nil
}
