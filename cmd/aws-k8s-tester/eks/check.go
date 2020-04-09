package eks

import (
	"encoding/base64"
	"fmt"

	k8sclient "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	checkRegion                   string
	checkClusterName              string
	checkClusterAPIServerEndpoint string
	checkClusterCA                string
	checkClientQPS                float32
	checkClientBurst              int
	checkKubeConfigPath           string
)

func newCheck() *cobra.Command {
	ac := &cobra.Command{
		Use:   "check <subcommand>",
		Short: "Check EKS resources",
	}
	ac.PersistentFlags().StringVar(&checkRegion, "region", "us-west-2", "EKS region")
	ac.PersistentFlags().StringVar(&checkClusterName, "cluster-name", "", "EKS cluster name")
	ac.PersistentFlags().StringVar(&checkClusterAPIServerEndpoint, "cluster-api-server-endpoint", "", "EKS cluster apiserver endpoint")
	ac.PersistentFlags().StringVar(&checkClusterCA, "cluster-ca", "", "EKS cluster CA encoded in base64")
	ac.PersistentFlags().Float32Var(&checkClientQPS, "client-qps", 5.0, "EKS client qps")
	ac.PersistentFlags().IntVar(&checkClientBurst, "client-burst", 10, "EKS client burst")
	ac.PersistentFlags().StringVar(&checkKubeConfigPath, "kubeconfig", "", "EKS KUBECONFIG")
	ac.AddCommand(
		newCheckCluster(),
	)
	return ac
}

func newCheckCluster() *cobra.Command {
	return &cobra.Command{
		Use:   "cluster",
		Short: "Check EKS cluster status",
		Run:   checkClusterFunc,
	}
}

func checkClusterFunc(cmd *cobra.Command, args []string) {
	kcfg := k8sclient.EKSConfig{
		Region:                   checkRegion,
		ClusterName:              checkClusterName,
		ClusterAPIServerEndpoint: checkClusterAPIServerEndpoint,
		ClientQPS:                checkClientQPS,
		ClientBurst:              checkClientBurst,
		KubeConfigPath:           checkKubeConfigPath,
	}
	if checkClusterCA != "" {
		d, err := base64.StdEncoding.DecodeString(checkClusterCA)
		if err != nil {
			panic(fmt.Errorf("failed to decode cluster CA %v", err))
		}
		kcfg.ClusterCADecoded = string(d)
	}

	lg, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	clientSet, err := k8sclient.NewEKS(lg, kcfg)
	if err != nil {
		panic(fmt.Errorf("failed to create client %v", err))
	}
	ns, err := clientSet.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		panic(fmt.Errorf("failed to list namespaces %v", err))
	}
	for _, v := range ns.Items {
		fmt.Println(v.GetName())
	}

	fmt.Println("'aws-k8s-tester eks check cluster' success")
}
