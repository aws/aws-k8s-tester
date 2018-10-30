// aws-k8s-tester is a set of AWS test commands.
package main

import (
	"fmt"
	"os"

	alblog "github.com/aws/aws-k8s-tester/cmd/alb-log"
	"github.com/aws/aws-k8s-tester/cmd/csi"
	"github.com/aws/aws-k8s-tester/cmd/ec2"
	"github.com/aws/aws-k8s-tester/cmd/ecr"
	"github.com/aws/aws-k8s-tester/cmd/eks"
	"github.com/aws/aws-k8s-tester/cmd/version"
	"github.com/aws/aws-k8s-tester/cmd/wrk"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:        "aws-k8s-tester",
	Short:      "AWS test CLI",
	SuggestFor: []string{"awstest"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	rootCmd.AddCommand(
		alblog.NewCommand(),
		csi.NewCommand(),
		ec2.NewCommand(),
		ecr.NewCommand(),
		eks.NewCommand(),
		wrk.NewCommand(),
		version.NewCommand(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "aws-k8s-tester failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
