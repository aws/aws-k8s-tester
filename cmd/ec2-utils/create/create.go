// Package create implements "ec2-utils create" commands.
package create

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/aws/aws-k8s-tester/ec2"
	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var (
	path         string
	autoPath     bool
	enablePrompt bool
)

// NewCommand implements "ec2-utils create" command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "create",
		Short:      "EC2 create commands",
		SuggestFor: []string{"creat"},
	}
	cmd.PersistentFlags().StringVarP(&path, "path", "p", "", "ec2 test configuration file path")
	cmd.PersistentFlags().BoolVarP(&autoPath, "auto-path", "a", false, "'true' to auto-generate path for create config/cluster, overwrites existing --path value")
	cmd.PersistentFlags().BoolVarP(&enablePrompt, "enable-prompt", "e", true, "'true' to enable prompt mode")
	cmd.AddCommand(
		newCreateConfig(),
		newCreateInstances(),
	)
	return cmd
}

func newCreateConfig() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Writes an ec2-utils configuration with default values",
		Long:  "Configuration values are overwritten by environment variables.",
		Run:   createConfigFunc,
	}
}

func createConfigFunc(cmd *cobra.Command, args []string) {
	if autoPath {
		path = filepath.Join(os.TempDir(), randutil.String(15)+".yaml")
	}
	if path == "" {
		fmt.Fprintln(os.Stderr, "'--path' flag is not specified")
		os.Exit(1)
	}
	cfg := ec2config.NewDefault()
	cfg.ConfigPath = path
	cfg.Sync()

	fmt.Printf("\n*********************************\n")
	fmt.Printf("overwriting config file from environment variables...\n")
	err := cfg.UpdateFromEnvs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration from environment variables: %v", err)
		os.Exit(1)
	}

	if err = cfg.ValidateAndSetDefaults(); err != nil {
		fmt.Printf("\n*********************************\n")
		fmt.Printf("'ec2-utils create config' fail %err\n", err)
		os.Exit(1)
	}

	txt, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read configuration %q (%v)\n", path, err)
		os.Exit(1)
	}
	println()
	fmt.Println(string(txt))
	println()

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'ec2-utils create config' success\n")
}

func newCreateInstances() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instances",
		Short: "Create EC2 instances",
		Long:  "Configuration values are overwritten by environment variables.",
		Run:   createInstancesFunc,
	}
	return cmd
}

func createInstancesFunc(cmd *cobra.Command, args []string) {
	if autoPath {
		path = filepath.Join(os.TempDir(), randutil.String(15)+".yaml")
	}
	if path == "" {
		fmt.Fprintln(os.Stderr, "'--path' flag is not specified")
		os.Exit(1)
	}

	var cfg *ec2config.Config
	var err error
	if fileutil.Exist(path) {
		cfg, err = ec2config.Load(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to load configuration %q (%v)\n", path, err)
			os.Exit(1)
		}
		if err = cfg.ValidateAndSetDefaults(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to validate configuration %q (%v)\n", path, err)
			os.Exit(1)
		}
	} else {
		fmt.Fprintf(os.Stderr, "cannot find configuration %q; writing...\n", path)
		cfg = ec2config.NewDefault()
		cfg.ConfigPath = path
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("overwriting config file from environment variables...\n")
	err = cfg.UpdateFromEnvs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration from environment variables: %v\n", err)
		os.Exit(1)
	}

	if err = cfg.ValidateAndSetDefaults(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to validate configuration %q (%v)\n", path, err)
		os.Exit(1)
	}

	txt, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read configuration %q (%v)\n", path, err)
		os.Exit(1)
	}
	println()
	fmt.Println(string(txt))
	println()

	if enablePrompt {
		prompt := promptui.Select{
			Label: "Ready to create EC2 resources, should we continue?",
			Items: []string{
				"No, cancel it!",
				"Yes, let's create!",
			},
		}
		idx, answer, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx != 1 {
			fmt.Printf("returning 'create' [index %d, answer %q]\n", idx, answer)
			return
		}
	}

	tester, err := ec2.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create ec2 deployer %v\n", err)
		os.Exit(1)
	}

	if err = tester.Up(); err != nil {
		fmt.Printf("\n*********************************\n")
		fmt.Printf("'ec2-utils create instances' fail %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'ec2-utils create instances' success\n")
}
