package wrk

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aws/awstester/pkg/wrk"

	"github.com/spf13/cobra"
)

func newAppend() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "append [input wrk command output files...]",
		Short: "Append wrk command outputs into one CSV file",
		Run:   appendFunc,
	}
	return cmd
}

func appendFunc(cmd *cobra.Command, args []string) {
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

	if err := wrk.ToCSV(output, ps...); err != nil {
		fmt.Fprintf(os.Stderr, "failed to convert to CSV %q (%v)\n", output, err)
		os.Exit(1)
	}
}
