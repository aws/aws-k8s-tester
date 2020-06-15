package eks

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-k8s-tester/eks"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
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
	if autoPath {
		path = filepath.Join(os.TempDir(), randutil.String(15)+".yaml")
	}
	if path == "" {
		fmt.Fprintln(os.Stderr, "'--path' flag is not specified")
		os.Exit(1)
	}

	var cfg *eksconfig.Config
	var err error
	if fileutil.Exist(path) {
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
		fmt.Fprintf(os.Stderr, "cannot find configuration %q; writing...\n", path)
		cfg = eksconfig.NewDefault()
		cfg.ConfigPath = path
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
	println()
	fmt.Println(string(txt))
	println()

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

	if err = tester.Up(); err != nil {
		fmt.Printf("\n*********************************\n")
		fmt.Printf("'aws-k8s-tester eks create cluster' fail %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'aws-k8s-tester eks create cluster' success\n")
}
