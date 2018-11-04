// Package ec2 implements EC2 related commands.
package ec2

import "github.com/spf13/cobra"

// NewCommand implements "awstest ec2" command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "ec2",
		Short:      "EC2 commands",
		SuggestFor: []string{"ec22"},
	}
	cmd.PersistentFlags().StringVarP(&path, "path", "p", "", "ec2 test configuration file path")
	cmd.AddCommand(
		newCreate(),
		newDelete(),
	)
	return cmd
}

var path string
