// Package ecr implements ECR related commands.
package ecr

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/pkg/awsapi"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// NewCommand returns a new 'ecr' command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ecr",
		Short: "ECR commands",
	}
	cmd.PersistentFlags().StringVar(&region, "region", "us-west-2", "AWS Region")
	cmd.PersistentFlags().StringVar(&customEndpoint, "custom-endpoint", "", "AWS custom endpoint")
	cmd.AddCommand(
		newGetRegistry(),
	)
	return cmd
}

var (
	region         string
	customEndpoint string
)

func newGetRegistry() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-registry",
		Short: "Returns registry name",
		Run:   getRegistryFunc,
	}
	return cmd
}

func getRegistryFunc(cmd *cobra.Command, args []string) {
	lg, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger (%v)\n", err)
		os.Exit(1)
	}
	awsCfg := &awsapi.Config{
		Logger:         lg,
		DebugAPICalls:  false,
		Region:         region,
		CustomEndpoint: customEndpoint,
	}
	var ss *session.Session
	ss, err = awsapi.New(awsCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create AWS API (%v)\n", err)
		os.Exit(1)
	}

	st := sts.New(ss)
	output, err := st.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get caller identity (%v)\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s.dkr.ecr.%s.amazonaws.com", *output.Account, region)
}
