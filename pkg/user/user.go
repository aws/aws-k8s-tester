// Package user implements system user utilities.
package user

import (
	"fmt"
	"os"
	"os/user"
)

// Get returns the current user name and ID.
func Get() string {
	u, err := user.Current()
	if err != nil {
		return os.Getenv("USER")
	}
	h, err := os.Hostname()
	if err != nil {
		h = os.Getenv("HOSTNAME")
	}
	return fmt.Sprintf("name=%s,user=%s,home=%s,hostname=%s", u.Name, u.Username, u.HomeDir, h)
}
