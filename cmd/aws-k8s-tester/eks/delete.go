package eks

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/eks"
	"github.com/aws/aws-k8s-tester/eksconfig"
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
		Short: "Delete eks cluster",
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

	tester, err := eks.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create eks deployer %v\n", err)
		os.Exit(1)
	}

	if err = tester.Down(); err != nil {
		fmt.Printf("\n#################################\n")
		fmt.Printf("'aws-k8s-tester eks delete cluster' fail %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n#################################\n")
	fmt.Printf("'aws-k8s-tester eks delete cluster' successs\n")
}
