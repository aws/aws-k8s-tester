package alblog

import "github.com/spf13/cobra"

func init() {
	cobra.EnablePrefixMatching = true
}

// NewCommand implements "awstest alb-log" command.
func NewCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "alb-log",
		Short: "ALB access log commands",
	}
	rootCmd.PersistentFlags().StringVar(&output, "output", "", "output file path")
	rootCmd.AddCommand(
		newConvertToCSV(),
		newCountTargets(),
		newMergeRaw(),
	)
	return rootCmd
}

var output string
