package e2e

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
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
	tester, err := etcd.NewTester(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if err = tester.Create(); err != nil {
		tester.Terminate()
		t.Fatal(err)
	}

	fmt.Printf("EC2 SSH:\n%s\n\n", cfg.EC2.SSHCommands())
	fmt.Printf("EC2Bastion SSH:\n%s\n\n", cfg.EC2Bastion.SSHCommands())

	fmt.Printf("%+v\n", tester.CheckHealth())

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-time.After(cfg.WaitBeforeDown):
	case sig := <-notifier:
		fmt.Fprintf(os.Stderr, "received %s\n", sig)
	}

	if err = tester.Terminate(); err != nil {
		t.Fatal(err)
	}
}
