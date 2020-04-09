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
		Long: `
aws-k8s-tester eks check \
  --kubeconfig /tmp/kubeconfig.yaml \
  <subcommand>

aws-k8s-tester eks check \
  --cluster-api-server-endpoint https://URL \
  --cluster-ca "LS0...=" \
  --cluster-name eks-2020040819-surfcrvabhtd \
  <subcommand>

`,
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
	clientSet, err := k8sclient.NewEKS(zap.NewExample(), kcfg)
	if err != nil {
		panic(fmt.Errorf("failed to create client %v", err))
	}

	println()
	ns, err := clientSet.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		panic(fmt.Errorf("failed to list namespaces %v", err))
	}
	if len(ns.Items) > 0 {
		for _, v := range ns.Items {
			fmt.Println("namespace:", v.GetName())
		}
	} else {
		fmt.Println("no namespace")
	}
	println()
	nodes, err := clientSet.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		panic(fmt.Errorf("failed to list nodes %v", err))
	}
	if len(nodes.Items) > 0 {
		for _, v := range nodes.Items {
			fmt.Println("node:", v.GetName())
		}
	} else {
		fmt.Println("no node")
	}

	println()
	evs, err := clientSet.CoreV1().Events("default").List(metav1.ListOptions{})
	if err != nil {
		panic(fmt.Errorf("failed to list events %v", err))
	}
	if len(evs.Items) > 0 {
		for _, v := range evs.Items {
			fmt.Println("event:", v.GetName())
		}
	} else {
		fmt.Println("no event")
	}

	println()
	fmt.Println("'aws-k8s-tester eks check cluster' success")
}
