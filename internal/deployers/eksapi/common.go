package eksapi

import (
	"os"
	"slices"
	"strings"
)

const AvailabilityZonePriorityEnv = "EKSAPI_AZ_PRIORITY"

func availabilityZoneHintedOrder(availabilityZones []string) []string {
	var priorityAZs []string
	if priorityAZsString, ok := os.LookupEnv(AvailabilityZonePriorityEnv); ok {
		priorityAZs = strings.Split(priorityAZsString, ",")
	}
	if len(priorityAZs) == 0 {
		return availabilityZones
	}
	return slices.SortedStableFunc(slices.Values(availabilityZones), func(az1, az2 string) int {
		if slices.Contains(priorityAZs, az1) {
			if slices.Contains(priorityAZs, az2) {
				return 0
			}
			return -1
		}
		return 0
	})
}
