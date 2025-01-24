package training

import (
	"flag"

	"sigs.k8s.io/e2e-framework/pkg/env"
)

// Shared global variables
var (
	testenv env.Environment

	bertTrainingImage *string
	efaEnabled        *bool
	nodeType          *string
	nodeCount         int
	efaPerNode        int
	neuronPerNode     int
	neuronCorePerNode int
	masterPort        *string
)

func init() {
	// Define command-line flags
	bertTrainingImage = flag.String("bertTrainingImage", "", "Docker image used for BERT training workload")
	efaEnabled = flag.Bool("efaEnabled", false, "Enable Elastic Fabric Adapter (EFA)")
	nodeType = flag.String("nodeType", "", "Instance type for cluster nodes (e.g., inf1.24xlarge)")
	masterPort = flag.String("masterPort", "12355", "Port to use for inter-process communication")
}
