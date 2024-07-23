package eksapi

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/kubetest2/internal/metrics"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

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
	klog.Infof("waiting up to %v for %d node(s) to be ready...", timeout, nodeCount)
	readyNodes := sets.NewString()
	watcher, err := client.CoreV1().Nodes().Watch(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to create node watcher: %v", err)
	}
	defer watcher.Stop()
	initialReadyNodes, err := getReadyNodes(client)
	if err != nil {
		return fmt.Errorf("failed to get ready nodes: %v", err)
	}
	counter := len(initialReadyNodes)
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
	klog.Infof("%d node(s) are ready: %v", readyNodes.Len(), readyNodes)
	return nil
}

func getReadyNodes(client kubernetes.Interface) ([]corev1.Node, error) {
	nodes, err := client.CoreV1().Nodes().List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var readyNodes []corev1.Node
	for _, node := range nodes.Items {
		if isNodeReady(&node) {
			readyNodes = append(readyNodes, node)
		}
	}
	return readyNodes, nil
}

func isNodeReady(node *corev1.Node) bool {
	c := getNodeReadyCondition(node)
	if c == nil {
		return false
	}
	return c.Status == corev1.ConditionTrue
}

func getNodeReadyCondition(node *corev1.Node) *corev1.NodeCondition {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return &c
		}
	}
	return nil
}

func createAWSAuthConfigMap(client *kubernetes.Clientset, nodeNameStrategy string, nodeRoleARN string) error {
	mapRoles, err := generateAuthMapRole(nodeNameStrategy, nodeRoleARN)
	if err != nil {
		return err
	}
	klog.Infof("generated AuthMapRole %s", mapRoles)
	_, err = client.CoreV1().ConfigMaps("kube-system").Create(context.TODO(), &corev1.ConfigMap{
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

func emitNodeMetrics(metricRegistry metrics.MetricRegistry, k8sClient *kubernetes.Clientset, ec2Client *ec2.Client) error {
	nodes, err := getReadyNodes(k8sClient)
	if err != nil {
		return err
	}
	var errs []error
	for _, node := range nodes {
		providerId, err := parseKubernetesProviderID(node.Spec.ProviderID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		instanceInfo, err := ec2Client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
			InstanceIds: []string{providerId.InstanceID},
		})
		if err != nil {
			errs = append(errs, err)
			continue
		}
		instance := instanceInfo.Reservations[0].Instances[0]
		launchTime := *instance.LaunchTime
		timeToRegistration := node.ObjectMeta.CreationTimestamp.Time.Sub(launchTime)
		timeToReady := getNodeReadyCondition(&node).LastTransitionTime.Time.Sub(launchTime)

		nodeDimensions := map[string]string{
			"instanceType": string(instance.InstanceType),
			"os":           node.Status.NodeInfo.OperatingSystem,
			"osImage":      node.Status.NodeInfo.OSImage,
			"arch":         node.Status.NodeInfo.Architecture,
		}

		metricRegistry.Record(nodeTimeToRegistrationSeconds, timeToRegistration.Seconds(), nodeDimensions)
		metricRegistry.Record(nodeTimeToReadySeconds, timeToReady.Seconds(), nodeDimensions)
	}
	return errors.Join(errs...)
}

type KubernetesProviderID struct {
	AvailabilityZone string
	InstanceID       string
}

func parseKubernetesProviderID(rawProviderId string) (*KubernetesProviderID, error) {
	url, err := url.Parse(rawProviderId)
	if err != nil {
		return nil, fmt.Errorf("malformed provider ID: %s", rawProviderId)
	}
	if url.Scheme != "aws" {
		return nil, fmt.Errorf("usupported provider ID scheme: %s", url.Scheme)
	}
	if url.Path == "" {
		return nil, fmt.Errorf("provider ID path is empty: %s", rawProviderId)
	}
	// example: /us-west-2a/i-12345abcdefg
	parts := strings.Split(url.Path, "/")
	if len(parts) != 3 {
		return nil, fmt.Errorf("provider ID path does not have 3 parts: %s", url.Path)
	}
	return &KubernetesProviderID{
		AvailabilityZone: parts[1],
		InstanceID:       parts[2],
	}, nil
}
