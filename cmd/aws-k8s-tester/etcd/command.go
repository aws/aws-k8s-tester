// Package etcd implements etcd commands.
package etcd

import (
	"github.com/spf13/cobra"
)

// NewCommand returns a new 'etcd' command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "etcd",
		Short: "etcd commands",
	}
	cmd.AddCommand(
		newTest(),
	)
	return cmd
}
