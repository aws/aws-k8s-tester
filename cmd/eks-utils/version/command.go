// Package version implements version command.
package version

import (
	"fmt"

	"github.com/aws/aws-k8s-tester/version"

	"github.com/spf13/cobra"
)

func init() {
	cobra.EnablePrefixMatching = true
}

// NewCommand implements "aws-k8s-tester eks" command.
func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Prints out eks-utils version",
		Run:   versionFunc,
	}
}

func versionFunc(cmd *cobra.Command, args []string) {
	fmt.Print(version.Version())
}
