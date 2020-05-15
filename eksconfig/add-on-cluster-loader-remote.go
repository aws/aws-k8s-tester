package eksconfig

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/randutil"
)

// AddOnClusterLoaderRemote defines parameters for EKS cluster
// add-on remote Cluster Loader.
// ref. https://github.com/kubernetes/perf-tests
type AddOnClusterLoaderRemote struct {
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

	// DeploymentReplicas is the number of replicas to deploy when cluster loaders are deployed via Pod.
	DeploymentReplicas int32 `json:"deployment-replicas,omitempty"`

	// Duration is the duration to run load testing.
	// The cluster loader waits "one" "Duration" for hollow ones.
	// And other one for cluster loader.
	Duration       time.Duration `json:"duration,omitempty"`
	DurationString string        `json:"duration-string,omitempty" read-only:"true"`

	// RepositoryAccountID is the account ID for tester ECR image.
	// e.g. "aws/aws-k8s-tester" for "[ACCOUNT_ID].dkr.ecr.us-west-2.amazonaws.com/aws/aws-k8s-tester"
	RepositoryAccountID string `json:"repository-account-id,omitempty"`
	// RepositoryName is the repositoryName for tester ECR image.
	// e.g. "aws/aws-k8s-tester" for "[ACCOUNT_ID].dkr.ecr.us-west-2.amazonaws.com/aws/aws-k8s-tester"
	RepositoryName string `json:"repository-name,omitempty"`
	// RepositoryURI is the repositoryUri for tester ECR image.
	// e.g. "[ACCOUNT_ID].dkr.ecr.us-west-2.amazonaws.com/aws/aws-k8s-tester"
	RepositoryURI string `json:"repository-uri,omitempty"`
	// RepositoryImageTag is the image tag for tester ECR image.
	// e.g. "latest" for image URI "[ACCOUNT_ID].dkr.ecr.us-west-2.amazonaws.com/aws/aws-k8s-tester:latest"
	RepositoryImageTag string `json:"repository-image-tag,omitempty"`

	// RequestSummaryReadOutputPathPrefix is the output path used in remote cluster loader.
	RequestSummaryReadOutputPathPrefix string `json:"request-summary-read-output-path-prefix"`
	// RequestsSummaryRead is the read cluster loader results, aggregated from remote nodes.
	RequestsSummaryRead RequestsSummary `json:"requests-summary-read,omitempty" read-only:"true"`
	// RequestsSummaryReadJSONPath is the file path to store requests summary results in JSON format.
	RequestsSummaryReadJSONPath string `json:"requests-summary-read-json-path" read-only:"true"`
	// RequestsSummaryReadTablePath is the file path to store requests summary results in table format.
	RequestsSummaryReadTablePath string `json:"requests-summary-read-table-path" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnClusterLoaderRemote is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnClusterLoaderRemote = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_CLUSTER_LOADER_REMOTE_"

// IsEnabledAddOnClusterLoaderRemote returns true if "AddOnClusterLoaderRemote" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnClusterLoaderRemote() bool {
	if cfg.AddOnClusterLoaderRemote == nil {
		return false
	}
	if cfg.AddOnClusterLoaderRemote.Enable {
		return true
	}
	cfg.AddOnClusterLoaderRemote = nil
	return false
}

func getDefaultAddOnClusterLoaderRemote() *AddOnClusterLoaderRemote {
	return &AddOnClusterLoaderRemote{
		Enable:                             false,
		DeploymentReplicas:                 5,
		Duration:                           time.Minute,
		RequestSummaryReadOutputPathPrefix: randutil.String(10),
	}
}

func (cfg *Config) validateAddOnClusterLoaderRemote() error {
	if !cfg.IsEnabledAddOnClusterLoaderRemote() {
		return nil
	}

	if cfg.AddOnClusterLoaderRemote.Namespace == "" {
		cfg.AddOnClusterLoaderRemote.Namespace = cfg.Name + "-cluster-loader-remote"
	}

	if cfg.AddOnClusterLoaderRemote.DeploymentReplicas == 0 {
		cfg.AddOnClusterLoaderRemote.DeploymentReplicas = 5
	}

	if cfg.AddOnClusterLoaderRemote.Duration == time.Duration(0) {
		cfg.AddOnClusterLoaderRemote.Duration = time.Minute
	}
	cfg.AddOnClusterLoaderRemote.DurationString = cfg.AddOnClusterLoaderRemote.Duration.String()

	if cfg.AddOnClusterLoaderRemote.RepositoryAccountID == "" {
		return errors.New("AddOnClusterLoaderRemote.RepositoryAccountID empty")
	}
	if cfg.AddOnClusterLoaderRemote.RepositoryName == "" {
		return errors.New("AddOnClusterLoaderRemote.RepositoryName empty")
	}
	if cfg.AddOnClusterLoaderRemote.RepositoryURI == "" {
		return errors.New("AddOnClusterLoaderRemote.RepositoryURI empty")
	}
	if strings.Contains(cfg.AddOnClusterLoaderRemote.RepositoryURI, cfg.AddOnClusterLoaderRemote.RepositoryAccountID) {
		return fmt.Errorf("AddOnClusterLoaderRemote.RepositoryURI %q does not have AddOnClusterLoaderRemote.RepositoryAccountID %q", cfg.AddOnClusterLoaderRemote.RepositoryURI, cfg.AddOnClusterLoaderRemote.RepositoryAccountID)
	}
	if cfg.AddOnClusterLoaderRemote.RepositoryImageTag == "" {
		return errors.New("AddOnClusterLoaderRemote.RepositoryImageTag empty")
	}

	if cfg.AddOnClusterLoaderRemote.RequestSummaryReadOutputPathPrefix == "" {
		cfg.AddOnClusterLoaderRemote.RequestSummaryReadOutputPathPrefix = randutil.String(10)
	}
	if cfg.AddOnClusterLoaderRemote.RequestsSummaryReadJSONPath == "" {
		cfg.AddOnClusterLoaderRemote.RequestsSummaryReadJSONPath = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-cluster-loader-remote-request-summary-read.json")
	}
	if cfg.AddOnClusterLoaderRemote.RequestsSummaryReadTablePath == "" {
		cfg.AddOnClusterLoaderRemote.RequestsSummaryReadTablePath = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-cluster-loader-remote-request-summary-read.txt")
	}

	return nil
}
