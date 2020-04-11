// Package apis implements EKS API related commands.
package apis

import (
	"io/ioutil"
	"os"
	"time"

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

	listBatch    int64
	listInterval time.Duration
	dir          string
)

var (
	defaultKubectlPath string
	defaultDir         string
)

func init() {
	cobra.EnablePrefixMatching = true
	defaultKubectlPath, _ = exec.New().LookPath("kubectl")

	var err error
	defaultDir, err = ioutil.TempDir(os.TempDir(), "eks-upgrade-dir")
	if err != nil {
		panic(err)
	}
}

// NewCommand implements "eks-utils apis" command.
func NewCommand() *cobra.Command {
	ac := &cobra.Command{
		Use:   "apis",
		Short: "EKS API commands",
	}
	ac.PersistentFlags().BoolVar(&enablePrompt, "enable-prompt", true, "'true' to enable prompt mode")
	ac.PersistentFlags().Float32Var(&clientQPS, "client-qps", 5.0, "EKS client qps")
	ac.PersistentFlags().IntVar(&clientBurst, "client-burst", 10, "EKS client burst")
	ac.PersistentFlags().StringVar(&kubeConfigPath, "kubeconfig", "", "EKS KUBECONFIG")
	ac.PersistentFlags().StringVar(&kubeConfigContext, "kubeconfig-context", "", "EKS KUBECONFIG context")
	ac.PersistentFlags().StringVar(&kubectlPath, "kubectl", defaultKubectlPath, "kubectl path")
	ac.AddCommand(
		newSupportedCommand(),
		newDeprecateCommand(),
	)
	return ac
}
