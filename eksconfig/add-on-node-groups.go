package eksconfig

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	aws_eks_v2_types "github.com/aws/aws-sdk-go-v2/service/eks/types"
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

	Role *Role `json:"role"`

	// FetchLogs is true to fetch logs from remote nodes using SSH.
	FetchLogs bool `json:"fetch-logs"`

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

	// BootstrapArgs additional bootstrap arguments.
	// e.g. '--pause-container-account 012345678901 --pause-container-version 3.3'
	BootstrapArgs string `json:"bootstrap-args"`

	// ClusterAutoscaler is enabled to run cluster auto-scaler per node group.
	// ref. https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler
	ClusterAutoscaler *NGClusterAutoscaler `json:"cluster-autoscaler,omitempty"`
}

const (
	// AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_PREFIX is the environment variable prefix used for "eksconfig".
	AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_PREFIX      = AWS_K8S_TESTER_EKS_PREFIX + "ADD_ON_NODE_GROUPS_"
	AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_PREFIX = AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_PREFIX + "ROLE_"
)

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
		Enable:    false,
		Role:      getDefaultRole(),
		FetchLogs: false,
		LogsDir:   "", // to be auto-generated
		ASGs: map[string]ASG{
			name + "-ng-asg-cpu": {
				ASG: ec2config.ASG{
					Name: name + "-ng-asg-cpu",
					SSM: &ec2config.SSM{
						DocumentCreate:                  false,
						DocumentName:                    "",
						DocumentCommands:                "",
						DocumentExecutionTimeoutSeconds: 3600,
					},
					RemoteAccessUserName: "ec2-user", // assume Amazon Linux 2
					AMIType:              eks.AMITypesAl2X8664,
					ImageID:              "",
					ImageIDSSMParameter:  "/aws/service/eks/optimized-ami/1.20/amazon-linux-2/recommended/image_id",
					InstanceType:         DefaultNodeInstanceTypeCPU,
					VolumeSize:           DefaultNodeVolumeSize,
					VolumeType:           DefaultNodeVolumeType,
					ASGMinSize:           1,
					ASGMaxSize:           1,
					ASGDesiredCapacity:   1,
					LaunchTemplateName:   name + "-launch-template",
				},
				KubeletExtraArgs:  "",
				BootstrapArgs:     "",
				ClusterAutoscaler: &NGClusterAutoscaler{Enable: false},
			},
		},
	}
}

func (cfg *Config) validateAddOnNodeGroups() error {
	if !cfg.IsEnabledAddOnNodeGroups() {
		return nil
	}

	switch cfg.AddOnNodeGroups.Role.Create {
	case true: // need create one, or already created
		if cfg.AddOnNodeGroups.Role.Name == "" {
			cfg.AddOnNodeGroups.Role.Name = cfg.Name + "-node-group-role"
		}
		// just ignore...
		// could be populated from previous run
		// do not error, so long as RoleCreate false, role won't be deleted

	case false: // use existing one
		if cfg.AddOnNodeGroups.Role.ARN == "" {
			return fmt.Errorf("Role.Create false; expect non-empty RoleARN but got %q", cfg.AddOnNodeGroups.Role.ARN)
		}
		if cfg.AddOnNodeGroups.Role.Name == "" {
			cfg.AddOnNodeGroups.Role.Name = getNameFromARN(cfg.AddOnNodeGroups.Role.ARN)
		}
	}
	if cfg.AddOnNodeGroups.Role.PolicyName == "" {
		cfg.AddOnNodeGroups.Role.PolicyName = cfg.Name + "-node-group-policy"
	}
	if cfg.AddOnNodeGroups.Role.InstanceProfileName == "" {
		cfg.AddOnNodeGroups.Role.InstanceProfileName = cfg.Name + "-node-group-instance-profile"
	}

	n := len(cfg.AddOnNodeGroups.ASGs)
	if n == 0 {
		return errors.New("empty ASGs")
	}
	if n > NGsMaxLimit {
		return fmt.Errorf("NGs %d exceeds maximum number of NGs which is %d", n, NGsMaxLimit)
	}

	if cfg.VersionValue < 1.14 {
		return fmt.Errorf("version %q not supported for AddOnNodeGroups", cfg.Version)
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

		if cur.VolumeSize == 0 {
			cur.VolumeSize = DefaultNodeVolumeSize
		}
		if cur.VolumeType == "" {
			cur.VolumeType = DefaultNodeVolumeType
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
		if cur.LaunchTemplateName == "" {
			cur.LaunchTemplateName = cur.Name + "-launch-template"
		}

		switch cur.AMIType {
		case ec2config.AMITypeBottleRocketCPU:
			if cur.RemoteAccessUserName != "ec2-user" {
				return fmt.Errorf("AMIType %q but unexpected RemoteAccessUserName %q", cur.AMIType, cur.RemoteAccessUserName)
			}
			if cur.SSM != nil {
				if cur.SSM.DocumentName != "" && cfg.S3.BucketName == "" {
					return fmt.Errorf("AMIType %q requires SSMDocumentName %q but no S3BucketName", cur.AMIType, cur.SSM.DocumentName)
				}
			}
			if cur.KubeletExtraArgs != "" {
				return fmt.Errorf("AMIType %q but unexpected KubeletExtraArgs %q", cur.AMIType, cur.KubeletExtraArgs)
			}
		case fmt.Sprint(aws_eks_v2_types.AMITypesAl2X8664):
			if cur.RemoteAccessUserName != "ec2-user" {
				return fmt.Errorf("AMIType %q but unexpected RemoteAccessUserName %q", cur.AMIType, cur.RemoteAccessUserName)
			}
		case fmt.Sprint(aws_eks_v2_types.AMITypesAl2Arm64):
			if cur.RemoteAccessUserName != "ec2-user" {
				return fmt.Errorf("AMIType %q but unexpected RemoteAccessUserName %q", cur.AMIType, cur.RemoteAccessUserName)
			}
		case fmt.Sprint(aws_eks_v2_types.AMITypesAl2X8664Gpu):
			if cur.RemoteAccessUserName != "ec2-user" {
				return fmt.Errorf("AMIType %q but unexpected RemoteAccessUserName %q", cur.AMIType, cur.RemoteAccessUserName)
			}
		default:
			return fmt.Errorf("unknown ASGs[%q].AMIType %q", k, cur.AMIType)
		}

		switch cur.AMIType {
		case ec2config.AMITypeBottleRocketCPU:
			if cur.InstanceType == "" {
				cur.InstanceType = DefaultNodeInstanceTypeCPU
			}
		case fmt.Sprint(aws_eks_v2_types.AMITypesAl2X8664):
			if cur.InstanceType == "" {
				cur.InstanceType = DefaultNodeInstanceTypeCPU
			}
		case fmt.Sprint(aws_eks_v2_types.AMITypesAl2X8664Gpu):
			if cur.InstanceType == "" {
				cur.InstanceType = DefaultNodeInstanceTypeGPU
			}
		default:
			return fmt.Errorf("unknown AddOnNodeGroups.ASGs[%q].AMIType %q", k, cur.AMIType)
		}

		if cfg.IsEnabledAddOnNLBHelloWorld() || cfg.IsEnabledAddOnALB2048() {
			// "m3.xlarge" or "c4.xlarge" will fail with "InvalidTarget: Targets {...} are not supported"
			// ref. https://github.com/aws/amazon-vpc-cni-k8s/pull/821
			// ref. https://github.com/kubernetes/kubernetes/issues/66044#issuecomment-408188524
			switch {
			case strings.HasPrefix(cur.InstanceType, "m3."),
				strings.HasPrefix(cur.InstanceType, "c4."):
				return fmt.Errorf("AddOnNLBHelloWorld.Enable[%v] || AddOnALB2048.Enable[%v], but older instance type InstanceType %q for %q",
					cfg.IsEnabledAddOnNLBHelloWorld(),
					cfg.IsEnabledAddOnALB2048(),
					cur.InstanceType, k)
			}
		}

		if cur.ASGMinSize == 0 {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].ASGMinSize must be >0", k)
		}
		if cur.ASGDesiredCapacity == 0 {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].ASGDesiredCapacity must be >0", k)
		}
		if cur.ASGMaxSize == 0 {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].ASGMaxSize must be >0", k)
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

		if cur.SSM != nil {
			switch cur.SSM.DocumentCreate {
			case true: // need create one, or already created
				if cur.SSM.DocumentName == "" {
					cur.SSM.DocumentName = cur.Name + "SSMDocument"
				}
				cur.SSM.DocumentName = strings.ReplaceAll(cur.SSM.DocumentName, "GetRef.Name", cfg.Name)
				cur.SSM.DocumentName = regex.ReplaceAllString(cur.SSM.DocumentName, "")
				if cur.SSM.DocumentExecutionTimeoutSeconds == 0 {
					cur.SSM.DocumentExecutionTimeoutSeconds = 3600
				}
				if cur.SSM.DocumentCommands == "" {
					return errors.New("empty SSM.DocumentCommands")
				}

			case false: // use existing one, or don't run any SSM
			}
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
