package e2e

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
)

func GetNonZeroResourceCapacityOrError(node v1.Node, resourceName string) (int, error) {
	capacity, ok := node.Status.Capacity[v1.ResourceName(resourceName)]
	if !ok {
		return 0, fmt.Errorf("node \"%s\" has no resource \"%s\"", node.Name, resourceName)
	}
	if capacity.Value() == 0 {
		return 0, fmt.Errorf("node \"%s\" has zero capacity for resource \"%s\"", node.Name, resourceName)
	}
	return int(capacity.Value()), nil
}
