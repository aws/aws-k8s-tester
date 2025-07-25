//go:build e2e

package efa

import (
	"context"
	_ "embed"
	"fmt"
	"log"

	"github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/aws-sdk-go-v2/aws"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	testenv   env.Environment
	ec2Client e2e.EC2Client

	testImage *string

	pingPongSize            *string
	pingPongIters           *int
	pingPongDeadlineSeconds *int

	nodeType               *string
	expectedEFADeviceCount *int

	verbose *bool
)

const (
	EFA_RESOURCE_NAME   = "vpc.amazonaws.com/efa"
	TEST_NAMESPACE_NAME = "efa-tests"
)

func getEfaCapacity(node corev1.Node) int {
	capacity, ok := node.Status.Capacity[v1.ResourceName(EFA_RESOURCE_NAME)]
	if !ok {
		return 0
	}
	return int(capacity.Value())
}

func getEfaNodes(ctx context.Context, config *envconf.Config) ([]corev1.Node, error) {
	var efaNodes []corev1.Node
	clientset, err := kubernetes.NewForConfig(config.Client().RESTConfig())
	if err != nil {
		return []corev1.Node{}, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return []corev1.Node{}, fmt.Errorf("failed to list nodes: %w", err)
	}

	if len(nodes.Items) == 0 {
		return []corev1.Node{}, fmt.Errorf("no nodes found in the cluster")
	}

	for _, node := range nodes.Items {
		instanceType := node.Labels["node.kubernetes.io/instance-type"]

		if aws.ToString(nodeType) != "" && instanceType != aws.ToString(nodeType) {
			log.Printf("[INFO] Skipping node %s (type: %s), node is not of target type %s", node.Name, instanceType, aws.ToString(nodeType))
			continue
		}

		numEfaDevices, err := e2e.GetNonZeroResourceCapacity(&node, EFA_RESOURCE_NAME)
		if err != nil {
			log.Printf("[INFO] Skipping node %s (type: %s): %v", node.Name, instanceType, err)
			continue
		}

		expectedDeviceCount := aws.ToInt(expectedEFADeviceCount)
		if expectedDeviceCount < 0 {
			instanceInfo, err := ec2Client.DescribeInstanceType(instanceType)
			if err != nil {
				return []corev1.Node{}, err
			}
			expectedDeviceCount = int(aws.ToInt32(instanceInfo.NetworkInfo.EfaInfo.MaximumEfaInterfaces))
		}

		if expectedDeviceCount != numEfaDevices {
			return []corev1.Node{}, fmt.Errorf("unexpected EFA device capacity on node %s: expected %d, got %d", node.Name, expectedDeviceCount, numEfaDevices)
		}

		efaNodes = append(efaNodes, node)
	}

	if len(efaNodes) == 0 {
		return []corev1.Node{}, fmt.Errorf("no nodes with EFA capacity found in the cluster")
	}

	return efaNodes, nil
}
