package eksconfig

import (
	"errors"
	"fmt"

	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnCronJobs defines parameters for EKS cluster
// add-on with CronJob.
type AddOnCronJobs struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`

	// Schedule is the cron schedule (e.g. "*/1 * * * *").
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

	// EchoSize is the job object size in bytes.
	// "Request entity too large: limit is 3145728" (3.1 MB).
	// "The Job "echo" is invalid: metadata.annotations:
	// Too long: must have at most 262144 characters". (0.26 MB)
	EchoSize int `json:"echo-size"`
}

// EnvironmentVariablePrefixAddOnCronJobs is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnCronJobs = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_CRON_JOBS_"

// IsEnabledAddOnCronJobs returns true if "AddOnCronJobs" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnCronJobs() bool {
	if cfg.AddOnCronJobs == nil {
		return false
	}
	if cfg.AddOnCronJobs.Enable {
		return true
	}
	cfg.AddOnCronJobs = nil
	return false
}

func getDefaultAddOnCronJobs() *AddOnCronJobs {
	return &AddOnCronJobs{
		Enable:                     false,
		Schedule:                   "*/10 * * * *", // every 10-min
		Completes:                  10,
		Parallels:                  10,
		SuccessfulJobsHistoryLimit: 3,
		FailedJobsHistoryLimit:     1,
		EchoSize:                   100 * 1024, // 100 KB
	}
}

func (cfg *Config) validateAddOnCronJobs() error {
	if !cfg.IsEnabledAddOnCronJobs() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnCronJobs.Enable true but no node group is enabled")
	}
	if cfg.AddOnCronJobs.Namespace == "" {
		cfg.AddOnCronJobs.Namespace = cfg.Name + "-cronjob"
	}
	if cfg.AddOnCronJobs.EchoSize > 250000 {
		return fmt.Errorf("echo size limit is 0.25 MB, got %d", cfg.AddOnCronJobs.EchoSize)
	}
	return nil
}
