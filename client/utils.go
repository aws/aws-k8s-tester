package client

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"k8s.io/utils/exec"
)

func int64Value(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// timeLeftTillDeadline returns the humanized string for time-left
// till deadline if there's any.
func timeLeftTillDeadline(ctx context.Context) string {
	if ctx.Err() != nil {
		return fmt.Sprintf("ctx error (%v)", ctx.Err())
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return "∞"
	}
	return deadline.UTC().Sub(time.Now().UTC()).String()
}

// durationTillDeadline returns the time.Duration left till deadline.
func durationTillDeadline(ctx context.Context) time.Duration {
	if ctx.Err() != nil {
		return time.Duration(0)
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return time.Hour
	}
	return deadline.UTC().Sub(time.Now().UTC())
}

// newSpinner returns a new spinner based on the time.
// If the local time is day, returns "🌍".
// If the local time is night, returns "🌓".
func newSpinner(wr io.Writer, suffix string) (s Spinner) {
	sets := spinner.CharSets[39]
	if time.Now().Hour() > 17 { // after business hours
		sets = spinner.CharSets[70]
	}
	if wr == nil {
		wr = os.Stderr
	}
	s = Spinner{wr: wr, suffix: suffix}
	if _, err := isColor(); err == nil {
		s.sp = spinner.New(sets, 500*time.Millisecond, spinner.WithWriter(wr))
		s.sp.Prefix = "🏊 🚣 ⛵ "
		s.sp.Suffix = "  ⚓ " + strings.TrimSpace(suffix)
		s.sp.FinalMSG = "\n"
	}
	return s
}

type Spinner struct {
	wr     io.Writer
	suffix string
	sp     *spinner.Spinner
}

func (s Spinner) Restart() {
	fmt.Fprintf(s.wr, "\n\n")
	if s.sp != nil {
		s.sp.Start()
	} else {
		fmt.Fprintf(s.wr, "🏊 🚣 ⛵  ⚓ "+s.suffix+"\n")
	}
}

func (s Spinner) Stop() {
	fmt.Fprintf(s.wr, "\n")
	if s.sp != nil {
		s.sp.Stop()
	}
}

// isColor returns an error if current terminal does not support color output.
func isColor() (string, error) {
	tputPath, err := exec.New().LookPath("tput")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(ctx, tputPath, "colors").CombinedOutput()
	cancel()
	out := strings.TrimSpace(string(output))
	if err != nil {
		return out, err
	}
	return out, nil
}
