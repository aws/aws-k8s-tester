package eks

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/eks/stresser"
	"github.com/aws/aws-k8s-tester/eksconfig"
	pkg_aws "github.com/aws/aws-k8s-tester/pkg/aws"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	stresserKubeConfigPath string

	stresserPartition    string
	stresserRegion       string
	stresserS3BucketName string

	stresserClients       int
	stresserClientQPS     float32
	stresserClientBurst   int
	stresserClientTimeout time.Duration
	stresserObjectSize    int
	stresserListLimit     int64
	stresserDuration      time.Duration

	stresserNamespaceWrite string
	stresserNamespacesRead []string

	stresserRequestsRawWritesJSONS3Dir      string
	stresserRequestsSummaryWritesJSONS3Dir  string
	stresserRequestsSummaryWritesTableS3Dir string
	stresserRequestsRawReadsJSONS3Dir       string
	stresserRequestsSummaryReadsJSONS3Dir   string
	stresserRequestsSummaryReadsTableS3Dir  string

	stresserWritesOutputNamePrefix string
	stresserReadsOutputNamePrefix  string
)

func newCreateStresser() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stresser",
		Short: "Creates cluster loader",
		Run:   createStresserFunc,
	}
	cmd.PersistentFlags().StringVar(&stresserKubeConfigPath, "kubeconfig", "", "kubeconfig path (optional, should be run in-cluster, useful for local testing)")
	cmd.PersistentFlags().StringVar(&stresserPartition, "partition", "aws", "partition for AWS API")
	cmd.PersistentFlags().StringVar(&stresserRegion, "region", "us-west-2", "region for AWS API")
	cmd.PersistentFlags().StringVar(&stresserS3BucketName, "s3-bucket-name", "", "S3 bucket name to upload results")
	cmd.PersistentFlags().IntVar(&stresserClients, "clients", eksconfig.DefaultClients, "Number of clients to create")
	cmd.PersistentFlags().Float32Var(&stresserClientQPS, "client-qps", eksconfig.DefaultClientQPS, "kubelet client setup for QPS")
	cmd.PersistentFlags().IntVar(&stresserClientBurst, "client-burst", eksconfig.DefaultClientBurst, "kubelet client setup for burst")
	cmd.PersistentFlags().DurationVar(&stresserClientTimeout, "client-timeout", eksconfig.DefaultClientTimeout, "kubelet client timeout")
	cmd.PersistentFlags().IntVar(&stresserObjectSize, "object-size", 0, "Size of object per write (0 to disable writes)")
	cmd.PersistentFlags().Int64Var(&stresserListLimit, "list-limit", 0, "Maximum number of items to return for list call (0 to list all)")
	cmd.PersistentFlags().DurationVar(&stresserDuration, "duration", 5*time.Minute, "duration to run cluster loader")
	cmd.PersistentFlags().StringVar(&stresserNamespaceWrite, "namespace-write", "default", "namespaces to send writes")
	cmd.PersistentFlags().StringSliceVar(&stresserNamespacesRead, "namespaces-read", []string{"default"}, "namespaces to send reads")

	cmd.PersistentFlags().StringVar(&stresserRequestsRawWritesJSONS3Dir, "requests-raw-writes-json-s3-dir", "", "s3 directory prefix to upload")
	cmd.PersistentFlags().StringVar(&stresserRequestsSummaryWritesJSONS3Dir, "requests-summary-writes-json-s3-dir", "", "s3 directory prefix to upload")
	cmd.PersistentFlags().StringVar(&stresserRequestsSummaryWritesTableS3Dir, "requests-summary-writes-table-s3-dir", "", "s3 directory prefix to upload")
	cmd.PersistentFlags().StringVar(&stresserRequestsRawReadsJSONS3Dir, "requests-raw-reads-json-s3-dir", "", "s3 directory prefix to upload")
	cmd.PersistentFlags().StringVar(&stresserRequestsSummaryReadsJSONS3Dir, "requests-summary-reads-json-s3-dir", "", "s3 directory prefix to upload")
	cmd.PersistentFlags().StringVar(&stresserRequestsSummaryReadsTableS3Dir, "requests-summary-reads-table-s3-dir", "", "s3 directory prefix to upload")

	cmd.PersistentFlags().StringVar(&stresserWritesOutputNamePrefix, "writes-output-name-prefix", "", "writes results output name prefix in /var/log/")
	cmd.PersistentFlags().StringVar(&stresserReadsOutputNamePrefix, "reads-output-name-prefix", "", "reads results output name prefix in /var/log/")
	return cmd
}

func createStresserFunc(cmd *cobra.Command, args []string) {
	// optional
	if stresserKubeConfigPath != "" && !fileutil.Exist(stresserKubeConfigPath) {
		fmt.Fprintf(os.Stderr, "kubeconfig not found %q\n", stresserKubeConfigPath)
		os.Exit(1)
	}
	if err := os.MkdirAll("/var/log", 0700); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create dir %v\n", err)
		os.Exit(1)
	}
	if err := fileutil.IsDirWriteable("/var/log"); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write dir %v\n", err)
		os.Exit(1)
	}

	lg, err := logutil.GetDefaultZapLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger %v\n", err)
		os.Exit(1)
	}

	awsCfg := &pkg_aws.Config{
		Logger:    lg,
		Partition: stresserPartition,
		Region:    stresserRegion,
	}
	awsSession, _, _, err := pkg_aws.New(awsCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create AWS session %v\n", err)
		os.Exit(1)
	}

	cli, err := k8s_client.NewEKS(&k8s_client.EKSConfig{
		Logger:         lg,
		KubeConfigPath: stresserKubeConfigPath,
		Clients:        stresserClients,
		ClientQPS:      stresserClientQPS,
		ClientBurst:    stresserClientBurst,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create client %v\n", err)
		os.Exit(1)
	}

	stopc := make(chan struct{})

	// to randomize results output files
	// when multiple pods are created via deployment
	// we do not want each pod to write to the same file
	// we want to avoid conflicts and run checks for each pod
	// enough for make them unique per worker
	sfx := randutil.String(7)

	loader := stresser.New(stresser.Config{
		Logger:                          lg,
		LogWriter:                       os.Stderr,
		Stopc:                           stopc,
		S3API:                           s3.New(awsSession),
		S3BucketName:                    stresserS3BucketName,
		Client:                          cli,
		ClientTimeout:                   stresserClientTimeout,
		Deadline:                        time.Now().Add(stresserDuration),
		NamespaceWrite:                  stresserNamespaceWrite,
		NamespacesRead:                  stresserNamespacesRead,
		ObjectSize:                      stresserObjectSize,
		ListLimit:                       stresserListLimit,
		RequestsRawWritesJSONPath:       "/var/log/" + stresserWritesOutputNamePrefix + "-" + sfx + "-writes-raw.json",
		RequestsRawWritesJSONS3Key:      filepath.Join(stresserRequestsRawWritesJSONS3Dir, stresserWritesOutputNamePrefix+"-"+sfx+"-writes-raw.json"),
		RequestsSummaryWritesJSONPath:   "/var/log/" + stresserWritesOutputNamePrefix + "-" + sfx + "-writes-summary.json",
		RequestsSummaryWritesJSONS3Key:  filepath.Join(stresserRequestsSummaryWritesJSONS3Dir, stresserWritesOutputNamePrefix+"-"+sfx+"-writes-summary.json"),
		RequestsSummaryWritesTablePath:  "/var/log/" + stresserWritesOutputNamePrefix + "-" + sfx + "-writes-summary.txt",
		RequestsSummaryWritesTableS3Key: filepath.Join(stresserRequestsSummaryWritesTableS3Dir, stresserWritesOutputNamePrefix+"-"+sfx+"-writes-summary.txt"),
		RequestsRawReadsJSONPath:        "/var/log/" + stresserReadsOutputNamePrefix + "-" + sfx + "-reads-raw.json",
		RequestsRawReadsJSONS3Key:       filepath.Join(stresserRequestsRawReadsJSONS3Dir, stresserReadsOutputNamePrefix+"-"+sfx+"-reads-raw.json"),
		RequestsSummaryReadsJSONPath:    "/var/log/" + stresserReadsOutputNamePrefix + "-" + sfx + "-reads-summary.json",
		RequestsSummaryReadsJSONS3Key:   filepath.Join(stresserRequestsSummaryReadsJSONS3Dir, stresserReadsOutputNamePrefix+"-"+sfx+"-reads-summary.json"),
		RequestsSummaryReadsTablePath:   "/var/log/" + stresserReadsOutputNamePrefix + "-" + sfx + "-reads-summary.txt",
		RequestsSummaryReadsTableS3Key:  filepath.Join(stresserRequestsSummaryReadsTableS3Dir, stresserReadsOutputNamePrefix+"-"+sfx+"-reads-summary.txt"),
	})
	loader.Start()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-sigs:
		lg.Info("received OS signal", zap.String("signal", sig.String()))
		close(stopc)
		loader.Stop()
		os.Exit(0)
	case <-time.After(stresserDuration):
	}

	close(stopc)
	loader.Stop()

	_, _, _, _, err = loader.CollectMetrics()
	if err != nil {
		lg.Warn("failed to get metrics", zap.Error(err))
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'aws-k8s-tester eks create stresser' success\n")
}
