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
	return fmt.Sprintf("name=%s,user=%s,home=%s,hostname=%s,os=%s,arch=%s",
		u.Name,
		u.Username,
		u.HomeDir,
		h,
		runtime.GOOS,
		runtime.GOARCH,
	)
}
