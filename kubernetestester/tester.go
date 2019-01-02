// Package kubernetestester defines Kubernetes test operations.
package kubernetestester

// Tester defines Kubernetes specific operations.
type Tester interface {
	Deployer
}

// Deployer defines Kubernetes storage deployer.
type Deployer interface {
	// Create starts the Kubernetes cluster the very first time.
	Create() error
	// Terminate terminates the Kubernetes cluster, deleting all provider resources (e.g. GCE, EC2, etc.).
	Terminate() error
	// KubeConfig returns the file path to KUBECONFIG.
	KubeConfig() (string, error)
}
