package etcdconfig

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestETCD(t *testing.T) {
	ee := NewDefault().Cluster
	e := *ee
	e.ValidateAndSetDefaults()
	e.Name = "s1"
	e.DataDir = "/tmp/etcd/s1"
	e.ListenClientURLs = "http://localhost:2379"
	e.AdvertiseClientURLs = "http://localhost:2379"
	e.ListenPeerURLs = "http://localhost:2380"
	e.AdvertisePeerURLs = "http://localhost:2380"
	e.InitialCluster = "s1=http://localhost:2380"
	e.InitialClusterState = "new"
	e.InitialElectionTickAdvance = true

	flags, err := e.Flags()
	if err != nil {
		t.Fatal(err)
	}

	dst := []string{
		`--name=s1`,
		`--data-dir=/tmp/etcd/s1`,
		`--listen-client-urls=http://localhost:2379`,
		`--advertise-client-urls=http://localhost:2379`,
		`--listen-peer-urls=http://localhost:2380`,
		`--initial-advertise-peer-urls=http://localhost:2380`,
		`--initial-cluster=s1=http://localhost:2380`,
		`--initial-cluster-state=new`,
		`--initial-cluster-token=tkn`,
		`--snapshot-count=10000`,
		`--heartbeat-interval=100`,
		`--election-timeout=1000`,
		`--quota-backend-bytes=2147483648`,
		`--enable-pprof=false`,
		// `--initial-election-tick-advance=true`,
	}
	if !reflect.DeepEqual(flags, dst) {
		t.Fatalf("expected %q, got %q", dst, flags)
	}

	s, serr := e.Service()
	if serr != nil {
		t.Fatal(serr)
	}
	fmt.Println(s)
}

func TestEnv(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("AWS_K8S_TESTER_ETCD_CLUSTER_SNAPSHOT_COUNT", "100")
	os.Setenv("AWS_K8S_TESTER_EC2_ETCD_BASTION_NODES_CLUSTER_SIZE", "2")
	os.Setenv("AWS_K8S_TESTER_ETCD_TEST_TIMEOUT", "20s")
	os.Setenv("AWS_K8S_TESTER_EC2_ETCD_NODES_DESTROY_WAIT_TIME", "3h")
	os.Setenv("AWS_K8S_TESTER_ETCD_DESTROY_WAIT_TIME", "2h")
	os.Setenv("AWS_K8S_TESTER_EC2_ETCD_NODES_CLUSTER_SIZE", "100")
	os.Setenv("AWS_K8S_TESTER_ETCD_CLUSTER_SIZE", "100")
	os.Setenv("AWS_K8S_TESTER_ETCD_TAG", "my-test")
	os.Setenv("AWS_K8S_TESTER_ETCD_CLUSTER_NAME", "my-cluster")
	os.Setenv("AWS_K8S_TESTER_ETCD_DOWN", "false")
	os.Setenv("AWS_K8S_TESTER_ETCD_LOG_LEVEL", "debug")
	os.Setenv("AWS_K8S_TESTER_ETCD_UPLOAD_TESTER_LOGS", "false")
	os.Setenv("AWS_K8S_TESTER_ETCD_CLUSTER_VERSION", "v3.2.15")
	os.Setenv("AWS_K8S_TESTER_ETCD_CLUSTER_TOP_LEVEL", "true")

	defer func() {
		os.Unsetenv("AWS_K8S_TESTER_ETCD_CLUSTER_SNAPSHOT_COUNT")
		os.Unsetenv("AWS_K8S_TESTER_EC2_ETCD_BASTION_NODES_CLUSTER_SIZE")
		os.Unsetenv("AWS_K8S_TESTER_ETCD_TEST_TIMEOUT")
		os.Unsetenv("AWS_K8S_TESTER_EC2_ETCD_NODES_DESTROY_WAIT_TIME")
		os.Unsetenv("AWS_K8S_TESTER_ETCD_DESTROY_WAIT_TIME")
		os.Unsetenv("AWS_K8S_TESTER_EC2_ETCD_NODES_CLUSTER_SIZE")
		os.Unsetenv("AWS_K8S_TESTER_ETCD_CLUSTER_SIZE")
		os.Unsetenv("AWS_K8S_TESTER_ETCD_TAG")
		os.Unsetenv("AWS_K8S_TESTER_ETCD_CLUSTER_NAME")
		os.Unsetenv("AWS_K8S_TESTER_ETCD_DOWN")
		os.Unsetenv("AWS_K8S_TESTER_ETCD_LOG_LEVEL")
		os.Unsetenv("AWS_K8S_TESTER_ETCD_UPLOAD_TESTER_LOGS")
		os.Unsetenv("AWS_K8S_TESTER_ETCD_CLUSTER_VERSION")
		os.Unsetenv("AWS_K8S_TESTER_ETCD_CLUSTER_TOP_LEVEL")
	}()

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}
	fmt.Println(cfg.ValidateAndSetDefaults())

	if cfg.Cluster.SnapshotCount != 100 {
		t.Fatalf("expected Cluster.SnapshotCount 100, got %d", cfg.Cluster.SnapshotCount)
	}
	if cfg.EC2Bastion.ClusterSize != 2 {
		t.Fatalf("expected EC2Bastion.ClusterSize 2, got %d", cfg.EC2Bastion.ClusterSize)
	}
	if cfg.TestTimeout != 20*time.Second {
		t.Fatalf("unexpected TestTimeout, got %v", cfg.TestTimeout)
	}
	if cfg.EC2.DestroyWaitTime != 3*time.Hour {
		t.Fatalf("unexpected DestroyWaitTime, got %v", cfg.EC2.DestroyWaitTime)
	}
	if cfg.DestroyWaitTime != 2*time.Hour {
		t.Fatalf("unexpected DestroyWaitTime, got %v", cfg.DestroyWaitTime)
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
	if cfg.LogLevel != "debug" {
		t.Fatalf("unexpected LogLevel, got %q", cfg.LogLevel)
	}
	if cfg.UploadTesterLogs {
		t.Fatalf("unexpected UploadTesterLogs, got %v", cfg.UploadTesterLogs)
	}
	if cfg.Cluster.Version != "3.2.15" {
		t.Fatalf("unexpected Cluster.Version, got %q", cfg.Cluster.Version)
	}
	if !cfg.Cluster.TopLevel {
		t.Fatalf("unexpected Cluster.TopLevel, got %v", cfg.Cluster.TopLevel)
	}
}
