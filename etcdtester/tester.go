// Package etcdtester defines etcd test operations.
package etcdtester

import (
	"github.com/aws/aws-k8s-tester/eksconfig"
)

// Tester defines aws-k8s-tester eks specific operations.
type Tester interface {
	Deployer
}

// Deployer defines etcd deployer.
type Deployer interface {
	Up() error
	IsUp() error
	Down() error
	Stop()
	// LoadConfig reloads configuration from disk to read the latest
	// cluster configuration and its states.
	// It's either reloaded from disk or returned from embedded EKS deployer.
	LoadConfig() (eksconfig.Config, error)
}

// TODO: test logs
// TODO: get system logs
