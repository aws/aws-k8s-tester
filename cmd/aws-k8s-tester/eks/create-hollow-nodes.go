package eks

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	hollow_nodes "github.com/aws/aws-k8s-tester/eks/hollow-nodes"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	hollowNodesKubeConfigPath string
	hollowNodesClients        int
	hollowNodesClientQPS      float32
	hollowNodesClientBurst    int
	hollowNodesNodes          int
	hollowNodeNamePrefix      string
	hollowNodeLabelPrefix     string
)

func newCreateHollowNodes() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hollow-nodes",
		Short: "Creates hollow nodes",
		Run:   createHollowNodesFunc,
	}
	cmd.PersistentFlags().StringVar(&hollowNodesKubeConfigPath, "kubeconfig", "", "kubeconfig path (optional, should be run in-cluster, useful for local testing)")
	cmd.PersistentFlags().IntVar(&hollowNodesClients, "clients", eksconfig.DefaultClients, "Number of clients to create")
	cmd.PersistentFlags().Float32Var(&hollowNodesClientQPS, "client-qps", eksconfig.DefaultClientQPS, "kubelet client setup for QPS")
	cmd.PersistentFlags().IntVar(&hollowNodesClientBurst, "client-burst", eksconfig.DefaultClientBurst, "kubelet client setup for burst")
	cmd.PersistentFlags().IntVar(&hollowNodesNodes, "nodes", 10, "Number of hollow nodes to create")
	cmd.PersistentFlags().StringVar(&hollowNodeNamePrefix, "node-name-prefix", "hollow"+randutil.String(5), "Prefix to name hollow nodes")
	cmd.PersistentFlags().StringVar(&hollowNodeLabelPrefix, "node-label-prefix", randutil.String(5), "Prefix to label hollow nodes")
	return cmd
}

func createHollowNodesFunc(cmd *cobra.Command, args []string) {
	// optional
	if hollowNodesKubeConfigPath != "" && !fileutil.Exist(hollowNodesKubeConfigPath) {
		fmt.Fprintf(os.Stderr, "kubeconfig not found %q\n", hollowNodesKubeConfigPath)
		os.Exit(1)
	}
	if len(hollowNodeLabelPrefix) > 40 {
		fmt.Fprintf(os.Stderr, "invalid node label prefix %q (%d characters, label value can not be more than 63 characters)\n", hollowNodeLabelPrefix, len(hollowNodeLabelPrefix))
		os.Exit(1)
	}

	lg, err := logutil.GetDefaultZapLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger %v\n", err)
		os.Exit(1)
	}

	cli, err := k8s_client.NewEKS(&k8s_client.EKSConfig{
		Logger:         lg,
		KubeConfigPath: hollowNodesKubeConfigPath,
		Clients:        hollowNodesClients,
		ClientQPS:      hollowNodesClientQPS,
		ClientBurst:    hollowNodesClientBurst,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create client %v\n", err)
		os.Exit(1)
	}

	// to randomize node names and labels
	// when multiple pods are created via deployment
	// we do not want each pod to assign same node name prefix or labels
	// we want to avoid conflicts and run checks for each pod
	// node checking is done via prefix check, so this should be good
	// enough for make them unique per worker
	sfx := randutil.String(5)

	stopc := make(chan struct{})
	ng, err := hollow_nodes.CreateNodeGroup(hollow_nodes.NodeGroupConfig{
		Logger:         lg,
		Client:         cli,
		Stopc:          stopc,
		Nodes:          hollowNodesNodes,
		NodeNamePrefix: hollowNodeNamePrefix + sfx,
		NodeLabels: map[string]string{
			"NodeType": "hollow-nodes",
			"AMIType":  hollowNodeLabelPrefix + "-ami-type-" + sfx,
			"NGType":   hollowNodeLabelPrefix + "-ng-type-" + sfx,
			"NGName":   hollowNodeLabelPrefix + "-ng-name-" + sfx,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create hollow nodes group %v\n", err)
		os.Exit(1)
	}

	ng.Start()

	lg.Info("waiting before checking hollow nodes")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	select {
	case sig := <-sigs:
		lg.Info("received OS signal", zap.String("signal", sig.String()))
		close(stopc)
		ng.Stop()
		os.Exit(0)
	case <-time.After(time.Minute):
	}

	readies, created, err := ng.CheckNodes()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to check hollow nodes group %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nready node names: %q\n", readies)
	fmt.Printf("created node names: %q\n\n", created)

	println()
	fmt.Println("waiting for os.Signal...")
	println()

	select {
	case sig := <-sigs:
		lg.Info("received OS signal", zap.String("signal", sig.String()))
		close(stopc)
		ng.Stop()
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'aws-k8s-tester eks create hollow-nodes' success\n")
}
