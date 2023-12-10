package eksapi

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	corev1 "k8s.io/api/core/v1"
)

func newKubernetesClient(kubeconfigPath string) (*kubernetes.Clientset, error) {
	c, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(c)
}

func waitForReadyNodes(client *kubernetes.Clientset, nodeCount int, timeout time.Duration) error {
	readyNodes := sets.NewString()
	watcher, err := client.CoreV1().Nodes().Watch(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "creating node watcher")
	}
	defer watcher.Stop()
	counter, err := getReadyNodes(client)
	if err != nil {
		return errors.Wrap(err, "listing nodes")
	}
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("the watcher channel for the nodes was closed by Kubernetes due to an unknown error")
			}
			if event.Type == watch.Error {
				msg := "unexpected error event type from node watcher"
				if statusErr, ok := event.Object.(*metav1.Status); ok {
					return fmt.Errorf("%s: %s", msg, statusErr.String())
				}
				return fmt.Errorf("%s: %+v", msg, event.Object)
			}
			if event.Object != nil && event.Type != watch.Deleted {
				if node, ok := event.Object.(*corev1.Node); ok {
					if isNodeReady(node) {
						readyNodes.Insert(node.Name)
						counter = readyNodes.Len()
					}
				}
			}
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for %d nodes to be ready: %w", nodeCount, ctx.Err())
		}
		if counter >= nodeCount {
			break
		}
	}
	return nil
}

func getReadyNodes(client kubernetes.Interface) (int, error) {
	nodes, err := client.CoreV1().Nodes().List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return 0, err
	}
	counter := 0
	for _, node := range nodes.Items {
		if isNodeReady(&node) {
			counter++
		}
	}
	return counter, nil
}

func isNodeReady(node *corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

const awsAuthMapRolesPrefix = `
- username: system:node:{{EC2PrivateDNSName}}
  groups:
    - system:bootstrappers
    - system:nodes
  rolearn: `

func createAWSAuthConfigMap(client *kubernetes.Clientset, nodeRoleARN string) error {
	mapRoles := awsAuthMapRolesPrefix + nodeRoleARN
	_, err := client.CoreV1().ConfigMaps("kube-system").Create(context.TODO(), &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "aws-auth",
			Namespace: "kube-system",
		},
		Data: map[string]string{
			"mapRoles": mapRoles,
		},
	}, metav1.CreateOptions{})
	return err
}
