package eks

import (
	"fmt"
	"os"

	"github.com/aws/awstester/eksconfig"
	"github.com/aws/awstester/eksdeployer"
	"github.com/aws/awstester/internal/eks"
	"github.com/aws/awstester/pkg/fileutil"

	"github.com/spf13/cobra"
)

func newCreate() *cobra.Command {
	ac := &cobra.Command{
		Use:   "create <subcommand>",
		Short: "Create commands",
	}
	ac.AddCommand(
		newCreateConfig(),
		newCreateCluster(),
	)
	return ac
}

func newCreateConfig() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Writes an awstester eks configuration with default values",
		Run:   configFunc,
	}
}

func configFunc(cmd *cobra.Command, args []string) {
	if path == "" {
		fmt.Fprintln(os.Stderr, "'--path' flag is not specified")
		os.Exit(1)
	}
	cfg := eksconfig.NewDefault()
	cfg.ConfigPath = path
	cfg.Sync()
	fmt.Fprintf(os.Stderr, "wrote awstester eks configuration to %q\n", cfg.ConfigPath)
}

func newCreateCluster() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Create an EKS cluster",
		Run:   createClusterFunc,
	}
	cmd.PersistentFlags().BoolVar(&clusterCreateAutoDelete, "down", false, "'true' to automatically delete cluster after creation (useful for testing)")
	return cmd
}

var clusterCreateAutoDelete bool

func createClusterFunc(cmd *cobra.Command, args []string) {
	if !fileutil.Exist(path) {
		fmt.Fprintf(os.Stderr, "cannot find configuration %q\n", path)
		os.Exit(1)
	}

	cfg, err := eksconfig.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration %q (%v)\n", path, err)
		os.Exit(1)
	}
	if err = cfg.ValidateAndSetDefaults(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to validate configuration %q (%v)\n", path, err)
		os.Exit(1)
	}

	var tester eksdeployer.Tester
	tester, err = eks.NewTester(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create EKS deployer %v\n", err)
		os.Exit(1)
	}

	if _, err = cfg.BackupConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to back up original config file %v\n", err)
		os.Exit(1)
	}
	if err = tester.Up(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create cluster %v\n", err)
		os.Exit(1)
	}
	fmt.Println("'awstester eks create cluster' success")

	if clusterCreateAutoDelete {
		if err = tester.Down(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to delete cluster %v\n", err)
			os.Exit(1)
		}
		fmt.Println("'awstester eks create cluster --down' success")
	}
}
