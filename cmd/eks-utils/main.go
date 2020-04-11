// eks-utils is a set of EKS utilities commands.
package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/cmd/eks-utils/apis"
	"github.com/aws/aws-k8s-tester/cmd/eks-utils/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:        "eks-utils",
	Short:      "AWS EKS utils CLI",
	SuggestFor: []string{"eksutils"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	rootCmd.AddCommand(
		apis.NewCommand(),
		version.NewCommand(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "eks-utils failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
