package prow

import (
	"fmt"
	"strings"
)

func toGroup(name, id string) string {
	if name == "GoogleCloudPlatform/k8s-multicluster-ingress" {
		// group the following to "k8s-multicluster-ingress"
		// GoogleCloudPlatform/k8s-multicluster-ingress:
		// - name: pull-kubernetes-multicluster-ingress-test
		return "k8s-multicluster-ingress"
	}
	if name == "periodic" {
		for i, v := range strings.Split(id, "-") {
			name += fmt.Sprintf("-%s", v)
			if i == 2 { // use up to 3
				break
			}
		}
	}
	return name
}
