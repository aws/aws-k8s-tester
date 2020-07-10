package eks

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-k8s-tester/eks"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

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
	if !autoPath && path == "" {
		fmt.Fprintln(os.Stderr, "'--path' flag is not specified")
		os.Exit(1)
	}

	var cfg *eksconfig.Config
	var err error
	if !autoPath && fileutil.Exist(path) {
		cfg, err = eksconfig.Load(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to load configuration %q (%v)\n", path, err)
			os.Exit(1)
		}
		if err = cfg.ValidateAndSetDefaults(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to validate configuration %q (%v)\n", path, err)
			os.Exit(1)
		}
	} else {
		cfg = eksconfig.NewDefault()
		if autoPath {
			path = filepath.Join(os.TempDir(), cfg.Name+".yaml")
		}
		cfg.ConfigPath = path
		fmt.Fprintf(os.Stderr, "cannot find configuration; wrote a new one %q\n", path)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("overwriting config file from environment variables with %s\n", version.Version())
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

	if enablePrompt {
		prompt := promptui.Select{
			Label: "Ready to create EKS resources, should we continue?",
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

	time.Sleep(5 * time.Second)

	tester, err := eks.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create EKS deployer %v\n", err)
		os.Exit(1)
	}
	logWriter := tester.LogWriter()

	if err = tester.Up(); err != nil {
		fmt.Fprintf(logWriter, cfg.Colorize("\n\n\n[yellow]*********************************\n"))
		fmt.Fprintf(logWriter, cfg.Colorize(fmt.Sprintf("[default]aws-k8s-tester eks create cluster [light_magenta]FAIL [default](%v)\n", err)))
		os.Exit(1)
	}

	fmt.Fprintf(logWriter, cfg.Colorize("\n\n\n[yellow]*********************************\n"))
	fmt.Fprintf(logWriter, cfg.Colorize("[default]aws-k8s-tester eks create cluster [light_green]SUCCESS\n"))
}
