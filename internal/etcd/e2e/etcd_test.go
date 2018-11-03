package e2e

import (
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/etcdconfig"
	"github.com/aws/aws-k8s-tester/internal/etcd"
)

func TestETCD(t *testing.T) {
	cfg := etcdconfig.NewDefault()
	cfg.LogDebug = true
	cfg.ClusterSize = 3

	tester, err := etcd.NewTester(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if err = tester.Deploy(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(cfg.WaitBeforeDown)

	if err = tester.Terminate(); err != nil {
		t.Fatal(err)
	}
}
