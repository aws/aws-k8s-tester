// Package ec2deployer defines EC2 deployer.
package ec2deployer

// Interface defines deployer.
type Interface interface {
	Create() error
	Stop()
	Delete() error

	SSHCommands() string
}
