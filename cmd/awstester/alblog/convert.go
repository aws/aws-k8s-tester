package alblog

import (
	"fmt"
	"os"

	"github.com/aws/awstester/internal/alblog"

	"github.com/spf13/cobra"
)

func newALBConvert() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert [list of ALB access log files to convert]",
		Short: "Converts ALB access log to CSV",
		Run:   albConvertFunc,
	}
	cmd.PersistentFlags().StringVar(&convertOutput, "output", "", "output file path")
	return cmd
}

var convertOutput string

func albConvertFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "expected at least 1 argument but got %v\n", args)
		os.Exit(1)
	}
	if convertOutput == "" {
		fmt.Fprintln(os.Stderr, "output path is not specified")
		os.Exit(1)
	}

	err := alblog.ConvertToCSV(convertOutput, args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to convert %q (%v)\n", args, err)
		os.Exit(1)
	}
	fmt.Printf("Converted %q to %q\n", args, convertOutput)
}
