// k8s-tester-cloudwatch-agent installs Kubernetes cloudwatch-agent tester.
package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/client"
	cloudwatch_agent "github.com/aws/aws-k8s-tester/k8s-tester/cloudwatch-agent"
	"github.com/aws/aws-k8s-tester/utils/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:        "k8s-tester-cloudwatch-agent",
	Short:      "Kubernetes cloudwatch-agent tester",
	SuggestFor: []string{"cloudwatch-agent"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

var (
	prompt         bool
	logLevel       string
	logOutputs     []string
	minimumNodes   int
	namespace      string
	kubectlPath    string
	kubeconfigPath string
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&prompt, "prompt", true, "'true' to enable prompt mode")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", log.DefaultLogLevel, "Logging level")
	rootCmd.PersistentFlags().StringSliceVar(&logOutputs, "log-outputs", []string{"stderr"}, "Additional logger outputs")
	rootCmd.PersistentFlags().IntVar(&minimumNodes, "minimum-nodes", cloudwatch_agent.DefaultMinimumNodes, "minimum number of Kubernetes nodes required for installing this addon")
	rootCmd.PersistentFlags().StringVar(&namespace, "namespace", "test-namespace", "'true' to auto-generate path for create config/cluster, overwrites existing --path value")
	rootCmd.PersistentFlags().StringVar(&kubectlPath, "kubectl-path", "", "kubectl path")
	rootCmd.PersistentFlags().StringVar(&kubeconfigPath, "kubeconfig-path", "", "KUBECONFIG path")

	rootCmd.AddCommand(
		newApply(),
		newDelete(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "k8s-tester-cloudwatch-agent failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

var (
	clusterName string
	region      string
)

func newApply() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply tests",
		Run:   createApplyFunc,
	}

	cmd.PersistentFlags().StringVar(&clusterName, "cluster-name", "", "cluster name")
	cmd.PersistentFlags().StringVar(&region, "region", "", "region")

	return cmd
}

func createApplyFunc(cmd *cobra.Command, args []string) {
	lg, logWriter, _, err := log.NewWithStderrWriter(logLevel, logOutputs)
	if err != nil {
		panic(err)
	}
	_ = zap.ReplaceGlobals(lg)

	clientConfig := &client.Config{
		Logger:         lg,
		KubectlPath:    kubectlPath,
		KubeconfigPath: kubeconfigPath,
	}
	ccfg, err := client.CreateConfig(clientConfig)
	if err != nil {
		lg.Panic("failed to create client config", zap.Error(err))
	}
	cli, err := k8s_client.NewForConfig(ccfg)
	if err != nil {
		lg.Panic("failed to create client", zap.Error(err))
	}

	// TODO: notify stopc
	cfg := &cloudwatch_agent.Config{
		Prompt:       prompt,
		Stopc:        make(chan struct{}),
		Logger:       lg,
		LogWriter:    logWriter,
		MinimumNodes: minimumNodes,
		Namespace:    namespace,
		ClientConfig: clientConfig,
		Client:       cli,
		Region:       region,
		ClusterName:  clusterName,
	}

	ts := cloudwatch_agent.New(cfg)
	if err := ts.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to apply (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-cloudwatch-agent apply' success\n")
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

	ccfg, err := client.CreateConfig(&client.Config{
		Logger:         lg,
		KubectlPath:    kubectlPath,
		KubeconfigPath: kubeconfigPath,
	})
	if err != nil {
		lg.Panic("failed to create client config", zap.Error(err))
	}
	cli, err := k8s_client.NewForConfig(ccfg)
	if err != nil {
		lg.Panic("failed to create client", zap.Error(err))
	}

	cfg := &cloudwatch_agent.Config{
		Prompt:       prompt,
		Logger:       lg,
		LogWriter:    logWriter,
		Namespace:    namespace,
		ClientConfig: clientConfig,
		Client:       cli,
	}

	ts := cloudwatch_agent.New(cfg)
	if err := ts.Delete(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-cloudwatch-agent delete' success\n")
}
