package e2e

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/internal/kubernetes"
	"github.com/aws/aws-k8s-tester/kubernetesconfig"
)

/*
RUN_AWS_TESTS=1 go test -v -timeout 2h -run TestKubernetes
*/
func TestKubernetes(t *testing.T) {
	if os.Getenv("RUN_AWS_TESTS") != "1" {
		t.Skip()
	}

	cfg := kubernetesconfig.NewDefault()
	dp, err := kubernetes.NewDeployer(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if err = dp.Create(); err != nil {
		dp.Terminate()
		t.Fatal(err)
	}

	fmt.Printf("EC2MasterNodes SSH:\n%s\n\n", cfg.EC2MasterNodes.SSHCommands())
	fmt.Printf("EC2WorkerNodes SSH:\n%s\n\n", cfg.EC2WorkerNodes.SSHCommands())

	fmt.Println("READY FOR EXPERIMENT!")

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-time.After(15 * time.Minute):
	case sig := <-notifier:
		fmt.Fprintf(os.Stderr, "received %s\n", sig)
	}

	if err = dp.Terminate(); err != nil {
		t.Fatal(err)
	}
}
