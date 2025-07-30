//go:build e2e

package training

import (
	"sigs.k8s.io/e2e-framework/pkg/env"
)

type Config struct {
	MetricDimensions  map[string]string `flag:"metricDimensions" desc:"CloudWatch metric dimensions as comma-separated key=value pairs"`
	BertTrainingImage string            `flag:"bertTrainingImage" desc:"Docker image used for BERT training workload"`
	EfaEnabled        bool              `flag:"efaEnabled" desc:"Enable Elastic Fabric Adapter (EFA)"`
	NodeType          string            `flag:"nodeType" desc:"Instance type for cluster nodes"`
}

// Shared global variables
var (
	testenv    env.Environment
	testConfig Config

	nodeCount  int
	gpuPerNode int
	efaPerNode int
)
