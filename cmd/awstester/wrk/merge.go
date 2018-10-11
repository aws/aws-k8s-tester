package wrk

import (
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aws/awstester/pkg/csvutil"
	"github.com/aws/awstester/pkg/wrk"

	"github.com/spf13/cobra"
)

func newMerge() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "merge [list of wrk command output files to merge]",
		Short: "Merge wrk command outputs into one CSV file",
		Run:   mergeFunc,
	}
	cmd.PersistentFlags().BoolVar(&mergeInputCSV, "input-csv", false, "'true' to merge CSV files")
	return cmd
}

var mergeInputCSV bool

func mergeFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "expected at least 1 argument, but got %v\n", args)
		os.Exit(1)
	}
	if output == "" {
		fmt.Fprintln(os.Stderr, "output path is not specified")
		os.Exit(1)
	}

	ps := make([]wrk.Result, 0)
	header := make([]string, 0)
	rows := make([][]string, 0)
	for _, p := range args {
		if !mergeInputCSV {
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
		} else {
			f, err := os.OpenFile(p, os.O_RDWR, 0600)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to open CSV %q (%v)\n", output, err)
				os.Exit(1)
			}
			defer f.Close()

			rd := csv.NewReader(f)
			rrs, rerr := rd.ReadAll()
			if rerr != nil {
				fmt.Fprintf(os.Stderr, "failed to read CSV %q (%v)\n", output, rerr)
				os.Exit(1)
			}
			if rrs[0][0] == "threads" && len(header) == 0 {
				header = rrs[0]
			}
			rows = append(rows, rrs[1])
		}
	}

	if !mergeInputCSV {
		if err := wrk.ToCSV(output, ps...); err != nil {
			fmt.Fprintf(os.Stderr, "failed to convert to CSV %q (%v)\n", output, err)
			os.Exit(1)
		}
	} else {
		if err := csvutil.Save(header, rows, output); err != nil {
			fmt.Fprintf(os.Stderr, "failed to save CSV %q (%v)\n", output, err)
			os.Exit(1)
		}
	}
}
