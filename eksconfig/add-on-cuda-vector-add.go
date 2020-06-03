package eksconfig

import (
	"errors"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/service/eks"
)

// AddOnCUDAVectorAdd defines parameters for EKS cluster
// add-on cuda-vector-add.
type AddOnCUDAVectorAdd struct {
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

// EnvironmentVariablePrefixAddOnCUDAVectorAdd is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnCUDAVectorAdd = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_CUDA_VECTOR_ADD_"

// IsEnabledAddOnCUDAVectorAdd returns true if "AddOnCUDAVectorAdd" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnCUDAVectorAdd() bool {
	if cfg.AddOnCUDAVectorAdd == nil {
		return false
	}
	if cfg.AddOnCUDAVectorAdd.Enable {
		return true
	}
	cfg.AddOnCUDAVectorAdd = nil
	return false
}

// TBD !!!!!!!!!!!
func getDefaultAddOnCUDAVectorAdd() *AddOnCUDAVectorAdd {
	return &AddOnCUDAVectorAdd{
		Enable: false,
	}
}

func (cfg *Config) validateAddOnCUDAVectorAdd() error {
	if !cfg.IsEnabledAddOnCUDAVectorAdd() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnCUDAVectorAdd.Enable true but no node group is enabled")
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
		return errors.New("AddOnCUDAVectorAdd requires GPU AMI")
	}

	if cfg.AddOnCUDAVectorAdd.Namespace == "" {
		cfg.AddOnCUDAVectorAdd.Namespace = cfg.Name + "-cuda-vector-add"
	}

	return nil
}
