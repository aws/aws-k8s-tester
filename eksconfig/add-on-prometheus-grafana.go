package eksconfig

import "time"

// IsEnabledAddOnPrometheusGrafana returns true if "AddOnPrometheusGrafana" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnPrometheusGrafana() bool {
	if cfg.AddOnPrometheusGrafana == nil {
		return false
	}
	if cfg.AddOnPrometheusGrafana.Enable {
		return true
	}
	cfg.AddOnPrometheusGrafana = nil
	return false
}

// AddOnPrometheusGrafana defines parameters for EKS cluster
// add-on Prometheus/Grafana.
// ref. https://docs.aws.amazon.com/eks/latest/userguide/prometheus.html
// ref. https://eksworkshop.com/intermediate/240_monitoring/deploy-prometheus/
type AddOnPrometheusGrafana struct {
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

	// GrafanaAdminPassword is the admin password for the Grafana service.
	GrafanaAdminPassword string `json:"grafana-admin-password"`
	// GrafanaNLBARN is the ARN of the NLB created from the Grafana service.
	GrafanaNLBARN string `json:"grafana-nlb-arn" read-only:"true"`
	// GrafanaNLBName is the name of the NLB created from the Grafana service.
	GrafanaNLBName string `json:"grafana-nlb-name" read-only:"true"`
	// GrafanaURL is the host name for Grafana service.
	GrafanaURL string `json:"grafana-url" read-only:"true"`
}
