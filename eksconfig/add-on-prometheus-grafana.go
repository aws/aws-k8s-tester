package eksconfig

import (
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-sdk-go/service/eks"
)

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

	// GrafanaAdminUserName is the admin user for the Grafana service.
	GrafanaAdminUserName string `json:"grafana-admin-user-name"`
	// GrafanaAdminPassword is the admin password for the Grafana service.
	GrafanaAdminPassword string `json:"grafana-admin-password"`
	// GrafanaNLBARN is the ARN of the NLB created from the Grafana service.
	GrafanaNLBARN string `json:"grafana-nlb-arn" read-only:"true"`
	// GrafanaNLBName is the name of the NLB created from the Grafana service.
	GrafanaNLBName string `json:"grafana-nlb-name" read-only:"true"`
	// GrafanaURL is the host name for Grafana service.
	GrafanaURL string `json:"grafana-url" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnPrometheusGrafana is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnPrometheusGrafana = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_PROMETHEUS_GRAFANA_"

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

func getDefaultAddOnPrometheusGrafana() *AddOnPrometheusGrafana {
	return &AddOnPrometheusGrafana{
		Enable:               false,
		GrafanaAdminUserName: "admin",
		GrafanaAdminPassword: "",
	}
}

func (cfg *Config) validateAddOnPrometheusGrafana() error {
	if !cfg.IsEnabledAddOnPrometheusGrafana() {
		return nil
	}
	if !cfg.IsEnabledAddOnCSIEBS() {
		return errors.New("AddOnPrometheusGrafana.Enable true but IsEnabledAddOnCSIEBS.Enable false")
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnPrometheusGrafana.Enable true but no node group is enabled")
	}

	// TODO: PVC not working on BottleRocket
	// do not assign mariadb to Bottlerocket
	// e.g. MountVolume.MountDevice failed for volume "pvc-8e035a13-4d33-472f-a4c0-f36c7d39d170" : executable file not found in $PATH
	// e.g. Unable to mount volumes for pod "wordpress-84c567b89b-2jgh5_eks-2020042114-exclusivea3i-wordpress(d02336a3-1799-4b08-9f15-b90871f6a2f0)": timeout expired waiting for volumes to attach or mount for pod "eks-2020042114-exclusivea3i-wordpress"/"wordpress-84c567b89b-2jgh5". list of unmounted volumes=[wordpress-data]. list of unattached volumes=[wordpress-data default-token-7bdc2]
	// TODO: fix CSI EBS https://github.com/bottlerocket-os/bottlerocket/issues/877
	if cfg.IsEnabledAddOnNodeGroups() {
		x86Found, rocketFound := false, false
		for _, asg := range cfg.AddOnNodeGroups.ASGs {
			switch asg.AMIType {
			case ec2config.AMITypeAL2X8664,
				ec2config.AMITypeAL2X8664GPU:
				x86Found = true
			case ec2config.AMITypeBottleRocketCPU:
				rocketFound = true
			}
		}
		if !x86Found && rocketFound {
			return fmt.Errorf("AddOnPrometheusGrafana.Enabled true but AddOnNodeGroups [x86Found %v, rocketFound %v]", x86Found, rocketFound)
		}
	}
	if cfg.IsEnabledAddOnManagedNodeGroups() {
		x86Found, rocketFound := false, false
		for _, asg := range cfg.AddOnManagedNodeGroups.MNGs {
			switch asg.AMIType {
			case eks.AMITypesAl2X8664,
				eks.AMITypesAl2X8664Gpu:
				x86Found = true
			case ec2config.AMITypeBottleRocketCPU:
				rocketFound = true
			}
		}
		if !x86Found && rocketFound {
			return fmt.Errorf("AddOnPrometheusGrafana.Enabled true but AddOnManagedNodeGroups [x86Found %v, rocketFound %v]", x86Found, rocketFound)
		}
	}

	if cfg.AddOnPrometheusGrafana.GrafanaAdminUserName == "" {
		cfg.AddOnPrometheusGrafana.GrafanaAdminUserName = randString(10)
	}
	if cfg.AddOnPrometheusGrafana.GrafanaAdminPassword == "" {
		cfg.AddOnPrometheusGrafana.GrafanaAdminPassword = randString(10)
	}

	return nil
}
