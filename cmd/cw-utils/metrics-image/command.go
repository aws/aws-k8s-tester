// Package metricsimage implements AWS CloudWatch related commands.
package metricsimage

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aws/aws-k8s-tester/pkg/aws"
	"github.com/aws/aws-k8s-tester/pkg/aws/cw"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	logLevel  string
	partition string
	region    string
	queryPath string
)

func init() {
	cobra.EnablePrefixMatching = true
}

// NewCommand implements "eks-utils nodes" command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics-image [OUTPUT-PATH]",
		Short: "AWS CloudWatch metrics image commands",
		Run:   metricsImageFunc,
	}
	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error, dpanic, panic, fatal)")
	cmd.PersistentFlags().StringVar(&partition, "partition", "aws", "AWS partition")
	cmd.PersistentFlags().StringVar(&region, "region", "us-west-2", "AWS region")
	cmd.PersistentFlags().StringVar(&queryPath, "query-path", "", "JSON query to load")
	return cmd
}

func metricsImageFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "expected 1 argument for image output; got %q", args)
		os.Exit(1)
	}

	lcfg := logutil.GetDefaultZapLoggerConfig()
	lcfg.Level = zap.NewAtomicLevelAt(logutil.ConvertToZapLevel(logLevel))
	lg, err := lcfg.Build()
	if err != nil {
		panic(err)
	}

	if !fileutil.Exist(queryPath) {
		fmt.Fprintf(os.Stderr, "query path %q does not exist", queryPath)
		os.Exit(1)
	}
	d, err := ioutil.ReadFile(queryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read %q (%v))", queryPath, err)
		os.Exit(1)
	}
	query := string(d)

	ss, _, _, err := aws.New(&aws.Config{
		Logger:    lg,
		Partition: partition,
		Region:    region,
	})
	if err != nil {
		lg.Fatal("failed to create AWS session", zap.Error(err))
	}
	cwAPI := cloudwatch.New(ss)
	if err = cw.GetMetricsImage(lg, cwAPI, query, args[0]); err != nil {
		lg.Fatal("failed to get CW metrics image", zap.Error(err))
	}
}
