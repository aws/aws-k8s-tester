// Package alblog implements AWS Application Load Balancer utilities.
package alblog

import (
	"github.com/spf13/cobra"
)

func init() {
	cobra.EnablePrefixMatching = true
}

// NewCommand implements "awstest alblog" command.
func NewCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "alblog",
		Short: "ALB access log commands",
	}
	rootCmd.AddCommand(
		newALBConvert(),
		newALBCount(),
	)
	return rootCmd
}
