package e2e

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// KubeletIsResponsive returns true if the kubelet /healthz endpoint responds with a 200 status code, and propagates
// any non-connection specific errors
func KubeletIsResponsive(ctx context.Context, cfg *rest.Config, nodeName string) (bool, error) {
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return false, fmt.Errorf("failed to initialize client set: %v", err)
	}

	nodeHealthResponse := client.CoreV1().RESTClient().Get().Resource("nodes").
		Name(nodeName).SubResource("proxy").Suffix("/healthz").
		Do(ctx)

	if nodeHealthResponse.Error() != nil {
		errMsg := nodeHealthResponse.Error().Error()
		// TODO: match errors against types, e.g. syscall.ECONNREFUSED instead, the k8s client doesn't
		// currently properly wrap the underlying error to allow this though
		if strings.Contains(errMsg, "connection refused") ||
			strings.Contains(errMsg, "connection reset by peer") ||
			strings.Contains(errMsg, "http2: client connection lost") {
			// these errors indicate reachability to the node in general but an unstable connection to kubelet
			return false, nil
		}

		// propagate other errors, e.g. i/o timeout, that may result from things unrelated to kubelet health,
		// e.g. security group rules on the instance restricting traffic from the CP
		return false, fmt.Errorf("could not reach /healthz endpoint for node %s: %w", nodeName, nodeHealthResponse.Error())
	}

	var statusCode int
	nodeHealthResponse.StatusCode(&statusCode)
	return statusCode == 200, nil
}
