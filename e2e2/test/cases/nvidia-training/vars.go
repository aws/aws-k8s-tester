package training

import (
	"flag"

	"sigs.k8s.io/e2e-framework/pkg/env"
)

// Global variables shared across the package
var (
	testenv           env.Environment
	bertTrainingImage *string
	efaEnabled        *bool
	nodeType          *string
)

func init() {
	// Parse command-line arguments
	bertTrainingImage = flag.String("bertTrainingImage", "", "Docker image used for BERT training workload")
	efaEnabled = flag.Bool("efaEnabled", false, "Enable Elastic Fabric Adapter (EFA) for the training workload")
	nodeType = flag.String("nodeType", "p5.48xlarge", "Instance type to be used for training nodes")
}
