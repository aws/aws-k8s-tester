package e2e

import (
	"context"
	"log"
	"testing"

	"sigs.k8s.io/e2e-framework/klient/wait"
)

// WaitForTimeoutMessage is a consistent message logged when wait.For times out.
const WaitForTimeoutMessage = "TIMEOUT: timed out waiting for test workload to complete"

// WaitForWithTimeout wraps wait.For and logs a consistent timeout message on context deadline exceeded.
func WaitForWithTimeout(t *testing.T, description string, fn func(ctx context.Context) (bool, error), opts ...wait.Option) error {
	err := wait.For(fn, opts...)
	if err != nil && IsTimeoutError(err) {
		log.Printf("%s: %s", WaitForTimeoutMessage, description)
	}
	return err
}

// IsTimeoutError checks if an error is a context deadline exceeded error.
func IsTimeoutError(err error) bool {
	return err != nil && err.Error() == context.DeadlineExceeded.Error()
}
