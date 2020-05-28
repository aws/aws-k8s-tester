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
	clusterLoaderKubeConfigPath string
	clusterLoaderClients        int
	clusterLoaderClientQPS      float32
	clusterLoaderClientBurst    int
	clusterLoaderNodes          int
	clusterLoaderNamePrefix     string
	clusterLoaderLabelPrefix    string
	clusterLoaderRemote         bool
)

func newCreateClusterLoader() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster-loader",
		Short: "Creates cluster loader",
		Run:   createClusterLoaderFunc,
	}
	cmd.PersistentFlags().StringVar(&clusterLoaderKubeConfigPath, "kubeconfig", "", "kubeconfig path (optional, should be run in-cluster, useful for local testing)")
	cmd.PersistentFlags().IntVar(&clusterLoaderClients, "clients", eksconfig.DefaultClients, "Number of clients to create")
	cmd.PersistentFlags().Float32Var(&clusterLoaderClientQPS, "client-qps", eksconfig.DefaultClientQPS, "kubelet client setup for QPS")
	cmd.PersistentFlags().IntVar(&clusterLoaderClientBurst, "client-burst", eksconfig.DefaultClientBurst, "kubelet client setup for burst")
	cmd.PersistentFlags().IntVar(&clusterLoaderNodes, "nodes", 10, "Number of cluster loader to create")
	cmd.PersistentFlags().StringVar(&clusterLoaderNamePrefix, "node-name-prefix", "hollow"+randutil.String(5), "Prefix to name cluster loader")
	cmd.PersistentFlags().StringVar(&clusterLoaderLabelPrefix, "node-label-prefix", randutil.String(5), "Prefix to label cluster loader")
	cmd.PersistentFlags().BoolVar(&clusterLoaderRemote, "remote", false, "'true' if run inside Pod")
	return cmd
}

func createClusterLoaderFunc(cmd *cobra.Command, args []string) {
	// optional
	if clusterLoaderKubeConfigPath != "" && !fileutil.Exist(clusterLoaderKubeConfigPath) {
		fmt.Fprintf(os.Stderr, "kubeconfig not found %q\n", clusterLoaderKubeConfigPath)
		os.Exit(1)
	}
	if len(clusterLoaderLabelPrefix) > 40 {
		fmt.Fprintf(os.Stderr, "invalid node label prefix %q (%d characters, label value can not be more than 63 characters)\n", clusterLoaderLabelPrefix, len(clusterLoaderLabelPrefix))
		os.Exit(1)
	}

	lg, err := logutil.GetDefaultZapLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger %v\n", err)
		os.Exit(1)
	}

	cli, err := k8s_client.NewEKS(&k8s_client.EKSConfig{
		Logger:         lg,
		KubeConfigPath: clusterLoaderKubeConfigPath,
		Clients:        clusterLoaderClients,
		ClientQPS:      clusterLoaderClientQPS,
		ClientBurst:    clusterLoaderClientBurst,
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
		Nodes:          clusterLoaderNodes,
		NodeNamePrefix: clusterLoaderNamePrefix + sfx,
		NodeLabels: map[string]string{
			"NodeType": "cluster-loader",
			"AMIType":  clusterLoaderLabelPrefix + "-ami-type-" + sfx,
			"NGType":   clusterLoaderLabelPrefix + "-ng-type-" + sfx,
			"NGName":   clusterLoaderLabelPrefix + "-ng-name-" + sfx,
		},
		Remote: clusterLoaderRemote,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create cluster loader group %v\n", err)
		os.Exit(1)
	}

	ng.Start()

	lg.Info("waiting before checking cluster loader")
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
		fmt.Fprintf(os.Stderr, "failed to check cluster loader group %v\n", err)
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
	fmt.Printf("'aws-k8s-tester eks create cluster-loader' success\n")
}
