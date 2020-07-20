// ecr-utils is a set of AWS utilities commands.
package main

import (
	"fmt"
	"os"

	ecr_create_repo "github.com/aws/aws-k8s-tester/cmd/ecr-utils/create-repo"
	ecr_set_policy "github.com/aws/aws-k8s-tester/cmd/ecr-utils/set-policy"
	"github.com/aws/aws-k8s-tester/cmd/ecr-utils/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:        "ecr-utils",
	Short:      "AWS utils CLI",
	SuggestFor: []string{"ecrutils"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	rootCmd.AddCommand(
		ecr_create_repo.NewCommand(),
		ecr_set_policy.NewCommand(),
		version.NewCommand(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "ecr-utils failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
