package eks

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/spf13/cobra"
)

func newCreateConfig() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Writes an aws-k8s-tester eks configuration with default values",
		Long:  "Configuration values are overwritten by environment variables.",
		Run:   createConfigFunc,
	}
}

func createConfigFunc(cmd *cobra.Command, args []string) {
	if path == "" {
		fmt.Fprintln(os.Stderr, "'--path' flag is not specified")
		os.Exit(1)
	}
	cfg := eksconfig.NewDefault()
	cfg.ConfigPath = path

	fmt.Printf("\n*********************************\n")
	fmt.Printf("overwriting config file from environment variables...\n")
	err := cfg.UpdateFromEnvs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration from environment variables: %v", err)
		os.Exit(1)
	}

	if err = cfg.ValidateAndSetDefaults(); err != nil {
		fmt.Printf("\n*********************************\n")
		fmt.Printf("'aws-k8s-tester eks create config' fail %v\n", err)
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
	fmt.Printf("'aws-k8s-tester eks create config' success\n")
}
