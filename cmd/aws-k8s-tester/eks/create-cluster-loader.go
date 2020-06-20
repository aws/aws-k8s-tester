package eks

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	cluster_loader "github.com/aws/aws-k8s-tester/eks/cluster-loader"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	clusterLoaderKubeConfigPath                 string
	clusterLoaderPath                           string
	clusterLoaderTestConfigPath                 string
	clusterLoaderReportDir                      string
	clusterLoaderReportTarGzPath                string
	clusterLoaderLogsPath                       string
	clusterLoaderPodStartupLatencyOutputPath    string
	clusterLoaderRuns                           int
	clusterLoaderTimeout                        time.Duration
	clusterLoaderNodes                          int
	clusterLoaderNodesPerNamespace              int
	clusterLoaderPodsPerNode                    int
	clusterLoaderBigGroupSize                   int
	clusterLoaderMediumGroupSize                int
	clusterLoaderSmallGroupSize                 int
	clusterLoaderSmallStatefulSetsPerNamespace  int
	clusterLoaderMediumStatefulSetsPerNamespace int
	clusterLoaderCL2LoadTestThroughput          int
	clusterLoaderCL2EnablePVS                   bool
	clusterLoaderPrometheusScrapeKubeProxy      bool
	clusterLoaderEnableSystemPodMetrics         bool
)

func newCreateClusterLoader() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster-loader",
		Short: "Creates cluster loader",
		Run:   createClusterLoaderFunc,
	}
	cmd.PersistentFlags().StringVar(&clusterLoaderKubeConfigPath, "kubeconfig", "", "kubeconfig path (optional, should be run in-cluster, useful for local testing)")
	cmd.PersistentFlags().StringVar(&clusterLoaderPath, "cluster-loader-path", "", "clusterloader executable binary path")
	cmd.PersistentFlags().StringVar(&clusterLoaderTestConfigPath, "test-config-path", "", "clusterloader test configuration path")
	cmd.PersistentFlags().StringVar(&clusterLoaderReportDir, "report-dir", "", "clusterloader report directory")
	cmd.PersistentFlags().StringVar(&clusterLoaderReportTarGzPath, "report-tar-gz-path", "", "clusterloader report .tar.gz path")
	cmd.PersistentFlags().StringVar(&clusterLoaderLogsPath, "logs-path", "", "clusterloader log path")
	cmd.PersistentFlags().StringVar(&clusterLoaderPodStartupLatencyOutputPath, "pod-startup-latency-output-path", "", "clusterloader pod startup latency output path")
	cmd.PersistentFlags().IntVar(&clusterLoaderRuns, "runs", 1, "number of clusterloader runs")
	cmd.PersistentFlags().DurationVar(&clusterLoaderTimeout, "timeout", 30*time.Minute, "clusterloader timeout")
	cmd.PersistentFlags().IntVar(&clusterLoaderNodes, "nodes", 10, "number of nodes")
	cmd.PersistentFlags().IntVar(&clusterLoaderNodesPerNamespace, "nodes-per-namespace", 10, "number of nodes per namespace")
	cmd.PersistentFlags().IntVar(&clusterLoaderPodsPerNode, "pods-per-node", 10, "number of pods per node")
	cmd.PersistentFlags().IntVar(&clusterLoaderBigGroupSize, "big-group-size", 25, "big group size")
	cmd.PersistentFlags().IntVar(&clusterLoaderMediumGroupSize, "medium-group-size", 10, "medium group size")
	cmd.PersistentFlags().IntVar(&clusterLoaderSmallGroupSize, "small-group-size", 5, "small group size")
	cmd.PersistentFlags().IntVar(&clusterLoaderSmallStatefulSetsPerNamespace, "small-stateful-sets-per-namespace", 0, "small stateful sets per namespace")
	cmd.PersistentFlags().IntVar(&clusterLoaderMediumStatefulSetsPerNamespace, "medium-stateful-sets-per-namespace", 0, "medium stateful sets per namespace")
	cmd.PersistentFlags().IntVar(&clusterLoaderCL2LoadTestThroughput, "cl2-load-test-throughput", 20, "clusterloader2 test throughput")
	cmd.PersistentFlags().BoolVar(&clusterLoaderCL2EnablePVS, "cl2-enable-pvs", false, "'true' to enable CL2 PVS")
	cmd.PersistentFlags().BoolVar(&clusterLoaderPrometheusScrapeKubeProxy, "prometheus-scrape-kube-proxy", false, "'true' to enable Prometheus scrape kube-proxy")
	cmd.PersistentFlags().BoolVar(&clusterLoaderEnableSystemPodMetrics, "enable-system-pod-metrics", false, "'true' to enable system pod metrics")
	return cmd
}

func createClusterLoaderFunc(cmd *cobra.Command, args []string) {
	// optional
	if clusterLoaderKubeConfigPath != "" && !fileutil.Exist(clusterLoaderKubeConfigPath) {
		fmt.Fprintf(os.Stderr, "kubeconfig not found %q\n", clusterLoaderKubeConfigPath)
		os.Exit(1)
	}

	lg, err := logutil.GetDefaultZapLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger %v\n", err)
		os.Exit(1)
	}

	stopc := make(chan struct{})
	loader := cluster_loader.New(cluster_loader.Config{
		Logger: lg,
		Stopc:  stopc,

		KubeConfigPath: clusterLoaderKubeConfigPath,

		ClusterLoaderPath:           clusterLoaderPath,
		ClusterLoaderDownloadURL:    "",
		TestConfigPath:              clusterLoaderTestConfigPath,
		ReportDir:                   clusterLoaderReportDir,
		ReportTarGzPath:             clusterLoaderReportTarGzPath,
		LogPath:                     clusterLoaderLogsPath,
		PodStartupLatencyOutputPath: clusterLoaderPodStartupLatencyOutputPath,

		Runs:    clusterLoaderRuns,
		Timeout: clusterLoaderTimeout,

		Nodes: clusterLoaderNodes,

		NodesPerNamespace: clusterLoaderNodesPerNamespace,
		PodsPerNode:       clusterLoaderPodsPerNode,

		BigGroupSize:    clusterLoaderBigGroupSize,
		MediumGroupSize: clusterLoaderMediumGroupSize,
		SmallGroupSize:  clusterLoaderSmallGroupSize,

		SmallStatefulSetsPerNamespace:  clusterLoaderSmallStatefulSetsPerNamespace,
		MediumStatefulSetsPerNamespace: clusterLoaderMediumStatefulSetsPerNamespace,

		CL2LoadTestThroughput:     clusterLoaderCL2LoadTestThroughput,
		CL2EnablePVS:              clusterLoaderCL2EnablePVS,
		PrometheusScrapeKubeProxy: clusterLoaderPrometheusScrapeKubeProxy,
		EnableSystemPodMetrics:    clusterLoaderEnableSystemPodMetrics,
	})

	errc := make(chan error)
	go func() {
		select {
		case <-stopc:
			errc <- errors.New("stopped")
		case errc <- loader.Start():
		}
	}()

	lg.Info("waiting before checking cluster loader")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	select {
	case sig := <-sigs:
		lg.Info("received OS signal", zap.String("signal", sig.String()))
		close(stopc)
		loader.Stop()
		<-errc
		os.Exit(0)
	case err = <-errc:
		lg.Info("comleted cluster loader", zap.Error(err))
		close(stopc)
		loader.Stop()
	}

	lg.Info("waiting for OS signal after test completion")
	select {
	case sig := <-sigs:
		lg.Info("received OS signal", zap.String("signal", sig.String()))
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'aws-k8s-tester eks create cluster-loader' success\n")
}
