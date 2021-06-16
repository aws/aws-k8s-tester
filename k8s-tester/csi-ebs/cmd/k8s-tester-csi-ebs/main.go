// k8s-tester-csi-ebs installs Kubernetes csi-ebs tester.
package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/client"
	csi_ebs "github.com/aws/aws-k8s-tester/k8s-tester/csi-ebs"
	"github.com/aws/aws-k8s-tester/utils/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:        "k8s-tester-csi-ebs",
	Short:      "Kubernetes CSI EBS tester",
	SuggestFor: []string{"csi-ebs"},
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
	helmChartRepoURL   string
	kubectlDownloadURL string
	kubectlPath        string
	kubeconfigPath     string
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&prompt, "prompt", true, "'true' to enable prompt mode")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", log.DefaultLogLevel, "Logging level")
	rootCmd.PersistentFlags().StringSliceVar(&logOutputs, "log-outputs", []string{"stderr"}, "Additional logger outputs")
	rootCmd.PersistentFlags().IntVar(&minimumNodes, "minimum-nodes", csi_ebs.DefaultMinimumNodes, "minimum number of Kubernetes nodes required for installing this addon")
	rootCmd.PersistentFlags().StringVar(&namespace, "namespace", "test-namespace", "'true' to auto-generate path for create config/cluster, overwrites existing --path value")
	rootCmd.PersistentFlags().StringVar(&helmChartRepoURL, "helm-chart-repo-url", csi_ebs.DefaultHelmChartRepoURL, "helm chart repo URL")
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
		fmt.Fprintf(os.Stderr, "k8s-tester-csi-ebs failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func newApply() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply tests",
		Run:   createApplyFunc,
	}
	return cmd
}

func createApplyFunc(cmd *cobra.Command, args []string) {
	lg, logWriter, _, err := log.NewWithStderrWriter(logLevel, logOutputs)
	if err != nil {
		panic(err)
	}
	_ = zap.ReplaceGlobals(lg)

	cli, err := client.New(&client.Config{
		Logger:         lg,
		KubectlPath:    kubectlPath,
		KubeconfigPath: kubeconfigPath,
	})
	if err != nil {
		lg.Panic("failed to create client", zap.Error(err))
	}

	cfg := &csi_ebs.Config{
		Prompt:           prompt,
		Logger:           lg,
		LogWriter:        logWriter,
		MinimumNodes:     minimumNodes,
		HelmChartRepoURL: helmChartRepoURL,
		Namespace:        namespace,
		Client:           cli,
	}

	ts := csi_ebs.New(cfg)
	if err := ts.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to apply (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-csi-ebs apply' success\n")
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
		Logger:         lg,
		KubectlPath:    kubectlPath,
		KubeconfigPath: kubeconfigPath,
	})
	if err != nil {
		lg.Panic("failed to create client", zap.Error(err))
	}

	cfg := &csi_ebs.Config{
		Prompt:    prompt,
		Logger:    lg,
		LogWriter: logWriter,
		Namespace: namespace,
		Client:    cli,
	}

	ts := csi_ebs.New(cfg)
	if err := ts.Delete(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-csi-ebs delete' success\n")
}
