// k8s-tester-splunk installs Splunk agent using helm, and tests that it's able to function correctly.
package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/client"
	splunk "github.com/aws/aws-k8s-tester/k8s-tester/splunk"
	"github.com/aws/aws-k8s-tester/utils/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:        "k8s-tester-splunk",
	Short:      "Kubernetes Splunk tester",
	SuggestFor: []string{"splunk"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

var (
	prompt             bool
	logLevel           string
	logOutputs         []string
	minimumNodes       int
	namespace          string
	kubectlDownloadURL string
	kubectlPath        string
	kubeconfigPath     string
	accessKey          string
	splunkRealm        string
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&prompt, "prompt", true, "'true' to enable prompt mode")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", log.DefaultLogLevel, "Logging level")
	rootCmd.PersistentFlags().StringSliceVar(&logOutputs, "log-outputs", []string{"stderr"}, "Additional logger outputs")
	rootCmd.PersistentFlags().IntVar(&minimumNodes, "minimum-nodes", splunk.DefaultMinimumNodes, "minimum number of Kubernetes nodes required for installing this addon")
	rootCmd.PersistentFlags().StringVar(&namespace, "namespace", "test-namespace", "'true' to auto-generate path for create config/cluster, overwrites existing --path value")
	rootCmd.PersistentFlags().StringVar(&kubectlDownloadURL, "kubectl-download-url", client.DefaultKubectlDownloadURL(), "kubectl download URL")
	rootCmd.PersistentFlags().StringVar(&kubectlPath, "kubectl-path", client.DefaultKubectlPath(), "kubectl path")
	rootCmd.PersistentFlags().StringVar(&kubeconfigPath, "kubeconfig-path", "", "KUBECONFIG path")
	rootCmd.PersistentFlags().StringVar(&accessKey, "access-key", "", "access Key for Splunk helm chart")
	rootCmd.PersistentFlags().StringVar(&splunkRealm, "splunk-realm", "", "Splunk realm is the region for your specfic splunk collector to be pointed at")

	rootCmd.AddCommand(
		newApply(),
		newDelete(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "k8s-tester-splunk failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

var helmChartRepoURL string

func newApply() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply tests",
		Run:   createApplyFunc,
	}
	cmd.PersistentFlags().StringVar(&helmChartRepoURL, "helm-chart-repo-url", splunk.DefaultHelmChartRepoURL, "helm chart repo URL")
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
	})
	if err != nil {
		lg.Panic("failed to create client", zap.Error(err))
	}

	cfg := &splunk.Config{
		Prompt:           prompt,
		Logger:           lg,
		LogWriter:        logWriter,
		MinimumNodes:     minimumNodes,
		Namespace:        namespace,
		HelmChartRepoURL: helmChartRepoURL,
		Client:           cli,
		AccessKey:        accessKey,
		SplunkRealm:      splunkRealm,
	}

	ts := splunk.New(cfg)
	if err := ts.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to apply (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-splunk apply' success\n")
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

	cfg := &splunk.Config{
		Prompt:      prompt,
		Logger:      lg,
		LogWriter:   logWriter,
		Namespace:   namespace,
		Client:      cli,
		AccessKey:   accessKey,
		SplunkRealm: splunkRealm,
	}

	ts := splunk.New(cfg)
	if err := ts.Delete(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-splunk delete' success\n")
}
