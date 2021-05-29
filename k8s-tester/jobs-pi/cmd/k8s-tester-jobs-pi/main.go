// k8s-tester-jobs-pi installs Kubernetes Jobs Pi tester.
package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/client"
	jobs_pi "github.com/aws/aws-k8s-tester/k8s-tester/jobs-pi"
	"github.com/aws/aws-k8s-tester/utils/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:        "k8s-tester-jobs-pi",
	Short:      "Kubernetes Jobs Pi tester",
	SuggestFor: []string{"jobs-pi"},
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
	rootCmd.PersistentFlags().IntVar(&minimumNodes, "minimum-nodes", jobs_pi.DefaultMinimumNodes, "minimum number of Kubernetes nodes required for installing this addon")
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
		fmt.Fprintf(os.Stderr, "k8s-tester-jobs-pi failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

var (
	completes int32
	parallels int32
)

func newApply() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply tests",
		Run:   createApplyFunc,
	}
	cmd.PersistentFlags().Int32Var(&completes, "completes", jobs_pi.DefaultCompletes, "desired number of successfully finished pods")
	cmd.PersistentFlags().Int32Var(&parallels, "parallels", jobs_pi.DefaultParallels, "maximum desired number of pods the job should run at any given time")
	return cmd
}

func createApplyFunc(cmd *cobra.Command, args []string) {
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

	cfg := &jobs_pi.Config{
		Prompt:       prompt,
		Logger:       lg,
		LogWriter:    logWriter,
		MinimumNodes: minimumNodes,
		Namespace:    namespace,
		ClientConfig: clientConfig,
		Client:       cli,
		Completes:    completes,
		Parallels:    parallels,
	}

	ts := jobs_pi.New(cfg)
	if err := ts.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to apply (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-jobs-pi apply' success\n")
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

	cfg := &jobs_pi.Config{
		Prompt:       prompt,
		Logger:       lg,
		LogWriter:    logWriter,
		Namespace:    namespace,
		ClientConfig: clientConfig,
		Client:       cli,
	}

	ts := jobs_pi.New(cfg)
	if err := ts.Delete(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-jobs-pi delete' success\n")
}
