// awstester is a set of AWS test commands.
package main

import (
	"fmt"
	"os"

	alblog "github.com/aws/awstester/cmd/awstester/alb-log"
	"github.com/aws/awstester/cmd/awstester/csi"
	"github.com/aws/awstester/cmd/awstester/ec2"
	"github.com/aws/awstester/cmd/awstester/ecr"
	"github.com/aws/awstester/cmd/awstester/eks"
	"github.com/aws/awstester/cmd/awstester/version"
	"github.com/aws/awstester/cmd/awstester/wrk"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:        "awstester",
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
		fmt.Fprintf(os.Stderr, "awstester failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
