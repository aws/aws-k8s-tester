// Package k8s implements etcd k8s related commands.
package k8s

import (
	"github.com/spf13/cobra"
)

var (
	enablePrompt bool
	endpoints    []string
)

func init() {
	cobra.EnablePrefixMatching = true
}

// NewCommand implements "eks-utils apis" command.
func NewCommand() *cobra.Command {
	ac := &cobra.Command{
		Use:   "k8s",
		Short: "k8s commands",
	}
	ac.PersistentFlags().BoolVar(&enablePrompt, "enable-prompt", true, "'true' to enable prompt mode")
	ac.PersistentFlags().StringSliceVar(&endpoints, "endpoints", []string{"localhost:2379"}, "etcd endpoints")
	ac.AddCommand(
		newListCommand(),
	)
	return ac
}
