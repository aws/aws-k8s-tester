package mpijobs

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var MPIJobGVK = schema.GroupVersionKind{
	Group:   "kubeflow.org",
	Version: "v2beta1",
	Kind:    "MPIJob",
}

func NewUnstructured(name, namespace string) *unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(MPIJobGVK)
	u.SetName(name)
	u.SetNamespace(namespace)
	return &u
}
