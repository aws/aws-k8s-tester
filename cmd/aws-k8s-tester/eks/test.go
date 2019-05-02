package eks

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/ekstester"
	"github.com/aws/aws-k8s-tester/eks"

	"github.com/spf13/cobra"
)

func newTest() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test <subcommand>",
		Short: "Test commands",
	}
	cmd.AddCommand(
		newTestGetWorkerNodeLogs(),
		newTestDumpClusterLogs(),
	)
	return cmd
}

func newTestGetWorkerNodeLogs() *cobra.Command {
	return &cobra.Command{
		Use:   "get-worker-node-logs",
		Short: "Downloads all cluster logs",
		Run:   testGetWorkerNodeLogs,
	}
}

func testGetWorkerNodeLogs(cmd *cobra.Command, args []string) {
	if path == "" {
		fmt.Fprintln(os.Stderr, "'--path' flag is not specified")
		os.Exit(1)
	}

	cfg, err := eksconfig.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration %q (%v)\n", path, err)
		os.Exit(1)
	}
	var tester ekstester.Tester
	tester, err = eks.NewTester(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create EKS deployer %v\n", err)
		os.Exit(1)
	}

	if err = tester.GetWorkerNodeLogs(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to get worker node logs %v\n", err)
		os.Exit(1)
	}
}

func newTestDumpClusterLogs() *cobra.Command {
	return &cobra.Command{
		Use:   "dump-cluster-logs [artifact-directory]",
		Short: "Dump all cluster logs to the artifact directory",
		Run:   testDumpClusterLogs,
	}
}

func testDumpClusterLogs(cmd *cobra.Command, args []string) {
	if path == "" {
		fmt.Fprintln(os.Stderr, "'--path' flag is not specified")
		os.Exit(1)
	}
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "expected 1 argument, got %v\n", args)
		os.Exit(1)
	}
	dir := args[0]

	cfg, err := eksconfig.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration %q (%v)\n", path, err)
		os.Exit(1)
	}
	var tester ekstester.Tester
	tester, err = eks.NewTester(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create EKS deployer %v\n", err)
		os.Exit(1)
	}

	if err = tester.DumpClusterLogs(dir, ""); err != nil {
		fmt.Fprintf(os.Stderr, "failed to dump cluster logs %v\n", err)
		os.Exit(1)
	}
}
