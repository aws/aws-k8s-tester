// k8s-tester-securecn installs securecn, and tests that it's able to function correctly.
package main

import (
	"fmt"
	"os"

	"go.uber.org/zap"

	"github.com/spf13/cobra"

	"github.com/aws/aws-k8s-tester/client"
	securecn "github.com/aws/aws-k8s-tester/k8s-tester/secureCN"
	"github.com/aws/aws-k8s-tester/utils/log"
)

var rootCmd = &cobra.Command{
	Use:        "k8s-tester-secureCN",
	Short:      "Kubernetes SecureCN tester",
	SuggestFor: []string{"secureCN"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

var (
	logLevel           string
	logOutputs         []string
	kubectlDownloadURL string
	kubectlPath        string
	kubeconfigPath     string
	AccessKey          string
	SecretKey          string
	URL                string
	ClusterName        string
)

func init() {
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", log.DefaultLogLevel, "Logging level")
	rootCmd.PersistentFlags().StringVar(&kubectlDownloadURL, "kubectl-download-url", client.DefaultKubectlDownloadURL(), "kubectl download URL")
	rootCmd.PersistentFlags().StringVar(&kubectlPath, "kubectl-path", client.DefaultKubectlPath(), "kubectl path")
	rootCmd.PersistentFlags().StringVar(&kubeconfigPath, "kubeconfig-path", "", "KUBECONFIG path")
	rootCmd.PersistentFlags().StringVar(&AccessKey, "access-key", "", "access key")
	rootCmd.PersistentFlags().StringVar(&SecretKey, "secret-key", "", "secret key")
	rootCmd.PersistentFlags().StringVar(&URL, "URL", "https://securecn.cisco.com", "URL")
	rootCmd.PersistentFlags().StringVar(&ClusterName, "ClusterName", "", "ClusterName")

	rootCmd.AddCommand(
		newApply(),
		newDelete(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "k8s-tester-secureCN failed %v\n", err)
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
		Logger:             lg,
		KubectlDownloadURL: kubectlDownloadURL,
		KubectlPath:        kubectlPath,
		KubeconfigPath:     kubeconfigPath,
	})
	if err != nil {
		lg.Panic("failed to create client", zap.Error(err))
	}

	cfg := &securecn.Config{
		Logger:      lg,
		LogWriter:   logWriter,
		Client:      cli,
		AccessKey:   AccessKey,
		SecretKey:   SecretKey,
		URL:         URL,
		ClusterName: ClusterName,
	}

	tester := securecn.New(cfg)
	if err := tester.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to apply (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-securecn apply' success\n")
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

	cfg := &securecn.Config{
		Logger:    lg,
		LogWriter: logWriter,
		Client:    cli,
	}

	tester := securecn.New(cfg)
	if err := tester.Delete(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-securecn delete' success\n")
}
