package eksconfig

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
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
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// S3Dir is the S3 directory to store all test results.
	// It is under the bucket "eksconfig.Config.S3BucketName".
	S3Dir string `json:"s3-dir"`

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
	RoleCFNStackYAMLPath  string   `json:"role-cfn-stack-yaml-path" read-only:"true"`
	RoleCFNStackYAMLS3Key string   `json:"role-cfn-stack-yaml-s3-key" read-only:"true"`

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
	// LogsTarGzPath is the .tar.gz archived file for "LogsDir".
	LogsTarGzPath string `json:"logs-tar-gz-path"`
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

	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// RemoteAccessUserName is the user name for managed node group SSH access.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	RemoteAccessUserName string `json:"remote-access-user-name,omitempty"`
	// Tags defines EKS managed node group create tags.
	Tags map[string]string `json:"tags,omitempty"`
	// ReleaseVersion is the AMI version of the Amazon EKS-optimized AMI for the node group.
	// The version may differ from EKS "cluster" version.
	// e.g. "1.16.8-20200609"
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/eks-linux-ami-versions.html
	ReleaseVersion      string  `json:"release-version,omitempty"`
	ReleaseVersionValue float64 `json:"release-version-value" read-only:"true"`

	// AMIType is the AMI type for the node group.
	// Allowed values are AL2_x86_64 and AL2_x86_64_GPU.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	AMIType string `json:"ami-type,omitempty"`

	// InstanceTypes is the EC2 instance types for the node instances.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	InstanceTypes []string `json:"instance-types,omitempty"`
	// VolumeSize is the node volume size.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	VolumeSize int `json:"volume-size,omitempty"`

	// ASGMinSize is the minimum size of Node Group Auto Scaling Group.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	ASGMinSize int `json:"asg-min-size,omitempty"`
	// ASGMaxSize is the maximum size of Node Group Auto Scaling Group.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	ASGMaxSize int `json:"asg-max-size,omitempty"`
	// ASGDesiredCapacity is the desired capacity of Node Group ASG.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	ASGDesiredCapacity int `json:"asg-desired-capacity,omitempty"`

	// CreateRequested is true if "CreateNodegroupRequest" has been sent.
	CreateRequested bool `json:"create-requested" read-only:"true"`

	// PhysicalID is the Physical ID for the created "AWS::EKS::Nodegroup".
	PhysicalID string `json:"physical-id" read-only:"true"`

	// MNGCFNStackID is the CloudFormation stack ID for a managed node group.
	MNGCFNStackID        string `json:"mng-cfn-stack-id" read-only:"true"`
	MNGCFNStackYAMLPath  string `json:"mng-cfn-stack-yaml-path" read-only:"true"`
	MNGCFNStackYAMLS3Key string `json:"mng-cfn-stack-yaml-s3-key" read-only:"true"`

	RemoteAccessSecurityGroupID                      string `json:"remote-access-security-group-id" read-only:"true"`
	RemoteAccessSecurityGroupIngressEgressCFNStackID string `json:"remote-access-security-group-ingress-egress-cfn-stack-id" read-only:"true"`
	RemoteAccessSecurityCFNStackYAMLPath             string `json:"remote-access-security-group-cfn-stack-yaml-path" read-only:"true"`
	RemoteAccessSecurityCFNStackYAMLS3Key            string `json:"remote-access-security-group-cfn-stack-yaml-s3-key" read-only:"true"`

	// Status is the current status of EKS "Managed Node Group".
	Status string `json:"status" read-only:"true"`
	// Instances maps an instance ID to an EC2 instance object for the node group.
	Instances map[string]ec2config.Instance `json:"instances" read-only:"true"`
	// Logs maps each instance ID to a list of log file paths fetched via SSH access.
	Logs map[string][]string `json:"logs" read-only:"true"`

	// ScaleUpdates configures MNG scale update.
	ScaleUpdates []MNGScaleUpdate `json:"scale-updates,omitempty"`

	// VersionUpgrade configures MNG version upgarde.
	VersionUpgrade *MNGVersionUpgrade `json:"version-upgrade,omitempty"`
}

// MNGScaleUpdate contains the minimum, maximum, and desired node counts for a nodegroup.
// ref, https://docs.aws.amazon.com/cli/latest/reference/eks/update-nodegroup-config.html
type MNGScaleUpdate struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	// InitialWait is the wait time before triggering version upgrades.
	// All managed node group upgrades are triggered after all existing
	// add-on installation is complete.
	InitialWait       time.Duration `json:"initial-wait" read-only:"true"`
	InitialWaitString string        `json:"initial-wait-string"`

	ID                 string `json:"id"`
	ASGMinSize         int64  `json:"asg-min-size,omitempty"`
	ASGMaxSize         int64  `json:"asg-max-size,omitempty"`
	ASGDesiredCapacity int64  `json:"asg-desired-capacity,omitempty"`
}

// MNGVersionUpgrade defines parameters
// for EKS managed node group version upgrade add-on.
// ref. https://docs.aws.amazon.com/cli/latest/reference/eks/update-nodegroup-version.html
type MNGVersionUpgrade struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`
	// InitialWait is the wait time before triggering version upgrades.
	// All managed node group upgrades are triggered after all existing
	// add-on installation is complete.
	InitialWait       time.Duration `json:"initial-wait" read-only:"true"`
	InitialWaitString string        `json:"initial-wait-string"`
	// Created is true when the resource has been created.
	// Used for delete operations.
	Created         bool               `json:"created" read-only:"true"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`

	// Version is the target version of EKS managed node group.
	// This cannot be empty. Must be provided by the user.
	// The value is passed via "aws eks update-nodegroup-version --kubernetes-version".
	// e.g. Upgrade to "Version" == "1.17" when Parameters.Version is "1.16"
	// that has created "1.16" MNG by default.
	Version      string  `json:"version"`
	VersionValue float64 `json:"version-value" read-only:"true"`
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
		FetchLogs:   false,
		SigningName: "eks",
		RoleCreate:  true,
		LogsDir:     "", // to be auto-generated
		MNGs: map[string]MNG{
			name + "-mng-cpu": {
				Name:                 name + "-mng-cpu",
				RemoteAccessUserName: "ec2-user", // assume Amazon Linux 2
				ReleaseVersion:       "",         // to be auto-filled by EKS API
				AMIType:              eks.AMITypesAl2X8664,
				InstanceTypes:        []string{DefaultNodeInstanceTypeCPU},
				VolumeSize:           DefaultNodeVolumeSize,
				ASGMinSize:           1,
				ASGMaxSize:           1,
				ASGDesiredCapacity:   1,
				VersionUpgrade:       &MNGVersionUpgrade{Enable: false},
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

	if cfg.VersionValue < 1.14 {
		return fmt.Errorf("Version %q not supported for AddOnManagedNodeGroups", cfg.Version)
	}

	if cfg.AddOnManagedNodeGroups.S3Dir == "" {
		cfg.AddOnManagedNodeGroups.S3Dir = path.Join(cfg.Name, "add-on-managed-node-groups")
	}

	if cfg.AddOnManagedNodeGroups.LogsDir == "" {
		cfg.AddOnManagedNodeGroups.LogsDir = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-logs-mngs")
	}
	if cfg.AddOnManagedNodeGroups.LogsTarGzPath == "" {
		cfg.AddOnManagedNodeGroups.LogsTarGzPath = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-logs-mngs.tar.gz")
	}
	if !strings.HasSuffix(cfg.AddOnManagedNodeGroups.LogsTarGzPath, ".tar.gz") {
		return fmt.Errorf("AddOnManagedNodeGroups.LogsTarGzPath %q must end with .tar.gz", cfg.AddOnManagedNodeGroups.LogsTarGzPath)
	}

	if cfg.AddOnManagedNodeGroups.RoleCFNStackYAMLPath == "" {
		cfg.AddOnManagedNodeGroups.RoleCFNStackYAMLPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".add-on-managed-node-groups.role.cfn.yaml"
	}
	if cfg.AddOnManagedNodeGroups.RoleCFNStackYAMLS3Key == "" {
		cfg.AddOnManagedNodeGroups.RoleCFNStackYAMLS3Key = path.Join(
			cfg.AddOnManagedNodeGroups.S3Dir,
			filepath.Base(cfg.AddOnManagedNodeGroups.RoleCFNStackYAMLPath),
		)
	}
	switch cfg.AddOnManagedNodeGroups.RoleCreate {
	case true: // need create one, or already created
		if cfg.AddOnManagedNodeGroups.RoleName == "" {
			cfg.AddOnManagedNodeGroups.RoleName = cfg.Name + "-mng-role"
		}
		if cfg.AddOnManagedNodeGroups.RoleARN != "" {
			// just ignore...
			// could be populated from previous run
			// do not error, so long as RoleCreate false, role won't be deleted
		}
		if len(cfg.AddOnManagedNodeGroups.RoleServicePrincipals) > 0 {
			/*
				(InvalidParameterException: Following required service principals [ec2.amazonaws.com] were not found in the trust relationships of nodeRole arn:aws:iam::...:role/test-mng-role
				{
				  ClusterName: "test",
				  Message_: "Following required service principals [ec2.amazonaws.com] were not found in the trust relationships of nodeRole arn:aws:iam::...:role/test-mng-role",
				  NodegroupName: "test-mng-cpu"
				})
			*/
			found := false
			for _, pv := range cfg.AddOnManagedNodeGroups.RoleServicePrincipals {
				if pv == "ec2.amazonaws.com" || pv == "ec2.amazonaws.com.cn" {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("AddOnManagedNodeGroups.RoleServicePrincipals %q must include 'ec2.amazonaws.com' or 'ec2.amazonaws.com.cn'", cfg.AddOnManagedNodeGroups.RoleServicePrincipals)
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
		if cfg.IsEnabledAddOnStresserRemote() {
			return errors.New("'AddOnStresserRemote.Enable == true' requires 'AddOnManagedNodeGroups.RoleCreate == true' but got 'false'")
		}
	}

	names, processed := make(map[string]struct{}), make(map[string]MNG)
	for k, cur := range cfg.AddOnManagedNodeGroups.MNGs {
		k = strings.ReplaceAll(k, "GetRef.Name", cfg.Name)
		cur.Name = strings.ReplaceAll(cur.Name, "GetRef.Name", cfg.Name)

		if cur.Name == "" {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].Name is empty", k)
		}
		if k != cur.Name {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].Name has different Name field %q", k, cur.Name)
		}
		_, ok := names[cur.Name]
		if !ok {
			names[cur.Name] = struct{}{}
		} else {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].Name %q is redundant", k, cur.Name)
		}
		if cfg.IsEnabledAddOnNodeGroups() {
			_, ok = cfg.AddOnNodeGroups.ASGs[cur.Name]
			if ok {
				return fmt.Errorf("MNGs[%q] name is conflicting with NG ASG", cur.Name)
			}
		}

		if cur.ReleaseVersion != "" {
			// e.g. "1.16.8-20200609"
			ss := strings.Split(cur.ReleaseVersion, ".")
			if len(ss) > 2 {
				sv := strings.Join(ss[:2], ".")
				var err error
				cur.ReleaseVersionValue, err = strconv.ParseFloat(sv, 64)
				if err != nil {
					return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q] invalid ReleaseVersion %q (%q, %v)", cur.Name, cur.ReleaseVersion, sv, err)
				}
			}
		}

		if len(cur.ScaleUpdates) > 0 {
			for idx := range cur.ScaleUpdates {
				if !cur.ScaleUpdates[idx].Enable {
					continue
				}
				var err error
				if cur.ScaleUpdates[idx].InitialWaitString != "" {
					cur.ScaleUpdates[idx].InitialWait, err = time.ParseDuration(cur.ScaleUpdates[idx].InitialWaitString)
					if err != nil {
						return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q] invalid cur.ScaleUpdates[%d].InitialWaitString %q (%v)", cur.Name, idx, cur.VersionUpgrade.InitialWaitString, err)
					}
				}
				if cur.ScaleUpdates[idx].ASGDesiredCapacity == 0 {
					return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q] invalid cur.ScaleUpdates[%d].ASGDesiredCapacity == 0", cur.Name, idx)
				}
				if cur.ScaleUpdates[idx].ASGDesiredCapacity < cur.ScaleUpdates[idx].ASGMinSize {
					return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q] invalid cur.ScaleUpdates[%d].ASGDesiredCapacity %d < ASGMinSize %d", cur.Name, idx, cur.ScaleUpdates[idx].ASGDesiredCapacity, cur.ScaleUpdates[idx].ASGMinSize)
				}
			}
		}

		// check optional mng version upgrade add-on
		if cur.VersionUpgrade != nil && cur.VersionUpgrade.Enable {
			var err error
			if cur.VersionUpgrade.InitialWaitString != "" {
				cur.VersionUpgrade.InitialWait, err = time.ParseDuration(cur.VersionUpgrade.InitialWaitString)
				if err != nil {
					return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q] invalid cur.VersionUpgrade.InitialWaitString %q (%v)", cur.Name, cur.VersionUpgrade.InitialWaitString, err)
				}
			}

			// do not set any defaults
			// hard to keep everything in sync and find right values:
			// - original cluster version
			// - cluster upgrade version
			// - default mng version
			// - custom mng version
			if cur.VersionUpgrade.Version == "" {
				return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q] VersionUpgrade.Enable but empty VersionUpgrade.Version", cur.Name)
			}
			cur.VersionUpgrade.VersionValue, err = strconv.ParseFloat(cur.VersionUpgrade.Version, 64)
			if err != nil {
				return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q] invalid VersionUpgrade.Version %q (%v)", cur.Name, cur.VersionUpgrade.Version, err)
			}
			origVer := cfg.VersionValue
			if cur.ReleaseVersionValue > 0.0 {
				// e.g. "1.16" in "1.16.8-20200609"
				origVer = cur.ReleaseVersionValue
			}

			delta := cur.VersionUpgrade.VersionValue - origVer
			if fmt.Sprintf("%.2f", delta) != "0.01" {
				return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q] VersionUpgrade only supports one minor version upgrade but got %.2f [cluster version %q, mng release version %q, mng upgrade version %q]", cur.Name, delta, cfg.Version, cur.ReleaseVersion, cur.VersionUpgrade.Version)
			}
			// target version must match with the Kubernetes control plane version
			// can't upgrade to 1.17 MNG when EKS is 1.16
			// e.g. "Nodegroup Kubernetes version should be equal to Cluster kubernetes version 1.16 or NodeGroup kubernetes version 1.16"
			if cur.ReleaseVersionValue == 0.0 && !cfg.IsEnabledAddOnClusterVersionUpgrade() {
				return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q] VersionUpgrade %q would diverge from Parameters.Version %q (IsEnabledAddOnClusterVersionUpgrade %v)", cur.Name, cur.VersionUpgrade.Version, cfg.Version, cfg.IsEnabledAddOnClusterVersionUpgrade())
			}
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

		if cur.MNGCFNStackYAMLPath == "" {
			cur.MNGCFNStackYAMLPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".mng.cfn." + k + ".yaml"
		}
		if cur.MNGCFNStackYAMLS3Key == "" {
			cur.MNGCFNStackYAMLS3Key = path.Join(
				cfg.AddOnManagedNodeGroups.S3Dir,
				filepath.Base(cur.MNGCFNStackYAMLPath),
			)
		}

		if cur.RemoteAccessSecurityCFNStackYAMLPath == "" {
			cur.RemoteAccessSecurityCFNStackYAMLPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".mng-sg.cfn." + k + ".yaml"
		}
		if cur.RemoteAccessSecurityCFNStackYAMLS3Key == "" {
			cur.RemoteAccessSecurityCFNStackYAMLS3Key = path.Join(
				cfg.AddOnManagedNodeGroups.S3Dir,
				filepath.Base(cur.RemoteAccessSecurityCFNStackYAMLPath),
			)
		}

		switch cur.AMIType {
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
		case eks.AMITypesAl2X8664:
			if len(cur.InstanceTypes) == 0 {
				cur.InstanceTypes = []string{DefaultNodeInstanceTypeCPU}
			}
		case eks.AMITypesAl2X8664Gpu:
			if len(cur.InstanceTypes) == 0 {
				cur.InstanceTypes = []string{DefaultNodeInstanceTypeGPU}
			}
		default:
			return fmt.Errorf("unknown AddOnManagedNodeGroups.MNGs[%q].AMIType %q", k, cur.AMIType)
		}

		if cfg.IsEnabledAddOnNLBHelloWorld() || cfg.IsEnabledAddOnALB2048() {
			for _, itp := range cur.InstanceTypes {
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

		if cur.ASGMinSize == 0 && cur.ASGDesiredCapacity == 0 {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].ASGMinSize/ASGDesiredCapacity must be >0", k)
		}
		if cur.ASGDesiredCapacity > 0 && cur.ASGMinSize == 0 {
			cur.ASGMinSize = cur.ASGDesiredCapacity
		}
		if cur.ASGDesiredCapacity > 0 && cur.ASGMaxSize == 0 {
			cur.ASGMaxSize = cur.ASGDesiredCapacity
		}

		if cur.ASGMinSize > cur.ASGMaxSize {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].ASGMinSize %d > ASGMaxSize %d", k, cur.ASGMinSize, cur.ASGMaxSize)
		}
		if cur.ASGDesiredCapacity > cur.ASGMaxSize {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].ASGDesiredCapacity %d > ASGMaxSize %d", k, cur.ASGDesiredCapacity, cur.ASGMaxSize)
		}
		if cur.ASGMaxSize > MNGMaxLimit {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].ASGMaxSize %d > MNGMaxLimit %d", k, cur.ASGMaxSize, MNGMaxLimit)
		}
		if cur.ASGDesiredCapacity > MNGMaxLimit {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].ASGDesiredCapacity %d > MNGMaxLimit %d", k, cur.ASGDesiredCapacity, MNGMaxLimit)
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

	cfg.AddOnManagedNodeGroups.MNGs = processed
	return nil
}
