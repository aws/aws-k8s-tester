package eks

import (
	"errors"
	"fmt"

	k8sclient "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

var (
	checkRegion            string
	checkClusterName       string
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
	ac := &cobra.Command{
		Use:   "check",
		Run:   checkFunc,
		Short: "Check EKS cluster status",
		Long: `
aws-k8s-tester eks check \
  --kubeconfig /tmp/kubeconfig.yaml \

e.g.

aws-k8s-tester eks check \
  --kubeconfig /tmp/kubeconfig.yaml

aws-k8s-tester eks check \
  --kubeconfig /tmp/kubeconfig.yaml \
  --server-version 1.16 \
  --encryption-enabled

aws-k8s-tester eks check \
  --region us-west-2 \
  --cluster-name eks-2020040819-surfcrvabhtd \
  --client-qps 20 \
  --client-burst 30 \
  --kubeconfig /tmp/kubeconfig.yaml \
  --server-version 1.16 \
  --encryption-enabled

aws-k8s-tester eks check \
  --kubeconfig ~/.kube/config \
  --kubeconfig-context prow-hkg
`,
	}
	ac.PersistentFlags().StringVar(&checkRegion, "region", "", "EKS region")
	ac.PersistentFlags().StringVar(&checkClusterName, "cluster-name", "", "EKS cluster name")
	ac.PersistentFlags().Float32Var(&checkClientQPS, "client-qps", 5.0, "EKS client qps")
	ac.PersistentFlags().IntVar(&checkClientBurst, "client-burst", 10, "EKS client burst")
	ac.PersistentFlags().StringVar(&checkKubeConfigPath, "kubeconfig", "", "EKS KUBECONFIG")
	ac.PersistentFlags().StringVar(&checkKubeConfigContext, "kubeconfig-context", "", "EKS KUBECONFIG context")
	ac.PersistentFlags().StringVar(&checkKubectlPath, "kubectl", defaultKubectlPath, "kubectl path")
	ac.PersistentFlags().StringVar(&checkServerVersion, "server-version", "", "EKS server version")
	ac.PersistentFlags().BoolVar(&checkEncryptionEnabled, "encryption-enabled", false, "'true' to check EKS encryption")
	return ac
}

func checkFunc(cmd *cobra.Command, args []string) {
	if checkKubectlPath == "" {
		panic(errors.New("'kubectl' not found"))
	}
	kcfg := &k8sclient.EKSConfig{
		Logger:            zap.NewExample(),
		Region:            checkRegion,
		ClusterName:       checkClusterName,
		ClientQPS:         checkClientQPS,
		ClientBurst:       checkClientBurst,
		KubeConfigPath:    checkKubeConfigPath,
		KubeConfigContext: checkKubeConfigContext,
		KubectlPath:       checkKubectlPath,
		ServerVersion:     checkServerVersion,
		EncryptionEnabled: checkEncryptionEnabled,
	}
	cli, err := k8sclient.NewEKS(kcfg)
	if err != nil {
		panic(fmt.Errorf("failed to create client %v", err))
	}

	if err = cli.CheckHealth(); err != nil {
		panic(fmt.Errorf("failed to check health %v", err))
	}

	println()
	fmt.Println("'aws-k8s-tester eks check cluster' success")
}
