// Pcmdkage nodes implements EKS node related commands.
package nodes

import (
	"github.com/spf13/cobra"
	"k8s.io/utils/exec"
)

var (
	logLevel          string
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
	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "EKS nodes commands",
	}
	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error, dpanic, panic, fatal)")
	cmd.PersistentFlags().BoolVar(&enablePrompt, "enable-prompt", true, "'true' to enable prompt mode")
	cmd.PersistentFlags().Float32Var(&clientQPS, "client-qps", 5.0, "EKS client qps")
	cmd.PersistentFlags().IntVar(&clientBurst, "client-burst", 10, "EKS client burst")
	cmd.PersistentFlags().StringVar(&kubeConfigPath, "kubeconfig", "", "EKS KUBECONFIG")
	cmd.PersistentFlags().StringVar(&kubeConfigContext, "kubeconfig-context", "", "EKS KUBECONFIG context")
	cmd.PersistentFlags().StringVar(&kubectlPath, "kubectl", defaultKubectlPath, "kubectl path")
	cmd.AddCommand(
		newListCommand(),
	)
	return cmd
}
