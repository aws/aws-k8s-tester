// k8s-tester implements k8s-tester on AWS.
package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester"
	"github.com/aws/aws-k8s-tester/k8s-tester/version"
	"github.com/aws/aws-k8s-tester/utils/file"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:        "k8s-tester",
	Short:      "Kubernetes tester",
	SuggestFor: []string{"kubernetes-tester"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	rootCmd.AddCommand(
		newApply(),
		newDelete(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "k8s-tester failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

var (
	path     string
	autoPath bool
)

func newApply() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply tests",
		Run:   createApplyFunc,
	}
	cmd.PersistentFlags().StringVarP(&path, "path", "p", "", "k8s-tester EKS configuration file path")
	cmd.PersistentFlags().BoolVarP(&autoPath, "auto-path", "a", false, "'true' to auto-generate path for create config/cluster, overwrites existing --path value")
	return cmd
}

func createApplyFunc(cmd *cobra.Command, args []string) {
	if !autoPath && path == "" {
		fmt.Fprintln(os.Stderr, "'--path' flag is not specified")
		os.Exit(1)
	}

	var cfg *k8s_tester.Config
	var err error
	if !autoPath && file.Exist(path) {
		cfg, err = k8s_tester.Load(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to load configuration %q (%v)\n", path, err)
			os.Exit(1)
		}
	} else {
		cfg = k8s_tester.NewDefault()
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

	txt, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read configuration %q (%v)\n", path, err)
		os.Exit(1)
	}
	fmt.Printf("\n\n%q:\n\n%s\n\n\n", path, string(txt))

	ts := k8s_tester.New(cfg)
	if err := ts.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to apply (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester apply' success\n")
}

func newDelete() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete resources",
		Run:   createDeleteFunc,
	}
	cmd.PersistentFlags().StringVarP(&path, "path", "p", "", "k8s-tester EKS configuration file path")
	return cmd
}

func createDeleteFunc(cmd *cobra.Command, args []string) {

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester delete' success\n")
}
