// sts-utils is a set of AWS utilities commands.
package main

import (
	"fmt"
	"os"

	get_caller_identity "github.com/aws/aws-k8s-tester/cmd/sts-utils/get-caller-identity"
	"github.com/aws/aws-k8s-tester/cmd/sts-utils/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:        "sts-utils",
	Short:      "AWS utils CLI",
	SuggestFor: []string{"stsutils"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	rootCmd.AddCommand(
		get_caller_identity.NewCommand(),
		version.NewCommand(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "sts-utils failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
