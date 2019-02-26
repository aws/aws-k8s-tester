// Package eks implements EKS related commands.
package eks

import "github.com/spf13/cobra"

func init() {
	cobra.EnablePrefixMatching = true
}

var path string

// NewCommand implements "awstest eks" command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "eks",
		Short:      "EKS commands",
		SuggestFor: []string{"ekk", "ekstester"},
	}
	cmd.PersistentFlags().StringVarP(&path, "path", "p", "", "aws-k8s-tester EKS configuration file path")
	cmd.AddCommand(
		newCreate(),
		newDelete(),
		newCheck(),
		newProw(),
		newS3Upload(),
		newSidecar(),
		newTest(),
	)
	return cmd
}
