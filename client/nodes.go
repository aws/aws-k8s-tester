package client

import (
	"context"
	"fmt"
	"net"
	"time"

	core_v1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	k8s_client "k8s.io/client-go/kubernetes"
)

const (
	// ssh port
	sshPort = "22"
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

// GetInternalIP returns node internal IP
func GetInternalIP(node *core_v1.Node) (string, error) {
	host := ""
	for _, address := range node.Status.Addresses {
		if address.Type == v1.NodeInternalIP && address.Address != "" {
			host = net.JoinHostPort(address.Address, sshPort)
			break
		}
	}
	if host == "" {
		return "", fmt.Errorf("Couldn't get the internal IP of host %s with addresses %v", node.Name, node.Status.Addresses)
	}
	return host, nil
}

// GetRandomReadySchedulableNode gets a single randomly-selected node which is available for
// running pods on. If there are no available nodes it will return an error.
func GetRandomReadySchedulableNode(cli k8s_client.Interface) (*core_v1.Node, error) {
	nodes, err := ListNodes(cli)
	if err != nil {
		return nil, err
	}

	return &nodes[rand.Intn(len(nodes))], nil
}
