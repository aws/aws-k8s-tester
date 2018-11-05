// Package etcdtester defines etcd test operations.
package etcdtester

import "go.etcd.io/etcd/etcdserver/etcdserverpb"

// Tester defines etcd specific operations.
type Tester interface {
	Deployer

	// ClusterStatus checks the cluster status with etcd 'Status' API.
	ClusterStatus() ClusterStatus
	// MemberList returns the member list from an etcd cluster.
	MemberList() (*etcdserverpb.MemberListResponse, error)

	// Put writes a key-value pair.
	Put(k, v string) error
}

// Deployer defines etcd deployer.
type Deployer interface {
	// Create starts the etcd cluster the very first time.
	Create() error
	// Cluster returns the cluster information with '/health' endpoints.
	Cluster() Cluster
	// Stop stops the specified node.
	Stop(id string) error
	// Restart restarts the node, with the specified etcd version.
	Restart(id, ver string) error
	// Terminate terminates the etcd cluster, deleting all provider resources (e.g. GCE, EC2, etc.).
	Terminate() error
	// MemberRemove removes a member from the cluster.
	MemberRemove(id string) error
	// MemberAdd adds a new member to the cluster.
	MemberAdd(ver string) error
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

// ClusterStatus is the cluster status.
type ClusterStatus struct {
	Members map[string]*etcdserverpb.StatusResponse `json:"members"`
}
