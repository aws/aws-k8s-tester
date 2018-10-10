// Package accesslog implements AWS Application Load Balancer utilities.
package accesslog

import (
	"github.com/spf13/cobra"
)

func init() {
	cobra.EnablePrefixMatching = true
}

// NewCommand implements "awstest accesslog" command.
func NewCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "accesslog",
		Short: "Access log commands",
	}
	rootCmd.AddCommand(
		newALB(),
	)
	return rootCmd
}

func newALB() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alb",
		Short: "ALB access log commands",
	}
	cmd.AddCommand(
		newALBConvert(),
		newALBCount(),
	)
	return cmd
}
