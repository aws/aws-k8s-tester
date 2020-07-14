// Package cp implements "aws s3 cp" commands.
package cp

import (
	"fmt"
	"os"
	"time"

	pkg_aws "github.com/aws/aws-k8s-tester/pkg/aws"
	pkg_s3 "github.com/aws/aws-k8s-tester/pkg/aws/s3"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	logLevel  string
	partition string
	region    string
	s3Bucket  string
	s3Key     string
	localPath string
	timeout   time.Duration
)

func init() {
	cobra.EnablePrefixMatching = true
}

// NewCommand implements "s3-utils cp" command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cp",
		Short: "AWS s3 cp commands",
		Run:   cpFunc,
	}
	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error, dpanic, panic, fatal)")
	cmd.PersistentFlags().StringVar(&partition, "partition", "aws", "AWS partition")
	cmd.PersistentFlags().StringVar(&region, "region", "us-west-2", "AWS region")
	cmd.PersistentFlags().StringVar(&s3Bucket, "s3-bucket", "", "s3 bucket")
	cmd.PersistentFlags().StringVar(&s3Key, "s3-key", "", "s3 key")
	cmd.PersistentFlags().StringVar(&localPath, "local-path", "", "local download path")
	cmd.PersistentFlags().DurationVar(&timeout, "timeout", 5*time.Minute, "request timeout")
	return cmd
}

func cpFunc(cmd *cobra.Command, args []string) {
	lcfg := logutil.GetDefaultZapLoggerConfig()
	lcfg.Level = zap.NewAtomicLevelAt(logutil.ConvertToZapLevel(logLevel))
	lg, err := lcfg.Build()
	if err != nil {
		panic(err)
	}
	ss, _, _, err := pkg_aws.New(&pkg_aws.Config{
		Logger:        lg,
		DebugAPICalls: logLevel == "debug",
		Partition:     partition,
		Region:        region,
	})
	if ss == nil {
		lg.Fatal("failed to create AWS session", zap.Error(err))
	}
	if err != nil {
		lg.Warn("failed to create AWS session or get sts caller identity", zap.Error(err))
	}

	reqOpts := []pkg_s3.OpOption{
		pkg_s3.WithOverwrite(true),
	}
	if timeout > 0 {
		reqOpts = append(reqOpts, pkg_s3.WithTimeout(timeout))
	}
	if err = pkg_s3.Download(lg, s3.New(ss), s3Bucket, s3Key, localPath, reqOpts...); err != nil {
		lg.Fatal("failed to download S3 file",
			zap.String("s3-bucket", s3Bucket),
			zap.String("s3-key", s3Key),
			zap.Error(err),
		)
	} else {
		fmt.Fprintf(os.Stderr, "SUCCESSFULLY DOWNLOADED %q %q to %q\n", s3Bucket, s3Key, localPath)
	}
}
