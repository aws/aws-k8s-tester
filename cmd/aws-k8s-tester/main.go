// aws-k8s-tester is a set of AWS test commands.
package main

import (
	"fmt"
	"os"

	alblog "github.com/aws/aws-k8s-tester/cmd/aws-k8s-tester/alb-log"
	"github.com/aws/aws-k8s-tester/cmd/aws-k8s-tester/csi"
	"github.com/aws/aws-k8s-tester/cmd/aws-k8s-tester/ec2"
	"github.com/aws/aws-k8s-tester/cmd/aws-k8s-tester/ecr"
	"github.com/aws/aws-k8s-tester/cmd/aws-k8s-tester/eks"
	"github.com/aws/aws-k8s-tester/cmd/aws-k8s-tester/etcd"
	"github.com/aws/aws-k8s-tester/cmd/aws-k8s-tester/version"
	"github.com/aws/aws-k8s-tester/cmd/aws-k8s-tester/wrk"

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
		etcd.NewCommand(),
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
