package mpijobs

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/e2e-framework/klient/k8s"
)

// MPIJobSucceeded returns true if the specified k8s.Object is an unstructured.Unstructured
// with .status.conditions["Succeeded"] = "True"
func MPIJobSucceeded(obj k8s.Object) bool {
	u := obj.(*unstructured.Unstructured)
	conditions, found, err := unstructured.NestedSlice(u.Object, "status", "conditions")
	if err != nil {
		panic(fmt.Errorf("MPIJob does not match expected schema: %v", err))
	}
	if !found {
		return false
	}
	for _, condition := range conditions {
		c := condition.(map[string]interface{})
		cType, found, err := unstructured.NestedString(c, "type")
		if err != nil {
			panic(fmt.Errorf("MPIJob does not match expected schema: %v", err))
		}
		if !found {
			continue
		}
		if cType == "Succeeded" {
			cStatus, found, err := unstructured.NestedString(c, "status")
			if err != nil {
				panic(fmt.Errorf("MPIJob does not match expected schema: %v", err))
			}
			if !found {
				continue
			}
			return cStatus == "True"
		}
	}
	return false
}
