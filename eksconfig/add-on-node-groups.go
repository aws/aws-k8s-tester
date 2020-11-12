package eksconfig

import (
	"errors"
	"fmt"
	"path"
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

	// S3Dir is the S3 directory to store all test results.
	// It is under the bucket "eksconfig.Config.S3BucketName".
	S3Dir string `json:"s3-dir"`

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
	RoleCFNStackYAMLPath  string   `json:"role-cfn-stack-yaml-path" read-only:"true"`
	RoleCFNStackYAMLS3Key string   `json:"role-cfn-stack-yaml-s3-key" read-only:"true"`

	// NodeGroupSecurityGroupID is the security group ID for the node group.
	NodeGroupSecurityGroupID                string `json:"node-group-security-group-id" read-only:"true"`
	NodeGroupSecurityGroupCFNStackID        string `json:"node-group-security-group-cfn-stack-id" read-only:"true"`
	NodeGroupSecurityGroupCFNStackYAMLPath  string `json:"node-group-security-group-cfn-stack-yaml-path" read-only:"true"`
	NodeGroupSecurityGroupCFNStackYAMLS3Key string `json:"node-group-security-group-cfn-stack-yaml-s3-key" read-only:"true"`

	// LogsDir is set to specify the target directory to store all remote log files.
	// If empty, it stores in the same directory as "ConfigPath".
	LogsDir string `json:"logs-dir,omitempty"`
	// LogsTarGzPath is the .tar.gz archived file for "LogsDir".
	LogsTarGzPath string `json:"logs-tar-gz-path"`
	// ASGs maps from EKS Node Group name to "ASG".
	// "GetRef.Name" is the reserved key and NG name from eksconfig.Config.Name.
	ASGs map[string]ASG `json:"asgs,omitempty"`
}

// NGClusterAutoscaler represents cluster auto-scaler.
// ref. https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler
type NGClusterAutoscaler struct {
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
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

	// ClusterAutoscaler is enabled to run cluster auto-scaler per node group.
	// ref. https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler
	ClusterAutoscaler *NGClusterAutoscaler `json:"cluster-autoscaler,omitempty"`
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
		FetchLogs:  false,
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
					ImageIDSSMParameter:                "/aws/service/eks/optimized-ami/1.16/amazon-linux-2/recommended/image_id",
					InstanceTypes:                      []string{DefaultNodeInstanceTypeCPU},
					VolumeSize:                         DefaultNodeVolumeSize,
					ASGMinSize:                         1,
					ASGMaxSize:                         1,
					ASGDesiredCapacity:                 1,
				},
				KubeletExtraArgs:  "",
				ClusterAutoscaler: &NGClusterAutoscaler{Enable: false},
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

	if cfg.AddOnNodeGroups.S3Dir == "" {
		cfg.AddOnNodeGroups.S3Dir = path.Join(cfg.Name, "add-on-node-groups")
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

	if cfg.AddOnNodeGroups.NodeGroupSecurityGroupCFNStackYAMLPath == "" {
		cfg.AddOnNodeGroups.NodeGroupSecurityGroupCFNStackYAMLPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".add-on-node-groups.sg.cfn.yaml"
	}
	if cfg.AddOnNodeGroups.NodeGroupSecurityGroupCFNStackYAMLS3Key == "" {
		cfg.AddOnNodeGroups.NodeGroupSecurityGroupCFNStackYAMLS3Key = path.Join(
			cfg.AddOnNodeGroups.S3Dir,
			filepath.Base(cfg.AddOnNodeGroups.NodeGroupSecurityGroupCFNStackYAMLPath),
		)
	}

	if cfg.AddOnNodeGroups.RoleCFNStackYAMLPath == "" {
		cfg.AddOnNodeGroups.RoleCFNStackYAMLPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".add-on-node-groups.role.cfn.yaml"
	}
	if cfg.AddOnNodeGroups.RoleCFNStackYAMLS3Key == "" {
		cfg.AddOnNodeGroups.RoleCFNStackYAMLS3Key = path.Join(
			cfg.AddOnNodeGroups.S3Dir,
			filepath.Base(cfg.AddOnNodeGroups.RoleCFNStackYAMLPath),
		)
	}
	switch cfg.AddOnNodeGroups.RoleCreate {
	case true: // need create one, or already created
		if cfg.AddOnNodeGroups.RoleName == "" {
			cfg.AddOnNodeGroups.RoleName = cfg.Name + "-ng-role"
		}
		if cfg.AddOnNodeGroups.RoleARN != "" {
			// just ignore...
			// could be populated from previous run
			// do not error, so long as RoleCreate false, role won't be deleted
		}
		if len(cfg.AddOnNodeGroups.RoleServicePrincipals) > 0 {
			/*
				(InvalidParameterException: Following required service principals [ec2.amazonaws.com] were not found in the trust relationships of nodeRole arn:aws:iam::...:role/test-ng-role
				{
				  ClusterName: "test",
				  Message_: "Following required service principals [ec2.amazonaws.com] were not found in the trust relationships of nodeRole arn:aws:iam::...:role/test-ng-role",
				  NodegroupName: "test-ng-cpu"
				})
			*/
			found := false
			validSps := []string{"ec2.amazonaws.com", "ec2.amazonaws.com.cn", "ec2.c2s.ic.gov", "ec2.sc2s.sgov.gov"}
			for _, pv := range cfg.AddOnNodeGroups.RoleServicePrincipals {
				for _, vsp := range validSps {
					if pv == vsp {
						found = true
						break
					}
				}
			}
			if !found {
				return fmt.Errorf("AddOnNodeGroups.RoleServicePrincipals %q must include one of: %q", cfg.AddOnNodeGroups.RoleServicePrincipals, validSps)
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
		if cfg.IsEnabledAddOnStresserRemote() {
			return errors.New("'AddOnStresserRemote.Enable == true' requires 'AddOnNodeGroups.RoleCreate == true' but got 'false'")
		}
	}

	names, processed := make(map[string]struct{}), make(map[string]ASG)
	for k, cur := range cfg.AddOnNodeGroups.ASGs {
		k = strings.ReplaceAll(k, "GetRef.Name", cfg.Name)
		cur.Name = strings.ReplaceAll(cur.Name, "GetRef.Name", cfg.Name)
		if cur.Name == "" {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].Name is empty", k)
		}
		if k != cur.Name {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].Name has different Name field %q", k, cur.Name)
		}
		_, ok := names[cur.Name]
		if !ok {
			names[cur.Name] = struct{}{}
		} else {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].Name %q is redundant", k, cur.Name)
		}

		if cur.ASGCFNStackYAMLPath == "" {
			cur.ASGCFNStackYAMLPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".asg.cfn." + k + ".yaml"
		}
		if cur.ASGCFNStackYAMLS3Key == "" {
			cur.ASGCFNStackYAMLS3Key = path.Join(
				cfg.AddOnNodeGroups.S3Dir,
				filepath.Base(cur.ASGCFNStackYAMLPath),
			)
		}

		if len(cur.InstanceTypes) > 4 {
			return fmt.Errorf("too many InstaceTypes[%q]", cur.InstanceTypes)
		}
		if cur.VolumeSize == 0 {
			cur.VolumeSize = DefaultNodeVolumeSize
		}
		if cur.RemoteAccessUserName == "" {
			cur.RemoteAccessUserName = "ec2-user"
		}

		if cur.ImageID == "" && cur.ImageIDSSMParameter == "" {
			return fmt.Errorf("%q both ImageID and ImageIDSSMParameter are empty", cur.Name)
		}
		// prefer "ImageIDSSMParameter"
		if cur.ImageID != "" && cur.ImageIDSSMParameter != "" {
			cur.ImageID = ""
		}

		if !cfg.AddOnNodeGroups.RoleCreate {
			if cur.ClusterAutoscaler != nil && cur.ClusterAutoscaler.Enable {
				return fmt.Errorf("'ASGs[%q].ClusterAutoscaler.Enable == true' requires 'AddOnNodeGroups.RoleCreate == true' but got 'false'", cur.ASG.Name)
			}
		}

		switch cur.AMIType {
		case ec2config.AMITypeBottleRocketCPU:
			if cur.RemoteAccessUserName != "ec2-user" {
				return fmt.Errorf("AMIType %q but unexpected RemoteAccessUserName %q", cur.AMIType, cur.RemoteAccessUserName)
			}
			if cur.SSMDocumentName != "" && cfg.S3BucketName == "" {
				return fmt.Errorf("AMIType %q requires SSMDocumentName %q but no S3BucketName", cur.AMIType, cur.SSMDocumentName)
			}
			if cur.KubeletExtraArgs != "" {
				return fmt.Errorf("AMIType %q but unexpected KubeletExtraArgs %q", cur.AMIType, cur.KubeletExtraArgs)
			}
		case eks.AMITypesAl2X8664:
			if cur.RemoteAccessUserName != "ec2-user" {
				return fmt.Errorf("AMIType %q but unexpected RemoteAccessUserName %q", cur.AMIType, cur.RemoteAccessUserName)
			}
		case eks.AMITypesAl2X8664Gpu:
			if cur.RemoteAccessUserName != "ec2-user" {
				return fmt.Errorf("AMIType %q but unexpected RemoteAccessUserName %q", cur.AMIType, cur.RemoteAccessUserName)
			}
		default:
			return fmt.Errorf("unknown ASGs[%q].AMIType %q", k, cur.AMIType)
		}

		switch cur.AMIType {
		case ec2config.AMITypeBottleRocketCPU:
			if len(cur.InstanceTypes) == 0 {
				cur.InstanceTypes = []string{DefaultNodeInstanceTypeCPU}
			}
		case eks.AMITypesAl2X8664:
			if len(cur.InstanceTypes) == 0 {
				cur.InstanceTypes = []string{DefaultNodeInstanceTypeCPU}
			}
		case eks.AMITypesAl2X8664Gpu:
			if len(cur.InstanceTypes) == 0 {
				cur.InstanceTypes = []string{DefaultNodeInstanceTypeGPU}
			}
		default:
			return fmt.Errorf("unknown AddOnNodeGroups.ASGs[%q].AMIType %q", k, cur.AMIType)
		}

		if cfg.IsEnabledAddOnNLBHelloWorld() || cfg.IsEnabledAddOnALB2048() {
			// "m3.xlarge" or "c4.xlarge" will fail with "InvalidTarget: Targets {...} are not supported"
			// ref. https://github.com/aws/amazon-vpc-cni-k8s/pull/821
			// ref. https://github.com/kubernetes/kubernetes/issues/66044#issuecomment-408188524
			for _, ivt := range cur.InstanceTypes {

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

		if cur.ASGMinSize == 0 && cur.ASGDesiredCapacity == 0 {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].ASGMinSize/ASGDesiredCapacity must be >0", k)
		}
		if cur.ASGDesiredCapacity > 0 && cur.ASGMinSize == 0 {
			cur.ASGMinSize = cur.ASGDesiredCapacity
		}
		if cur.ASGDesiredCapacity > 0 && cur.ASGMaxSize == 0 {
			cur.ASGMaxSize = cur.ASGDesiredCapacity
		}

		if cur.ASGMinSize > cur.ASGMaxSize {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].ASGMinSize %d > ASGMaxSize %d", k, cur.ASGMinSize, cur.ASGMaxSize)
		}
		if cur.ASGDesiredCapacity > cur.ASGMaxSize {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].ASGDesiredCapacity %d > ASGMaxSize %d", k, cur.ASGDesiredCapacity, cur.ASGMaxSize)
		}
		if cur.ASGMaxSize > NGMaxLimit {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].ASGMaxSize %d > NGMaxLimit %d", k, cur.ASGMaxSize, NGMaxLimit)
		}
		if cur.ASGDesiredCapacity > NGMaxLimit {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].ASGDesiredCapacity %d > NGMaxLimit %d", k, cur.ASGDesiredCapacity, NGMaxLimit)
		}

		if cur.SSMDocumentCFNStackYAMLPath == "" {
			cur.SSMDocumentCFNStackYAMLPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".ssm.cfn." + k + ".yaml"
		}
		if cur.SSMDocumentCFNStackYAMLS3Key == "" {
			cur.SSMDocumentCFNStackYAMLS3Key = path.Join(
				cfg.AddOnNodeGroups.S3Dir,
				filepath.Base(cur.SSMDocumentCFNStackYAMLPath),
			)
		}
		switch cur.SSMDocumentCreate {
		case true: // need create one, or already created
			if cur.SSMDocumentCFNStackName == "" {
				cur.SSMDocumentCFNStackName = cur.Name + "-ssm-document"
			}
			if cur.SSMDocumentName == "" {
				cur.SSMDocumentName = cur.Name + "SSMDocument"
			}
			cur.SSMDocumentCFNStackName = strings.ReplaceAll(cur.SSMDocumentCFNStackName, "GetRef.Name", cfg.Name)
			cur.SSMDocumentName = strings.ReplaceAll(cur.SSMDocumentName, "GetRef.Name", cfg.Name)
			cur.SSMDocumentName = regex.ReplaceAllString(cur.SSMDocumentName, "")
			if cur.SSMDocumentExecutionTimeoutSeconds == 0 {
				cur.SSMDocumentExecutionTimeoutSeconds = 3600
			}

		case false: // use existing one, or don't run any SSM
		}

		if cfg.IsEnabledAddOnNLBHelloWorld() && cfg.AddOnNLBHelloWorld.DeploymentReplicas < int32(cur.ASGDesiredCapacity) {
			cfg.AddOnNLBHelloWorld.DeploymentReplicas = int32(cur.ASGDesiredCapacity)
		}
		if cfg.IsEnabledAddOnNLBGuestbook() && cfg.AddOnNLBGuestbook.DeploymentReplicas < int32(cur.ASGDesiredCapacity) {
			cfg.AddOnNLBGuestbook.DeploymentReplicas = int32(cur.ASGDesiredCapacity)
		}
		if cfg.IsEnabledAddOnALB2048() && cfg.AddOnALB2048.DeploymentReplicasALB < int32(cur.ASGDesiredCapacity) {
			cfg.AddOnALB2048.DeploymentReplicasALB = int32(cur.ASGDesiredCapacity)
		}
		if cfg.IsEnabledAddOnALB2048() && cfg.AddOnALB2048.DeploymentReplicas2048 < int32(cur.ASGDesiredCapacity) {
			cfg.AddOnALB2048.DeploymentReplicas2048 = int32(cur.ASGDesiredCapacity)
		}

		processed[k] = cur
	}

	cfg.AddOnNodeGroups.ASGs = processed
	return nil
}

func (addOn *AddOnNodeGroups) IsEnabledClusterAutoscaler() bool {
	if addOn == nil {
		return false
	}
	if len(addOn.ASGs) == 0 {
		return false
	}
	for _, cur := range addOn.ASGs {
		if cur.ClusterAutoscaler != nil && cur.ClusterAutoscaler.Enable {
			return true
		}
	}
	return false
}
