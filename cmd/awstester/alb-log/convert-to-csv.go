package alblog

import (
	"fmt"
	"os"

	alblog "github.com/aws/awstester/internal/alb-log"

	"github.com/spf13/cobra"
)

func newConvertToCSV() *cobra.Command {
	return &cobra.Command{
		Use:   "convert-to-csv [list of ALB access log files to convert]",
		Short: "Converts raw ALB access log files to one CSV file",
		Run:   convertToCSVFunc,
	}
}

func convertToCSVFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "expected at least 1 argument but got %v\n", args)
		os.Exit(1)
	}
	if output == "" {
		fmt.Fprintln(os.Stderr, "output path is not specified")
		os.Exit(1)
	}

	err := alblog.ConvertToCSV(output, args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to convert %q (%v)\n", args, err)
		os.Exit(1)
	}
	fmt.Printf("Converted %q to %q\n", args, output)
}
