// Package ec2 implements EC2 related commands.
package ec2

import "github.com/spf13/cobra"

var (
	path         string
	enablePrompt bool
)

// NewCommand implements "awstest ec2" command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "ec2",
		Short:      "ec2 commands",
		SuggestFor: []string{"ec22"},
	}
	cmd.PersistentFlags().StringVarP(&path, "path", "p", "", "ec2 test configuration file path")
	cmd.PersistentFlags().BoolVarP(&enablePrompt, "enable-prompt", "e", true, "'true' to enable prompt mode")
	cmd.AddCommand(
		newCreate(),
		newDelete(),
	)
	return cmd
}
