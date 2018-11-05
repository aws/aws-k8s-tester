package etcd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/etcdconfig"
	"github.com/aws/aws-k8s-tester/etcdtester"
	"github.com/aws/aws-k8s-tester/internal/etcd"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"

	"github.com/spf13/cobra"
)

func newCreate() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <subcommand>",
		Short: "Create commands",
	}
	cmd.PersistentFlags().BoolVar(&terminateOnExit, "terminate-on-exit", false, "true to terminate etcd cluster on test exit")
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
		Short: "Writes an aws-k8s-tester etcd configuration with default values",
		Run:   configFunc,
	}
}

func configFunc(cmd *cobra.Command, args []string) {
	if path == "" {
		fmt.Fprintln(os.Stderr, "'--path' flag is not specified")
		os.Exit(1)
	}
	cfg := etcdconfig.NewDefault()
	cfg.ConfigPath = path
	cfg.Sync()
	fmt.Fprintf(os.Stderr, "wrote aws-k8s-tester etcd configuration to %q\n", cfg.ConfigPath)
}

func newCreateCluster() *cobra.Command {
	return &cobra.Command{
		Use:   "cluster",
		Short: "Create etcd instances",
		Run:   createClusterFunc,
	}
}

func createClusterFunc(cmd *cobra.Command, args []string) {
	if !fileutil.Exist(path) {
		fmt.Fprintf(os.Stderr, "cannot find configuration %q\n", path)
		os.Exit(1)
	}

	cfg, err := etcdconfig.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration %q (%v)\n", path, err)
		os.Exit(1)
	}

	var tester etcdtester.Tester
	tester, err = etcd.NewTester(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create etcd deployer %v\n", err)
		os.Exit(1)
	}

	if _, err = cfg.BackupConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to back up original config file %v\n", err)
		os.Exit(1)
	}
	if err = tester.Create(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create instances %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Cluster: %+v\n", tester.Cluster())

	fmt.Printf("EC2 SSH:\n%s\n\n", cfg.EC2.SSHCommands())
	fmt.Printf("EC2Bastion SSH:\n%s\n\n", cfg.EC2Bastion.SSHCommands())

	if terminateOnExit {
		notifier := make(chan os.Signal, 1)
		signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-time.After(cfg.WaitBeforeDown):
		case sig := <-notifier:
			fmt.Fprintf(os.Stderr, "received %s\n", sig)
		}
		if err = tester.Terminate(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to terminate cluster %v\n", err)
			os.Exit(1)
		} else {
			fmt.Fprintf(os.Stderr, "terminated cluster\n")
		}
	}
}
