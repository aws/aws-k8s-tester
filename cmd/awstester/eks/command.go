package eks

import "github.com/spf13/cobra"

func init() {
	cobra.EnablePrefixMatching = true
}

var path string

// NewCommand implements "awstest eks" command.
func NewCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:        "eks",
		Short:      "EKS commands",
		SuggestFor: []string{"ekk", "ekstester"},
	}
	rootCmd.PersistentFlags().StringVarP(&path, "path", "p", "", "eks awstester configuration file path")
	rootCmd.AddCommand(
		newCreate(),
		newDelete(),
		newCheck(),
		newProw(),
		newS3Upload(),
		newIngress(),
		newSidecar(),
		newTest(),
	)
	return rootCmd
}
