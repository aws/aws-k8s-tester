package ec2

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/ec2"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/spf13/cobra"
)

func newDelete() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <subcommand>",
		Short: "Delete commands",
	}
	cmd.AddCommand(newDeleteCluster())
	return cmd
}

func newDeleteCluster() *cobra.Command {
	return &cobra.Command{
		Use:   "cluster",
		Short: "Delete EC2 cluster",
		Run:   deleteClusterFunc,
	}
}

func deleteClusterFunc(cmd *cobra.Command, args []string) {
	if !fileutil.Exist(path) {
		fmt.Fprintf(os.Stderr, "cannot find configuration %q\n", path)
		os.Exit(1)
	}

	cfg, err := ec2config.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration %q (%v)\n", path, err)
		os.Exit(1)
	}

	var dp ec2.Deployer
	dp, err = ec2.NewDeployer(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create EC2 cluster %v\n", err)
		os.Exit(1)
	}

	if err = dp.Terminate(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to terminate cluster %v", err)
		os.Exit(1)
	}

	fmt.Println("'aws-k8s-tester ec2 delete cluster' success")
}
