package e2e

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
)

func GetNonZeroResourceCapacity(node v1.Node, resourceName string) (int, error) {
	capacity, ok := node.Status.Capacity[v1.ResourceName(resourceName)]
	if !ok {
		return 0, fmt.Errorf("node %q has no resource %q", node.Name, resourceName)
	}
	if capacity.Value() == 0 {
		return 0, fmt.Errorf("node %q has zero capacity for resource %q", node.Name, resourceName)
	}
	return int(capacity.Value()), nil
}
