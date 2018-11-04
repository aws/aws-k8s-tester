// Package etcd implements etcd commands.
package etcd

import "github.com/spf13/cobra"

// NewCommand returns a new 'etcd' command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "etcd",
		Short: "etcd commands",
	}
	cmd.PersistentFlags().StringVarP(&path, "path", "p", "", "ec2 test configuration file path")
	cmd.AddCommand(
		newTest(),
		newCreate(),
		newDelete(),
	)
	return cmd
}

var path string
