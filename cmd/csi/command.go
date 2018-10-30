package csi

import "github.com/spf13/cobra"

func init() {
	cobra.EnablePrefixMatching = true
}

// NewCommand returns a new 'csi' command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "csi",
		Short: "CSI commands",
	}
	cmd.AddCommand(
		newTest(),
	)
	return cmd
}
