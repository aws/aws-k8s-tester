package mpioperator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

const mpiJobYaml = `---
apiVersion: kubeflow.org/v2beta1
kind: MPIJob
metadata:
  name: foo
status:
  conditions:
    - type: "Succeeded"
      status: "True"
`

func Test_serialization(t *testing.T) {
	scheme := runtime.NewScheme()
	err := AddFacadesToScheme(scheme)
	if err != nil {
		t.Errorf("failed to add facades to scheme: %v", err)
		return
	}
	codecs := serializer.NewCodecFactory(scheme)
	obj, gvk, err := codecs.UniversalDecoder().Decode([]byte(mpiJobYaml), nil, nil)
	if err != nil {
		t.Errorf("failed to decode: %v", err)
		return
	}
	if gvk.Kind != "MPIJob" {
		t.Errorf("failed to decode %q (wrong Kind)", gvk.Kind)
	}
	if facade, ok := obj.(*MPIJobFacade); !ok {
		t.Errorf("failed to cast MPIJob to MPIJob facade!")
	} else {
		assert.Equal(t, "Succeeded", facade.Status.Conditions[0].Type)
	}
}
