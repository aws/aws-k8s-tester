package eksconfig

import "time"

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

// AddOnCronJobs defines parameters for EKS cluster
// add-on with CronJob.
type AddOnCronJobs struct {
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

	// Namespace is the namespace to create "Job" objects in.
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
