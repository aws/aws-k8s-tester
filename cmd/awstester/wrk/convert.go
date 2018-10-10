package wrk

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aws/awstester/pkg/wrk"

	"github.com/spf13/cobra"
)

func newConvert() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert [wrk command output file to convert]",
		Short: "Convert wrk command output to a CSV file",
		Run:   convertFunc,
	}
	return cmd
}

func convertFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "expected 1 arguments but got %v\n", args)
		os.Exit(1)
	}
	if output == "" {
		fmt.Fprintln(os.Stderr, "output path is not specified")
		os.Exit(1)
	}

	p := args[0]
	d, err := ioutil.ReadFile(p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read wrk output %q (%v)\n", p, err)
		os.Exit(1)
	}
	op, err := wrk.Parse(string(d))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse wrk output %q (%v)\n", p, err)
		os.Exit(1)
	}

	if err = wrk.ToCSV(output, op); err != nil {
		fmt.Fprintf(os.Stderr, "failed to convert to CSV %q (%v)\n", p, err)
		os.Exit(1)
	}

	fmt.Printf("Converted %q to %q\n", p, output)
}
