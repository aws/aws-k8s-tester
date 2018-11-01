package etcdconfig

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func TestETCD(t *testing.T) {
	e := defaultETCD
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
		`--enable-pprof=true`,
		`--initial-election-tick-advance=true`,
	}
	if !reflect.DeepEqual(flags, dst) {
		t.Fatalf("expected %q, got %q", dst, flags)
	}
}

func TestValidateAndSetDefaults(t *testing.T) {
	cfg := NewDefault()
	cfg.Cluster.Version = "v3.1.0"

	err := cfg.ValidateAndSetDefaults()
	if err.Error() != "expected >= 3.1.12, got 3.1.0" {
		t.Fatalf("unexpected error %v", err)
	}

	cfg.Cluster.Version = "v3.1.12"
	err = cfg.ValidateAndSetDefaults()
	if err != nil {
		t.Fatal(err)
	}

	if err = cfg.Sync(); err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(cfg.ConfigPath)
}

func TestEnv(t *testing.T) {
	cfg := NewDefault()

	os.Setenv("AWS_K8S_TESTER_EC2_COUNT", "100")
	os.Setenv("AWS_K8S_TESTER_ETCD_TAG", "my-test")
	os.Setenv("AWS_K8S_TESTER_ETCD_CLUSTER_NAME", "my-cluster")
	os.Setenv("AWS_K8S_TESTER_ETCD_DOWN", "false")
	os.Setenv("AWS_K8S_TESTER_ETCD_LOG_DEBUG", "true")
	os.Setenv("AWS_K8S_TESTER_ETCD_UPLOAD_TESTER_LOGS", "false")
	os.Setenv("AWS_K8S_TESTER_ETCD_WAIT_BEFORE_DOWN", "2h")
	os.Setenv("AWS_K8S_TESTER_ETCD__VERSION", "v3.1.12")
	os.Setenv("AWS_K8S_TESTER_ETCD__TOP_LEVEL", "true")

	defer func() {
		os.Unsetenv("AWS_K8S_TESTER_ETCD_TAG")
		os.Unsetenv("AWS_K8S_TESTER_ETCD_CLUSTER_NAME")
		os.Unsetenv("AWS_K8S_TESTER_ETCD_DOWN")
		os.Unsetenv("AWS_K8S_TESTER_ETCD_LOG_DEBUG")
		os.Unsetenv("AWS_K8S_TESTER_ETCD_UPLOAD_TESTER_LOGS")
		os.Unsetenv("AWS_K8S_TESTER_ETCD_WAIT_BEFORE_DOWN")
		os.Unsetenv("AWS_K8S_TESTER_ETCD__VERSION")
		os.Unsetenv("AWS_K8S_TESTER_ETCD__TOP_LEVEL")
	}()

	if err := cfg.UpdateFromEnvs(); err != nil {
		t.Fatal(err)
	}

	if cfg.EC2.Count != 100 {
		t.Fatalf("EC2.Count expected 100, got %d", cfg.EC2.Count)
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
	if cfg.WaitBeforeDown != 2*time.Hour {
		t.Fatalf("unexpected WaitBeforeDown, got %v", cfg.WaitBeforeDown)
	}
	if cfg.Cluster.Version != "v3.1.12" {
		t.Fatalf("unexpected Cluster.Version, got %q", cfg.Cluster.Version)
	}
	if !cfg.Cluster.TopLevel {
		t.Fatalf("unexpected Cluster.TopLevel, got %v", cfg.Cluster.TopLevel)
	}
}
