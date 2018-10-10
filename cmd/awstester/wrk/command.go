// Package wrk implements wrk related utilities.
package wrk

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/awstester/pkg/wrk"

	"github.com/spf13/cobra"
)

func init() {
	cobra.EnablePrefixMatching = true
}

// NewCommand implements "awstest wrk" command.
func NewCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "wrk",
		Short: "wrk commands",
		Run:   runFunc,
	}
	rootCmd.AddCommand(
		newAvg(),
		newAppend(),
		newConvert(),
	)
	rootCmd.PersistentFlags().StringVar(&output, "output", "", "file path to store output")
	rootCmd.PersistentFlags().IntVar(&startMinute, "start-minute", 0, "minute to start the command (temporary dumb feature to be removed after batch integration...)")
	rootCmd.PersistentFlags().IntVar(&wrkCfg.Threads, "threads", 2, "number of threads")
	rootCmd.PersistentFlags().IntVar(&wrkCfg.Connections, "connections", 200, "number of connections")
	rootCmd.PersistentFlags().DurationVar(&wrkCfg.Duration, "duration", 15*time.Second, "duration to run 'wrk' command")
	return rootCmd
}

var (
	output      string
	startMinute int
	wrkCfg      wrk.Config
)

func runFunc(cmd *cobra.Command, args []string) {
	if output == "" {
		fmt.Fprintln(os.Stderr, "output path is not specified")
		os.Exit(1)
	}

	if startMinute != 0 {
		for time.Now().Minute() == startMinute {
			fmt.Fprintf(os.Stderr, "wait until start minute matches target %d != current %d", startMinute, time.Now().Minute())
			time.Sleep(5 * time.Second)
		}
	}

	rs, err := wrk.Run(wrkCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to run wrk (%v)\n", err)
		os.Exit(1)
	}

	if err := wrk.ToCSV(output, rs); err != nil {
		fmt.Fprintf(os.Stderr, "failed to convert to CSV %q (%v)\n", output, err)
		os.Exit(1)
	}
}
