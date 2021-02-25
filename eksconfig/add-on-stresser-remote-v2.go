package eksconfig

import (
	"errors"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

type AddOnStresserRemoteV2 struct {
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

	// RepositoryBusyBoxName is the repositoryName for busybox ECR image.
	// e.g. "busybox" for image URI "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/aws/busybox:latest"
	RepositoryBusyBoxName string `json:"repository-busybox-name,omitempty"`
	// RepositoryBusyBoxImageTag is the image tag for busybox ECR image.
	// e.g. "latest" for image URI "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/aws/busybox:latest"
	RepositoryBusyBoxImageTag string `json:"repository-busybox-image-tag,omitempty"`

	// Schedule is the cron schedule (e.g. "*/5 * * * *").
	Schedule string `json:"schedule"`
	// Completes is the desired number of successfully finished pods.
	Completes int `json:"completes"`
	// Parallels is the the maximum desired number of pods the
	// job should run at any given time.
	Parallels int `json:"parallels"`
	// SuccessfulJobsHistoryLimit is the number of successful finished
	// jobs to retain. Defaults to 3.
	SuccessfulJobsHistoryLimit int32 `json:"successful-jobs-history-limit"`
	// FailedJobsHistoryLimit is the number of failed finished jobs
	// to retain. Defaults to 1.
	FailedJobsHistoryLimit int32 `json:"failed-jobs-history-limit"`

	// ObjectSize is the value size in bytes for write objects.
	// If 0, do not write anything.
	ObjectSize int `json:"object-size"`

	// Duration is the duration to run stress2 testing.
	Duration       time.Duration `json:"duration,omitempty"`
	DurationString string        `json:"duration-string,omitempty" read-only:"true"`

	// Coroutines is the number of concurrent go routines run per job
	Coroutines int `json:"coroutines"`
	// Secrets is the number of secrets generated per job
	Secrets int `json:"secrets"`
}

// EnvironmentVariablePrefixAddOnStresserRemoteV2 is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnStresserRemoteV2 = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_STRESSER_REMOTE_V2_"

// IsEnabledAddOnStresserRemote returns true if "AddOnStresserRemote" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnStresserRemoteV2() bool {
	if cfg.AddOnStresserRemoteV2 == nil {
		return false
	}
	if cfg.AddOnStresserRemoteV2.Enable {
		return true
	}
	cfg.AddOnStresserRemoteV2 = nil
	return false
}

func getDefaultAddOnStresserRemoteV2() *AddOnStresserRemoteV2 {
	return &AddOnStresserRemoteV2{
		Enable:                     false,
		Schedule:                   "0 */6 * * *", // every 6 hours align with etcd defrag interval
		Completes:                  10,
		Parallels:                  10,
		SuccessfulJobsHistoryLimit: 3,
		FailedJobsHistoryLimit:     1,
		ObjectSize:                 8, // 8 bytes
		Duration:                   10 * time.Minute,
		Coroutines:                 10,
		Secrets:                    10,
	}
}

func (cfg *Config) GetAddOnStresserRemoteV2RepositoryRegion() string {
	if !cfg.IsEnabledAddOnStresserRemoteV2() {
		return cfg.Region
	}
	return cfg.AddOnStresserRemoteV2.RepositoryRegion
}

func (cfg *Config) validateAddOnStresserRemoteV2() error {
	if !cfg.IsEnabledAddOnStresserRemoteV2() {
		return nil
	}

	if cfg.AddOnStresserRemoteV2.Namespace == "" {
		cfg.AddOnStresserRemoteV2.Namespace = cfg.Name + "-stresser-remote-v2"
	}

	if cfg.AddOnStresserRemoteV2.RepositoryAccountID == "" {
		return errors.New("AddOnStresserRemoteV2.RepositoryAccountID empty")
	}
	if cfg.AddOnStresserRemoteV2.RepositoryRegion == "" {
		cfg.AddOnStresserRemoteV2.RepositoryRegion = cfg.Region
	}
	if cfg.AddOnStresserRemoteV2.RepositoryName == "" {
		return errors.New("AddOnStresserRemoteV2.RepositoryName empty")
	}
	if cfg.AddOnStresserRemoteV2.RepositoryImageTag == "" {
		return errors.New("AddOnStresserRemoteV2.RepositoryImageTag empty")
	}
	if cfg.AddOnStresserRemoteV2.RepositoryBusyBoxName == "" {
		return errors.New("AddOnStresserRemoteV2.RepositoryBusyBoxName empty")
	}
	if cfg.AddOnStresserRemoteV2.RepositoryBusyBoxImageTag == "" {
		return errors.New("AddOnStresserRemoteV2.RepositoryBusyBoxImageTag empty")
	}

	if cfg.AddOnStresserRemoteV2.Duration == time.Duration(0) {
		cfg.AddOnStresserRemoteV2.Duration = 10 * time.Minute
	}
	cfg.AddOnStresserRemoteV2.DurationString = cfg.AddOnStresserRemoteV2.Duration.String()
	if cfg.AddOnStresserRemoteV2.Coroutines <= 0 {
		cfg.AddOnStresserRemoteV2.Coroutines = 10
	}
	if cfg.AddOnStresserRemoteV2.Secrets >= 0 {
		cfg.AddOnStresserRemoteV2.Secrets = 10
	}

	return nil
}
