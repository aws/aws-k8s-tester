//go:build e2e

package training

import (
	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	"sigs.k8s.io/e2e-framework/pkg/env"
)

type Config struct {
	fwext.MetricOps
	BertTrainingImage string `flag:"bertTrainingImage" desc:"Docker image used for BERT training workload"`
	EfaEnabled        bool   `flag:"efaEnabled" desc:"Enable Elastic Fabric Adapter (EFA)"`
	NodeType          string `flag:"nodeType" desc:"Instance type for cluster nodes"`
}

// Shared global variables
var (
	testenv    env.Environment
	testConfig Config

	nodeCount  int
	gpuPerNode int
	efaPerNode int
)
