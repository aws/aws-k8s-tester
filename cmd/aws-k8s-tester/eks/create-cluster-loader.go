package eks

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	cluster_loader "github.com/aws/aws-k8s-tester/eks/cluster-loader"
	pkg_aws "github.com/aws/aws-k8s-tester/pkg/aws"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	clusterLoaderPartition    string
	clusterLoaderRegion       string
	clusterLoaderS3BucketName string

	clusterLoaderKubeConfigPath                 string
	clusterLoaderPath                           string
	clusterLoaderTestConfigPath                 string
	clusterLoaderReportDir                      string
	clusterLoaderReportTarGzPath                string
	clusterLoaderReportTarGzS3Key               string
	clusterLoaderLogPath                        string
	clusterLoaderLogS3Key                       string
	clusterLoaderPodStartupLatencyPath          string
	clusterLoaderPodStartupLatencyS3Key         string
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
	clusterLoaderCL2UseHostNetworkPods          bool
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

	cmd.PersistentFlags().StringVar(&clusterLoaderPartition, "partition", "aws", "partition for AWS API")
	cmd.PersistentFlags().StringVar(&clusterLoaderRegion, "region", "us-west-2", "region for AWS API")
	cmd.PersistentFlags().StringVar(&clusterLoaderS3BucketName, "s3-bucket-name", "", "S3 bucket name to upload results")

	cmd.PersistentFlags().StringVar(&clusterLoaderKubeConfigPath, "kubeconfig", "", "kubeconfig path (optional, should be run in-cluster, useful for local testing)")
	cmd.PersistentFlags().StringVar(&clusterLoaderPath, "cluster-loader-path", "", "clusterloader executable binary path")
	cmd.PersistentFlags().StringVar(&clusterLoaderTestConfigPath, "test-config-path", "", "clusterloader test configuration path")
	cmd.PersistentFlags().StringVar(&clusterLoaderReportDir, "report-dir", "", "clusterloader report directory")
	cmd.PersistentFlags().StringVar(&clusterLoaderReportTarGzPath, "report-tar-gz-path", "", "clusterloader report .tar.gz path")
	cmd.PersistentFlags().StringVar(&clusterLoaderReportTarGzS3Key, "report-tar-gz-s3-key", "", "clusterloader report .tar.gz path")
	cmd.PersistentFlags().StringVar(&clusterLoaderLogPath, "log-path", "", "clusterloader log path")
	cmd.PersistentFlags().StringVar(&clusterLoaderLogS3Key, "log-s3-key", "", "clusterloader log path")
	cmd.PersistentFlags().StringVar(&clusterLoaderPodStartupLatencyPath, "pod-startup-latency-path", "", "clusterloader pod startup latency output path")
	cmd.PersistentFlags().StringVar(&clusterLoaderPodStartupLatencyS3Key, "pod-startup-latency-s3-key", "", "clusterloader pod startup latency output path")
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
	cmd.PersistentFlags().BoolVar(&clusterLoaderCL2UseHostNetworkPods, "cl2-use-host-network-pods", false, "clusterloader2 use host network pods to bypass CNI")
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

	awsCfg := &pkg_aws.Config{
		Logger:    lg,
		Partition: configmapsPartition,
		Region:    configmapsRegion,
	}
	awsSession, stsOutput, _, err := pkg_aws.New(awsCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create AWS session %v\n", err)
		os.Exit(1)
	}
	awsAccountID := aws.StringValue(stsOutput.Account)
	awsUserID := aws.StringValue(stsOutput.UserId)
	awsIAMRoleARN := aws.StringValue(stsOutput.Arn)
	lg.Info("created AWS session",
		zap.String("aws-account-id", awsAccountID),
		zap.String("aws-user-id", awsUserID),
		zap.String("aws-iam-role-arn", awsIAMRoleARN),
	)

	stopc := make(chan struct{})
	loader := cluster_loader.New(cluster_loader.Config{
		Logger:    lg,
		LogWriter: os.Stderr,

		Stopc: stopc,

		S3API:        s3.New(awsSession),
		S3BucketName: clusterLoaderS3BucketName,

		KubeConfigPath: clusterLoaderKubeConfigPath,

		ClusterLoaderPath:        clusterLoaderPath,
		ClusterLoaderDownloadURL: "",
		TestConfigPath:           clusterLoaderTestConfigPath,

		ReportDir:              clusterLoaderReportDir,
		ReportTarGzPath:        clusterLoaderReportTarGzPath,
		ReportTarGzS3Key:       clusterLoaderReportTarGzS3Key,
		LogPath:                clusterLoaderLogPath,
		LogS3Key:               clusterLoaderLogS3Key,
		PodStartupLatencyPath:  clusterLoaderPodStartupLatencyPath,
		PodStartupLatencyS3Key: clusterLoaderPodStartupLatencyS3Key,

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

		CL2UseHostNetworkPods:     clusterLoaderCL2UseHostNetworkPods,
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
		lg.Info("completed cluster loader", zap.Error(err))
		close(stopc)
		loader.Stop()
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'aws-k8s-tester eks create cluster-loader' success\n")
}
