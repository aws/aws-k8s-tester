package alblog

import (
	"fmt"
	"os"

	alblog "github.com/aws/aws-k8s-tester/internal/alb-log"

	"github.com/spf13/cobra"
)

func newCountTargets() *cobra.Command {
	return &cobra.Command{
		Use:   "count-targets [list of input access log files]",
		Short: "Counts targets from raw ALB access log files",
		Run:   countTargetsFunc,
	}
}

func countTargetsFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "expected at least 1 arguments but got %v\n", args)
		os.Exit(1)
	}

	targets, total := make(map[string]int), 0
	for _, p := range args {
		logs, err := alblog.Parse(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse %q (%v)\n", p, err)
			os.Exit(1)
		}
		for _, l := range logs {
			targets[l.TargetPort]++
		}
		total += len(logs)
	}
	fmt.Printf("Found %+v out of %d log entries\n", targets, total)
}
