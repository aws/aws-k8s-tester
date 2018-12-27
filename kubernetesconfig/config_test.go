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
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_LOG_DEBUG", "true")
	os.Setenv("AWS_K8S_TESTER_KUBERNETES_UPLOAD_TESTER_LOGS", "false")
	os.Setenv("AWS_K8S_TESTER_EC2_MASTER_NODES_PLUGINS", "update-amazon-linux-2,install-start-docker-amazon-linux-2,install-kubernetes-amazon-linux-2-1.13.1")
	os.Setenv("AWS_K8S_TESTER_ETCD_CLUSTER_SIZE", "5")
	os.Setenv("AWS_K8S_TESTER_ETCD_CLUSTER_VERSION", "v3.2.15")

	defer func() {
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
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_LOG_DEBUG")
		os.Unsetenv("AWS_K8S_TESTER_KUBERNETES_UPLOAD_TESTER_LOGS")
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
	if cfg.UploadTesterLogs {
		t.Fatalf("unexpected UploadTesterLogs, got %v", cfg.UploadTesterLogs)
	}
	exp := []string{"update-amazon-linux-2", "install-start-docker-amazon-linux-2", "install-kubernetes-amazon-linux-2-1.13.1"}
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
