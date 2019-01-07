package e2e

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/internal/kubeadm"
	"github.com/aws/aws-k8s-tester/kubeadmconfig"
)

/*
RUN_AWS_TESTS=1 go test -v -timeout 2h -run TestKubeadm
*/
func TestKubeadm(t *testing.T) {
	if os.Getenv("RUN_AWS_TESTS") != "1" {
		t.Skip()
	}

	cfg := kubeadmconfig.NewDefault()
	tester, err := kubeadm.NewTester(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if err = tester.Create(); err != nil {
		tester.Terminate()
		t.Fatal(err)
	}

	fmt.Printf("EC2 SSH:\n%s\n\n", cfg.EC2.SSHCommands())

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-time.After(3 * time.Hour):
	case sig := <-notifier:
		fmt.Fprintf(os.Stderr, "received %s\n", sig)
	}

	if err = tester.Terminate(); err != nil {
		t.Fatal(err)
	}
}
