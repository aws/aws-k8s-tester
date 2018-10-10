// Package ec2 implements EC2 related commands.
package ec2

import "github.com/spf13/cobra"

func init() {
	cobra.EnablePrefixMatching = true
}

var path string

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
	)
	return rootCmd
}
