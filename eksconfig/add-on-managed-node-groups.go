package eksconfig

import (
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
)

// IsEnabledAddOnManagedNodeGroups returns true if "AddOnManagedNodeGroups" is enabled.
// Otherwise, nil the field for "omitempty".
func (cfg *Config) IsEnabledAddOnManagedNodeGroups() bool {
	if cfg.AddOnManagedNodeGroups == nil {
		return false
	}
	if cfg.AddOnManagedNodeGroups.Enable {
		return len(cfg.AddOnManagedNodeGroups.MNGs) > 0
	}
	cfg.AddOnManagedNodeGroups = nil
	return false
}

// AddOnManagedNodeGroups defines parameters for EKS "Managed Node Group" creation.
// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
type AddOnManagedNodeGroups struct {
	// Enable is true to auto-create a managed node group.
	Enable bool `json:"enable"`

	// Created is true when the resource has been created.
	// Used for delete operations.
	Created bool `json:"created" read-only:"true"`
	// CreateTook is the duration that took to create the resource.
	CreateTook time.Duration `json:"create-took,omitempty" read-only:"true"`
	// CreateTookString is the duration that took to create the resource.
	CreateTookString string `json:"create-took-string,omitempty" read-only:"true"`
	// DeleteTook is the duration that took to create the resource.
	DeleteTook time.Duration `json:"delete-took,omitempty" read-only:"true"`
	// DeleteTookString is the duration that took to create the resource.
	DeleteTookString string `json:"delete-took-string,omitempty" read-only:"true"`

	// FetchLogs is true to fetch logs from remote nodes using SSH.
	FetchLogs bool `json:"fetch-logs"`

	// RoleName is the name of the managed node group.
	RoleName string `json:"role-name"`
	// RoleCreate is true to auto-create and delete role.
	RoleCreate bool `json:"role-create"`
	// RoleARN is the role ARN that EKS managed node group uses to create AWS
	// resources for Kubernetes.
	// By default, it's empty which triggers tester to create one.
	RoleARN string `json:"role-arn"`
	// RoleServicePrincipals is the node group Service Principals
	RoleServicePrincipals []string `json:"role-service-principals"`
	// RoleManagedPolicyARNs is node group managed policy ARNs.
	RoleManagedPolicyARNs []string `json:"role-managed-policy-arns"`
	RoleCFNStackID        string   `json:"role-cfn-stack-id" read-only:"true"`

	// RequestHeaderKey defines EKS managed node group create cluster request header key.
	RequestHeaderKey string `json:"request-header-key,omitempty"`
	// RequestHeaderValue defines EKS managed node group create cluster request header value.
	RequestHeaderValue string `json:"request-header-value,omitempty"`
	// ResolverURL defines an AWS resolver endpoint for EKS API.
	// Must be left empty to use production EKS managed node group service.
	ResolverURL string `json:"resolver-url"`
	// SigningName is the EKS managed node group create request signing name.
	SigningName string `json:"signing-name"`

	// LogsDir is set to specify the target directory to store all remote log files.
	// If empty, it stores in the same directory as "ConfigPath".
	LogsDir string `json:"logs-dir,omitempty"`
	// MNGs maps from EKS Managed Node Group name to "MNG".
	// "GetRef.Name" is the reserved key and MNG name from eksconfig.Config.Name.
	MNGs map[string]MNG `json:"mngs,omitempty"`
}

// MNG represents parameters for one EKS "Managed Node Group".
type MNG struct {
	// Name is the name of the managed node group.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	Name string `json:"name,omitempty"`
	// ASGName is the ASG name from a created managed node group.
	ASGName string `json:"asg-name,omitempty" read-only:"true"`

	// RemoteAccessUserName is the user name for managed node group SSH access.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	RemoteAccessUserName string `json:"remote-access-user-name,omitempty"`
	// Tags defines EKS managed node group create tags.
	Tags map[string]string `json:"tags,omitempty"`
	// ReleaseVersion is the AMI version of the Amazon EKS-optimized AMI for the node group.
	// The version may differ from EKS "cluster" version.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/eks-linux-ami-versions.html
	ReleaseVersion string `json:"release-version,omitempty"`
	// AMIType is the AMI type for the node group.
	// Allowed values are AL2_x86_64 and AL2_x86_64_GPU.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	AMIType string `json:"ami-type,omitempty"`
	// ASGMinSize is the minimum size of Node Group Auto Scaling Group.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	ASGMinSize int `json:"asg-min-size,omitempty"`
	// ASGMaxSize is the maximum size of Node Group Auto Scaling Group.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	ASGMaxSize int `json:"asg-max-size,omitempty"`
	// ASGDesiredCapacity is is the desired capacity of Node Group ASG.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	ASGDesiredCapacity int `json:"asg-desired-capacity,omitempty"`
	// InstanceTypes is the EC2 instance types for the node instances.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	InstanceTypes []string `json:"instance-types,omitempty"`
	// VolumeSize is the node volume size.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	VolumeSize int `json:"volume-size,omitempty"`

	// CreateRequested is true if "CreateNodegroupRequest" has been sent.
	CreateRequested bool `json:"create-requested" read-only:"true"`

	// PhysicalID is the Physical ID for the created "AWS::EKS::Nodegroup".
	PhysicalID string `json:"physical-id" read-only:"true"`

	// CFNStackID is the CloudFormation stack ID for a managed node group.
	CFNStackID string `json:"cfn-stack-id" read-only:"true"`

	RemoteAccessSecurityGroupID                      string `json:"remote-access-security-group-id" read-only:"true"`
	RemoteAccessSecurityGroupIngressEgressCFNStackID string `json:"remote-access-security-group-ingress-egress-cfn-stack-id" read-only:"true"`

	// Status is the current status of EKS "Managed Node Group".
	Status string `json:"status" read-only:"true"`
	// Instances maps an instance ID to an EC2 instance object for the node group.
	Instances map[string]ec2config.Instance `json:"instances" read-only:"true"`
	// Logs maps each instance ID to a list of log file paths fetched via SSH access.
	Logs map[string][]string `json:"logs" read-only:"true"`
}
