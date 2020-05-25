// Package timeutil implements time utilities.
package timeutil

import "time"

// TimeFrame records create/delete time frame.
type TimeFrame struct {
	// StartUTC is the time when resource is complete.
	StartUTC time.Time `json:"start-utc" read-only:"true"`
	// StartUTCRFC3339Nano is the timestamp in RFC3339 format with nano-second scale.
	// e.g. "2006-01-02T15:04:05.999999999Z07:00"
	StartUTCRFC3339Nano string `json:"start-utc-rfc3339-nano" read-only:"true"`
	// EndUTC is the time when resource is complete.
	EndUTC time.Time `json:"complete-utc" read-only:"true"`
	// EndUTCRFC3339Nano is the timestamp in RFC3339 format with nano-second scale.
	// e.g. "2006-01-02T15:04:05.999999999Z07:00"
	EndUTCRFC3339Nano string `json:"complete-utc-rfc3339-nano" read-only:"true"`
	// Took is the duration that took to create the resource.
	Took time.Duration `json:"took" read-only:"true"`
	// TookString is the duration that took to create the resource.
	TookString string `json:"took-string" read-only:"true"`
}

// NewTimeFrame returns a new TimeFrame.
func NewTimeFrame(start time.Time, end time.Time) TimeFrame {
	took := end.Sub(start)
	return TimeFrame{
		StartUTC:            start.UTC(),
		StartUTCRFC3339Nano: start.UTC().Format(time.RFC3339Nano),
		EndUTC:              end.UTC(),
		EndUTCRFC3339Nano:   end.UTC().Format(time.RFC3339Nano),
		Took:                took,
		TookString:          took.String(),
	}
}
