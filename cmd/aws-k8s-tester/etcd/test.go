package etcd

import (
	"fmt"
	"os"
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
	cfg.LogDebug = true
	tester, err := etcd.NewTester(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create etcd tester %v\n", err)
		os.Exit(1)
	}
	time.Sleep(cfg.WaitBeforeDown)
	if err = tester.Terminate(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create etcd tester %v\n", err)
		os.Exit(1)
	}
}
