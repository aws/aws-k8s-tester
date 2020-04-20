// Package eks implements EKS related commands.
package eks

import "github.com/spf13/cobra"

func init() {
	cobra.EnablePrefixMatching = true
}

var (
	path         string
	enablePrompt bool
)

// NewCommand implements "aws-k8s-tester eks" command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "eks",
		Short:      "eks commands",
		SuggestFor: []string{"ekk", "ekstester"},
	}
	cmd.PersistentFlags().StringVarP(&path, "path", "p", "", "aws-k8s-tester EKS configuration file path")
	cmd.PersistentFlags().BoolVarP(&enablePrompt, "enable-prompt", "e", true, "'true' to enable prompt mode")
	cmd.AddCommand(
		newCreate(),
		newDelete(),
		newCheck(),
		newList(),
	)
	return cmd
}
