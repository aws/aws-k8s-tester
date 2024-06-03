package eksapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var cluster = Cluster{
	name:                     "cluster",
	endpoint:                 "https://example.com",
	certificateAuthorityData: "certificateAuthority",
	cidr:                     "10.100.0.0/16",
}

const bootstrapShUserData = `Content-Type: text/x-shellscript; charset="us-ascii"
MIME-Version: 1.0

#!/usr/bin/env bash
/etc/eks/bootstrap.sh cluster \
  --b64-cluster-ca certificateAuthority \
  --apiserver-endpoint https://example.com
`

const nodeadmUserData = `Content-Type: application/node.eks.aws
MIME-Version: 1.0

---
apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
  cluster:
    name: cluster
    apiServerEndpoint: https://example.com
    certificateAuthority: certificateAuthority
	cidr: 10.100.0.0/16
`

func Test_generateUserData(t *testing.T) {
	cases := []struct {
		format             string
		expected           string
		expectedIsMimePart bool
	}{
		{
			format:             "bootstrap.sh",
			expected:           bootstrapShUserData,
			expectedIsMimePart: true,
		},
		{
			format:             "nodeadm",
			expected:           nodeadmUserData,
			expectedIsMimePart: true,
		},
	}
	for _, c := range cases {
		t.Run(c.format, func(t *testing.T) {
			actual, isMimePart, err := generateUserData(c.format, &cluster)
			if err != nil {
				t.Log(err)
				t.Error(err)
			}
			assert.Equal(t, c.expected, actual)
			assert.Equal(t, c.expectedIsMimePart, isMimePart)
		})
	}
}
