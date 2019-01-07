// Package kubeadm implements kubeadm commands.
package kubeadm

import "github.com/spf13/cobra"

// NewCommand returns a new 'kubeadm' command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubeadm",
		Short: "kubeadm commands",
	}
	cmd.PersistentFlags().StringVarP(&path, "path", "p", "", "kubeadm test configuration file path")
	cmd.AddCommand(
		newCreate(),
		newDelete(),
	)
	return cmd
}

var path string
