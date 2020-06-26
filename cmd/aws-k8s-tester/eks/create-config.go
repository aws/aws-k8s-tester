package eks

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/version"
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
	if !autoPath && path == "" {
		fmt.Fprintln(os.Stderr, "'--path' flag is not specified")
		os.Exit(1)
	}

	cfg := eksconfig.NewDefault()
	if autoPath {
		path = filepath.Join(os.TempDir(), cfg.Name+".yaml")
	}
	cfg.ConfigPath = path

	fmt.Printf("\n*********************************\n")
	fmt.Printf("overwriting config file from environment variables with %s\n", version.Version())
	err := cfg.UpdateFromEnvs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration from environment variables: %v", err)
		os.Exit(1)
	}

	if err = cfg.ValidateAndSetDefaults(); err != nil {
		fmt.Printf("\n*********************************\n")
		fmt.Printf("'aws-k8s-tester eks create config --path %q' fail %v\n", path, err)
		os.Exit(1)
	}

	txt, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read configuration %q (%v)\n", path, err)
		os.Exit(1)
	}
	fmt.Printf("\n\n%q:\n\n%s\n\n\n", path, string(txt))
	if cfg.IsEnabledAddOnNodeGroups() {
		body, err := json.MarshalIndent(cfg.AddOnNodeGroups, "", "    ")
		if err != nil {
			panic(err)
		}
		fmt.Printf("AddOnNodeGroups:\n\n%s\n\n\n", string(body))
	}
	if cfg.IsEnabledAddOnManagedNodeGroups() {
		body, err := json.MarshalIndent(cfg.AddOnManagedNodeGroups, "", "    ")
		if err != nil {
			panic(err)
		}
		fmt.Printf("AddOnManagedNodeGroups:\n\n%s\n\n\n", string(body))
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'aws-k8s-tester eks create config --path %q' success\n", path)
}
