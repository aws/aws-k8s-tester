package eksconfig

import "time"

// IsEnabledAddOnWordpress returns true if "AddOnWordpress" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnWordpress() bool {
	if cfg.AddOnWordpress == nil {
		return false
	}
	if cfg.AddOnWordpress.Enable {
		return true
	}
	cfg.AddOnWordpress = nil
	return false
}

// AddOnWordpress defines parameters for EKS cluster
// add-on WordPress.
// ref. https://github.com/helm/charts/blob/master/stable/wordpress/requirements.yaml
// ref. https://github.com/helm/charts/tree/master/stable/mariadb
// ref. https://github.com/bitnami/charts/tree/master/bitnami/wordpress/#installing-the-chart
type AddOnWordpress struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`

	// Namespace is the namespace to create "AppMesh" controller/injector.
	Namespace string `json:"namespace"`

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

	// UserName is the user name.
	// ref. https://github.com/helm/charts/tree/master/stable/wordpress
	UserName string `json:"user-name"`
	// Password is the user password.
	// ref. https://github.com/helm/charts/tree/master/stable/wordpress
	Password string `json:"password"`

	// NLBARN is the ARN of the NLB created from the service.
	NLBARN string `json:"nlb-arn" read-only:"true"`
	// NLBName is the name of the NLB created from the service.
	NLBName string `json:"nlb-name" read-only:"true"`
	// URL is the host name for WordPress service.
	URL string `json:"url" read-only:"true"`
}
