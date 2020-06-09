package eksconfig

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/service/eks"
)

// AddOnNodeGroups defines parameters for EKS "Managed Node Group" creation.
// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
type AddOnNodeGroups struct {
	// Enable is true to auto-create ad node group.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

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
	// LogsTarGzPath is the .tar.gz archived file for "LogsDir".
	LogsTarGzPath string `json:"logs-tar-gz-path"`
	// ASGs maps from EKS Node Group name to "ASG".
	// "GetRef.Name" is the reserved key and NG name from eksconfig.Config.Name.
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

// EnvironmentVariablePrefixAddOnNodeGroups is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixAddOnNodeGroups = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_NODE_GROUPS_"

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

func getDefaultAddOnNodeGroups(name string) *AddOnNodeGroups {
	return &AddOnNodeGroups{
		Enable:     false,
		FetchLogs:  true,
		RoleCreate: true,
		LogsDir:    "", // to be auto-generated
		ASGs: map[string]ASG{
			name + "-ng-asg-cpu": {
				ASG: ec2config.ASG{
					Name:                               name + "-ng-asg-cpu",
					SSMDocumentCreate:                  false,
					SSMDocumentName:                    "",
					SSMDocumentCommands:                "",
					SSMDocumentExecutionTimeoutSeconds: 3600,
					RemoteAccessUserName:               "ec2-user", // assume Amazon Linux 2
					AMIType:                            eks.AMITypesAl2X8664,
					ImageID:                            "",
					ImageIDSSMParameter:                "/aws/service/eks/optimized-ami/1.17/amazon-linux-2/recommended/image_id",
					InstanceTypes:                      []string{DefaultNodeInstanceTypeCPU},
					VolumeSize:                         DefaultNodeVolumeSize,
					ASGMinSize:                         1,
					ASGMaxSize:                         1,
					ASGDesiredCapacity:                 1,
				},
				KubeletExtraArgs: "",
			},
		},
	}
}

func (cfg *Config) validateAddOnNodeGroups() error {
	if !cfg.IsEnabledAddOnNodeGroups() {
		return nil
	}

	n := len(cfg.AddOnNodeGroups.ASGs)
	if n == 0 {
		return errors.New("empty ASGs")
	}
	if n > NGsMaxLimit {
		return fmt.Errorf("NGs %d exceeds maximum number of NGs which is %d", n, NGsMaxLimit)
	}

	if cfg.Parameters.VersionValue < 1.14 {
		return fmt.Errorf("Version %q not supported for AddOnNodeGroups", cfg.Parameters.Version)
	}

	if cfg.AddOnNodeGroups.LogsDir == "" {
		cfg.AddOnNodeGroups.LogsDir = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-logs-ngs")
	}
	if cfg.AddOnNodeGroups.LogsTarGzPath == "" {
		cfg.AddOnNodeGroups.LogsTarGzPath = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-logs-ngs.tar.gz")
	}
	if !strings.HasSuffix(cfg.AddOnNodeGroups.LogsTarGzPath, ".tar.gz") {
		return fmt.Errorf("AddOnNodeGroups.LogsTarGzPath %q must end with .tar.gz", cfg.AddOnNodeGroups.LogsTarGzPath)
	}

	switch cfg.AddOnNodeGroups.RoleCreate {
	case true: // need create one, or already created
		if cfg.AddOnNodeGroups.RoleName == "" {
			cfg.AddOnNodeGroups.RoleName = cfg.Name + "-role-ng"
		}
		if cfg.AddOnNodeGroups.RoleARN != "" {
			// just ignore...
			// could be populated from previous run
			// do not error, so long as RoleCreate false, role won't be deleted
		}
		if len(cfg.AddOnNodeGroups.RoleServicePrincipals) > 0 {
			/*
				create node group request failed (InvalidParameterException: Following required service principals [ec2.amazonaws.com] were not found in the trust relationships of nodeRole arn:aws:iam::...:role/test-ng-role
				{
				  ClusterName: "test",
				  Message_: "Following required service principals [ec2.amazonaws.com] were not found in the trust relationships of nodeRole arn:aws:iam::...:role/test-ng-role",
				  NodegroupName: "test-ng-cpu"
				})
			*/
			found := false
			for _, pv := range cfg.AddOnNodeGroups.RoleServicePrincipals {
				if pv == "ec2.amazonaws.com" { // TODO: support China regions ec2.amazonaws.com.cn or eks.amazonaws.com.cn
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("AddOnNodeGroups.RoleServicePrincipals %q must include 'ec2.amazonaws.com'", cfg.AddOnNodeGroups.RoleServicePrincipals)
			}
		}

	case false: // use existing one
		if cfg.AddOnNodeGroups.RoleARN == "" {
			return fmt.Errorf("AddOnNodeGroups.RoleCreate false; expect non-empty RoleARN but got %q", cfg.AddOnNodeGroups.RoleARN)
		}
		if cfg.AddOnNodeGroups.RoleName == "" {
			cfg.AddOnNodeGroups.RoleName = getNameFromARN(cfg.AddOnNodeGroups.RoleARN)
		}
		if len(cfg.AddOnNodeGroups.RoleManagedPolicyARNs) > 0 {
			return fmt.Errorf("AddOnNodeGroups.RoleCreate false; expect empty RoleManagedPolicyARNs but got %q", cfg.AddOnNodeGroups.RoleManagedPolicyARNs)
		}
		if len(cfg.AddOnNodeGroups.RoleServicePrincipals) > 0 {
			return fmt.Errorf("AddOnNodeGroups.RoleCreate false; expect empty RoleServicePrincipals but got %q", cfg.AddOnNodeGroups.RoleServicePrincipals)
		}
	}

	names, processed := make(map[string]struct{}), make(map[string]ASG)
	for k, v := range cfg.AddOnNodeGroups.ASGs {
		k = strings.ReplaceAll(k, "GetRef.Name", cfg.Name)
		v.Name = strings.ReplaceAll(v.Name, "GetRef.Name", cfg.Name)

		if v.Name == "" {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].Name is empty", k)
		}
		if k != v.Name {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].Name has different Name field %q", k, v.Name)
		}
		_, ok := names[v.Name]
		if !ok {
			names[v.Name] = struct{}{}
		} else {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].Name %q is redundant", k, v.Name)
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

		if v.ImageID == "" && v.ImageIDSSMParameter == "" {
			return fmt.Errorf("%q both ImageID and ImageIDSSMParameter are empty", v.Name)
		}

		switch v.AMIType {
		case ec2config.AMITypeBottleRocketCPU:
			if v.RemoteAccessUserName != "ec2-user" {
				return fmt.Errorf("AMIType %q but unexpected RemoteAccessUserName %q", v.AMIType, v.RemoteAccessUserName)
			}
			if v.SSMDocumentName != "" && cfg.S3BucketName == "" {
				return fmt.Errorf("AMIType %q requires SSMDocumentName %q but no S3BucketName", v.AMIType, v.SSMDocumentName)
			}
			if v.KubeletExtraArgs != "" {
				return fmt.Errorf("AMIType %q but unexpected KubeletExtraArgs %q", v.AMIType, v.KubeletExtraArgs)
			}
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
		case ec2config.AMITypeBottleRocketCPU:
			if len(v.InstanceTypes) == 0 {
				v.InstanceTypes = []string{DefaultNodeInstanceTypeCPU}
			}
		case eks.AMITypesAl2X8664:
			if len(v.InstanceTypes) == 0 {
				v.InstanceTypes = []string{DefaultNodeInstanceTypeCPU}
			}
		case eks.AMITypesAl2X8664Gpu:
			if len(v.InstanceTypes) == 0 {
				v.InstanceTypes = []string{DefaultNodeInstanceTypeGPU}
			}
		default:
			return fmt.Errorf("unknown AddOnNodeGroups.ASGs[%q].AMIType %q", k, v.AMIType)
		}

		if cfg.IsEnabledAddOnNLBHelloWorld() || cfg.IsEnabledAddOnALB2048() {
			// "m3.xlarge" or "c4.xlarge" will fail with "InvalidTarget: Targets {...} are not supported"
			// ref. https://github.com/aws/amazon-vpc-cni-k8s/pull/821
			// ref. https://github.com/kubernetes/kubernetes/issues/66044#issuecomment-408188524
			for _, ivt := range v.InstanceTypes {

				switch {
				case strings.HasPrefix(ivt, "m3."),
					strings.HasPrefix(ivt, "c4."):
					return fmt.Errorf("AddOnNLBHelloWorld.Enable[%v] || AddOnALB2048.Enable[%v], but older instance type InstanceType %q for %q",
						cfg.IsEnabledAddOnNLBHelloWorld(),
						cfg.IsEnabledAddOnALB2048(),
						ivt, k)
				}
			}
		}

		if v.ASGMinSize > v.ASGMaxSize {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].ASGMinSize %d > ASGMaxSize %d", k, v.ASGMinSize, v.ASGMaxSize)
		}
		if v.ASGDesiredCapacity > v.ASGMaxSize {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].ASGDesiredCapacity %d > ASGMaxSize %d", k, v.ASGDesiredCapacity, v.ASGMaxSize)
		}
		if v.ASGMaxSize > NGMaxLimit {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].ASGMaxSize %d > NGMaxLimit %d", k, v.ASGMaxSize, NGMaxLimit)
		}
		if v.ASGDesiredCapacity > NGMaxLimit {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].ASGDesiredCapacity %d > NGMaxLimit %d", k, v.ASGDesiredCapacity, NGMaxLimit)
		}

		switch v.SSMDocumentCreate {
		case true: // need create one, or already created
			if v.SSMDocumentCFNStackName == "" {
				v.SSMDocumentCFNStackName = v.Name + "-ssm-document"
			}
			if v.SSMDocumentName == "" {
				v.SSMDocumentName = v.Name + "SSMDocument"
			}
			v.SSMDocumentCFNStackName = strings.ReplaceAll(v.SSMDocumentCFNStackName, "GetRef.Name", cfg.Name)
			v.SSMDocumentName = strings.ReplaceAll(v.SSMDocumentName, "GetRef.Name", cfg.Name)
			v.SSMDocumentName = regex.ReplaceAllString(v.SSMDocumentName, "")
			if v.SSMDocumentExecutionTimeoutSeconds == 0 {
				v.SSMDocumentExecutionTimeoutSeconds = 3600
			}

		case false: // use existing one, or don't run any SSM
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

	cfg.AddOnNodeGroups.ASGs = processed
	return nil
}
