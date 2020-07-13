// s3-utils is a set of AWS utilities commands.
package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/cmd/s3-utils/cp"
	"github.com/aws/aws-k8s-tester/cmd/s3-utils/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "s3-utils",
	Short: "AWS utils CLI",
}

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	rootCmd.AddCommand(
		cp.NewCommand(),
		version.NewCommand(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "s3-utils failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
