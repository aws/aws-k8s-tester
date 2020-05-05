// Package nodes implements EKS node related commands.
package nodes

import (
	"github.com/spf13/cobra"
	"k8s.io/utils/exec"
)

var (
	enablePrompt      bool
	clientQPS         float32
	clientBurst       int
	kubeConfigPath    string
	kubeConfigContext string
	kubectlPath       string
)

var (
	defaultKubectlPath string
)

func init() {
	cobra.EnablePrefixMatching = true
	defaultKubectlPath, _ = exec.New().LookPath("kubectl")
}

// NewCommand implements "eks-utils nodes" command.
func NewCommand() *cobra.Command {
	ac := &cobra.Command{
		Use:   "nodes",
		Short: "EKS nodes commands",
	}
	ac.PersistentFlags().BoolVar(&enablePrompt, "enable-prompt", true, "'true' to enable prompt mode")
	ac.PersistentFlags().Float32Var(&clientQPS, "client-qps", 5.0, "EKS client qps")
	ac.PersistentFlags().IntVar(&clientBurst, "client-burst", 10, "EKS client burst")
	ac.PersistentFlags().StringVar(&kubeConfigPath, "kubeconfig", "", "EKS KUBECONFIG")
	ac.PersistentFlags().StringVar(&kubeConfigContext, "kubeconfig-context", "", "EKS KUBECONFIG context")
	ac.PersistentFlags().StringVar(&kubectlPath, "kubectl", defaultKubectlPath, "kubectl path")
	ac.AddCommand(
		newListCommand(),
	)
	return ac
}
