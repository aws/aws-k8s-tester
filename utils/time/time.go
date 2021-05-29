// Package time implements time utilities.
package time

import (
	"fmt"
	"time"
)

// GetTS returns the timestamp string with truncation by index.
func GetTS(idx int) string {
	now := time.Now()
	ts := fmt.Sprintf(
		"%04d%02d%02d%02d%02d",
		now.Year(),
		int(now.Month()),
		now.Day(),
		now.Hour(),
		now.Second(),
	)
	if idx < 0 {
		return ts
	}
	return ts[:idx]
}
