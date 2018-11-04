package etcd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/etcdconfig"
	"github.com/aws/aws-k8s-tester/internal/etcd"

	"github.com/spf13/cobra"
)

func newTest() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run etcd tests",
	}
	cmd.AddCommand(
		newTestE2E(),
	)
	return cmd
}

func newTestE2E() *cobra.Command {
	return &cobra.Command{
		Use:   "e2e",
		Short: "Run etcd e2e tests",
		Run:   testE2EFunc,
	}
}

func testE2EFunc(cmd *cobra.Command, args []string) {
	cfg := etcdconfig.NewDefault()

	tester, err := etcd.NewTester(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create etcd tester %v\n", err)
		os.Exit(1)
	}

	if err = tester.Create(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to deploy etcd tester %v\n", err)
		os.Exit(1)
	}

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-time.After(cfg.WaitBeforeDown):
	case sig := <-notifier:
		fmt.Fprintf(os.Stderr, "received %s\n", sig)
	}

	if err = tester.Terminate(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create etcd tester %v\n", err)
		os.Exit(1)
	}
}
