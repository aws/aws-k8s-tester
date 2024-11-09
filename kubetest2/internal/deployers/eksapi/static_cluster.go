package eksapi

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/kubetest2/internal/deployers/eksapi/templates"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/yaml"
)

type StaticClusterManager struct {
	k8sClient       *kubernetes.Clientset
	karpenterClient client.Client
	options         *deployerOptions
}

type NodeCondition func(nodes []corev1.Node) bool

func NewStaticClusterManager(options *deployerOptions) *StaticClusterManager {
	return &StaticClusterManager{
		options: options,
	}
}

func (s *StaticClusterManager) SetK8sClient(kubeconfig string) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("Failed to build kubeconfig: %v", err)
	}

	s.k8sClient, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	s.karpenterClient, err = client.New(cfg, client.Options{})
	if err != nil {
		log.Fatalf("Failed to create Karpenter client: %v", err)
	}
}

func (s *StaticClusterManager) EnsureNodeForStaticCluster() error {
	if err := s.CreateNodePool(); err != nil {
		return err
	}
	return s.DeployBusyboxAndWaitForNodes()
}

func (s *StaticClusterManager) TearDownNodeForStaticCluster() error {
	if err := s.TearDownBusyboxAndNodes(); err != nil {
		return err
	}
	return s.TearDownNodePool()
}

func (s *StaticClusterManager) CreateNodePool() error {
	if !strings.Contains(strings.ToLower(s.options.StaticClusterName), "nvidia") {
		klog.Info("NVIDIA not in cluster name, skipping node pool creation")
		return nil
	}

	var arch string
	if strings.Contains(s.options.StaticClusterName, "x86_64") {
		arch = "amd64"
	} else if strings.Contains(s.options.StaticClusterName, "aarch64") {
		arch = "arm64"
	} else {
		return fmt.Errorf("unable to determine architecture from cluster name")
	}

	t := templates.NvidiaStaticClusterNodepool
	var buf bytes.Buffer
	if err := t.Execute(&buf, templates.NvidiaStaticClusterNodepoolTemplateData{
		Arch:          arch,
		InstanceTypes: s.options.InstanceTypes,
	}); err != nil {
		return err
	}

	nodePool := &karpv1.NodePool{}
	if err := yaml.Unmarshal(buf.Bytes(), nodePool); err != nil {
		return fmt.Errorf("failed to unmarshal nodepool YAML: %v", err)
	}

	ctx := context.TODO()
	existing := &karpv1.NodePool{}
	err := s.karpenterClient.Get(ctx, client.ObjectKey{Name: nodePool.Name}, existing)
	if client.IgnoreNotFound(err) != nil {
		return err
	}

	if errors.IsNotFound(err) {
		return s.karpenterClient.Create(ctx, nodePool)
	}
	return nil
}

func (s *StaticClusterManager) TearDownNodePool() error {
	if !strings.Contains(strings.ToLower(s.options.StaticClusterName), "nvidia") {
		klog.Info("NVIDIA not in cluster name, skipping node pool deletion")
		return nil
	}

	nodePool := &karpv1.NodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nvidia",
		},
	}

	if err := s.karpenterClient.Delete(context.TODO(), nodePool); err != nil {
		if errors.IsNotFound(err) {
			klog.Info("NodePool 'nvidia' not found, skipping deletion")
			return nil
		}
		return fmt.Errorf("failed to delete nodepool: %v", err)
	}

	klog.Info("NodePool deleted successfully")
	return nil
}

func (s *StaticClusterManager) DeployBusyboxAndWaitForNodes() error {
	klog.Infof("Deploying busybox pods")

	t := templates.BusyboxDeployment
	var buf bytes.Buffer
	if err := t.Execute(&buf, templates.BusyboxDeploymentTemplateData{
		Nodes: s.options.Nodes,
	}); err != nil {
		return err
	}

	deployment := &v1.Deployment{}
	err := yaml.Unmarshal(buf.Bytes(), deployment)
	if err != nil {
		return fmt.Errorf("failed to unmarshal deployment: %v", err)
	}

	result, err := s.k8sClient.AppsV1().Deployments("default").Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	klog.Infof("Created deployment %q.\n", result.GetObjectMeta().GetName())
	return waitForNodeCondition(s.k8sClient, func(nodes []corev1.Node) bool {
		readyNodes := 0
		for _, node := range nodes {
			if isNodeReady(&node) {
				readyNodes++
			}
		}
		klog.Infof("Ready nodes: %d, Expected nodes: %d", readyNodes, s.options.Nodes)
		return readyNodes >= s.options.Nodes
	}, 15*time.Minute, "Waiting for nodes to be ready")
}

func (s *StaticClusterManager) TearDownBusyboxAndNodes() error {
	klog.Infof("Cleaning up busybox pods")

	err := s.k8sClient.AppsV1().Deployments("default").Delete(context.TODO(), "busybox-deployment", metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete deployment: %v", err)
	}
	klog.Info("Busybox deployment deleted successfully")

	return waitForNodeCondition(s.k8sClient, func(nodes []corev1.Node) bool {
		return len(nodes) == 0
	}, 10*time.Minute, "Waiting for nodes to be removed")
}

func waitForNodeCondition(clientset *kubernetes.Clientset, condition NodeCondition, timeout time.Duration, description string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return wait.PollUntilContextTimeout(ctx, 15*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		conditionMet := condition(nodes.Items)
		klog.Infof("%s: Current node count: %d", description, len(nodes.Items))
		return conditionMet, nil
	})
}
