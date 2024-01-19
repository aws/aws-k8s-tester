package eksapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

rolearn := "mock-role-arn"

const sessionNamedAuthMapRole = `
- username: system:node:{{SessionName}} 
  groups:
    - system:bootstrappers
    - system:nodes
  rolearn: mock-role-arn`

const privateDNSNamedAuthMapRole = `
- username: system:node:{{EC2PrivateDNSName}} 
  groups:
    - system:bootstrappers
    - system:nodes
  rolearn: mock-role-arn`

func Test_generateAuthRoleMap(t *testing.T) {
	cases := []struct {
		nodeNameStrategy string
		expected string
	}{
		{
			nodeNameStrategy: "SessionName",
			expected:         sessionNamedAuthMapRole,
		},
		{
			nodeNameStrategy: "EC2PrivateDNSName",
			expected:         privateDNSNamedAuthMapRole,
		},
	}
	for _, c := range cases {
		t.Run(c.nodeNameStrategy, func(t *testing.T) {
			actual, err := generateAuthMapRole(c.nodeNameStrategy, rolearn)
			if err != nil {
				t.Log(err)
				t.Error(err)
			}
			assert.Equal(t, c.expected, actual)
		})
	}
}
