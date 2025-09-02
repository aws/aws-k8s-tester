package eksapi

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/internal/metrics"
	"github.com/aws/aws-k8s-tester/internal/util"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	corev1 "k8s.io/api/core/v1"
)

func init() {
	// controller-runtime will complain loudly if this isn't set, even though we don't use this logger
	crlog.SetLogger(zap.New())
}

type k8sClient struct {
	config    *rest.Config
	clientset kubernetes.Interface
	client    client.Client
	dclient   *dynamic.DynamicClient
}

func newK8sClient(kubeconfigPath string) (*k8sClient, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}
	return &k8sClient{
		config:    config,
		clientset: kubernetes.NewForConfigOrDie(config),
		client:    util.Must(client.New(config, client.Options{})),
		dclient:   util.Must(dynamic.NewForConfig(config)),
	}, nil
}

func (k *k8sClient) waitForReadyNodes(nodeCount int, timeout time.Duration) error {
	klog.Infof("waiting up to %v for %d node(s) to be ready...", timeout, nodeCount)
	readyNodes := sets.NewString()
	watcher, err := k.clientset.CoreV1().Nodes().Watch(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to create node watcher: %v", err)
	}
	defer watcher.Stop()
	initialReadyNodes, err := k.getReadyNodes()
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

func (k *k8sClient) waitForNodeDeletion(timeout time.Duration) error {
	klog.Infof("waiting up to %v for node(s) to be deleted...", timeout)
	nodes := sets.NewString()
	watcher, err := k.clientset.CoreV1().Nodes().Watch(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to create node watcher: %v", err)
	}
	defer watcher.Stop()
	initialNodes, err := k.clientset.CoreV1().Nodes().List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %v", err)
	}
	for _, node := range initialNodes.Items {
		nodes.Insert(node.Name)
	}
	ctx, cancelFunc := context.WithTimeout(context.Background(), timeout)
	defer cancelFunc()
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
			if event.Object != nil {
				if node, ok := event.Object.(*corev1.Node); !ok {
					return fmt.Errorf("node watcher received an object that isn't a Node: %+v", event.Object)
				} else {
					switch event.Type {
					case watch.Added:
						nodes.Insert(node.Name)
					case watch.Deleted:
						nodes.Delete(node.Name)
					}
				}
			}
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for nodes to be deleted: %w", ctx.Err())
		}
		if len(nodes) == 0 {
			break
		}
	}
	klog.Info("all nodes deleted!")
	return nil
}

func (k *k8sClient) getReadyNodes() ([]corev1.Node, error) {
	nodes, err := k.clientset.CoreV1().Nodes().List(context.TODO(), v1.ListOptions{})
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

func (k *k8sClient) createAWSAuthConfigMap(nodeNameStrategy string, nodeRoleARN string) error {
	mapRoles, err := generateAuthMapRole(nodeNameStrategy, nodeRoleARN)
	if err != nil {
		return err
	}
	klog.Infof("generated AuthMapRole %s", mapRoles)
	_, err = k.clientset.CoreV1().ConfigMaps("kube-system").Create(context.TODO(), &corev1.ConfigMap{
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

func getNodeInstanceIDs(nodes []corev1.Node) ([]string, error) {
	var instanceIds []string
	var errs []error
	for _, node := range nodes {
		providerId, err := parseKubernetesProviderID(node.Spec.ProviderID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		instanceIds = append(instanceIds, providerId.InstanceID)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return instanceIds, nil
}

func (k *k8sClient) emitNodeMetrics(metricRegistry metrics.MetricRegistry, ec2Client *ec2.Client) error {
	nodes, err := k.getReadyNodes()
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

		// we'll emit the metrics with different subset(s) of dimensions, to make aggregation simpler
		var nodeDimensionSets []map[string]string
		nodeDimensionSets = append(nodeDimensionSets, nodeDimensions)

		var osDistro string
		if strings.HasPrefix(node.Status.NodeInfo.OSImage, "Amazon Linux") {
			// on al2: "Amazon Linux 2"
			// on al2023: "Amazon Linux 2023.6.20241010"
			parts := strings.Split(node.Status.NodeInfo.OSImage, ".")
			amazonLinuxMajorVersion := parts[0]
			osDistro = amazonLinuxMajorVersion
		}

		if osDistro != "" {
			nodeDimensions["osDistro"] = osDistro

			// if we have an osDistro, add a pared-down dimension set that includes it
			nodeDimensionSets = append(nodeDimensionSets, map[string]string{
				"osDistro":     nodeDimensions["osDistro"],
				"instanceType": nodeDimensions["instanceType"],
				"arch":         nodeDimensions["arch"],
			})
		}

		for _, nodeDimensionSet := range nodeDimensionSets {
			metricRegistry.Record(nodeTimeToRegistrationSeconds, timeToRegistration.Seconds(), nodeDimensionSet)
			metricRegistry.Record(nodeTimeToReadySeconds, timeToReady.Seconds(), nodeDimensionSet)
		}
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
