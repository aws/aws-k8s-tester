//go:build e2e

package training

import (
	"flag"
	"github.com/octago/sflags/gen/gpflag"
	"github.com/spf13/pflag"
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
	testenv    env.Environment
	testConfig = &Config{}

	nodeCount  int
	gpuPerNode int
	efaPerNode int
)

func init() {
	flags, err := gpflag.Parse(testConfig)
	if err != nil {
		panic(err)
	}
	flags.VisitAll(func(pf *pflag.Flag) {
		flag.CommandLine.Var(pf.Value, pf.Name, pf.Usage)
	})
}
