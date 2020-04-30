package eksconfig

import "time"

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

	// Namespace is the namespace to create "CertificateSigningRequest" objects in.
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
}
