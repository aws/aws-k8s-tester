// aws-utils is a set of AWS utilities commands.
package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/cmd/aws-utils/sts"
	"github.com/aws/aws-k8s-tester/cmd/aws-utils/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:        "aws-utils",
	Short:      "AWS utils CLI",
	SuggestFor: []string{"awsutils"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	rootCmd.AddCommand(
		sts.NewCommand(),
		version.NewCommand(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "aws-utils failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
