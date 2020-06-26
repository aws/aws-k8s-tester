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
