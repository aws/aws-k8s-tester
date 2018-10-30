package eks

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/eksdeployer"
	"github.com/aws/aws-k8s-tester/internal/eks"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"

	"github.com/spf13/cobra"
)

func newDelete() *cobra.Command {
	ac := &cobra.Command{
		Use:   "delete <subcommand>",
		Short: "Delete commands",
	}
	ac.AddCommand(newDeleteCluster())
	return ac
}

func newDeleteCluster() *cobra.Command {
	return &cobra.Command{
		Use:   "cluster",
		Short: "Delete EKS cluster",
		Run:   deleteClusterFunc,
	}
}

func deleteClusterFunc(cmd *cobra.Command, args []string) {
	if !fileutil.Exist(path) {
		fmt.Fprintf(os.Stderr, "cannot find configuration %q\n", path)
		os.Exit(1)
	}

	cfg, err := eksconfig.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration %q (%v)\n", path, err)
		os.Exit(1)
	}

	var tester eksdeployer.Tester
	tester, err = eks.NewTester(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create EKS deployer %v\n", err)
		os.Exit(1)
	}

	if err = tester.Down(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete cluster %v", err)
		os.Exit(1)
	}

	fmt.Println("'aws-k8s-tester eks delete cluster' success")
}
