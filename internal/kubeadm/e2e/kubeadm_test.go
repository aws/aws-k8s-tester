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
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		t.Fatal(err)
	}

	dp, err := kubeadm.NewDeployer(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if err = dp.Create(); err != nil {
		dp.Terminate()
		t.Fatal(err)
	}

	fmt.Printf("EC2MasterNodes SSH:\n%s\n\n", cfg.EC2MasterNodes.SSHCommands())
	fmt.Printf("EC2WorkerNodes SSH:\n%s\n\n", cfg.EC2WorkerNodes.SSHCommands())

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-time.After(3 * time.Hour):
	case sig := <-notifier:
		fmt.Fprintf(os.Stderr, "received %s\n", sig)
	}

	if err = dp.Terminate(); err != nil {
		t.Fatal(err)
	}
}
