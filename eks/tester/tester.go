// Package tester defines EKS tester interface.
package tester

// Tester defines tester.
type Tester interface {
	// Name returns the name of the tester.
	Name() string
	// Create creates test objects, and waits for completion.
	Create() error
	// Delete deletes all test objects.
	Delete() error
}

// Addon is a new interface similar to tester.
// Instead of Name() reflection is used for object names
// IsEnabled() allows for generic addon skipping.
type Addon interface {
	// Apply idempotently creates test objects
	Apply() error
	// Delete idempotently deletes test objects
	Delete() error
	// IsEnabled automatically skips the addon if false
	IsEnabled() bool
}
