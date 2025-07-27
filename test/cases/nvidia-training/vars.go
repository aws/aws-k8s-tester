//go:build e2e

package training

import (
	"github.com/urfave/sflags/gen/gflag"
	"sigs.k8s.io/e2e-framework/pkg/env"
)

type Config struct {
	BertTrainingImage string   `flag:"bertTrainingImage" desc:"Docker image used for BERT training workload"`
	EfaEnabled        bool     `flag:"efaEnabled" desc:"Enable Elastic Fabric Adapter (EFA)"`
	NodeType          string   `flag:"nodeType" desc:"Instance type for cluster nodes"`
	MetricDimensions  []string `flag:"metricDimensions" desc:"Metric dimensions as comma-separated key=value pairs"`
}

// Shared global variables
var (
	testenv           env.Environment
	bertTrainingImage *string
	efaEnabled        *bool
	nodeType          *string
	metricDimensions  *[]string
	testConfig        = &Config{}

	nodeCount  int
	gpuPerNode int
	efaPerNode int
)

func init() {
	gflag.ParseToDef(testConfig)
	bertTrainingImage = &testConfig.BertTrainingImage
	efaEnabled = &testConfig.EfaEnabled
	nodeType = &testConfig.NodeType
	metricDimensions = &testConfig.MetricDimensions
}
