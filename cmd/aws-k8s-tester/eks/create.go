package eks

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/eks"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
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
		newCreateMNG(),
	)
	return ac
}

func newCreateConfig() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Writes an aws-k8s-tester eks configuration with default values",
		Long:  "Configuration values are overwritten by environment variables.",
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

	fmt.Println("overwriting config file from environment variables")
	err := cfg.UpdateFromEnvs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration from environment variables: %v", err)
		os.Exit(1)
	}
	cfg.Sync()

	if err = cfg.ValidateAndSetDefaults(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to validate configuration %q (%v)\n", path, err)
		os.Exit(1)
	}
	cfg.Sync()

	fmt.Fprintf(os.Stderr, "wrote aws-k8s-tester eks configuration to %q\n", cfg.ConfigPath)
}

func newCreateCluster() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Create an EKS cluster",
		Long:  "Configuration values are overwritten by environment variables.",
		Run:   createClusterFunc,
	}
	return cmd
}

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
	cfg.Sync()

	fmt.Println("overwriting config file from environment variables")
	err = cfg.UpdateFromEnvs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration from environment variables: %v\n", err)
		os.Exit(1)
	}
	cfg.Sync()

	if err = cfg.ValidateAndSetDefaults(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to validate configuration %q (%v)\n", path, err)
		os.Exit(1)
	}
	cfg.Sync()

	tester, err := eks.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create EKS deployer %v\n", err)
		os.Exit(1)
	}

	if err = tester.Up(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create cluster %v\n", err)
		os.Exit(1)
	}
	fmt.Println("'aws-k8s-tester eks create cluster' success")
}

func newCreateMNG() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mng",
		Short: "Create/Add an EKS Managed Node Group",
		Long:  "Configuration values are overwritten by environment variables.",
		Run:   createMNGFunc,
	}
	return cmd
}

func createMNGFunc(cmd *cobra.Command, args []string) {
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
	cfg.Sync()

	fmt.Println("overwriting config file from environment variables")
	err = cfg.UpdateFromEnvs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration from environment variables: %v\n", err)
		os.Exit(1)
	}
	cfg.Sync()

	if err = cfg.ValidateAndSetDefaults(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to validate configuration %q (%v)\n", path, err)
		os.Exit(1)
	}
	cfg.Sync()

	tester, err := eks.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create EKS deployer %v\n", err)
		os.Exit(1)
	}

	if err = tester.CreateMNG(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create mng %v\n", err)
		os.Exit(1)
	}
	fmt.Println("'aws-k8s-tester eks create mng' success")
}
