package alblog

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"

	"github.com/spf13/cobra"
)

func newMergeRaw() *cobra.Command {
	return &cobra.Command{
		Use:   "merge-raw [list of ALB access log files to merge]",
		Short: "Merge raw ALB access log files to one log file",
		Run:   mergeRawFunc,
	}
}

func mergeRawFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "expected at least 1 argument but got %v\n", args)
		os.Exit(1)
	}
	if output == "" {
		fmt.Fprintln(os.Stderr, "output path is not specified")
		os.Exit(1)
	}
	if fileutil.Exist(output) {
		fmt.Fprintf(os.Stderr, "%q already exists\n", output)
		os.Exit(1)
	}

	f, err := os.Create(output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create a file %q (%v)\n", output, err)
		os.Exit(1)
	}

	for _, p := range args {
		var d []byte
		d, err = ioutil.ReadFile(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read ALB access log %q (%v)\n", output, err)
			os.Exit(1)
		}
		_, err = f.Write(d)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to append ALB access log %q (%v)\n", output, err)
			os.Exit(1)
		}
	}
	f.Sync()
	f.Close()

	fmt.Printf("Merged %q to %q\n", args, output)
}
