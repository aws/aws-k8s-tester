// Package alb implements ALB Ingress Controller plugin.
package alb

// Plugin defines ALB Ingress Controller deployer operations.
type Plugin interface {
	DeployBackend() error
	CreateRBAC() error
	DeployIngressController() error
	CreateSecurityGroup() error
	DeleteSecurityGroup() error
	CreateIngressObjects() error
	DeleteIngressObjects() error
}
