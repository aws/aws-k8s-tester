package eks

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/eks/secrets"
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
	secretsKubeConfigPath string

	secretsPartition    string
	secretsRegion       string
	secretsS3BucketName string

	secretsClients       int
	secretsClientQPS     float32
	secretsClientBurst   int
	secretsClientTimeout time.Duration
	secretsObjects       int
	secretsObjectSize    int

	secretsNamespace  string
	secretsNamePrefix string

	secretsRequestsRawWritesJSONS3Dir      string
	secretsRequestsSummaryWritesJSONS3Dir  string
	secretsRequestsSummaryWritesTableS3Dir string
	secretsRequestsRawReadsJSONS3Dir       string
	secretsRequestsSummaryReadsJSONS3Dir   string
	secretsRequestsSummaryReadsTableS3Dir  string

	secretsWritesOutputNamePrefix string
	secretsReadsOutputNamePrefix  string
)

func newCreateSecrets() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "Creates cluster loader",
		Run:   createSecretsFunc,
	}
	cmd.PersistentFlags().StringVar(&secretsKubeConfigPath, "kubeconfig", "", "kubeconfig path (optional, should be run in-cluster, useful for local testing)")
	cmd.PersistentFlags().StringVar(&secretsPartition, "partition", "aws", "partition for AWS API")
	cmd.PersistentFlags().StringVar(&secretsRegion, "region", "us-west-2", "region for AWS API")
	cmd.PersistentFlags().StringVar(&secretsS3BucketName, "s3-bucket-name", "", "S3 bucket name to upload results")
	cmd.PersistentFlags().IntVar(&secretsClients, "clients", eksconfig.DefaultClients, "Number of clients to create")
	cmd.PersistentFlags().Float32Var(&secretsClientQPS, "client-qps", eksconfig.DefaultClientQPS, "kubelet client setup for QPS")
	cmd.PersistentFlags().IntVar(&secretsClientBurst, "client-burst", eksconfig.DefaultClientBurst, "kubelet client setup for burst")
	cmd.PersistentFlags().DurationVar(&secretsClientTimeout, "client-timeout", eksconfig.DefaultClientTimeout, "kubelet client timeout")
	cmd.PersistentFlags().IntVar(&secretsObjects, "objects", 0, "Size of object per write (0 to disable writes)")
	cmd.PersistentFlags().IntVar(&secretsObjectSize, "object-size", 0, "Size of object per write (0 to disable writes)")
	cmd.PersistentFlags().StringVar(&secretsNamespace, "namespace", "default", "namespace to send writes")
	cmd.PersistentFlags().StringVar(&secretsNamePrefix, "name-prefix", "", "prefix of Secret name")

	cmd.PersistentFlags().StringVar(&secretsRequestsRawWritesJSONS3Dir, "requests-raw-writes-json-s3-dir", "", "s3 directory prefix to upload")
	cmd.PersistentFlags().StringVar(&secretsRequestsSummaryWritesJSONS3Dir, "requests-summary-writes-json-s3-dir", "", "s3 directory prefix to upload")
	cmd.PersistentFlags().StringVar(&secretsRequestsSummaryWritesTableS3Dir, "requests-summary-writes-table-s3-dir", "", "s3 directory prefix to upload")
	cmd.PersistentFlags().StringVar(&secretsRequestsRawReadsJSONS3Dir, "requests-raw-reads-json-s3-dir", "", "s3 directory prefix to upload")
	cmd.PersistentFlags().StringVar(&secretsRequestsSummaryReadsJSONS3Dir, "requests-summary-reads-json-s3-dir", "", "s3 directory prefix to upload")
	cmd.PersistentFlags().StringVar(&secretsRequestsSummaryReadsTableS3Dir, "requests-summary-reads-table-s3-dir", "", "s3 directory prefix to upload")

	cmd.PersistentFlags().StringVar(&secretsWritesOutputNamePrefix, "writes-output-name-prefix", "", "writes results output name prefix in /var/log/")
	cmd.PersistentFlags().StringVar(&secretsReadsOutputNamePrefix, "reads-output-name-prefix", "", "reads results output name prefix in /var/log/")
	return cmd
}

func createSecretsFunc(cmd *cobra.Command, args []string) {
	// optional
	if secretsKubeConfigPath != "" && !fileutil.Exist(secretsKubeConfigPath) {
		fmt.Fprintf(os.Stderr, "kubeconfig not found %q\n", secretsKubeConfigPath)
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
		Partition: secretsPartition,
		Region:    secretsRegion,
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
		KubeConfigPath: secretsKubeConfigPath,
		Clients:        secretsClients,
		ClientQPS:      secretsClientQPS,
		ClientBurst:    secretsClientBurst,
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

	loader := secrets.New(secrets.Config{
		Logger:                          lg,
		Stopc:                           stopc,
		S3API:                           s3.New(awsSession),
		S3BucketName:                    secretsS3BucketName,
		Client:                          cli,
		ClientTimeout:                   secretsClientTimeout,
		Namespace:                       secretsNamespace,
		NamePrefix:                      secretsNamePrefix,
		Objects:                         secretsObjects,
		ObjectSize:                      secretsObjectSize,
		RequestsRawWritesJSONPath:       "/var/log/" + secretsWritesOutputNamePrefix + "-" + sfx + "-writes-raw.json",
		RequestsRawWritesJSONS3Key:      filepath.Join(secretsRequestsRawWritesJSONS3Dir, secretsWritesOutputNamePrefix+"-"+sfx+"-writes-raw.json"),
		RequestsSummaryWritesJSONPath:   "/var/log/" + secretsWritesOutputNamePrefix + "-" + sfx + "-writes-summary.json",
		RequestsSummaryWritesJSONS3Key:  filepath.Join(secretsRequestsSummaryWritesJSONS3Dir, secretsWritesOutputNamePrefix+"-"+sfx+"-writes-summary.json"),
		RequestsSummaryWritesTablePath:  "/var/log/" + secretsWritesOutputNamePrefix + "-" + sfx + "-writes-summary.txt",
		RequestsSummaryWritesTableS3Key: filepath.Join(secretsRequestsSummaryWritesTableS3Dir, secretsWritesOutputNamePrefix+"-"+sfx+"-writes-summary.txt"),
		RequestsRawReadsJSONPath:        "/var/log/" + secretsReadsOutputNamePrefix + "-" + sfx + "-reads-raw.json",
		RequestsRawReadsJSONS3Key:       filepath.Join(secretsRequestsRawReadsJSONS3Dir, secretsReadsOutputNamePrefix+"-"+sfx+"-reads-raw.json"),
		RequestsSummaryReadsJSONPath:    "/var/log/" + secretsReadsOutputNamePrefix + "-" + sfx + "-reads-summary.json",
		RequestsSummaryReadsJSONS3Key:   filepath.Join(secretsRequestsSummaryReadsJSONS3Dir, secretsReadsOutputNamePrefix+"-"+sfx+"-reads-summary.json"),
		RequestsSummaryReadsTablePath:   "/var/log/" + secretsReadsOutputNamePrefix + "-" + sfx + "-reads-summary.txt",
		RequestsSummaryReadsTableS3Key:  filepath.Join(secretsRequestsSummaryReadsTableS3Dir, secretsReadsOutputNamePrefix+"-"+sfx+"-reads-summary.txt"),
	})
	loader.Start()
	loader.Stop()
	close(donec)

	if err != nil {
		lg.Warn("failed to get metrics", zap.Error(err))
	}

	_, _, _, _, err = loader.CollectMetrics()
	if err != nil {
		lg.Warn("failed to get metrics", zap.Error(err))
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'aws-k8s-tester eks create secrets' success\n")
}
