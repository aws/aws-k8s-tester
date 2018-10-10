// Package eks implements EKS related commands.
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
	rootCmd.PersistentFlags().StringVarP(&path, "path", "p", "", "eks test configuration file path")
	rootCmd.AddCommand(
		newCreate(),
		newDelete(),
		newCheck(),
		newProw(),
		newUpload(),
		newIngress(),
		newSidecar(),
	)
	return rootCmd
}
