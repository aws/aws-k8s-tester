package apis

import (
	"errors"
	"fmt"
	"time"

	k8sclient "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func newDeprecateCommand() *cobra.Command {
	ac := &cobra.Command{
		Use:   "deprecate",
		Run:   deprecatedFunc,
		Short: "Check deprecated APIs",
		Long: `
eks-utils apis \
  --kubeconfig /tmp/kubeconfig.yaml \
  deprecate

eks-utils apis \
  --kubeconfig ~/.kube/config \
  --kubeconfig-context prow-hkg \
  deprecate
`,
	}
	ac.PersistentFlags().Int64Var(&listBatch, "list-batch", 30, "List batch limit (e.g. 30 items at a time)")
	ac.PersistentFlags().DurationVar(&listInterval, "list-interval", 5*time.Second, "List interval")
	ac.PersistentFlags().StringVar(&dir, "dir", defaultDir, "Directory to save all resource specs for upgrades and rollbacks")
	return ac
}

func deprecatedFunc(cmd *cobra.Command, args []string) {
	if kubectlPath == "" {
		panic(errors.New("'kubectl' not found"))
	}

	fmt.Printf("\n\n************************\nstarting 'eks-utils apis upgrade'\n\n")

	lg := zap.NewExample()
	kcfg := &k8sclient.EKSConfig{
		KubeConfigPath:    kubeConfigPath,
		KubeConfigContext: kubeConfigContext,
		KubectlPath:       kubectlPath,
		EnablePrompt:      enablePrompt,
		Dir:               dir,
		Clients:           1,
		ClientQPS:         clientQPS,
		ClientBurst:       clientBurst,
		ClientTimeout:     30 * time.Second,
		ListBatch:         listBatch,
		ListInterval:      listInterval,
	}
	cli, err := k8sclient.NewEKS(kcfg)
	if err != nil {
		lg.Fatal("failed to create client", zap.Error(err))
	}

	if err = cli.Deprecate(); err != nil {
		lg.Fatal("failed to upgrade", zap.Error(err))
	}

	println()
	fmt.Println("'eks-utils apis upgrade' success")
}
