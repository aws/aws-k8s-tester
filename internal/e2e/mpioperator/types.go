package mpioperator

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	facadeSchemeBuilder = runtime.NewSchemeBuilder(addFacadeTypes)
	AddFacadesToScheme  = facadeSchemeBuilder.AddToScheme
)

func addFacadeTypes(s *runtime.Scheme) error {
	for _, version := range []string{"v2beta1", runtime.APIVersionInternal} {
		s.AddKnownTypeWithName(schema.GroupVersionKind{
			Group:   "kubeflow.org",
			Version: version,
			Kind:    "MPIJob",
		}, &MPIJobFacade{})
	}
	return nil
}

// MPIJobFacade is a utility type for working with specific fields of a kubeflow.org/v2beta1 MPIJob
// without needing a code dependency on the full external types
type MPIJobFacade struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Status            MPIJobFacadeStatus `json:"status"`
}

func (m *MPIJobFacade) DeepCopyObject() runtime.Object {
	// no-op
	return nil
}

type MPIJobFacadeStatus struct {
	Conditions []MPIJobFacadeStatusCondition `json:"conditions"`
}

type MPIJobFacadeStatusCondition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}
