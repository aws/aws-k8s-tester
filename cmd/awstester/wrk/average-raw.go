package wrk

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aws/awstester/pkg/wrk"

	"github.com/spf13/cobra"
)

func newAverageRaw() *cobra.Command {
	return &cobra.Command{
		Use:   "average-raw [list of wrk command raw output files to get averages from]",
		Short: "Combine wrk command raw outputs and return average values to a CSV file",
		Run:   averageRawFunc,
	}
}

func averageRawFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "expected at least 1 argument, but got %v\n", args)
		os.Exit(1)
	}
	if output == "" {
		fmt.Fprintln(os.Stderr, "output path is not specified")
		os.Exit(1)
	}

	ps := make([]wrk.Result, 0)
	for _, p := range args {
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
		ps = append(ps, op)
	}
	rs := wrk.Combine(ps...)
	if err := wrk.ToCSV(output, rs); err != nil {
		fmt.Fprintf(os.Stderr, "failed to convert to CSV %q (%v)\n", output, err)
		os.Exit(1)
	}
}
