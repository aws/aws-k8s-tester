// Package k8s implements etcd k8s related commands.
package k8s

import (
	"github.com/spf13/cobra"
)

var (
	logLevel     string
	enablePrompt bool
	endpoints    []string
)

func init() {
	cobra.EnablePrefixMatching = true
}

// NewCommand implements "eks-utils apis" command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "k8s",
		Short: "k8s commands",
	}
	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error, dpanic, panic, fatal)")
	cmd.PersistentFlags().BoolVar(&enablePrompt, "enable-prompt", true, "'true' to enable prompt mode")
	cmd.PersistentFlags().StringSliceVar(&endpoints, "endpoints", []string{"localhost:2379"}, "etcd endpoints")
	cmd.AddCommand(
		newListCommand(),
	)
	return cmd
}
