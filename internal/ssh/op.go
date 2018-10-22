package ssh

// Op represents a SSH operation.
type Op struct {
	verbose bool
	envs    map[string]string
}

// OpOption configures archiver operations.
type OpOption func(*Op)

// WithVerbose configures verbose level in SSH operations.
func WithVerbose(b bool) OpOption {
	return func(op *Op) { op.verbose = b }
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
