package e2e

import (
	"context"
	"fmt"
	"io"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// PrintDaemonSetPodLogs retrieves logs from each container in each pod of a DaemonSet.
// namespace & labelSelector identify the DaemonSet's pods (e.g. "default", "app=containerd-check").
func PrintDaemonSetPodLogs(
	ctx context.Context,
	restConfig *rest.Config,
	namespace string,
	labelSelector string,
	t *testing.T,
) {
	clientset, err := getClientset(restConfig)
	if err != nil {
		t.Logf("failed to create typed clientset: %v", err)
		return
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		t.Logf("failed to list pods: %v", err)
		return
	}
	if len(pods.Items) == 0 {
		t.Logf("No pods found for DaemonSet with label %q in namespace %q.", labelSelector, namespace)
		return
	}

	for _, pod := range pods.Items {
		t.Logf("Pod %s status: %s", pod.Name, pod.Status.Phase)
		for _, container := range pod.Spec.Containers {
			logs, logErr := readPodLogs(ctx, clientset, pod.Namespace, pod.Name, container.Name)
			if logErr != nil {
				t.Logf("Failed reading logs from %s/%s: %v", pod.Name, container.Name, logErr)
			} else {
				t.Logf("=== Logs from %s/%s ===\n%s", pod.Name, container.Name, logs)
			}
		}
	}
}

// readPodLogs streams logs for a specific container in a pod.
func readPodLogs(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	namespace, podName, containerName string,
) (string, error) {
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: containerName,
	})
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to open log stream for %s/%s: %w", podName, containerName, err)
	}
	defer stream.Close()

	var out string
	buf := make([]byte, 4096)
	for {
		n, readErr := stream.Read(buf)
		if n > 0 {
			out += string(buf[:n])
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return out, fmt.Errorf("error reading logs: %w", readErr)
		}
	}
	return out, nil
}

// getClientset returns a typed Kubernetes clientset from the given rest.Config.
func getClientset(restConfig *rest.Config) (*kubernetes.Clientset, error) {
	cs, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}
	return cs, nil
}
