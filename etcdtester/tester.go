// Package etcdtester defines etcd test operations.
package etcdtester

// Tester defines etcd specific operations.
type Tester interface {
	Deployer

	// CheckHealth checks the health endpoints.
	CheckHealth() map[string]Health
}

// Health is the health status of an etcd node.
type Health struct {
	Status string
	Error  error
}

// Deployer defines etcd deployer.
type Deployer interface {
	// Deploy starts the etcd cluster the very first time.
	Deploy() error
	// IDToClientURL returns the map from ID to client URL.
	IDToClientURL() map[string]string
	// Stop stops the specified node.
	Stop(id string) error
	// Restart restarts the specified node.
	Restart(id string) error
	// Terminate terminates the etcd cluster, deleting all provider resources (e.g. EC2).
	Terminate() error
	// MemberAdd adds a new member to the cluster.
	MemberAdd(id string) error
	// MemberRemove removes a member from the cluster.
	MemberRemove(id string) error
}
