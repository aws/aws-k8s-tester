// Package ctxutil implements context utilities.
package ctxutil

import (
	"context"
	"fmt"
	"time"
)

// TimeLeftTillDeadline returns the humanized string for time-left
// till deadline if there's any.
func TimeLeftTillDeadline(ctx context.Context) string {
	if ctx.Err() != nil {
		return fmt.Sprintf("ctx error (%v)", ctx.Err())
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return "∞"
	}
	return deadline.UTC().Sub(time.Now().UTC()).String()
}

// DurationTillDeadline returns the time.Duration left till deadline.
func DurationTillDeadline(ctx context.Context) time.Duration {
	if ctx.Err() != nil {
		return time.Duration(0)
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return time.Hour
	}
	return deadline.UTC().Sub(time.Now().UTC())
}
