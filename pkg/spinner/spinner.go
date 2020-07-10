// Package spinner implements spinner.
package spinner

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
)

// New returns a new spinner based on the time.
// If the local time is day, returns "🌍".
// If the local time is night, returns "🌓".
func New(suffix string, wr io.Writer) (s Spinner) {
	sets := spinner.CharSets[39]
	if time.Now().Hour() > 17 { // after business hours
		sets = spinner.CharSets[70]
	}
	if wr == nil {
		wr = os.Stderr
	}
	s = Spinner{wr: wr, Spinner: spinner.New(sets, 100*time.Millisecond, spinner.WithWriter(wr))}
	s.Suffix = " " + strings.TrimSpace(suffix)
	s.FinalMSG = "\n"
	return s
}

type Spinner struct {
	wr io.Writer
	*spinner.Spinner
}

func (s Spinner) Start() {
	fmt.Fprintf(s.wr, "\n")
	s.Spinner.Start()
}

func (s Spinner) Stop() {
	fmt.Fprintf(s.wr, "\n")
	s.Spinner.Stop()
}
