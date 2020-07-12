// Package sts implements "aws sts get-caller-identity" commands.
package sts

import (
	"fmt"
	"os"
	"strings"

	pkg_aws "github.com/aws/aws-k8s-tester/pkg/aws"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	logLevel            string
	partition           string
	region              string
	matchExactRoleARN   string
	matchContainRoleARN string
)

func init() {
	cobra.EnablePrefixMatching = true
}

// NewCommand implements "aws-utils sts" command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sts [OUTPUT-PATH]",
		Short: "AWS sts commands",
		Run:   stsFunc,
	}
	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error, dpanic, panic, fatal)")
	cmd.PersistentFlags().StringVar(&partition, "partition", "aws", "AWS partition")
	cmd.PersistentFlags().StringVar(&region, "region", "us-west-2", "AWS region")
	cmd.PersistentFlags().StringVar(&matchExactRoleARN, "match-exact-role-arn", "", "Expected IAM role ARN, error if not empty and not match")
	cmd.PersistentFlags().StringVar(&matchContainRoleARN, "match-contain-role-arn", "", "Expected IAM role ARN, error if not empty and not contain")
	return cmd
}

func stsFunc(cmd *cobra.Command, args []string) {
	lcfg := logutil.GetDefaultZapLoggerConfig()
	lcfg.Level = zap.NewAtomicLevelAt(logutil.ConvertToZapLevel(logLevel))
	lg, err := lcfg.Build()
	if err != nil {
		panic(err)
	}
	_, stsOutput, _, err := pkg_aws.New(&pkg_aws.Config{
		Logger:        lg,
		DebugAPICalls: logLevel == "debug",
		Partition:     partition,
		Region:        region,
	})
	if err != nil {
		lg.Fatal("failed to create AWS session", zap.Error(err))
	}
	roleARN := aws.StringValue(stsOutput.Arn)
	fmt.Fprintf(os.Stderr, "Account: %q\n", aws.StringValue(stsOutput.Account))
	fmt.Fprintf(os.Stderr, "Role Arn: %q\n", roleARN)
	fmt.Fprintf(os.Stderr, "UserId: %q\n", aws.StringValue(stsOutput.UserId))
	if matchExactRoleARN != "" {
		if matchExactRoleARN != roleARN {
			fmt.Fprintf(os.Stderr, "STS CALLER ROLE MATCH EXACT FAIL: Role ARN expected %q, but got %q\n", matchExactRoleARN, roleARN)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "STS CALLER ROLE MATCH EXACT SUCCESS: Role ARN expected %q == %q\n", matchExactRoleARN, roleARN)
	}
	if matchContainRoleARN != "" {
		if !strings.Contains(roleARN, matchContainRoleARN) {
			fmt.Fprintf(os.Stderr, "STS CALLER ROLE MATCH CONTAIN FAIL: Role ARN expected to contain %q, but got %q\n", matchContainRoleARN, roleARN)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "STS CALLER ROLE MATCH CONTAIN SUCCESS: Role ARN expected to contain %q, and got %q\n", matchContainRoleARN, roleARN)
	}
}
