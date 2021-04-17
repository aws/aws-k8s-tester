// k8s-tester-nlb-hello-world installs Kubernetes NLB hello world tester.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	nlb_hello_world "github.com/aws/aws-k8s-tester/k8s-tester/nlb-hello-world"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:        "k8s-tester-nlb-hello-world",
	Short:      "Kubernetes NLB hello world tester",
	SuggestFor: []string{"nlb-hello-world"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

var (
	enablePrompt   bool
	namespace      string
	kubectlPath    string
	kubeConfigPath string
)

// TODO: make logging configurable
// TODO: make deployment replicas, node selector configurable

func init() {
	rootCmd.PersistentFlags().BoolVar(&enablePrompt, "enable-prompt", true, "'true' to enable prompt mode")
	rootCmd.PersistentFlags().StringVar(&namespace, "namespace", "test-namespace", "'true' to auto-generate path for create config/cluster, overwrites existing --path value")
	rootCmd.PersistentFlags().StringVar(&kubectlPath, "kubectl-path", "", "kubectl path")
	rootCmd.PersistentFlags().StringVar(&kubeConfigPath, "kubeconfig-path", "", "KUBECONFIG path")

	rootCmd.AddCommand(
		newApply(),
		newDelete(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "k8s-tester-nlb-hello-world failed %v\n", err)
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
	if enablePrompt {
		prompt := promptui.Select{
			Label: "Ready to apply resources, should we continue?",
			Items: []string{
				"No, cancel it!",
				"Yes, let's apply!",
			},
		}
		idx, answer, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx != 1 {
			fmt.Printf("returning 'apply' [index %d, answer %q]\n", idx, answer)
			return
		}
	}

	time.Sleep(5 * time.Second)

	lg, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	cfg := nlb_hello_world.Config{
		Logger:    lg,
		LogWriter: os.Stderr,
		Namespace: namespace,
		ClientConfig: &client.Config{
			Logger:         lg,
			KubectlPath:    kubectlPath,
			KubeConfigPath: kubeConfigPath,
		},
		DeploymentReplicas: 1,
	}

	ts := nlb_hello_world.New(cfg)
	if err := ts.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to apply (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-nlb-hello-world apply' success\n")
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
	if enablePrompt {
		prompt := promptui.Select{
			Label: "Ready to delete resources, should we continue?",
			Items: []string{
				"No, cancel it!",
				"Yes, let's delete!",
			},
		}
		idx, answer, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx != 1 {
			fmt.Printf("returning 'delete' [index %d, answer %q]\n", idx, answer)
			return
		}
	}

	time.Sleep(5 * time.Second)

	lg, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	cfg := nlb_hello_world.Config{
		Logger:    lg,
		LogWriter: os.Stderr,
		Namespace: namespace,
		ClientConfig: &client.Config{
			Logger:         lg,
			KubectlPath:    kubectlPath,
			KubeConfigPath: kubeConfigPath,
		},
	}

	ts := nlb_hello_world.New(cfg)
	if err := ts.Delete(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-nlb-hello-world delete' success\n")
}
