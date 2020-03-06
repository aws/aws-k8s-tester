// Package version defines aws-k8s-tester version.
package version

import (
	"fmt"
	"time"
)

var (
	// GitCommit is the git commit on build.
	GitCommit = ""
	// ReleaseVersion is the release version.
	ReleaseVersion = ""
	// BuildTime is the build timestamp.
	BuildTime = ""
)

func init() {
	now := time.Now()
	if ReleaseVersion == "" {
		ReleaseVersion = fmt.Sprintf(
			"%d%02d%02d%02d%02d",
			now.Year(),
			int(now.Month()),
			now.Day(),
			now.Hour(),
			now.Minute(),
		)
	}
	if BuildTime == "" {
		BuildTime = now.String()
	}
}
