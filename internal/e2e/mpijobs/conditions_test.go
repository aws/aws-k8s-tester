package mpijobs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_MPIJobSucceeded(t *testing.T) {
	u := unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Succeeded",
						"status": "True",
					},
				},
			},
		},
	}
	assert.True(t, MPIJobSucceeded(&u))

	u = unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Succeeded",
						"status": "False",
					},
				},
			},
		},
	}
	assert.False(t, MPIJobSucceeded(&u))
}
