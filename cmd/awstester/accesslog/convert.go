package accesslog

import (
	"fmt"
	"os"

	"github.com/aws/awstester/internal/accesslog"

	"github.com/spf13/cobra"
)

func newALBConvert() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert [input access log file] [output CSV file]",
		Short: "Converts ALB access log to CSV",
		Run:   albConvertFunc,
	}
	return cmd
}

func albConvertFunc(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "expected 2 arguments but got %v\n", args)
		os.Exit(1)
	}
	p, output := args[0], args[1]
	err := accesslog.ConvertALBToCSV(p, output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to convert %q (%v)\n", p, err)
		os.Exit(1)
	}
	fmt.Printf("Converted %q to %q\n", p, output)
}
