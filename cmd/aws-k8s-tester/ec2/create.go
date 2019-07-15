package ec2

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/internal/ec2"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"

	"github.com/spf13/cobra"
)

func newCreate() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <subcommand>",
		Short: "Create commands",
	}
	cmd.PersistentFlags().BoolVar(&terminateOnExit, "terminate-on-exit", false, "true to terminate EC2 instance on test exit")
	cmd.AddCommand(
		newCreateConfig(),
		newCreateCluster(),
	)
	return cmd
}

var terminateOnExit bool

func newCreateConfig() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Writes an aws-k8s-tester eks configuration with default values",
		Run:   configFunc,
	}
}

func configFunc(cmd *cobra.Command, args []string) {
	if path == "" {
		fmt.Fprintln(os.Stderr, "'--path' flag is not specified")
		os.Exit(1)
	}
	cfg := ec2config.NewDefault()
	cfg.ConfigPath = path
	cfg.Sync()
	fmt.Fprintf(os.Stderr, "wrote aws-k8s-tester eks configuration to %q\n", cfg.ConfigPath)
}

func newCreateCluster() *cobra.Command {
	return &cobra.Command{
		Use:   "cluster",
		Short: "Create EC2 instances",
		Run:   createClusterFunc,
	}
}

func createClusterFunc(cmd *cobra.Command, args []string) {
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
		fmt.Fprintf(os.Stderr, "failed to create EKS deployer %v\n", err)
		os.Exit(1)
	}

	if err = dp.Create(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create cluster %v\n", err)
		os.Exit(1)
	}

	fmt.Println(cfg.SSHCommands())

	if terminateOnExit {
		notifier := make(chan os.Signal, 1)
		signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-time.After(cfg.WaitBeforeDown):
		case sig := <-notifier:
			fmt.Fprintf(os.Stderr, "received %s\n", sig)
		}
		if err = dp.Terminate(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to delete cluster %v\n", err)
			os.Exit(1)
		} else {
			fmt.Fprintf(os.Stderr, "deleted cluster\n")
		}
	}
}
