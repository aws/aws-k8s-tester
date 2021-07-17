package eksconfig

import (
	"errors"

	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnFluentd defines parameters for EKS cluster
// add-on Fluentd.
// Publishes worker nodes logs to:
//  - /aws/containerinsights/[CLUSTER-NAME]/application
//  - /aws/containerinsights/[CLUSTER-NAME]/dataplane
//  - /aws/containerinsights/[CLUSTER-NAME]/host
type AddOnFluentd struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`

	// RepositoryBusyboxAccountID is the account ID for tester ECR image.
	// e.g. "busybox" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/busybox"
	RepositoryBusyboxAccountID string `json:"repository-busybox-account-id,omitempty"`
	// RepositoryBusyboxRegion is the ECR repository region to pull from.
	RepositoryBusyboxRegion string `json:"repository-busybox-region,omitempty"`
	// RepositoryBusyboxName is the repositoryName for tester ECR image.
	// e.g. "busybox" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/busybox"
	RepositoryBusyboxName string `json:"repository-busybox-name,omitempty"`
	// RepositoryBusyboxImageTag is the image tag for tester ECR image.
	// e.g. "latest" for image URI "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/busybox:latest"
	RepositoryBusyboxImageTag string `json:"repository-busybox-image-tag,omitempty"`

	// Threads is the number of threads for fluentd.
	// ref. https://docs.fluentd.org/v/0.12/output#num_threads
	Threads int `json:"threads"`
	// MetadataLogLevel is the log level for "@type kubernetes_metadata".
	// ref. https://docs.fluentd.org/deployment/system-config
	// ref. https://github.com/fabric8io/fluent-plugin-kubernetes_metadata_filter
	MetadataLogLevel string `json:"metadata-log-level"`
	// MetadataCacheSize is the size of the cache of Kubernetes metadata
	// to reduce the requests to the kube-apiserver.
	// ref. "@type kubernetes_metadata"
	// ref. https://github.com/fabric8io/fluent-plugin-kubernetes_metadata_filter
	MetadataCacheSize int `json:"metadata-cache-size"`
	// MetadataWatch is true to enable watch on pods on the kube-apiserver
	// for updates to the metadata.
	// ref. "@type kubernetes_metadata"
	// ref. https://github.com/fabric8io/fluent-plugin-kubernetes_metadata_filter
	MetadataWatch bool `json:"metadata-watch"`
	// MetadataSkipLabels is true to skip all label fields from the metadata.
	// ref. "@type kubernetes_metadata"
	// ref. https://github.com/fabric8io/fluent-plugin-kubernetes_metadata_filter
	MetadataSkipLabels bool `json:"metadata-skip-labels"`
	// MetadataSkipMasterURL is true to skip "master_url" field from the metadata.
	// ref. "@type kubernetes_metadata"
	// ref. https://github.com/fabric8io/fluent-plugin-kubernetes_metadata_filter
	MetadataSkipMasterURL bool `json:"metadata-skip-master-url"`
	// MetadataSkipContainerMetadata is true to skip some container data of the metadata.
	// For example, if true, it skips container image and image ID fields.
	// ref. "@type kubernetes_metadata"
	// ref. https://github.com/fabric8io/fluent-plugin-kubernetes_metadata_filter
	MetadataSkipContainerMetadata bool `json:"metadata-skip-container-metadata"`
	// MetadataSkipNamespaceMetadata is true to skip "namespace_id" field from the metadata.
	// If true, the plugin will be faster with less CPU consumption.
	// ref. "@type kubernetes_metadata"
	// ref. https://github.com/fabric8io/fluent-plugin-kubernetes_metadata_filter
	MetadataSkipNamespaceMetadata bool `json:"metadata-skip-namespace-metadata"`
}

// AWS_K8S_TESTER_EKS_ADD_ON_FLUENTD_PREFIX is the environment variable prefix used for "eksconfig".
const AWS_K8S_TESTER_EKS_ADD_ON_FLUENTD_PREFIX = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_FLUENTD_"

// IsEnabledAddOnFluentd returns true if "AddOnFluentd" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnFluentd() bool {
	if cfg.AddOnFluentd == nil {
		return false
	}
	if cfg.AddOnFluentd.Enable {
		return true
	}
	cfg.AddOnFluentd = nil
	return false
}

func getDefaultAddOnFluentd() *AddOnFluentd {
	return &AddOnFluentd{
		Enable: false,

		Threads:                       8,
		MetadataLogLevel:              "warn",
		MetadataCacheSize:             20000,
		MetadataWatch:                 false,
		MetadataSkipLabels:            true,
		MetadataSkipMasterURL:         true,
		MetadataSkipContainerMetadata: true,
		MetadataSkipNamespaceMetadata: true,
	}
}

func (cfg *Config) GetAddOnFluentdRepositoryBusyboxRegion() string {
	if !cfg.IsEnabledAddOnFluentd() {
		return cfg.Region
	}
	return cfg.AddOnFluentd.RepositoryBusyboxRegion
}

func (cfg *Config) validateAddOnFluentd() error {
	if !cfg.IsEnabledAddOnFluentd() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnFluentd.Enable true but no node group is enabled")
	}
	if cfg.AddOnFluentd.Namespace == "" {
		cfg.AddOnFluentd.Namespace = cfg.Name + "-fluentd"
	}
	return nil
}
