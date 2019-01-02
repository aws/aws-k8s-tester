// Package kubernetes implements kubernetes commands.
package kubernetes

import "github.com/spf13/cobra"

// NewCommand returns a new 'kubernetes' command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubernetes",
		Short: "kubernetes commands",
	}
	cmd.PersistentFlags().StringVarP(&path, "path", "p", "", "Kubernetes test configuration file path")
	cmd.AddCommand(
		newCreate(),
		newDelete(),
	)
	return cmd
}

var path string
