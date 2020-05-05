package apis

import (
	"errors"
	"fmt"
	"time"

	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	listBatchLimit    int64
	listBatchInterval time.Duration
	dir               string
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
	ac.PersistentFlags().Int64Var(&listBatchLimit, "batch-limit", 30, "List batch limit (e.g. 30 items at a time)")
	ac.PersistentFlags().DurationVar(&listBatchInterval, "batch-interval", 5*time.Second, "List interval")
	ac.PersistentFlags().StringVar(&dir, "dir", defaultDir, "Directory to save all resource specs for upgrades and rollbacks")
	return ac
}

func deprecatedFunc(cmd *cobra.Command, args []string) {
	if kubectlPath == "" {
		panic(errors.New("'kubectl' not found"))
	}

	fmt.Printf("\n\n************************\nstarting 'eks-utils apis deprecate'\n\n")
	lg, err := logutil.GetDefaultZapLogger()
	if err != nil {
		panic(err)
	}
	kcfg := &k8s_client.EKSConfig{
		KubeConfigPath:    kubeConfigPath,
		KubeConfigContext: kubeConfigContext,
		KubectlPath:       kubectlPath,
		EnablePrompt:      enablePrompt,
		Dir:               dir,
		Clients:           1,
		ClientQPS:         clientQPS,
		ClientBurst:       clientBurst,
		ClientTimeout:     30 * time.Second,
	}
	cli, err := k8s_client.NewEKS(kcfg)
	if err != nil {
		lg.Fatal("failed to create client", zap.Error(err))
	}

	if err = cli.Deprecate(listBatchLimit, listBatchInterval); err != nil {
		lg.Fatal("failed to deprecate", zap.Error(err))
	}

	println()
	fmt.Println("'eks-utils apis deprecate' success")
}
