package eksapi

import (
	"bytes"
	"context"
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const vpcCNIDaemonSetPatch = `{
	"spec": {
		"template": {
			"spec": {
				"containers": [
					{
						"name": "aws-node",
						"env": [
							{
								"name": "ENABLE_PREFIX_DELEGATION",
								"value": "true"
							},
							{
								"name": "MINIMUM_IP_TARGET",
								"value": "80"
							},
							{
								"name": "WARM_IP_TARGET",
								"value": "10"
							}
						]
					}
				]
			}
		}
	}
}`

// tuneVPCCNI applies configuration to the VPC CNI DaemonSet that helps prevent test flakiness
func (k *k8sClient) tuneVPCCNI() error {
	var patch bytes.Buffer
	if err := json.Compact(&patch, []byte(vpcCNIDaemonSetPatch)); err != nil {
		return err
	}
	_, err := k.clientset.AppsV1().DaemonSets("kube-system").Patch(context.TODO(), "aws-node", types.StrategicMergePatchType, patch.Bytes(), metav1.PatchOptions{})
	return err
}
