package client

import (
	core_v1 "k8s.io/api/core/v1"
)

// Op represents a Kubernetes client operation.
type Op struct {
	labelSelector   string
	fieldSelector   string
	queryFunc       func()
	podFunc         func(core_v1.Pod)
	forceDelete     bool
	forceDeleteFunc func()
}

// OpOption configures Kubernetes client operations.
type OpOption func(*Op)

// WithLabelSelector configures label selector for list operations.
func WithLabelSelector(s string) OpOption {
	return func(op *Op) { op.labelSelector = s }
}

// WithFieldSelector configures field selector for list operations.
func WithFieldSelector(s string) OpOption {
	return func(op *Op) { op.fieldSelector = s }
}

// WithQueryFunc configures query function to be called in retry func.
func WithQueryFunc(f func()) OpOption {
	return func(op *Op) { op.queryFunc = f }
}

// WithPodFunc configures function to be called for pod.
func WithPodFunc(f func(core_v1.Pod)) OpOption {
	return func(op *Op) { op.podFunc = f }
}

// WithForceDelete configures force delete.
// Useful for namespace deletion.
// ref. https://github.com/kubernetes/kubernetes/issues/60807
func WithForceDelete(forceDelete bool) OpOption {
	return func(op *Op) { op.forceDelete = forceDelete }
}

// WithForceDeleteFunc configures force delete.
// Useful for namespace deletion.
// ref. https://github.com/kubernetes/kubernetes/issues/60807
func WithForceDeleteFunc(forceDeleteFunc func()) OpOption {
	return func(op *Op) { op.forceDeleteFunc = forceDeleteFunc }
}

func (op *Op) applyOpts(opts []OpOption) {
	for _, opt := range opts {
		opt(op)
	}
}
