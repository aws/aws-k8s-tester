package eksconfig

import (
	"errors"
	"fmt"
	"time"
)

// AddOnCSRs defines parameters for EKS cluster
// add-on "CertificateSigningRequest".
type AddOnCSRs struct {
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

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`

	// InitialRequestConditionType is the initial CSR condition type
	// to simulate CSR condition.
	//
	// Valid values are:
	//   "k8s.io/api/certificates/v1beta1.CertificateApproved" == "Approved"
	//   "k8s.io/api/certificates/v1beta1.CertificateDenied" == "Denied"
	//   "Random"
	//   "Pending"
	//   ""
	//
	InitialRequestConditionType string `json:"initial-request-condition-type"`

	// Objects is the number of "CertificateSigningRequest" objects to create.
	Objects int `json:"objects"`
	// CreatedNames is the list of created "CertificateSigningRequest" object names.
	CreatedNames []string `json:"created-names" read-only:"true"`

	// FailThreshold is the number of write failures to allow.
	FailThreshold int `json:"fail-threshold"`
}

// EnvironmentVariablePrefixAddOnCSRs is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnCSRs = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_CSRS_"

// IsEnabledAddOnCSRs returns true if "AddOnCSRs" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnCSRs() bool {
	if cfg.AddOnCSRs == nil {
		return false
	}
	if cfg.AddOnCSRs.Enable {
		return true
	}
	cfg.AddOnCSRs = nil
	return false
}

func getDefaultAddOnCSRs() *AddOnCSRs {
	return &AddOnCSRs{
		Enable: false,

		InitialRequestConditionType: "",

		Objects: 10,

		// writes total 5 MB data to etcd
		// Objects: 1000,
	}
}

func (cfg *Config) validateAddOnCSRs() error {
	if !cfg.IsEnabledAddOnCSRs() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnCSRs.Enable true but no node group is enabled")
	}
	if cfg.AddOnCSRs.Namespace == "" {
		cfg.AddOnCSRs.Namespace = cfg.Name + "-csrs"
	}
	switch cfg.AddOnCSRs.InitialRequestConditionType {
	case "Approved":
	case "Denied":
	case "Pending", "":
	case "Random":
	default:
		return fmt.Errorf("unknown AddOnCSRs.InitialRequestConditionType %q", cfg.AddOnCSRs.InitialRequestConditionType)
	}
	return nil
}
