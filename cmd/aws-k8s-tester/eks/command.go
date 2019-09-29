// Package eks implements EKS related commands.
package eks

import "github.com/spf13/cobra"

func init() {
	cobra.EnablePrefixMatching = true
}

var (
	path string

	region string

	resolverURL string
	signingName string

	kubernetesVersion string
	amiType           string

	more    bool
	cleanUp bool
)

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
		newGet(),
		newList(),
		newProw(),
		newS3Upload(),
		newTest(),
	)
	return cmd
}
