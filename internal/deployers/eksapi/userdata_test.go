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

const nodeadmUserDataKubeletDRA = `Content-Type: application/node.eks.aws
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
  kubelet:
    config:
      featureGates:
        DynamicResourceAllocation: true
`

const nodeadmUserDataFeatureGate = `Content-Type: application/node.eks.aws
MIME-Version: 1.0

---
apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
  featureGates:
    foo: true
  cluster:
    name: cluster
    apiServerEndpoint: https://example.com
    certificateAuthority: certificateAuthority
    cidr: 10.100.0.0/16
`

const bottlerocketUserData = `[settings.kubernetes]
"cluster-name" = "cluster"
"api-server" = "https://example.com"
"cluster-certificate" = "certificateAuthority"
device-ownership-from-security-context = true

[settings.host-containers.admin]
"enabled" = true
`

func Test_generateUserData(t *testing.T) {
	cases := []struct {
		format              string
		expected            string
		expectedIsMimePart  bool
		kubernetesVersion   string
		NodeadmFeatureGates []string
		wantErr             bool
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
		{
			format:             "bottlerocket",
			expected:           bottlerocketUserData,
			expectedIsMimePart: false,
		},
		{
			format:             "nodeadm",
			expected:           nodeadmUserDataKubeletDRA,
			kubernetesVersion:  "1.33",
			expectedIsMimePart: true,
		},
		{
			format:              "nodeadm",
			expected:            nodeadmUserDataFeatureGate,
			kubernetesVersion:   "1.30",
			NodeadmFeatureGates: []string{"foo=true"},
			expectedIsMimePart:  true,
		},
	}
	for _, c := range cases {
		t.Run(c.format, func(t *testing.T) {
			deployerOpts := &deployerOptions{
				KubernetesVersion:   c.kubernetesVersion,
				NodeadmFeatureGates: c.NodeadmFeatureGates,
				UserDataFormat:      c.format,
			}
			actual, isMimePart, err := generateUserData(&cluster, deployerOpts)
			if err != nil {
				t.Log(err)
				t.Error(err)
			}
			assert.Equal(t, c.expected, actual)
			assert.Equal(t, c.expectedIsMimePart, isMimePart)
		})
	}
}

func Test_extractFeatureGates(t *testing.T) {
	testCases := []struct {
		input     []string
		expected  map[string]bool
		expectErr bool
	}{
		{
			input: []string{"foo=true", "bar=false"},
			expected: map[string]bool{
				"foo": true,
				"bar": false,
			},
		},
		{
			input:     []string{"foo:true"},
			expectErr: true,
		},
		{
			input:     []string{"foo=bar"},
			expectErr: true,
		},
	}
	for _, testCase := range testCases {
		output, err := extractFeatureGates(testCase.input)
		if testCase.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, testCase.expected, output)
		}
	}
}
