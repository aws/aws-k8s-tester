package ec2

import (
	"fmt"
	"os"

	"github.com/aws/awstester/internal/ec2"
	ec2config "github.com/aws/awstester/internal/ec2/config"
	"github.com/aws/awstester/pkg/fileutil"

	"github.com/spf13/cobra"
)

func newDelete() *cobra.Command {
	ac := &cobra.Command{
		Use:   "delete <subcommand>",
		Short: "Delete commands",
	}
	ac.AddCommand(newDeleteInstances())
	return ac
}

func newDeleteInstances() *cobra.Command {
	return &cobra.Command{
		Use:   "instances",
		Short: "Delete EC2 instances",
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
		fmt.Fprintf(os.Stderr, "failed to create EC2 instances %v\n", err)
		os.Exit(1)
	}

	if err = dp.Delete(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete instances %v", err)
		os.Exit(1)
	}

	fmt.Println("'awstester ec2 delete instances' success")
}
