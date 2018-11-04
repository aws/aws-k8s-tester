// Package etcdtester defines etcd test operations.
package etcdtester

// Tester defines etcd specific operations.
type Tester interface {
	Deployer

	// Check checks the cluster status.
	Check() Cluster
}

// Deployer defines etcd deployer.
type Deployer interface {
	// Create starts the etcd cluster the very first time.
	Create() error
	// Cluster returns the cluster information.
	Cluster() Cluster
	// Stop stops the specified node.
	Stop(id string) error
	// Restart restarts the specified node.
	Restart(id string) error
	// Terminate terminates the etcd cluster, deleting all provider resources (e.g. GCE, EC2, etc.).
	Terminate() error
	// MemberAdd adds a new member to the cluster.
	MemberAdd(id string) error
	// MemberRemove removes a member from the cluster.
	MemberRemove(id string) error
}

// Cluster is the cluster state.
type Cluster struct {
	Members map[string]Member `json:"members"`
}

// Member is the member status information.
type Member struct {
	ID        string `json:"id"`
	ClientURL string `json:"client-url"`
	Status    string `json:"status"`
	OK        bool   `json:"ok"`
}
