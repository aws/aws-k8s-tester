package eksconfig

import "github.com/aws/aws-k8s-tester/ec2config"

// IsEnabledAddOnNodeGroups returns true if "AddOnNodeGroups" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnNodeGroups() bool {
	if cfg.AddOnNodeGroups == nil {
		return false
	}
	if cfg.AddOnNodeGroups.Enable {
		return len(cfg.AddOnNodeGroups.ASGs) > 0
	}
	cfg.AddOnNodeGroups = nil
	return false
}

// AddOnNodeGroups defines parameters for EKS "Managed Node Group" creation.
// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
type AddOnNodeGroups struct {
	// Enable is true to auto-create ad node group.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created bool `json:"created" read-only:"true"`
	// FetchLogs is true to fetch logs from remote nodes using SSH.
	FetchLogs bool `json:"fetch-logs"`

	// RoleName is the name of thed node group.
	RoleName string `json:"role-name"`
	// RoleCreate is true to auto-create and delete role.
	RoleCreate bool `json:"role-create"`
	// RoleARN is the role ARN that EKS node group uses to create AWS
	// resources for Kubernetes.
	// By default, it's empty which triggers tester to create one.
	RoleARN string `json:"role-arn"`
	// RoleServicePrincipals is the node group Service Principals
	RoleServicePrincipals []string `json:"role-service-principals"`
	// RoleManagedPolicyARNs is node groupd policy ARNs.
	RoleManagedPolicyARNs []string `json:"role-managed-policy-arns"`
	RoleCFNStackID        string   `json:"role-cfn-stack-id" read-only:"true"`

	// NodeGroupSecurityGroupID is the security group ID for the node group.
	NodeGroupSecurityGroupID         string `json:"node-group-security-group-id" read-only:"true"`
	NodeGroupSecurityGroupCFNStackID string `json:"node-group-security-group-cfn-stack-id" read-only:"true"`

	// LogsDir is set to specify the target directory to store all remote log files.
	// If empty, it stores in the same directory as "ConfigPath".
	LogsDir string `json:"logs-dir,omitempty"`
	// ASGs maps from EKS Node Group name to "ASG".
	ASGs map[string]ASG `json:"asgs,omitempty"`
}

// ASG represents an EKS Node Group ASG.
type ASG struct {
	ec2config.ASG

	// KubeletExtraArgs represents "--kubelet-extra-args".
	// e.g. '--kubelet-extra-args --node-labels=nodesgroup=main,subnets=private'
	// e.g. '--kubelet-extra-args --hostname-override=string'
	// ref. https://github.com/awslabs/amazon-eks-ami/blob/master/files/bootstrap.sh
	//
	// TODO: handle conflicting flag '--cloud-provider aws'
	// ref. https://github.com/kubernetes/kubernetes/issues/64659
	KubeletExtraArgs string `json:"kubelet-extra-args"`
}
