package e2e

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/etcdconfig"
	"github.com/aws/aws-k8s-tester/internal/etcd"
)

/*
RUN_AWS_TESTS=1 go test -v -run TestETCD
*/
func TestETCD(t *testing.T) {
	if os.Getenv("RUN_AWS_TESTS") != "1" {
		t.Skip()
	}

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

	fmt.Printf("%+v\n", tester.CheckHealth())

	time.Sleep(cfg.WaitBeforeDown)

	if err = tester.Terminate(); err != nil {
		t.Fatal(err)
	}
}
