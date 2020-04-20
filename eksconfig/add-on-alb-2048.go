package eksconfig

import "time"

// IsEnabledAddOnALB2048 returns true if "AddOnALB2048" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnALB2048() bool {
	if cfg.AddOnALB2048 == nil {
		return false
	}
	if cfg.AddOnALB2048.Enable {
		return true
	}
	cfg.AddOnALB2048 = nil
	return false
}

// AddOnALB2048 defines parameters for EKS cluster
// add-on ALB 2048 service.
type AddOnALB2048 struct {
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

	// Namespace is the namespace to create "ALB" objects in.
	Namespace string `json:"namespace"`
	// DeploymentReplicasALB is the number of ALB replicas to deploy using "Deployment" object.
	DeploymentReplicasALB int32 `json:"deployment-replicas-alb"`
	// DeploymentReplicas2048 is the number of 2048 replicas to deploy using "Deployment" object.
	DeploymentReplicas2048 int32 `json:"deployment-replicas-2048"`

	// ALBARN is the ARN of the ALB created from the service.
	ALBARN string `json:"alb-arn" read-only:"true"`
	// ALBName is the name of the ALB created from the service.
	ALBName string `json:"alb-name" read-only:"true"`
	// URL is the URL for ALB 2048 Service.
	URL string `json:"url" read-only:"true"`
}
