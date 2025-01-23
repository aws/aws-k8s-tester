//go:build e2e

package inference

import (
	"flag"

	"sigs.k8s.io/e2e-framework/pkg/env"
)

// Shared global variables
var (
	// The e2e-framework environment
	testenv env.Environment

	// Passed in as flags
	bertInferenceImage *string
	nodeType           *string
	inferenceMode      *string

	// Discovered in main_test.go
	neuronPerNode     int
	neuronCorePerNode int
)

// init() runs before TestMain and sets up the flags
func init() {
	bertInferenceImage = flag.String("bertInferenceImage", "",
		"[REQUIRED] Docker image used for Neuron-based BERT inference")
	nodeType = flag.String("nodeType", "",
		"Node type label for K8s nodes, e.g., trn1.32xlarge or inf2.xlarge")
	inferenceMode = flag.String("inferenceMode", "throughput",
		"Inference mode for BERT (throughput or latency)")
}
