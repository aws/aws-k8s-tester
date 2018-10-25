package wrk

import (
	"encoding/csv"
	"fmt"
	"os"

	"github.com/aws/awstester/pkg/csvutil"

	"github.com/spf13/cobra"
)

func newMergeCSV() *cobra.Command {
	return &cobra.Command{
		Use:   "merge-csv [list of wrk command CSV output files to merge]",
		Short: "Merge wrk command CSV outputs into one CSV file",
		Run:   mergeCSVFunc,
	}
}

func mergeCSVFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "expected at least 1 argument, but got %v\n", args)
		os.Exit(1)
	}
	if output == "" {
		fmt.Fprintln(os.Stderr, "output path is not specified")
		os.Exit(1)
	}

	header := make([]string, 0)
	table := make([][]string, 0)
	for _, p := range args {
		f, err := os.OpenFile(p, os.O_RDWR, 0600)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open CSV %q (%v)\n", output, err)
			os.Exit(1)
		}
		defer f.Close()

		rd := csv.NewReader(f)
		rows, rerr := rd.ReadAll()
		if rerr != nil {
			fmt.Fprintf(os.Stderr, "failed to read CSV %q (%v)\n", output, rerr)
			os.Exit(1)
		}
		if rows[0][0] == "threads" && len(header) == 0 {
			header = rows[0]
		}
		table = append(table, rows[1])
	}

	if err := csvutil.Save(header, table, output); err != nil {
		fmt.Fprintf(os.Stderr, "failed to save CSV %q (%v)\n", output, err)
		os.Exit(1)
	}
	fmt.Printf("merged %q to %q\n", args, output)
}
