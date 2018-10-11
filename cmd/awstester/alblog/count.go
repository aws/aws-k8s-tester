package alblog

import (
	"fmt"
	"os"

	"github.com/aws/awstester/internal/alblog"

	"github.com/spf13/cobra"
)

func newALBCount() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "count [input access log file]",
		Short: "Counts targets",
		Run:   albCountFunc,
	}
	return cmd
}

func albCountFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "expected 1 arguments but got %v\n", args)
		os.Exit(1)
	}
	p := args[0]
	logs, err := alblog.Parse(p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse %q (%v)\n", p, err)
		os.Exit(1)
	}
	targets := make(map[string]int)
	for _, l := range logs {
		targets[l.TargetPort]++
	}
	fmt.Printf("Found %+v out of %d log entries\n", targets, len(logs))
}
