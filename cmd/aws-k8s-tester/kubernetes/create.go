package kubernetes

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/internal/kubernetes"
	"github.com/aws/aws-k8s-tester/kubernetesconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/spf13/cobra"
)

func newCreate() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <subcommand>",
		Short: "Create commands",
	}
	cmd.PersistentFlags().BoolVar(&terminateOnExit, "terminate-on-exit", false, "true to terminate kubernetes cluster on test exit")
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
		Short: "Writes an aws-k8s-tester kubernetes configuration with default values",
		Run:   configFunc,
	}
}

func configFunc(cmd *cobra.Command, args []string) {
	if path == "" {
		fmt.Fprintln(os.Stderr, "'--path' flag is not specified")
		os.Exit(1)
	}
	cfg := kubernetesconfig.NewDefault()
	cfg.ConfigPath = path
	cfg.Sync()
	fmt.Fprintf(os.Stderr, "wrote aws-k8s-tester kubernetes configuration to %q\n", cfg.ConfigPath)
}

func newCreateCluster() *cobra.Command {
	return &cobra.Command{
		Use:   "cluster",
		Short: "Create Kubernetes cluster",
		Run:   createClusterFunc,
	}
}

func createClusterFunc(cmd *cobra.Command, args []string) {
	if !fileutil.Exist(path) {
		fmt.Fprintf(os.Stderr, "cannot find configuration %q\n", path)
		os.Exit(1)
	}

	cfg, err := kubernetesconfig.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration %q (%v)\n", path, err)
		os.Exit(1)
	}

	var dp kubernetes.Deployer
	dp, err = kubernetes.NewDeployer(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create kubernetes deployer %v\n", err)
		os.Exit(1)
	}

	errc := make(chan error)
	go func() {
		errc <- dp.Create()
	}()
	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
	terminate := false
	select {
	case err = <-errc:
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create Kubernetes cluster %v\n", err)
			terminate = true
		}
	case sig := <-notifier:
		fmt.Fprintf(os.Stderr, "received %s\n", sig)
		terminate = true
	}

	if terminate {
		if err = dp.Terminate(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to terminate cluster %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "terminated cluster\n")
		}
		os.Exit(1)
	}

	fmt.Println("'aws-k8s-tester kubernetes create cluster' success")
	fmt.Printf("EC2 Master Nodes SSH:\n%s\n\n", cfg.EC2MasterNodes.SSHCommands())
	fmt.Printf("EC2 Worker Nodes SSH:\n%s\n\n", cfg.EC2WorkerNodes.SSHCommands())

	if terminateOnExit {
		notifier = make(chan os.Signal, 1)
		signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-time.After(cfg.WaitBeforeDown):
		case sig := <-notifier:
			fmt.Fprintf(os.Stderr, "received %s\n", sig)
		}
		if err = dp.Terminate(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to terminate cluster %v\n", err)
			os.Exit(1)
		} else {
			fmt.Fprintf(os.Stderr, "terminated cluster\n")
		}
	}
}
