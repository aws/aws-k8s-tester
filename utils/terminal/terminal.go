// Package terminal implements terminal related utilities.
package terminal

import (
	"context"
	"strings"
	"time"

	"k8s.io/utils/exec"
)

// IsColor returns an error if current terminal does not support color output.
func IsColor() (string, error) {
	tputPath, err := exec.New().LookPath("tput")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(ctx, tputPath, "colors").CombinedOutput()
	cancel()
	out := strings.TrimSpace(string(output))
	if err != nil {
		return out, err
	}
	return out, nil
}
