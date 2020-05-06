// cw-utils is a set of AWS CloudWatch utilities commands.
package main

import (
	"fmt"
	"os"

	metrics_image "github.com/aws/aws-k8s-tester/cmd/cw-utils/metrics-image"
	"github.com/aws/aws-k8s-tester/cmd/cw-utils/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:        "cw-utils",
	Short:      "AWS CloudWatch utils CLI",
	SuggestFor: []string{"cwutils"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	rootCmd.AddCommand(
		metrics_image.NewCommand(),
		version.NewCommand(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "cw-utils failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
