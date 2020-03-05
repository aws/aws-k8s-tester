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

func newCreate() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <subcommand>",
		Short: "Create commands",
	}
	cmd.AddCommand(
		newCreateConfig(),
		newCreateCluster(),
	)
	return cmd
}

func newCreateConfig() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Writes an aws-k8s-tester ec2 configuration with default values",
		Long:  "Configuration values are overwritten by environment variables.",
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

	colorstring.Printf("\n\n[light_blue][bold]overwriting config file from environment variables[default]\n\n\n")
	err := cfg.UpdateFromEnvs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration from environment variables: %v", err)
		os.Exit(1)
	}

	if err = cfg.ValidateAndSetDefaults(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to validate configuration %q (%v)\n", path, err)
		colorstring.Printf("\n\n[red][bold]'aws-k8s-tester ec2 create config' fail[default] %v\n\n\n", err)
		os.Exit(1)
	}

	colorstring.Printf("\n\n[light_blue][bold]'aws-k8s-tester ec2 create config' success[default] %q\n\n\n", cfg.ConfigPath)
}

func newCreateCluster() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Create an ec2 cluster",
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

	cfg, err := ec2config.Load(path)
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

	tester, err := ec2.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create ec2 deployer %v\n", err)
		os.Exit(1)
	}

	if err = tester.Up(); err != nil {
		colorstring.Printf("\n\n[red][bold]'aws-k8s-tester ec2 create cluster' fail[default] %v\n\n\n", err)
		os.Exit(1)
	}
	colorstring.Printf("\n\n[light_blue][bold]'aws-k8s-tester ec2 create cluster' success[default]\n\n\n")
}
