// k8s-tester-clusterloader installs Kubernetes clusterloader tester.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	"github.com/aws/aws-k8s-tester/k8s-tester/clusterloader"
	"github.com/aws/aws-k8s-tester/utils/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:        "k8s-tester-clusterloader",
	Short:      "Kubernetes clusterloader tester",
	SuggestFor: []string{"clusterloader"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

var (
	prompt             bool
	logLevel           string
	logOutputs         []string
	minimumNodes       int
	kubectlDownloadURL string
	kubectlPath        string
	kubeconfigPath     string
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&prompt, "prompt", true, "'true' to enable prompt mode")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", log.DefaultLogLevel, "Logging level")
	rootCmd.PersistentFlags().StringSliceVar(&logOutputs, "log-outputs", []string{"stderr"}, "Additional logger outputs")
	rootCmd.PersistentFlags().IntVar(&minimumNodes, "minimum-nodes", clusterloader.DefaultMinimumNodes, "minimum number of Kubernetes nodes required for installing this addon")
	rootCmd.PersistentFlags().StringVar(&kubectlDownloadURL, "kubectl-download-url", client.DefaultKubectlDownloadURL(), "kubectl download URL")
	rootCmd.PersistentFlags().StringVar(&kubectlPath, "kubectl-path", client.DefaultKubectlPath(), "kubectl path")
	rootCmd.PersistentFlags().StringVar(&kubeconfigPath, "kubeconfig-path", "", "KUBECONFIG path")

	rootCmd.AddCommand(
		newApply(),
		newDelete(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "k8s-tester-clusterloader failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

var (
	clusterloaderPath        string
	clusterloaderDownloadURL string

	provider string

	runs       int
	runTimeout time.Duration

	testConfigPath string

	runFromCluster bool
	nodes          int

	nodesPerNamespace int
	podsPerNode       int

	bigGroupSize    int
	mediumGroupSize int
	smallGroupSize  int

	smallStatefulSetsPerNamespace  int
	mediumStatefulSetsPerNamespace int

	cl2UseHostNetworkPods           bool
	cl2LoadTestThroughput           int
	cl2EnablePVS                    bool
	cl2SchedulerThroughputThreshold int
	prometheusScrapeKubeProxy       bool
	enableSystemPodMetrics          bool
)

func newApply() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply tests",
		Run:   createApplyFunc,
	}

	rootCmd.PersistentFlags().StringVar(&clusterloaderPath, "clusterloader-path", clusterloader.DefaultClusterloaderPath(), "clusterloader path")
	rootCmd.PersistentFlags().StringVar(&clusterloaderDownloadURL, "clusterloader-download-url", clusterloader.DefaultClusterloaderDownloadURL(), "clusterloader download URL")
	rootCmd.PersistentFlags().StringVar(&provider, "provider", clusterloader.DefaultProvider, "clusterloader provider")
	rootCmd.PersistentFlags().IntVar(&runs, "runs", clusterloader.DefaultRuns, "clusterloader runs")
	rootCmd.PersistentFlags().DurationVar(&runTimeout, "run-timeout", clusterloader.DefaultRunTimeout, "clusterloader run timeout")
	rootCmd.PersistentFlags().StringVar(&testConfigPath, "test-config-path", "", "clusterloader test config path")
	rootCmd.PersistentFlags().BoolVar(&runFromCluster, "run-from-cluster", clusterloader.DefaultRunFromCluster, "to run clusterloader2 in cluster")
	rootCmd.PersistentFlags().IntVar(&nodes, "nodes", clusterloader.DefaultNodes, "clusterloader nodes")
	rootCmd.PersistentFlags().IntVar(&nodesPerNamespace, "nodes-per-namespace", clusterloader.DefaultNodesPerNamespace, "clusterloader nodes per namespace")
	rootCmd.PersistentFlags().IntVar(&podsPerNode, "pods-per-node", clusterloader.DefaultPodsPerNode, "clusterloader pods per node")
	rootCmd.PersistentFlags().IntVar(&bigGroupSize, "big-group-size", clusterloader.DefaultBigGroupSize, "clusterloader big group size")
	rootCmd.PersistentFlags().IntVar(&mediumGroupSize, "medium-group-size", clusterloader.DefaultMediumGroupSize, "clusterloader medium group size")
	rootCmd.PersistentFlags().IntVar(&smallGroupSize, "small-group-size", clusterloader.DefaultSmallGroupSize, "clusterloader small group size")
	rootCmd.PersistentFlags().IntVar(&smallStatefulSetsPerNamespace, "small-stateful-sets-per-namespace", clusterloader.DefaultSmallStatefulSetsPerNamespace, "clusterloader small stateful sets per namespace")
	rootCmd.PersistentFlags().IntVar(&mediumStatefulSetsPerNamespace, "medium-stateful-sets-per-namespace", clusterloader.DefaultMediumStatefulSetsPerNamespace, "clusterloader medium stateful sets per namespace")
	rootCmd.PersistentFlags().BoolVar(&cl2UseHostNetworkPods, "cl2-use-host-network-pods", clusterloader.DefaultCL2UseHostNetworkPods, "clusterloader CL2 use host network pods")
	rootCmd.PersistentFlags().IntVar(&cl2LoadTestThroughput, "cl2-load-test-throughput", clusterloader.DefaultCL2LoadTestThroughput, "clusterloader CL2 load test throughput")
	rootCmd.PersistentFlags().BoolVar(&cl2EnablePVS, "cl2-enable-pvs", clusterloader.DefaultCL2UseHostNetworkPods, "clusterloader CL2 use host network pods")
	rootCmd.PersistentFlags().IntVar(&cl2SchedulerThroughputThreshold, "cl2-scheduler-throughput-threshold", clusterloader.DefaultCL2SchedulerThroughputThreshold, "clusterloader CL2 scheduler throughput threshold")
	rootCmd.PersistentFlags().BoolVar(&prometheusScrapeKubeProxy, "prometheus-scrape-kube-proxy", clusterloader.DefaultPrometheusScrapeKubeProxy, "clusterloader prometheus scrape kube-proxy")
	rootCmd.PersistentFlags().BoolVar(&enableSystemPodMetrics, "enable-system-pod-metrics", clusterloader.DefaultEnableSystemPodMetrics, "clusterloader enable system pod metrics")

	return cmd
}

func createApplyFunc(cmd *cobra.Command, args []string) {
	lg, logWriter, _, err := log.NewWithStderrWriter(logLevel, logOutputs)
	if err != nil {
		panic(err)
	}
	_ = zap.ReplaceGlobals(lg)

	cli, err := client.New(&client.Config{
		Logger:             lg,
		KubectlDownloadURL: kubectlDownloadURL,
		KubectlPath:        kubectlPath,
		KubeconfigPath:     kubeconfigPath,
	})
	if err != nil {
		lg.Panic("failed to create client", zap.Error(err))
	}

	cfg := &clusterloader.Config{
		Prompt:       prompt,
		Logger:       lg,
		LogWriter:    logWriter,
		MinimumNodes: minimumNodes,
		Client:       cli,

		ClusterloaderPath:        clusterloaderPath,
		ClusterloaderDownloadURL: clusterloaderDownloadURL,

		Provider: provider,

		Runs:       runs,
		RunTimeout: runTimeout,

		TestConfigPath: testConfigPath,

		RunFromCluster: runFromCluster,
		Nodes:          nodes,

		TestOverride: &clusterloader.TestOverride{
			Path: clusterloader.DefaultTestOverridePath(),

			NodesPerNamespace: nodesPerNamespace,
			PodsPerNode:       podsPerNode,

			BigGroupSize:    bigGroupSize,
			MediumGroupSize: mediumGroupSize,
			SmallGroupSize:  smallGroupSize,

			SmallStatefulSetsPerNamespace:  smallStatefulSetsPerNamespace,
			MediumStatefulSetsPerNamespace: mediumStatefulSetsPerNamespace,

			CL2UseHostNetworkPods:           cl2UseHostNetworkPods,
			CL2LoadTestThroughput:           cl2LoadTestThroughput,
			CL2EnablePVS:                    cl2EnablePVS,
			CL2SchedulerThroughputThreshold: cl2SchedulerThroughputThreshold,
			PrometheusScrapeKubeProxy:       prometheusScrapeKubeProxy,
			EnableSystemPodMetrics:          enableSystemPodMetrics,
		},
	}
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to validate (%v)\n", err)
		os.Exit(1)
	}

	ts := clusterloader.New(cfg)
	if err := ts.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to apply (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-clusterloader apply' success\n")
}

func newDelete() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete resources",
		Run:   createDeleteFunc,
	}
	return cmd
}

func createDeleteFunc(cmd *cobra.Command, args []string) {
	lg, logWriter, _, err := log.NewWithStderrWriter(logLevel, logOutputs)
	if err != nil {
		panic(err)
	}
	_ = zap.ReplaceGlobals(lg)

	cli, err := client.New(&client.Config{
		Logger:             lg,
		KubectlDownloadURL: kubectlDownloadURL,
		KubectlPath:        kubectlPath,
		KubeconfigPath:     kubeconfigPath,
	})
	if err != nil {
		lg.Panic("failed to create client", zap.Error(err))
	}

	cfg := &clusterloader.Config{
		Prompt:    prompt,
		Logger:    lg,
		LogWriter: logWriter,
		Client:    cli,
	}

	ts := clusterloader.New(cfg)
	if err := ts.Delete(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-clusterloader delete' success\n")
}
