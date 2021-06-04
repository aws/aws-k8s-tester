// k8s-tester-csrs installs Kubernetes CSR tester.
package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/client"
	"github.com/aws/aws-k8s-tester/k8s-tester/csrs"
	"github.com/aws/aws-k8s-tester/utils/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:        "k8s-tester-csrs",
	Short:      "Kubernetes csrs tester",
	SuggestFor: []string{"csrs"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

var (
	prompt             bool
	logLevel           string
	logOutputs         []string
	minimumNodes       int
	kubectlDownloadURL string
	kubectlPath        string
	kubeconfigPath     string
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&prompt, "prompt", true, "'true' to enable prompt mode")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", log.DefaultLogLevel, "Logging level")
	rootCmd.PersistentFlags().StringSliceVar(&logOutputs, "log-outputs", []string{"stderr"}, "Additional logger outputs")
	rootCmd.PersistentFlags().IntVar(&minimumNodes, "minimum-nodes", csrs.DefaultMinimumNodes, "minimum number of Kubernetes nodes required for installing this addon")
	rootCmd.PersistentFlags().StringVar(&kubectlDownloadURL, "kubectl-download-url", client.DefaultKubectlDownloadURL(), "kubectl download URL")
	rootCmd.PersistentFlags().StringVar(&kubectlPath, "kubectl-path", client.DefaultKubectlPath(), "kubectl path")
	rootCmd.PersistentFlags().StringVar(&kubeconfigPath, "kubeconfig-path", "", "KUBECONFIG path")

	rootCmd.AddCommand(
		newApply(),
		newDelete(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "k8s-tester-csrs failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

var (
	clients                     int
	objects                     int
	initialRequestConditionType string
)

func newApply() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply tests",
		Run:   createApplyFunc,
	}
	cmd.PersistentFlags().IntVar(&clients, "clients", 5, "number of clients")
	cmd.PersistentFlags().IntVar(&objects, "objects", csrs.DefaultObjects, "number of objects")
	cmd.PersistentFlags().StringVar(&initialRequestConditionType, "initial-condition-type", csrs.DefaultInitialRequestConditionType, "initial CSR condition type")
	return cmd
}

func createApplyFunc(cmd *cobra.Command, args []string) {
	lg, logWriter, _, err := log.NewWithStderrWriter(logLevel, logOutputs)
	if err != nil {
		panic(err)
	}
	_ = zap.ReplaceGlobals(lg)

	cli, err := client.New(&client.Config{
		Logger:             lg,
		KubectlDownloadURL: kubectlDownloadURL,
		KubectlPath:        kubectlPath,
		KubeconfigPath:     kubeconfigPath,
		Clients:            clients,
	})
	if err != nil {
		lg.Panic("failed to create client", zap.Error(err))
	}

	cfg := &csrs.Config{
		Prompt:                      prompt,
		Logger:                      lg,
		LogWriter:                   logWriter,
		MinimumNodes:                minimumNodes,
		Client:                      cli,
		Objects:                     objects,
		InitialRequestConditionType: initialRequestConditionType,
	}

	ts := csrs.New(cfg)
	if err := ts.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to apply (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-csrs apply' success\n")
}

func newDelete() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete resources",
		Run:   createDeleteFunc,
	}
	return cmd
}

func createDeleteFunc(cmd *cobra.Command, args []string) {
	lg, logWriter, _, err := log.NewWithStderrWriter(logLevel, logOutputs)
	if err != nil {
		panic(err)
	}
	_ = zap.ReplaceGlobals(lg)

	cli, err := client.New(&client.Config{
		Logger:             lg,
		KubectlDownloadURL: kubectlDownloadURL,
		KubectlPath:        kubectlPath,
		KubeconfigPath:     kubeconfigPath,
	})
	if err != nil {
		lg.Panic("failed to create client", zap.Error(err))
	}

	cfg := &csrs.Config{
		Prompt:    prompt,
		Logger:    lg,
		LogWriter: logWriter,
		Client:    cli,
	}

	ts := csrs.New(cfg)
	if err := ts.Delete(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-csrs delete' success\n")
}
