package eks

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/eks"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/mitchellh/colorstring"
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

	colorstring.Printf("\n\n[light_blue][bold]overwriting config file from environment variables[default]\n\n\n")
	err := cfg.UpdateFromEnvs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration from environment variables: %v", err)
		os.Exit(1)
	}

	if err = cfg.ValidateAndSetDefaults(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to validate configuration %q (%v)\n", path, err)
		colorstring.Printf("\n\n[red][bold]'aws-k8s-tester eks create config' fail[default] %v\n\n\n", err)
		os.Exit(1)
	}

	colorstring.Printf("\n\n[light_blue][bold]'aws-k8s-tester eks create config' success[default] %q\n\n\n", cfg.ConfigPath)
}

func newCreateCluster() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Create an eks cluster",
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

	colorstring.Printf("\n\n[light_blue][bold]overwriting config file from environment variables[default]\n\n\n")
	err = cfg.UpdateFromEnvs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration from environment variables: %v\n", err)
		os.Exit(1)
	}

	if err = cfg.ValidateAndSetDefaults(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to validate configuration %q (%v)\n", path, err)
		os.Exit(1)
	}

	tester, err := eks.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create EKS deployer %v\n", err)
		os.Exit(1)
	}

	if err = tester.Up(); err != nil {
		colorstring.Printf("\n\n[red][bold]'aws-k8s-tester eks create cluster' fail[default] %v\n\n\n", err)
		os.Exit(1)
	}
	colorstring.Printf("\n\n[light_blue][bold]'aws-k8s-tester eks create cluster' success[default]\n\n\n")
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

	colorstring.Printf("\n\n[light_blue][bold]overwriting config file from environment variables[default]\n\n\n")
	err = cfg.UpdateFromEnvs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration from environment variables: %v\n", err)
		os.Exit(1)
	}

	if err = cfg.ValidateAndSetDefaults(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to validate configuration %q (%v)\n", path, err)
		os.Exit(1)
	}

	tester, err := eks.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create eks deployer %v\n", err)
		os.Exit(1)
	}

	if err = tester.CreateMNG(); err != nil {
		colorstring.Printf("\n\n[red][bold]'aws-k8s-tester eks create mng' fail[default] %v\n\n\n", err)
		os.Exit(1)
	}
	colorstring.Printf("\n\n[light_blue][bold]'aws-k8s-tester eks create mng' success[default]\n\n\n")
}
