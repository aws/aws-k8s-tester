//go:build e2e

package quick

import (
	"context"
	_ "embed"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/aws-sdk-go-v2/aws"
	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider-aws/pkg/providers/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestNodeTopology(t *testing.T) {
	topology := features.New("node-topology").
		WithLabel("suite", "node-topology").
		Assess("Nodes have correct network topology labels", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {

			var nodes v1.NodeList
			cfg.Client().Resources().List(ctx, &nodes)

			if len(nodes.Items) == 0 {
				t.Fatal("no nodes found in the cluster")
			}

			nodeMap := make(map[string]v1.Node)
			var instanceIDs []string
			ec2Client := e2e.NewEC2Client()
			for _, node := range nodes.Items {
				providerIDParts := strings.Split(node.Spec.ProviderID, "/")
				instanceID := providerIDParts[len(providerIDParts)-1]
				instanceIDs = append(instanceIDs, instanceID)
				nodeMap[instanceID] = node
			}

			nodeTopologies, err := ec2Client.DescribeInstanceTopology(instanceIDs)
			if err != nil {
				t.Fatalf("could not describe instance topologies: %v", err)
			}

			t.Logf("checking instance topologies for %d node(s) (out of %d node(s) in the cluster)", len(nodeTopologies), len(instanceIDs))

			for _, nodeTopology := range nodeTopologies {
				node := nodeMap[aws.ToString(nodeTopology.InstanceId)]
				instanceType := node.Labels["node.kubernetes.io/instance-type"]

				t.Logf("verifying instance topology for node %s (type: %s)", node.Name, instanceType)

				for i, networkNode := range nodeTopology.NetworkNodes {
					// https://github.com/kubernetes/cloud-provider-aws/blob/b47d2cf2a33ae655cd353ec42ea43362b804c397/pkg/providers/v1/well_known_labels.go#L26
					expectedLabel := cloudprovider.LabelNetworkNodePrefix + strconv.Itoa(i+1)
					if actualValue, ok := node.Labels[expectedLabel]; !ok {
						t.Errorf("node %s (type: %s) does not have expected network label %s", node.Name, instanceType, expectedLabel)
					} else if actualValue != networkNode {
						t.Errorf("node %s (type: %s) has incorrect value for label %s: expected %s, got %s", node.Name, instanceType, expectedLabel, networkNode, actualValue)
					}
				}

				// https://github.com/kubernetes/cloud-provider-aws/blob/b47d2cf2a33ae655cd353ec42ea43362b804c397/pkg/providers/v1/well_known_labels.go#L22C2-L22C13
				if aws.ToString(nodeTopology.ZoneId) != node.Labels[cloudprovider.LabelZoneID] {
					t.Logf("node %s (type: %s) has incorrect value for label %s: expected %s, got %s", node.Name, instanceType, cloudprovider.LabelZoneID, aws.ToString(nodeTopology.ZoneId), node.Labels[cloudprovider.LabelZoneID])
					t.Fail()
				}
			}

			return ctx
		}).Feature()

	testenv.Test(t, topology)
}
