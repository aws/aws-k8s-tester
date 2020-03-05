package ec2

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/ec2"
	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/mitchellh/colorstring"
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

	tester, err := ec2.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create ec2 deployer %v\n", err)
		os.Exit(1)
	}

	if err = tester.Down(); err != nil {
		colorstring.Printf("\n\n[red][bold]'aws-k8s-tester ec2 delete cluster' fail[default] %v\n\n\n", err)
		os.Exit(1)
	}

	colorstring.Printf("\n\n[light_blue][bold]'aws-k8s-tester ec2 delete cluster' success[default]\n\n\n")
}
