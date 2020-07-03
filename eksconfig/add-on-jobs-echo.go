package eksconfig

import (
	"errors"
	"fmt"

	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnJobsEcho defines parameters for EKS cluster
// add-on Job with echo.
type AddOnJobsEcho struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`

	// RepositoryBusyboxAccountID is the account ID for tester ECR image.
	// e.g. "busybox" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/busybox"
	RepositoryBusyboxAccountID string `json:"repository-busybox-account-id,omitempty"`
	// RepositoryBusyboxRegion is the ECR repository region to pull from.
	RepositoryBusyboxRegion string `json:"repository-busybox-region,omitempty"`
	// RepositoryBusyboxName is the repositoryName for tester ECR image.
	// e.g. "busybox" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/busybox"
	RepositoryBusyboxName string `json:"repository-busybox-name,omitempty"`
	// RepositoryBusyboxImageTag is the image tag for tester ECR image.
	// e.g. "latest" for image URI "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/busybox:latest"
	RepositoryBusyboxImageTag string `json:"repository-busybox-image-tag,omitempty"`

	// Completes is the desired number of successfully finished pods.
	Completes int `json:"completes"`
	// Parallels is the the maximum desired number of pods the
	// job should run at any given time.
	Parallels int `json:"parallels"`
	// EchoSize is the job object size in bytes.
	// "Request entity too large: limit is 3145728" (3.1 MB).
	// "The Job "echo" is invalid: metadata.annotations:
	// Too long: must have at most 262144 characters". (0.26 MB)
	EchoSize int `json:"echo-size"`
}

// EnvironmentVariablePrefixAddOnJobsEcho is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnJobsEcho = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_JOBS_ECHO_"

// IsEnabledAddOnJobsEcho returns true if "AddOnJobsEcho" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnJobsEcho() bool {
	if cfg.AddOnJobsEcho == nil {
		return false
	}
	if cfg.AddOnJobsEcho.Enable {
		return true
	}
	cfg.AddOnJobsEcho = nil
	return false
}

func getDefaultAddOnJobsEcho() *AddOnJobsEcho {
	return &AddOnJobsEcho{
		Enable:    false,
		Completes: 10,
		Parallels: 10,
		EchoSize:  100 * 1024, // 100 KB

		// writes total 100 MB data to etcd
		// Completes: 1000,
		// Parallels: 100,
		// EchoSize: 100 * 1024, // 100 KB
	}
}

func (cfg *Config) GetAddOnJobsEchoRepositoryBusyboxRegion() string {
	if !cfg.IsEnabledAddOnJobsEcho() {
		return cfg.Region
	}
	return cfg.AddOnJobsEcho.RepositoryBusyboxRegion
}

func (cfg *Config) validateAddOnJobsEcho() error {
	if !cfg.IsEnabledAddOnJobsEcho() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnJobsEcho.Enable true but no node group is enabled")
	}
	if cfg.AddOnJobsEcho.Namespace == "" {
		cfg.AddOnJobsEcho.Namespace = cfg.Name + "-jobs-echo"
	}
	if cfg.AddOnJobsEcho.EchoSize > 250000 {
		return fmt.Errorf("echo size limit is 0.25 MB, got %d", cfg.AddOnJobsEcho.EchoSize)
	}
	return nil
}
