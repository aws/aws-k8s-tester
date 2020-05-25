package eksconfig

import (
	"errors"
	"fmt"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/service/eks"
)

// AddOnWordpress defines parameters for EKS cluster
// add-on WordPress.
// ref. https://github.com/helm/charts/blob/master/stable/wordpress/requirements.yaml
// ref. https://github.com/helm/charts/tree/master/stable/mariadb
// ref. https://github.com/bitnami/charts/tree/master/bitnami/wordpress/#installing-the-chart
type AddOnWordpress struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created bool `json:"created" read-only:"true"`

	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`

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

// EnvironmentVariablePrefixAddOnWordpress is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnWordpress = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_WORDPRESS_"

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

func getDefaultAddOnWordpress() *AddOnWordpress {
	return &AddOnWordpress{
		Enable:   false,
		UserName: "user",
		Password: "",
	}
}

func (cfg *Config) validateAddOnWordpress() error {
	if !cfg.IsEnabledAddOnWordpress() {
		return nil
	}
	if !cfg.IsEnabledAddOnCSIEBS() {
		return errors.New("AddOnWordpress.Enable true but IsEnabledAddOnCSIEBS.Enable false")
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnWordpress.Enable true but no node group is enabled")
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
			return fmt.Errorf("AddOnWordpress.Enabled true but AddOnNodeGroups [x86Found %v, rocketFound %v]", x86Found, rocketFound)
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
			return fmt.Errorf("AddOnWordpress.Enabled true but AddOnManagedNodeGroups [x86Found %v, rocketFound %v]", x86Found, rocketFound)
		}
	}

	if cfg.AddOnWordpress.Namespace == "" {
		cfg.AddOnWordpress.Namespace = cfg.Name + "-wordpress"
	}
	if cfg.AddOnWordpress.UserName == "" {
		cfg.AddOnWordpress.UserName = "user"
	}
	if cfg.AddOnWordpress.Password == "" {
		cfg.AddOnWordpress.Password = randutil.String(10)
	}

	return nil
}
