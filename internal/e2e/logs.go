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
	t *testing.T,
	ctx context.Context,
	restConfig *rest.Config,
	namespace string,
	labelSelector string,
) {
	clientset, err := kubernetes.NewForConfig(restConfig)
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
			logs, logErr := ReadPodLogs(ctx, restConfig, pod.Namespace, pod.Name, container.Name)
			if logErr != nil {
				t.Logf("Failed reading logs from %s/%s: %v", pod.Name, container.Name, logErr)
			} else {
				t.Logf("=== Logs from %s/%s ===\n%s", pod.Name, container.Name, logs)
			}
		}
	}
}

// ReadPodLogs streams logs for a specific container in a pod.
func ReadPodLogs(
	ctx context.Context,
	restConfig *rest.Config,
	namespace, podName, containerName string,
) (string, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create typed clientset: %w", err)
	}
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: containerName,
	})
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to open log stream for %s/%s: %w", podName, containerName, err)
	}
	defer stream.Close()

	data, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("error reading logs: %w", err)
	}
	return string(data), nil
}
