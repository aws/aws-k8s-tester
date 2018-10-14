package path

import "fmt"

const (
	// Path is the path to send ELB ingress test workloads to.
	Path = "/ingress-test"
	// PathMetrics serves ELB ingress workload metrics.
	PathMetrics = "/ingress-test-metrics"
)

// Create creates a path with index.
func Create(n int) string {
	return fmt.Sprintf("%s-%07d", Path, n)
}
