package version

import (
	"fmt"

	"github.com/aws/aws-k8s-tester/version"

	"github.com/spf13/cobra"
)

func init() {
	cobra.EnablePrefixMatching = true
}

// NewCommand implements "awstest eks" command.
func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Prints out aws-k8s-tester version",
		Run:   versionFunc,
	}
}

func versionFunc(cmd *cobra.Command, args []string) {
	fmt.Printf(`GitCommit: %s
ReleaseVersion: %s
BuildTime: %s
`,
		version.GitCommit,
		version.ReleaseVersion,
		version.BuildTime,
	)
}
