package eks

import (
	"errors"
	"fmt"

	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/spf13/cobra"
	"k8s.io/utils/exec"
)

var (
	checkClientQPS         float32
	checkClientBurst       int
	checkKubeConfigPath    string
	checkKubeConfigContext string
	checkKubectlPath       string
	checkServerVersion     string
	checkEncryptionEnabled bool
)

var defaultKubectlPath string

func init() {
	defaultKubectlPath, _ = exec.New().LookPath("kubectl")
}

func newCheck() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Run:   checkFunc,
		Short: "Check EKS cluster status",
		Long: `
aws-k8s-tester eks check \
  --kubeconfig /tmp/kubeconfig.yaml \

aws-k8s-tester eks check \
  --kubeconfig /tmp/kubeconfig.yaml \
  --server-version 1.16 \
  --encryption-enabled

aws-k8s-tester eks check \
  --kubeconfig ~/.kube/config \
  --kubeconfig-context prow-hkg
`,
	}
	cmd.PersistentFlags().Float32Var(&checkClientQPS, "client-qps", 5.0, "EKS client qps")
	cmd.PersistentFlags().IntVar(&checkClientBurst, "client-burst", 10, "EKS client burst")
	cmd.PersistentFlags().StringVar(&checkKubeConfigPath, "kubeconfig", "", "EKS KUBECONFIG")
	cmd.PersistentFlags().StringVar(&checkKubeConfigContext, "kubeconfig-context", "", "EKS KUBECONFIG context")
	cmd.PersistentFlags().StringVar(&checkKubectlPath, "kubectl", defaultKubectlPath, "kubectl path")
	cmd.PersistentFlags().StringVar(&checkServerVersion, "server-version", "", "EKS server version")
	cmd.PersistentFlags().BoolVar(&checkEncryptionEnabled, "encryption-enabled", false, "'true' to check EKS encryption")
	return cmd
}

func checkFunc(cmd *cobra.Command, args []string) {
	if checkKubectlPath == "" {
		panic(errors.New("'kubectl' not found"))
	}
	kcfg := &k8s_client.EKSConfig{
		KubeConfigPath:    checkKubeConfigPath,
		KubeConfigContext: checkKubeConfigContext,
		KubectlPath:       checkKubectlPath,
		ServerVersion:     checkServerVersion,
		EncryptionEnabled: checkEncryptionEnabled,
		Clients:           1,
		ClientQPS:         checkClientQPS,
		ClientBurst:       checkClientBurst,
	}
	cli, err := k8s_client.NewEKS(kcfg)
	if err != nil {
		panic(fmt.Errorf("failed to create client %v", err))
	}

	fmt.Printf("\n\n************************\nfetching version\n\n")
	ver, err := cli.FetchServerVersion()
	if err != nil {
		panic(fmt.Errorf("failed to check version %v", err))
	}
	fmt.Printf("\n\nVersion:\n%+v\n\n", ver)

	fmt.Printf("\n\n************************\nchecking health...\n\n")
	if err = cli.CheckHealth(); err != nil {
		panic(fmt.Errorf("failed to check health %v", err))
	}

	println()
	fmt.Println("'aws-k8s-tester eks check cluster' success")
}
