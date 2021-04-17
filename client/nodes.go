package client

import (
	"context"
	"time"

	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_client "k8s.io/client-go/kubernetes"
)

// ListNodes returns list of cluster nodes.
func ListNodes(cli k8s_client.Interface) ([]core_v1.Node, error) {
	return ListNodesWithOptions(cli, meta_v1.ListOptions{})
}

// ListNodesWithOptions lists the cluster nodes using the provided options.
func ListNodesWithOptions(cli k8s_client.Interface, listOpts meta_v1.ListOptions) ([]core_v1.Node, error) {
	var nodes []core_v1.Node
	listFunc := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		nodesList, err := cli.CoreV1().Nodes().List(ctx, listOpts)
		cancel()
		if err != nil {
			return err
		}
		nodes = nodesList.Items
		return nil
	}
	if err := RetryWithExponentialBackOff(RetryFunction(listFunc)); err != nil {
		return nodes, err
	}
	return nodes, nil
}
