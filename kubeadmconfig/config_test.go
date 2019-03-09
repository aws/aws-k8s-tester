package kubeadmconfig

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

	os.Setenv("AWS_K8S_TESTER_KUBEADM_AWS_REGION", "us-east-1")
	os.Setenv("AWS_K8S_TESTER_KUBEADM_TEST_TIMEOUT", "20s")
	os.Setenv("AWS_K8S_TESTER_KUBEADM_WAIT_BEFORE_DOWN", "3h")
	os.Setenv("AWS_K8S_TESTER_EC2_MASTER_NODES_CLUSTER_SIZE", "100")
	os.Setenv("AWS_K8S_TESTER_KUBEADM_TAG", "my-test")
	os.Setenv("AWS_K8S_TESTER_KUBEADM_CLUSTER_NAME", "my-cluster")
	os.Setenv("AWS_K8S_TESTER_KUBEADM_DOWN", "false")
	os.Setenv("AWS_K8S_TESTER_KUBEADM_LOG_DEBUG", "true")
	os.Setenv("AWS_K8S_TESTER_KUBEADM_UPLOAD_TESTER_LOGS", "false")
	os.Setenv("AWS_K8S_TESTER_EC2_MASTER_NODES_PLUGINS", "update-amazon-linux-2,install-start-docker-amazon-linux-2,install-kubeadm-amazon-linux-2-1.6.0")

	defer func() {
		os.Unsetenv("AWS_K8S_TESTER_KUBEADM_AWS_REGION")
		os.Unsetenv("AWS_K8S_TESTER_KUBEADM_TEST_TIMEOUT")
		os.Unsetenv("AWS_K8S_TESTER_KUBEADM_WAIT_BEFORE_DOWN")
		os.Unsetenv("AWS_K8S_TESTER_EC2_MASTER_NODES_CLUSTER_SIZE")
		os.Unsetenv("AWS_K8S_TESTER_KUBEADM_TAG")
		os.Unsetenv("AWS_K8S_TESTER_KUBEADM_CLUSTER_NAME")
		os.Unsetenv("AWS_K8S_TESTER_KUBEADM_DOWN")
		os.Unsetenv("AWS_K8S_TESTER_KUBEADM_LOG_DEBUG")
		os.Unsetenv("AWS_K8S_TESTER_KUBEADM_UPLOAD_TESTER_LOGS")
		os.Unsetenv("AWS_K8S_TESTER_EC2_MASTER_NODES_PLUGINS")
	}()

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		t.Fatal(err)
	}

	if cfg.AWSRegion != "us-east-1" {
		t.Fatalf("unexpected AWSRegion, got %v", cfg.TestTimeout)
	}

	if cfg.TestTimeout != 20*time.Second {
		t.Fatalf("unexpected TestTimeout, got %v", cfg.TestTimeout)
	}
	if cfg.WaitBeforeDown != 3*time.Hour {
		t.Fatalf("unexpected WaitBeforeDown, got %v", cfg.WaitBeforeDown)
	}
	if cfg.EC2MasterNodes.ClusterSize != 100 {
		t.Fatalf("EC2.EC2MasterNodes.ClusterSize expected 100, got %d", cfg.EC2MasterNodes.ClusterSize)
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
	exp := []string{"update-amazon-linux-2", "install-start-docker-amazon-linux-2", "install-kubeadm-amazon-linux-2-1.6.0"}
	if !reflect.DeepEqual(cfg.EC2MasterNodes.Plugins, exp) {
		t.Fatalf("expected EC2.Plugins %v, got %v", exp, cfg.EC2MasterNodes.Plugins)
	}

	cfg.KubeadmJoin.Target = "192.168.116.240:6443"
	fmt.Println(cfg.KubeadmJoin.Flags())
	fmt.Println(cfg.KubeadmJoin.Command())

	var d []byte
	d, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(d))
}
