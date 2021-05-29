// Package version defines k8s-tester version.
package version

import (
	"encoding/json"
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

type version struct {
	GitCommit      string `json:"git-commit"`
	ReleaseVersion string `json:"release-version"`
	BuildTime      string `json:"build-time"`
}

// Version returns the version string.
func Version() string {
	vv := version{
		GitCommit:      GitCommit,
		ReleaseVersion: ReleaseVersion,
		BuildTime:      BuildTime,
	}
	b, err := json.Marshal(vv)
	if err != nil {
		panic(err)
	}
	return string(b)
}
