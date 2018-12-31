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

	os.Setenv("AWS_K8S_TESTER_KUBERNETES_DOWN", "false")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBE_PROXY_PATH", "/usr/local/bin/kube-proxy")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBE_PROXY_DOWNLOAD_URL", "https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/kube-proxy")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBE_PROXY_VERSION_COMMAND", "/usr/local/bin/kube-proxy --version || true")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBECTL_PATH", "/usr/local/bin/kubectl")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBECTL_DOWNLOAD_URL", "https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/kubectl")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBECTL_VERSION_COMMAND", "/usr/local/bin/kubectl version --client || true")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBELET_PATH", "/usr/local/bin/kubelet")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBELET_DOWNLOAD_URL", "https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/kubelet")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBELET_VERSION_COMMAND", "/usr/local/bin/kubelet --version || true")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBE_APISERVER_PATH", "/usr/local/bin/kube-apiserver")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBE_APISERVER_DOWNLOAD_URL", "https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/kube-apiserver")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBE_APISERVER_VERSION_COMMAND", "/usr/local/bin/kube-apiserver --version || true")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBE_CONTROLLER_MANAGER_PATH", "/usr/local/bin/kube-controller-manager")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBE_CONTROLLER_MANAGER_DOWNLOAD_URL", "https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/kube-controller-manager")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBE_CONTROLLER_MANAGER_VERSION_COMMAND", "/usr/local/bin/kube-controller-manager --version || true")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBE_SCHEDULER_PATH", "/usr/local/bin/kube-scheduler")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBE_SCHEDULER_DOWNLOAD_URL", "https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/kube-scheduler")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_KUBE_SCHEDULER_VERSION_COMMAND", "/usr/local/bin/kube-scheduler --version || true")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_CLOUD_CONTROLLER_MANAGER_PATH", "/usr/local/bin/cloud-controller-manager")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_CLOUD_CONTROLLER_MANAGER_DOWNLOAD_URL", "https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/cloud-controller-manager")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_CLOUD_CONTROLLER_MANAGER_VERSION_COMMAND", "/usr/local/bin/cloud-controller-manager --version || true")
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
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_DOWN")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBE_PROXY_PATH")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBE_PROXY_DOWNLOAD_URL")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBE_PROXY_VERSION_COMMAND")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBECTL_PATH")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBECTL_DOWNLOAD_URL")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBECTL_VERSION_COMMAND")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBELET_PATH")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBELET_DOWNLOAD_URL")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBELET_VERSION_COMMAND")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBE_APISERVER_PATH")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBE_APISERVER_DOWNLOAD_URL")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBE_APISERVER_VERSION_COMMAND")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBE_CONTROLLER_MANAGER_PATH")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBE_CONTROLLER_MANAGER_DOWNLOAD_URL")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBE_CONTROLLER_MANAGER_VERSION_COMMAND")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBE_SCHEDULER_PATH")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBE_SCHEDULER_DOWNLOAD_URL")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_KUBE_SCHEDULER_VERSION_COMMAND")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_CLOUD_CONTROLLER_MANAGER_PATH")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_CLOUD_CONTROLLER_MANAGER_DOWNLOAD_URL")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_CLOUD_CONTROLLER_MANAGER_VERSION_COMMAND")
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

	if cfg.Down {
		t.Fatalf("unexpected Down, got %v", cfg.Down)
	}

	if cfg.KubeProxy.Path != "/usr/local/bin/kube-proxy" {
		t.Fatalf("unexpected KubeProxy.Path, got %q", cfg.KubeProxy.Path)
	}
	if cfg.KubeProxy.DownloadURL != "https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/kube-proxy" {
		t.Fatalf("unexpected KubeProxy.DownloadURL, got %q", cfg.KubeProxy.DownloadURL)
	}
	if cfg.KubeProxy.VersionCommand != "/usr/local/bin/kube-proxy --version || true" {
		t.Fatalf("unexpected KubeProxy.VersionCommand, got %q", cfg.KubeProxy.DownloadURL)
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
	if cfg.Kubelet.Path != "/usr/local/bin/kubelet" {
		t.Fatalf("unexpected Kubelet.Path, got %q", cfg.Kubelet.Path)
	}
	if cfg.Kubelet.DownloadURL != "https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/kubelet" {
		t.Fatalf("unexpected Kubelet.DownloadURL, got %q", cfg.Kubelet.DownloadURL)
	}
	if cfg.Kubelet.VersionCommand != "/usr/local/bin/kubelet --version || true" {
		t.Fatalf("unexpected Kubelet.VersionCommand, got %q", cfg.Kubelet.VersionCommand)
	}
	if cfg.KubeAPIServer.Path != "/usr/local/bin/kube-apiserver" {
		t.Fatalf("unexpected KubeAPIServer.Path, got %q", cfg.KubeAPIServer.Path)
	}
	if cfg.KubeAPIServer.DownloadURL != "https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/kube-apiserver" {
		t.Fatalf("unexpected KubeAPIServer.DownloadURL, got %q", cfg.KubeAPIServer.DownloadURL)
	}
	if cfg.KubeAPIServer.VersionCommand != "/usr/local/bin/kube-apiserver --version || true" {
		t.Fatalf("unexpected KubeAPIServer.VersionCommand, got %q", cfg.KubeAPIServer.VersionCommand)
	}
	if cfg.KubeControllerManager.Path != "/usr/local/bin/kube-controller-manager" {
		t.Fatalf("unexpected KubeControllerManager.Path, got %q", cfg.KubeControllerManager.Path)
	}
	if cfg.KubeControllerManager.DownloadURL != "https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/kube-controller-manager" {
		t.Fatalf("unexpected KubeControllerManager.DownloadURL, got %q", cfg.KubeControllerManager.DownloadURL)
	}
	if cfg.KubeControllerManager.VersionCommand != "/usr/local/bin/kube-controller-manager --version || true" {
		t.Fatalf("unexpected KubeControllerManager.VersionCommand, got %q", cfg.KubeControllerManager.VersionCommand)
	}
	if cfg.KubeScheduler.Path != "/usr/local/bin/kube-scheduler" {
		t.Fatalf("unexpected KubeScheduler.Path, got %q", cfg.KubeScheduler.Path)
	}
	if cfg.KubeScheduler.DownloadURL != "https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/kube-scheduler" {
		t.Fatalf("unexpected KubeScheduler.DownloadURL, got %q", cfg.KubeScheduler.DownloadURL)
	}
	if cfg.KubeScheduler.VersionCommand != "/usr/local/bin/kube-scheduler --version || true" {
		t.Fatalf("unexpected KubeScheduler.VersionCommand, got %q", cfg.KubeScheduler.VersionCommand)
	}
	if cfg.CloudControllerManager.Path != "/usr/local/bin/cloud-controller-manager" {
		t.Fatalf("unexpected CloudControllerManager.Path, got %q", cfg.CloudControllerManager.Path)
	}
	if cfg.CloudControllerManager.DownloadURL != "https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/cloud-controller-manager" {
		t.Fatalf("unexpected CloudControllerManager.DownloadURL, got %q", cfg.CloudControllerManager.DownloadURL)
	}
	if cfg.CloudControllerManager.VersionCommand != "/usr/local/bin/cloud-controller-manager --version || true" {
		t.Fatalf("unexpected CloudControllerManager.VersionCommand, got %q", cfg.CloudControllerManager.VersionCommand)
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
