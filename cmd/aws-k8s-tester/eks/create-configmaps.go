package eks

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	config_maps "github.com/aws/aws-k8s-tester/eks/configmaps"
	"github.com/aws/aws-k8s-tester/eksconfig"
	pkg_aws "github.com/aws/aws-k8s-tester/pkg/aws"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	configmapsKubeConfigPath string

	configmapsPartition    string
	configmapsRegion       string
	configmapsS3BucketName string

	configmapsClients       int
	configmapsClientQPS     float32
	configmapsClientBurst   int
	configmapsClientTimeout time.Duration
	configmapsObjects       int
	configmapsObjectSize    int

	configmapsNamespace string

	configmapsRequestsRawWritesJSONS3Dir      string
	configmapsRequestsSummaryWritesJSONS3Dir  string
	configmapsRequestsSummaryWritesTableS3Dir string

	configmapsWritesOutputNamePrefix string
)

func newCreateConfigMaps() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "configmaps",
		Short: "Creates cluster loader",
		Run:   createConfigMapsFunc,
	}
	cmd.PersistentFlags().StringVar(&configmapsKubeConfigPath, "kubeconfig", "", "kubeconfig path (optional, should be run in-cluster, useful for local testing)")
	cmd.PersistentFlags().StringVar(&configmapsPartition, "partition", "aws", "partition for AWS API")
	cmd.PersistentFlags().StringVar(&configmapsRegion, "region", "us-west-2", "region for AWS API")
	cmd.PersistentFlags().StringVar(&configmapsS3BucketName, "s3-bucket-name", "", "S3 bucket name to upload results")
	cmd.PersistentFlags().IntVar(&configmapsClients, "clients", eksconfig.DefaultClients, "Number of clients to create")
	cmd.PersistentFlags().Float32Var(&configmapsClientQPS, "client-qps", eksconfig.DefaultClientQPS, "kubelet client setup for QPS")
	cmd.PersistentFlags().IntVar(&configmapsClientBurst, "client-burst", eksconfig.DefaultClientBurst, "kubelet client setup for burst")
	cmd.PersistentFlags().DurationVar(&configmapsClientTimeout, "client-timeout", eksconfig.DefaultClientTimeout, "kubelet client timeout")
	cmd.PersistentFlags().IntVar(&configmapsObjects, "objects", 0, "Size of object per write (0 to disable writes)")
	cmd.PersistentFlags().IntVar(&configmapsObjectSize, "object-size", 0, "Size of object per write (0 to disable writes)")
	cmd.PersistentFlags().StringVar(&configmapsNamespace, "namespace", "default", "namespace to send writes")

	cmd.PersistentFlags().StringVar(&configmapsRequestsRawWritesJSONS3Dir, "requests-raw-writes-json-s3-dir", "", "s3 directory prefix to upload")
	cmd.PersistentFlags().StringVar(&configmapsRequestsSummaryWritesJSONS3Dir, "requests-summary-writes-json-s3-dir", "", "s3 directory prefix to upload")
	cmd.PersistentFlags().StringVar(&configmapsRequestsSummaryWritesTableS3Dir, "requests-summary-writes-table-s3-dir", "", "s3 directory prefix to upload")

	cmd.PersistentFlags().StringVar(&configmapsWritesOutputNamePrefix, "writes-output-name-prefix", "", "Write results output name prefix in /var/log/")
	return cmd
}

func createConfigMapsFunc(cmd *cobra.Command, args []string) {
	// optional
	if configmapsKubeConfigPath != "" && !fileutil.Exist(configmapsKubeConfigPath) {
		fmt.Fprintf(os.Stderr, "kubeconfig not found %q\n", configmapsKubeConfigPath)
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

	cli, err := k8s_client.NewEKS(&k8s_client.EKSConfig{
		Logger:         lg,
		KubeConfigPath: configmapsKubeConfigPath,
		Clients:        configmapsClients,
		ClientQPS:      configmapsClientQPS,
		ClientBurst:    configmapsClientBurst,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create client %v\n", err)
		os.Exit(1)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	stopc := make(chan struct{})
	donec := make(chan struct{})
	go func() {
		select {
		case sig := <-sigs:
			lg.Info("received OS signal; closing stopc", zap.String("signal", sig.String()))
			close(stopc)
		case <-donec:
		}
	}()

	// to randomize results output files
	// when multiple pods are created via deployment
	// we do not want each pod to write to the same file
	// we want to avoid conflicts and run checks for each pod
	// enough for make them unique per worker
	sfx := randutil.String(7)

	loader := config_maps.New(config_maps.Config{
		Logger:                          lg,
		Stopc:                           stopc,
		S3API:                           s3.New(awsSession),
		S3BucketName:                    configmapsS3BucketName,
		Client:                          cli,
		ClientTimeout:                   configmapsClientTimeout,
		Namespace:                       configmapsNamespace,
		Objects:                         configmapsObjects,
		ObjectSize:                      configmapsObjectSize,
		RequestsRawWritesJSONPath:       "/var/log/" + configmapsWritesOutputNamePrefix + "-" + sfx + "-writes-raw.json",
		RequestsRawWritesJSONS3Key:      filepath.Join(configmapsRequestsRawWritesJSONS3Dir, configmapsWritesOutputNamePrefix+"-"+sfx+"-writes-raw.json"),
		RequestsSummaryWritesJSONPath:   "/var/log/" + configmapsWritesOutputNamePrefix + "-" + sfx + "-writes-summary.json",
		RequestsSummaryWritesJSONS3Key:  filepath.Join(configmapsRequestsSummaryWritesJSONS3Dir, configmapsWritesOutputNamePrefix+"-"+sfx+"-writes-summary.json"),
		RequestsSummaryWritesTablePath:  "/var/log/" + configmapsWritesOutputNamePrefix + "-" + sfx + "-writes-summary.txt",
		RequestsSummaryWritesTableS3Key: filepath.Join(configmapsRequestsSummaryWritesTableS3Dir, configmapsWritesOutputNamePrefix+"-"+sfx+"-writes-summary.txt"),
	})
	loader.Start()
	loader.Stop()
	close(donec)

	_, _, err = loader.CollectMetrics()
	if err != nil {
		lg.Warn("failed to get metrics", zap.Error(err))
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'aws-k8s-tester eks create configmaps' success\n")
}
