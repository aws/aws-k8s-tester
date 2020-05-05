package apis

import (
	"errors"
	"fmt"
	"sort"
	"time"

	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func newSupportedCommand() *cobra.Command {
	ac := &cobra.Command{
		Use:        "supported",
		SuggestFor: []string{"support", "getsupportedapis", "getsupportedapi", "apiresources", "apiresource", "api-resource"},
		Run:        supportedFunc,
		Short:      "List all supported APIs",
		Long: `
eks-utils apis \
  --kubeconfig /tmp/kubeconfig.yaml \
  supported

eks-utils apis \
  --kubeconfig ~/.kube/config \
  --kubeconfig-context prow-hkg \
  supported
`,
	}
	ac.PersistentFlags().Float32Var(&clientQPS, "client-qps", 5.0, "EKS client qps")
	ac.PersistentFlags().IntVar(&clientBurst, "client-burst", 10, "EKS client burst")
	ac.PersistentFlags().StringVar(&kubeConfigPath, "kubeconfig", "", "EKS KUBECONFIG")
	ac.PersistentFlags().StringVar(&kubeConfigContext, "kubeconfig-context", "", "EKS KUBECONFIG context")
	ac.PersistentFlags().StringVar(&kubectlPath, "kubectl", defaultKubectlPath, "kubectl path")
	return ac
}

func supportedFunc(cmd *cobra.Command, args []string) {
	if kubectlPath == "" {
		panic(errors.New("'kubectl' not found"))
	}

	fmt.Printf("\n\n************************\nstarting 'eks-utils apis supported'\n\n")
	lg, err := logutil.GetDefaultZapLogger()
	if err != nil {
		panic(err)
	}

	kcfg := &k8s_client.EKSConfig{
		KubeConfigPath:    kubeConfigPath,
		KubeConfigContext: kubeConfigContext,
		KubectlPath:       kubectlPath,
		Clients:           1,
		ClientQPS:         clientQPS,
		ClientBurst:       clientBurst,
		ClientTimeout:     30 * time.Second,
	}
	cli, err := k8s_client.NewEKS(kcfg)
	if err != nil {
		lg.Fatal("failed to create client", zap.Error(err))
	}

	vv, apiVersions, err := cli.FetchSupportedAPIGroupVersions()
	if err != nil {
		panic(fmt.Errorf("failed to check health %v", err))
	}
	ss := make([]string, 0, len(apiVersions))

	fmt.Printf("\n\n************************\nchecking supported API group veresion for %.2f\n\n", vv)
	for k := range apiVersions {
		ss = append(ss, k)
	}
	sort.Strings(ss)
	for _, v := range ss {
		fmt.Println(v)
	}

	println()
	fmt.Println("'eks-utils apis supported' success")
}
