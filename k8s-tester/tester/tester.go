// Package tester defines Kubernetes "tester client" interface without "cluster provisioner" dependency.
package tester

// Tester defines Kubernetes tester interface.
type Tester interface {
	// Name returns the name of the tester.
	Name() string
	// Apply installs the test case and also "validates".
	Apply() error
	// Delete removes all resources for the installed test case.
	Delete() error
}
