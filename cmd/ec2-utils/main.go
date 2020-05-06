// ec2-utils is a set of AWS EC2 commands.
package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/cmd/ec2-utils/create"
	"github.com/aws/aws-k8s-tester/cmd/ec2-utils/delete"
	"github.com/aws/aws-k8s-tester/cmd/ec2-utils/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:        "ec2-utils",
	Short:      "AWS EC2 utils CLI",
	SuggestFor: []string{"ec22"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	rootCmd.AddCommand(
		create.NewCommand(),
		delete.NewCommand(),
		version.NewCommand(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "ec2-utils failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
