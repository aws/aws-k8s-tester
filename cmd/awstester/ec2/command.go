package ec2

import "github.com/spf13/cobra"

// NewCommand implements "awstest ec2" command.
func NewCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:        "ec2",
		Short:      "EC2 commands",
		SuggestFor: []string{"ec22"},
	}
	rootCmd.PersistentFlags().StringVarP(&path, "path", "p", "", "ec2 test configuration file path")
	rootCmd.AddCommand(
		newCreate(),
		newDelete(),
		newWait(),
	)
	return rootCmd
}

var path string
