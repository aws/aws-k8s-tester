package kubeadmconfig

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestEnv(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("AWS_K8S_TESTER_KUBEADM_CLUSTER_SNAPSHOT_COUNT", "100")
	os.Setenv("AWS_K8S_TESTER_KUBEADM_TEST_TIMEOUT", "20s")
	os.Setenv("AWS_K8S_TESTER_EC2_WAIT_BEFORE_DOWN", "3h")
	os.Setenv("AWS_K8S_TESTER_KUBEADM_WAIT_BEFORE_DOWN", "2h")
	os.Setenv("AWS_K8S_TESTER_EC2_CLUSTER_SIZE", "100")
	os.Setenv("AWS_K8S_TESTER_KUBEADM_CLUSTER_SIZE", "100")
	os.Setenv("AWS_K8S_TESTER_KUBEADM_TAG", "my-test")
	os.Setenv("AWS_K8S_TESTER_KUBEADM_CLUSTER_NAME", "my-cluster")
	os.Setenv("AWS_K8S_TESTER_KUBEADM_DOWN", "false")
	os.Setenv("AWS_K8S_TESTER_KUBEADM_LOG_DEBUG", "true")
	os.Setenv("AWS_K8S_TESTER_KUBEADM_UPLOAD_TESTER_LOGS", "false")
	os.Setenv("AWS_K8S_TESTER_KUBEADM_CLUSTER_VERSION", "v1.10.9")
	os.Setenv("AWS_K8S_TESTER_KUBEADM_CLUSTER_INIT_POD_NETWORK_CIDR", "10.244.0.0/16")

	defer func() {
		os.Unsetenv("AWS_K8S_TESTER_KUBEADM_CLUSTER_SNAPSHOT_COUNT")
		os.Unsetenv("AWS_K8S_TESTER_KUBEADM_TEST_TIMEOUT")
		os.Unsetenv("AWS_K8S_TESTER_EC2_WAIT_BEFORE_DOWN")
		os.Unsetenv("AWS_K8S_TESTER_KUBEADM_WAIT_BEFORE_DOWN")
		os.Unsetenv("AWS_K8S_TESTER_EC2_CLUSTER_SIZE")
		os.Unsetenv("AWS_K8S_TESTER_KUBEADM_CLUSTER_SIZE")
		os.Unsetenv("AWS_K8S_TESTER_KUBEADM_TAG")
		os.Unsetenv("AWS_K8S_TESTER_KUBEADM_CLUSTER_NAME")
		os.Unsetenv("AWS_K8S_TESTER_KUBEADM_DOWN")
		os.Unsetenv("AWS_K8S_TESTER_KUBEADM_LOG_DEBUG")
		os.Unsetenv("AWS_K8S_TESTER_KUBEADM_UPLOAD_TESTER_LOGS")
		os.Unsetenv("AWS_K8S_TESTER_KUBEADM_CLUSTER_VERSION")
		os.Unsetenv("AWS_K8S_TESTER_KUBEADM_CLUSTER_INIT_POD_NETWORK_CIDR")
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
	if cfg.EC2.WaitBeforeDown != 3*time.Hour {
		t.Fatalf("unexpected WaitBeforeDown, got %v", cfg.EC2.WaitBeforeDown)
	}
	if cfg.WaitBeforeDown != 2*time.Hour {
		t.Fatalf("unexpected WaitBeforeDown, got %v", cfg.WaitBeforeDown)
	}
	if cfg.EC2.ClusterSize != 100 {
		t.Fatalf("EC2.ClusterSize expected 100, got %d", cfg.EC2.ClusterSize)
	}
	if cfg.ClusterSize != 100 {
		t.Fatalf("ClusterSize expected 100, got %d", cfg.ClusterSize)
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
	if cfg.Cluster.Version != "1.10.9" {
		t.Fatalf("unexpected Cluster.Version, got %q", cfg.Cluster.Version)
	}
	if cfg.Cluster.InitPodNetworkCIDR != "10.244.0.0/16" {
		t.Fatalf("unexpected Cluster.InitPodNetworkCIDR, got %q", cfg.Cluster.InitPodNetworkCIDR)
	}

	fmt.Println(cfg.Cluster.Flags())
}
