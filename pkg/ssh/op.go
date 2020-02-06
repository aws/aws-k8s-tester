package ssh

import (
	"time"
)

// Op represents a SSH operation.
type Op struct {
	verbose       bool
	retries       int
	retryInterval time.Duration
	timeout       time.Duration
	envs          map[string]string
}

// OpOption configures archiver operations.
type OpOption func(*Op)

// WithVerbose configures verbose level in SSH operations.
func WithVerbose(b bool) OpOption {
	return func(op *Op) { op.verbose = b }
}

// WithRetry automatically retries the command on closed TCP connection error.
// (e.g. retry immutable operation).
// WithRetry(-1) to retry forever until success.
func WithRetry(retries int, interval time.Duration) OpOption {
	return func(op *Op) {
		op.retries = retries
		op.retryInterval = interval
	}
}

// WithTimeout configures timeout for command run.
func WithTimeout(timeout time.Duration) OpOption {
	return func(op *Op) { op.timeout = timeout }
}

// WithEnv adds an environment variable that will be applied to any
// command executed by Shell or Run. It overwrites the ones set by
// "*ssh.Session.Setenv".
func WithEnv(k, v string) OpOption {
	return func(op *Op) { op.envs[k] = v }
}

func (op *Op) applyOpts(opts []OpOption) {
	for _, opt := range opts {
		opt(op)
	}
}
