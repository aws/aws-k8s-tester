// k8s-tester implements k8s-tester on AWS.
package main

import (
	"fmt"
	"os"

	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester"
	"github.com/aws/aws-k8s-tester/utils/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:        "k8s-tester",
	Short:      "Kubernetes tester",
	SuggestFor: []string{"kubernetes-tester"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

var (
	prompt         bool
	logLevel       string
	logOutputs     []string
	minimumNodes   int
	kubectlPath    string
	kubeconfigPath string
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&prompt, "prompt", true, "'true' to enable prompt mode")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", log.DefaultLogLevel, "Logging level")
	rootCmd.PersistentFlags().StringSliceVar(&logOutputs, "log-outputs", []string{"stderr"}, "Additional logger outputs")
	rootCmd.PersistentFlags().IntVar(&minimumNodes, "minimum-nodes", k8s_tester.DefaultMinimumNodes, "minimum number of Kubernetes nodes required")
	rootCmd.PersistentFlags().StringVar(&kubectlPath, "kubectl-path", "", "kubectl path")
	rootCmd.PersistentFlags().StringVar(&kubeconfigPath, "kubeconfig-path", "", "KUBECONFIG path")

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
	clusterName string
)

func newApply() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply tests",
		Run:   createApplyFunc,
	}

	cmd.PersistentFlags().StringVar(&clusterName, "cluster-name", "", "cluster name")

	return cmd
}

func createApplyFunc(cmd *cobra.Command, args []string) {
	lg, _, _, err := log.NewWithStderrWriter(logLevel, logOutputs)
	if err != nil {
		panic(err)
	}
	_ = zap.ReplaceGlobals(lg)

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester apply' success\n")
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
	lg, _, _, err := log.NewWithStderrWriter(logLevel, logOutputs)
	if err != nil {
		panic(err)
	}
	_ = zap.ReplaceGlobals(lg)

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester delete' success\n")
}
