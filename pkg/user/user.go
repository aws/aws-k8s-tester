// Package user implements system user utilities.
package user

import (
	"fmt"
	"os"
	"os/user"
	"runtime"
)

// Get returns the current user name and ID.
func Get() string {
	u, err := user.Current()
	if err != nil {
		return fmt.Sprintf("user=%s", os.Getenv("USER"))
	}
	h, err := os.Hostname()
	if err != nil {
		h = os.Getenv("HOSTNAME")
	}
	// ref. https://docs.aws.amazon.com/general/latest/gr/aws_tagging.html#tag-conventions
	return fmt.Sprintf("name=%s_user=%s_home=%s_hostname=%s_os=%s_arch=%s",
		u.Name,
		u.Username,
		u.HomeDir,
		h,
		runtime.GOOS,
		runtime.GOARCH,
	)
}
