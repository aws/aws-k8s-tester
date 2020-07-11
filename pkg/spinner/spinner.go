// Package spinner implements spinner.
package spinner

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/terminal"
	"github.com/briandowns/spinner"
)

// New returns a new spinner based on the time.
// If the local time is day, returns "🌍".
// If the local time is night, returns "🌓".
func New(wr io.Writer, suffix string) (s Spinner) {
	sets := spinner.CharSets[39]
	if time.Now().Hour() > 17 { // after business hours
		sets = spinner.CharSets[70]
	}
	if wr == nil {
		wr = os.Stderr
	}
	s = Spinner{wr: wr, suffix: suffix}
	if _, err := terminal.IsColor(); err == nil {
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
