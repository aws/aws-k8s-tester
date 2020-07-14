// Package user implements system user utilities.
package user

import (
	"fmt"
	"os"
	"os/user"
	"runtime"
)

// Get returns the current user name and ID.
func Get() (uv string) {
	u, err := user.Current()
	if err != nil {
		return fmt.Sprintf("user=%s", os.Getenv("USER"))
	}

	homeDir := u.HomeDir
	if len(homeDir) > 65 {
		homeDir = "..." + homeDir[len(homeDir)-65:]
	}

	h, err := os.Hostname()
	if err != nil {
		h = os.Getenv("HOSTNAME")
	}
	if len(h) > 65 {
		h = "..." + h[len(h)-65:]
	}

	// ref. https://docs.aws.amazon.com/general/latest/gr/aws_tagging.html#tag-conventions
	uv = fmt.Sprintf("name=%s_user=%s_home=%s_hostname=%s_os=%s_arch=%s",
		u.Name,
		u.Username,
		homeDir,
		h,
		runtime.GOOS,
		runtime.GOARCH,
	)
	if len(uv) > 253 {
		uv = uv[:250] + "..."
	}
	return uv
}
