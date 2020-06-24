package eks

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/eks/csrs"
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
	csrsKubeConfigPath string

	csrsPartition    string
	csrsRegion       string
	csrsS3BucketName string

	csrsClients                     int
	csrsClientQPS                   float32
	csrsClientBurst                 int
	csrsClientTimeout               time.Duration
	csrsObjects                     int
	csrsInitialRequestConditionType string

	csrsWritesRawJSONS3Dir      string
	csrsWritesSummaryJSONS3Dir  string
	csrsWritesSummaryTableS3Dir string

	csrsWritesOutputNamePrefix string

	csrsBlock bool
)

func newCreateCSRs() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "csrs",
		Short: "Creates cluster loader",
		Run:   createCSRsFunc,
	}
	cmd.PersistentFlags().StringVar(&csrsKubeConfigPath, "kubeconfig", "", "kubeconfig path (optional, should be run in-cluster, useful for local testing)")
	cmd.PersistentFlags().StringVar(&csrsPartition, "partition", "aws", "partition for AWS API")
	cmd.PersistentFlags().StringVar(&csrsRegion, "region", "us-west-2", "region for AWS API")
	cmd.PersistentFlags().StringVar(&csrsS3BucketName, "s3-bucket-name", "", "S3 bucket name to upload results")
	cmd.PersistentFlags().IntVar(&csrsClients, "clients", eksconfig.DefaultClients, "Number of clients to create")
	cmd.PersistentFlags().Float32Var(&csrsClientQPS, "client-qps", eksconfig.DefaultClientQPS, "kubelet client setup for QPS")
	cmd.PersistentFlags().IntVar(&csrsClientBurst, "client-burst", eksconfig.DefaultClientBurst, "kubelet client setup for burst")
	cmd.PersistentFlags().DurationVar(&csrsClientTimeout, "client-timeout", eksconfig.DefaultClientTimeout, "kubelet client timeout")
	cmd.PersistentFlags().IntVar(&csrsObjects, "objects", 0, "Size of object per write (0 to disable writes)")

	cmd.PersistentFlags().StringVar(&csrsWritesRawJSONS3Dir, "writes-raw-json-s3-dir", "", "s3 directory prefix to upload")
	cmd.PersistentFlags().StringVar(&csrsWritesSummaryJSONS3Dir, "writes-summary-json-s3-dir", "", "s3 directory prefix to upload")
	cmd.PersistentFlags().StringVar(&csrsWritesSummaryTableS3Dir, "writes-summary-table-s3-dir", "", "s3 directory prefix to upload")

	cmd.PersistentFlags().StringVar(&csrsInitialRequestConditionType, "initial-request-condition-type", "", "Initial CSR condition type")
	cmd.PersistentFlags().StringVar(&csrsWritesOutputNamePrefix, "writes-output-name-prefix", "", "Write results output name prefix in /var/log/")
	cmd.PersistentFlags().BoolVar(&csrsBlock, "block", false, "true to block process exit after cluster loader complete")
	return cmd
}

func createCSRsFunc(cmd *cobra.Command, args []string) {
	// optional
	if csrsKubeConfigPath != "" && !fileutil.Exist(csrsKubeConfigPath) {
		fmt.Fprintf(os.Stderr, "kubeconfig not found %q\n", csrsKubeConfigPath)
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
		Partition: csrsPartition,
		Region:    csrsRegion,
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
		KubeConfigPath: csrsKubeConfigPath,
		Clients:        csrsClients,
		ClientQPS:      csrsClientQPS,
		ClientBurst:    csrsClientBurst,
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

	loader := csrs.New(csrs.Config{
		Logger:                      lg,
		Stopc:                       stopc,
		S3API:                       s3.New(awsSession),
		S3BucketName:                csrsS3BucketName,
		Client:                      cli,
		ClientTimeout:               csrsClientTimeout,
		Objects:                     csrsObjects,
		InitialRequestConditionType: "",
		WritesRawJSONPath:           "/var/log/" + csrsWritesOutputNamePrefix + "-" + sfx + "-writes.json",
		WritesRawJSONS3Key:          filepath.Join(csrsWritesRawJSONS3Dir, csrsWritesOutputNamePrefix+"-"+sfx+"-writes.json"),
		WritesSummaryJSONPath:       "/var/log/" + csrsWritesOutputNamePrefix + "-" + sfx + "-writes-summary.json",
		WritesSummaryJSONS3Key:      filepath.Join(csrsWritesSummaryJSONS3Dir, csrsWritesOutputNamePrefix+"-"+sfx+"-writes-summary.json"),
		WritesSummaryTablePath:      "/var/log/" + csrsWritesOutputNamePrefix + "-" + sfx + "-writes-summary.txt",
		WritesSummaryTableS3Key:     filepath.Join(csrsWritesSummaryTableS3Dir, csrsWritesOutputNamePrefix+"-"+sfx+"-writes-summary.txt"),
	})
	loader.Start()
	loader.Stop()
	close(donec)

	_, err = loader.CollectMetrics()
	if err != nil {
		lg.Warn("failed to get metrics", zap.Error(err))
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'aws-k8s-tester eks create csrs' success\n")

	if csrsBlock {
		lg.Info("waiting for OS signal")
		select {
		case sig := <-sigs:
			lg.Info("received OS signal", zap.String("signal", sig.String()))
			os.Exit(0)
		}
	}
}
