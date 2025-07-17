//go:build e2e

package training

import (
	"flag"

	"sigs.k8s.io/e2e-framework/pkg/env"
)

// Shared global variables
var (
	testenv           env.Environment
	bertTrainingImage *string
	efaEnabled        *bool
	nodeType          *string
	kubernetesVersion *string
	amiVariant	  	  *string
	teamIdentifier    *string

	nodeCount  int
	gpuPerNode int
	efaPerNode int
)

func init() {
	bertTrainingImage = flag.String("bertTrainingImage", "", "Docker image used for BERT training workload")
	efaEnabled = flag.Bool("efaEnabled", false, "Enable Elastic Fabric Adapter (EFA)")
	nodeType = flag.String("nodeType", "", "Instance type for cluster nodes")
	kubernetesVersion = flag.String("kubernetesVersion", "1.32", "Kubernetes version for the cluster")
	amiVariant = flag.String("amiVariant", "al2023", "AMI variant for the cluster nodes")
	teamIdentifier = flag.String("teamIdentifier", "unknown", "Team identifier for resource tagging")
}
