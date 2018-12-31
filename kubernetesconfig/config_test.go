package kubernetesconfig

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"sigs.k8s.io/yaml"
)

func TestEnv(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("AWS_K8S_TESTER_KUBERNETES_LOAD_BALANCER_NAME", "hello")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_DOWN", "false")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBELET_MASTER_NODES_PATH", "/usr/local/bin/kubelet")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBELET_MASTER_NODES_DOWNLOAD_URL", "https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/kubelet")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBELET_MASTER_NODES_VERSION_COMMAND", "/usr/local/bin/kubelet --version || true")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBELET_WORKER_NODES_PATH", "/usr/local/bin/kubelet")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBELET_WORKER_NODES_DOWNLOAD_URL", "https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/kubelet")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBELET_WORKER_NODES_VERSION_COMMAND", "/usr/local/bin/kubelet --version || true")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBE_PROXY_MASTER_NODES_IMAGE", "k8s.gcr.io/kube-proxy:v1.20.0")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBE_PROXY_WORKER_NODES_IMAGE", "k8s.gcr.io/kube-proxy:v1.20.0")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBECTL_PATH", "/usr/local/bin/kubectl")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBECTL_DOWNLOAD_URL", "https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/kubectl")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBECTL_VERSION_COMMAND", "/usr/local/bin/kubectl version --client || true")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBE_APISERVER_IMAGE", "k8s.gcr.io/kube-apiserver:v1.20.0")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBE_CONTROLLER_MANAGER_IMAGE", "k8s.gcr.io/kube-controller-manager:v1.20.0")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBE_SCHEDULER_IMAGE", "k8s.gcr.io/kube-scheduler:v1.20.0")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_CLOUD_CONTROLLER_MANAGER_IMAGE", "k8s.gcr.io/cloud-controller-manager:v1.20.0")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_CLUSTER_SNAPSHOT_COUNT", "100")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_TEST_TIMEOUT", "20s")
	os.Setenv("AWS_K8S_TESTER_EC2_MASTER_NODES_WAIT_BEFORE_DOWN", "3h")
	os.Setenv("AWS_K8S_TESTER_EC2_WORKER_NODES_WAIT_BEFORE_DOWN", "33h")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_WAIT_BEFORE_DOWN", "2h")
	os.Setenv("AWS_K8S_TESTER_EC2_MASTER_NODES_CLUSTER_SIZE", "100")
	os.Setenv("AWS_K8S_TESTER_EC2_WORKER_NODES_CLUSTER_SIZE", "1000")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_TAG", "my-test")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_CLUSTER_NAME", "my-cluster")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_DOWN", "false")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_AWS_REGION", "us-east-1")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_LOG_DEBUG", "true")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_UPLOAD_TESTER_LOGS", "true")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_UPLOAD_KUBECONFIG", "true")
	os.Setenv("AWS_K8S_TESTER_EC2_MASTER_NODES_PLUGINS", "update-amazon-linux-2,install-start-docker-amazon-linux-2,install-kubernetes-amazon-linux-2")
	os.Setenv("AWS_K8S_TESTER_ETCD_CLUSTER_SIZE", "5")
	os.Setenv("AWS_K8S_TESTER_ETCD_CLUSTER_VERSION", "v3.2.15")

	defer func() {
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_LOAD_BALANCER_NAME")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_DOWN")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBELET_MASTER_NODES_PATH")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBELET_MASTER_NODES_DOWNLOAD_URL")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBELET_MASTER_NODES_VERSION_COMMAND")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBELET_WORKER_NODES_PATH")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBELET_WORKER_NODES_DOWNLOAD_URL")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBELET_WORKER_NODES_VERSION_COMMAND")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBE_PROXY_MASTER_NODES_IMAGE")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBE_PROXY_WORKER_NODES_IMAGE")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBECTL_PATH")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBECTL_DOWNLOAD_URL")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBECTL_VERSION_COMMAND")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBE_APISERVER_IMAGE")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBE_CONTROLLER_MANAGER_IMAGE")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBE_SCHEDULER_IMAGE")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_CLOUD_CONTROLLER_MANAGER_IMAGE")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_CLUSTER_SNAPSHOT_COUNT")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_TEST_TIMEOUT")
		os.Unsetenv("AWS_K8S_TESTER_EC2_MASTER_NODES_WAIT_BEFORE_DOWN")
		os.Unsetenv("AWS_K8S_TESTER_EC2_WORKER_NODES_WAIT_BEFORE_DOWN")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_WAIT_BEFORE_DOWN")
		os.Unsetenv("AWS_K8S_TESTER_EC2_MASTER_NODES_CLUSTER_SIZE")
		os.Unsetenv("AWS_K8S_TESTER_EC2_WORKER_NODES_CLUSTER_SIZE")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_CLUSTER_SIZE")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_TAG")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_CLUSTER_NAME")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_DOWN")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_AWS_REGION")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_LOG_DEBUG")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_UPLOAD_TESTER_LOGS")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_UPLOAD_KUBECONFIG")
		os.Unsetenv("AWS_K8S_TESTER_EC2_MASTER_NODES_PLUGINS")
		os.Unsetenv("AWS_K8S_TESTER_ETCD_CLUSTER_SIZE")
		os.Unsetenv("AWS_K8S_TESTER_ETCD_CLUSTER_VERSION")
	}()

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		t.Fatal(err)
	}

	if cfg.LoadBalancerName != "hello" {
		t.Fatalf("unexpected LoadBalancerName, got %q", cfg.LoadBalancerName)
	}

	if cfg.Down {
		t.Fatalf("unexpected Down, got %v", cfg.Down)
	}

	if cfg.KubeletMasterNodes.Path != "/usr/local/bin/kubelet" {
		t.Fatalf("unexpected KubeletMasterNodes.Path, got %q", cfg.KubeletMasterNodes.Path)
	}
	if cfg.KubeletMasterNodes.DownloadURL != "https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/kubelet" {
		t.Fatalf("unexpected KubeletMasterNodes.DownloadURL, got %q", cfg.KubeletMasterNodes.DownloadURL)
	}
	if cfg.KubeletMasterNodes.VersionCommand != "/usr/local/bin/kubelet --version || true" {
		t.Fatalf("unexpected KubeletMasterNodes.VersionCommand, got %q", cfg.KubeletMasterNodes.VersionCommand)
	}
	if cfg.KubeletWorkerNodes.Path != "/usr/local/bin/kubelet" {
		t.Fatalf("unexpected KubeletWorkerNodes.Path, got %q", cfg.KubeletWorkerNodes.Path)
	}
	if cfg.KubeletWorkerNodes.DownloadURL != "https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/kubelet" {
		t.Fatalf("unexpected KubeletWorkerNodes.DownloadURL, got %q", cfg.KubeletWorkerNodes.DownloadURL)
	}
	if cfg.KubeletWorkerNodes.VersionCommand != "/usr/local/bin/kubelet --version || true" {
		t.Fatalf("unexpected KubeletWorkerNodes.VersionCommand, got %q", cfg.KubeletWorkerNodes.VersionCommand)
	}
	if cfg.KubeProxyMasterNodes.Image != "k8s.gcr.io/kube-proxy:v1.20.0" {
		t.Fatalf("unexpected KubeProxyMasterNodes.Image, got %q", cfg.KubeProxyMasterNodes.Image)
	}
	if cfg.KubeProxyWorkerNodes.Image != "k8s.gcr.io/kube-proxy:v1.20.0" {
		t.Fatalf("unexpected KubeProxyWorkerNodes.Image, got %q", cfg.KubeProxyWorkerNodes.Image)
	}
	if cfg.Kubectl.Path != "/usr/local/bin/kubectl" {
		t.Fatalf("unexpected Kubectl.Path, got %q", cfg.Kubectl.Path)
	}
	if cfg.Kubectl.DownloadURL != "https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/kubectl" {
		t.Fatalf("unexpected Kubectl.DownloadURL, got %q", cfg.Kubectl.DownloadURL)
	}
	if cfg.Kubectl.VersionCommand != "/usr/local/bin/kubectl version --client || true" {
		t.Fatalf("unexpected Kubectl.VersionCommand, got %q", cfg.Kubectl.VersionCommand)
	}
	if cfg.KubeAPIServer.Image != "k8s.gcr.io/kube-apiserver:v1.20.0" {
		t.Fatalf("unexpected KubeAPIServer.Image, got %q", cfg.KubeAPIServer.Image)
	}
	if cfg.KubeControllerManager.Image != "k8s.gcr.io/kube-controller-manager:v1.20.0" {
		t.Fatalf("unexpected KubeControllerManager.Image, got %q", cfg.KubeControllerManager.Image)
	}
	if cfg.KubeScheduler.Image != "k8s.gcr.io/kube-scheduler:v1.20.0" {
		t.Fatalf("unexpected KubeScheduler.Image, got %q", cfg.KubeScheduler.Image)
	}
	if cfg.CloudControllerManager.Image != "k8s.gcr.io/cloud-controller-manager:v1.20.0" {
		t.Fatalf("unexpected CloudControllerManager.Image, got %q", cfg.CloudControllerManager.Image)
	}

	if cfg.TestTimeout != 20*time.Second {
		t.Fatalf("unexpected TestTimeout, got %v", cfg.TestTimeout)
	}
	if cfg.EC2MasterNodes.WaitBeforeDown != 3*time.Hour {
		t.Fatalf("unexpected EC2MasterNodes.WaitBeforeDown, got %v", cfg.EC2MasterNodes.WaitBeforeDown)
	}
	if cfg.EC2WorkerNodes.WaitBeforeDown != 33*time.Hour {
		t.Fatalf("unexpected EC2WorkerNodes.WaitBeforeDown, got %v", cfg.EC2WorkerNodes.WaitBeforeDown)
	}
	if cfg.WaitBeforeDown != 2*time.Hour {
		t.Fatalf("unexpected WaitBeforeDown, got %v", cfg.WaitBeforeDown)
	}
	if cfg.EC2MasterNodes.ClusterSize != 100 {
		t.Fatalf("EC2MasterNodes.ClusterSize expected 100, got %d", cfg.EC2MasterNodes.ClusterSize)
	}
	if cfg.EC2WorkerNodes.ClusterSize != 1000 {
		t.Fatalf("EC2WorkerNodes.ClusterSize expected 1000, got %d", cfg.EC2WorkerNodes.ClusterSize)
	}
	if cfg.Tag != "my-test" {
		t.Fatalf("unexpected Tag, got %q", cfg.Tag)
	}
	if cfg.ClusterName != "my-cluster" {
		t.Fatalf("unexpected Tag, got %q", cfg.ClusterName)
	}
	if cfg.Down {
		t.Fatalf("unexpected Down, got %v", cfg.Down)
	}
	if !cfg.LogDebug {
		t.Fatalf("unexpected LogDebug, got %v", cfg.LogDebug)
	}
	if !cfg.UploadTesterLogs {
		t.Fatalf("unexpected UploadTesterLogs, got %v", cfg.UploadTesterLogs)
	}
	if !cfg.UploadKubeConfig {
		t.Fatalf("unexpected UploadKubeConfig, got %v", cfg.UploadKubeConfig)
	}
	exp := []string{"update-amazon-linux-2", "install-start-docker-amazon-linux-2", "install-kubernetes-amazon-linux-2"}
	if !reflect.DeepEqual(cfg.EC2MasterNodes.Plugins, exp) {
		t.Fatalf("expected EC2MasterNodes.Plugins %v, got %v", exp, cfg.EC2MasterNodes.Plugins)
	}
	if cfg.ETCDNodes.ClusterSize != 5 {
		t.Fatalf("expected ETCDNodes.ClusterSize 5, got %v", cfg.ETCDNodes.ClusterSize)
	}
	if cfg.ETCDNodes.Cluster.Version != "3.2.15" {
		t.Fatalf("unexpected ETCDNodes.Cluster.Version, got %q", cfg.ETCDNodes.Cluster.Version)
	}

	var d []byte
	d, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(d))
}
