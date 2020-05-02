package eksconfig

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-sdk-go/service/eks"
)

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

// EnvironmentVariablePrefixAddOnManagedNodeGroups is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnManagedNodeGroups = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_MANAGED_NODE_GROUPS_"

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

func getDefaultAddOnManagedNodeGroups(name string) *AddOnManagedNodeGroups {
	return &AddOnManagedNodeGroups{
		Enable:      false,
		FetchLogs:   true,
		SigningName: "eks",
		RoleCreate:  true,
		LogsDir:     "", // to be auto-generated
		MNGs: map[string]MNG{
			name + "-mng-cpu": {
				Name:                 name + "-mng-cpu",
				RemoteAccessUserName: "ec2-user", // assume Amazon Linux 2
				ReleaseVersion:       "",         // to be auto-filled by EKS API
				AMIType:              eks.AMITypesAl2X8664,
				ASGMinSize:           1,
				ASGMaxSize:           1,
				ASGDesiredCapacity:   1,
				InstanceTypes:        []string{DefaultNodeInstanceTypeCPU},
				VolumeSize:           DefaultNodeVolumeSize,
			},
		},
	}
}

func (cfg *Config) validateAddOnManagedNodeGroups() error {
	if !cfg.IsEnabledAddOnManagedNodeGroups() {
		return nil
	}

	n := len(cfg.AddOnManagedNodeGroups.MNGs)
	if n == 0 {
		return errors.New("empty MNGs")
	}
	if n > MNGsMaxLimit {
		return fmt.Errorf("MNGs %d exceeds maximum number of MNGs which is %d", n, MNGsMaxLimit)
	}

	if cfg.Parameters.VersionValue < 1.14 {
		return fmt.Errorf("Version %q not supported for AddOnManagedNodeGroups", cfg.Parameters.Version)
	}

	if cfg.AddOnManagedNodeGroups.LogsDir == "" {
		cfg.AddOnManagedNodeGroups.LogsDir = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-logs-mngs")
	}

	switch cfg.AddOnManagedNodeGroups.RoleCreate {
	case true: // need create one, or already created
		if cfg.AddOnManagedNodeGroups.RoleName == "" {
			cfg.AddOnManagedNodeGroups.RoleName = cfg.Name + "-role-mng"
		}
		if cfg.AddOnManagedNodeGroups.RoleARN != "" {
			// just ignore...
			// could be populated from previous run
			// do not error, so long as RoleCreate false, role won't be deleted
		}
		if len(cfg.AddOnManagedNodeGroups.RoleServicePrincipals) > 0 {
			/*
				create node group request failed (InvalidParameterException: Following required service principals [ec2.amazonaws.com] were not found in the trust relationships of nodeRole arn:aws:iam::...:role/test-mng-role
				{
				  ClusterName: "test",
				  Message_: "Following required service principals [ec2.amazonaws.com] were not found in the trust relationships of nodeRole arn:aws:iam::...:role/test-mng-role",
				  NodegroupName: "test-mng-cpu"
				})
			*/
			found := false
			for _, pv := range cfg.AddOnManagedNodeGroups.RoleServicePrincipals {
				if pv == "ec2.amazonaws.com" { // TODO: support China regions ec2.amazonaws.com.cn or eks.amazonaws.com.cn
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("AddOnManagedNodeGroups.RoleServicePrincipals %q must include 'ec2.amazonaws.com'", cfg.AddOnManagedNodeGroups.RoleServicePrincipals)
			}
		}

	case false: // use existing one
		if cfg.AddOnManagedNodeGroups.RoleARN == "" {
			return fmt.Errorf("AddOnManagedNodeGroups.RoleCreate false; expect non-empty RoleARN but got %q", cfg.AddOnManagedNodeGroups.RoleARN)
		}
		if cfg.AddOnManagedNodeGroups.RoleName == "" {
			cfg.AddOnManagedNodeGroups.RoleName = getNameFromARN(cfg.AddOnManagedNodeGroups.RoleARN)
		}
		if len(cfg.AddOnManagedNodeGroups.RoleManagedPolicyARNs) > 0 {
			return fmt.Errorf("AddOnManagedNodeGroups.RoleCreate false; expect empty RoleManagedPolicyARNs but got %q", cfg.AddOnManagedNodeGroups.RoleManagedPolicyARNs)
		}
		if len(cfg.AddOnManagedNodeGroups.RoleServicePrincipals) > 0 {
			return fmt.Errorf("AddOnManagedNodeGroups.RoleCreate false; expect empty RoleServicePrincipals but got %q", cfg.AddOnManagedNodeGroups.RoleServicePrincipals)
		}
	}

	names, processed := make(map[string]struct{}), make(map[string]MNG)
	for k, v := range cfg.AddOnManagedNodeGroups.MNGs {
		k = strings.ReplaceAll(k, "GetRef.Name", cfg.Name)
		v.Name = strings.ReplaceAll(v.Name, "GetRef.Name", cfg.Name)

		if v.Name == "" {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].Name is empty", k)
		}
		if k != v.Name {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].Name has different Name field %q", k, v.Name)
		}
		_, ok := names[v.Name]
		if !ok {
			names[v.Name] = struct{}{}
		} else {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].Name %q is redundant", k, v.Name)
		}
		if cfg.IsEnabledAddOnNodeGroups() {
			_, ok = cfg.AddOnNodeGroups.ASGs[v.Name]
			if ok {
				return fmt.Errorf("MNGs[%q] name is conflicting with NG ASG", v.Name)
			}
		}

		if len(v.InstanceTypes) > 4 {
			return fmt.Errorf("too many InstaceTypes[%q]", v.InstanceTypes)
		}
		if v.VolumeSize == 0 {
			v.VolumeSize = DefaultNodeVolumeSize
		}
		if v.RemoteAccessUserName == "" {
			v.RemoteAccessUserName = "ec2-user"
		}

		if v.RemoteAccessUserName == "" {
			v.RemoteAccessUserName = "ec2-user"
		}

		switch v.AMIType {
		case eks.AMITypesAl2X8664:
			if v.RemoteAccessUserName != "ec2-user" {
				return fmt.Errorf("AMIType %q but unexpected RemoteAccessUserName %q", v.AMIType, v.RemoteAccessUserName)
			}
		case eks.AMITypesAl2X8664Gpu:
			if v.RemoteAccessUserName != "ec2-user" {
				return fmt.Errorf("AMIType %q but unexpected RemoteAccessUserName %q", v.AMIType, v.RemoteAccessUserName)
			}
		default:
			return fmt.Errorf("unknown ASGs[%q].AMIType %q", k, v.AMIType)
		}

		switch v.AMIType {
		case eks.AMITypesAl2X8664:
			if len(v.InstanceTypes) == 0 {
				v.InstanceTypes = []string{DefaultNodeInstanceTypeCPU}
			}
		case eks.AMITypesAl2X8664Gpu:
			if len(v.InstanceTypes) == 0 {
				v.InstanceTypes = []string{DefaultNodeInstanceTypeGPU}
			}
		default:
			return fmt.Errorf("unknown AddOnManagedNodeGroups.MNGs[%q].AMIType %q", k, v.AMIType)
		}

		if cfg.IsEnabledAddOnNLBHelloWorld() || cfg.IsEnabledAddOnALB2048() {
			for _, itp := range v.InstanceTypes {
				// "m3.xlarge" or "c4.xlarge" will fail with "InvalidTarget: Targets {...} are not supported"
				// ref. https://github.com/aws/amazon-vpc-cni-k8s/pull/821
				// ref. https://github.com/kubernetes/kubernetes/issues/66044#issuecomment-408188524
				switch {
				case strings.HasPrefix(itp, "m3."),
					strings.HasPrefix(itp, "c4."):
					return fmt.Errorf("AddOnNLBHelloWorld.Enable[%v] || AddOnALB2048.Enable[%v], but older instance type InstanceTypes %q for %q",
						cfg.IsEnabledAddOnNLBHelloWorld(),
						cfg.IsEnabledAddOnALB2048(),
						itp, k)
				default:
				}
			}
		}

		if v.ASGMinSize > v.ASGMaxSize {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].ASGMinSize %d > ASGMaxSize %d", k, v.ASGMinSize, v.ASGMaxSize)
		}
		if v.ASGDesiredCapacity > v.ASGMaxSize {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].ASGDesiredCapacity %d > ASGMaxSize %d", k, v.ASGDesiredCapacity, v.ASGMaxSize)
		}
		if v.ASGMaxSize > MNGMaxLimit {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].ASGMaxSize %d > MNGMaxLimit %d", k, v.ASGMaxSize, MNGMaxLimit)
		}
		if v.ASGDesiredCapacity > MNGMaxLimit {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].ASGDesiredCapacity %d > MNGMaxLimit %d", k, v.ASGDesiredCapacity, MNGMaxLimit)
		}

		if cfg.IsEnabledAddOnNLBHelloWorld() && cfg.AddOnNLBHelloWorld.DeploymentReplicas < int32(v.ASGDesiredCapacity) {
			cfg.AddOnNLBHelloWorld.DeploymentReplicas = int32(v.ASGDesiredCapacity)
		}
		if cfg.IsEnabledAddOnALB2048() && cfg.AddOnALB2048.DeploymentReplicasALB < int32(v.ASGDesiredCapacity) {
			cfg.AddOnALB2048.DeploymentReplicasALB = int32(v.ASGDesiredCapacity)
		}
		if cfg.IsEnabledAddOnALB2048() && cfg.AddOnALB2048.DeploymentReplicas2048 < int32(v.ASGDesiredCapacity) {
			cfg.AddOnALB2048.DeploymentReplicas2048 = int32(v.ASGDesiredCapacity)
		}

		processed[k] = v
	}

	cfg.AddOnManagedNodeGroups.MNGs = processed
	return nil
}
