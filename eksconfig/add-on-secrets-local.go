package eksconfig

import (
	"errors"
	"strings"

	"github.com/aws/aws-k8s-tester/pkg/metrics"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
)

// AddOnSecretsLocal defines parameters for EKS cluster
// add-on "Secrets" local.
// It generates loads from the local host machine.
// Every object is written serially with no concurrency.
// Use remote tester to write with concurrency.
// The main use case is to write a large number of objects to fill up etcd database.
// And measure latencies for secret encryption.
type AddOnSecretsLocal struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// Namespace is the namespace to create objects in.
	Namespace string `json:"namespace"`

	// Objects is the number of "Secret" objects to write/read.
	Objects int `json:"objects"`
	// ObjectSize is the "Secret" value size in bytes.
	ObjectSize int `json:"object-size"`

	// NamePrefix is the prefix of Secret name.
	// If multiple Secret loader is running,
	// this must be unique per worker to avoid name conflicts.
	NamePrefix string `json:"name-prefix"`

	// RequestsSummaryWrites is the writes results.
	RequestsSummaryWrites metrics.RequestsSummary `json:"requests-summary-writes,omitempty" read-only:"true"`
	// RequestsSummaryWritesJSONPath is the file path to store writes requests summary in JSON format.
	RequestsSummaryWritesJSONPath string `json:"requests-summary-writes-json-path" read-only:"true"`
	// RequestsSummaryWritesTablePath is the file path to store writes requests summary in table format.
	RequestsSummaryWritesTablePath string `json:"requests-summary-writes-table-path" read-only:"true"`
	// RequestsSummaryReads is the reads results.
	RequestsSummaryReads metrics.RequestsSummary `json:"requests-summary-reads,omitempty" read-only:"true"`
	// RequestsSummaryReadsJSONPath is the file path to store reads requests summary in JSON format.
	RequestsSummaryReadsJSONPath string `json:"requests-summary-reads-json-path" read-only:"true"`
	// RequestsSummaryReadsTablePath is the file path to store reads requests summary in table format.
	RequestsSummaryReadsTablePath string `json:"requests-summary-reads-table-path" read-only:"true"`
}

// EnvironmentVariablePrefixAddOnSecretsLocal is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnSecretsLocal = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_SECRETS_LOCAL_"

// IsEnabledAddOnSecretsLocal returns true if "AddOnSecretsLocal" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnSecretsLocal() bool {
	if cfg.AddOnSecretsLocal == nil {
		return false
	}
	if cfg.AddOnSecretsLocal.Enable {
		return true
	}
	cfg.AddOnSecretsLocal = nil
	return false
}

func getDefaultAddOnSecretsLocal() *AddOnSecretsLocal {
	return &AddOnSecretsLocal{
		Enable:     false,
		Objects:    10,
		ObjectSize: 10 * 1024, // 10 KB

		// writes total 100 MB for "Secret" objects,
		// plus "Pod" objects, writes total 330 MB to etcd
		//
		// with 3 nodes, takes about 1.5 hour for all
		// these "Pod"s to complete
		//
		// Objects: 10000,
		// ObjectSize: 10 * 1024, // 10 KB

		NamePrefix: "secret" + randutil.String(5),
	}
}

func (cfg *Config) validateAddOnSecretsLocal() error {
	if !cfg.IsEnabledAddOnSecretsLocal() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnSecretsLocal.Enable true but no node group is enabled")
	}
	if cfg.AddOnSecretsLocal.Namespace == "" {
		cfg.AddOnSecretsLocal.Namespace = cfg.Name + "-secrets-local"
	}

	if cfg.AddOnSecretsLocal.Objects == 0 {
		cfg.AddOnSecretsLocal.Objects = 10
	}
	if cfg.AddOnSecretsLocal.ObjectSize == 0 {
		cfg.AddOnSecretsLocal.ObjectSize = 10 * 1024
	}

	if cfg.AddOnSecretsLocal.NamePrefix == "" {
		cfg.AddOnSecretsLocal.NamePrefix = "secret" + randutil.String(5)
	}

	if cfg.AddOnSecretsLocal.RequestsSummaryWritesJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryWritesJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-summary-writes.json"
	}
	if cfg.AddOnSecretsLocal.RequestsSummaryWritesTablePath == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryWritesTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-summary-writes.txt"
	}
	if cfg.AddOnSecretsLocal.RequestsSummaryReadsJSONPath == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryReadsJSONPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-summary-reads.json"
	}
	if cfg.AddOnSecretsLocal.RequestsSummaryReadsTablePath == "" {
		cfg.AddOnSecretsLocal.RequestsSummaryReadsTablePath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + "-secrets-local-requests-summary-reads.txt"
	}

	return nil
}
