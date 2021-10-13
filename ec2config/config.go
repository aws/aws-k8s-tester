// Package ec2config defines EC2 configuration.
package ec2config

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	aws_v2 "github.com/aws/aws-sdk-go-v2/aws"
	aws_ec2_v2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/mitchellh/colorstring"
	"sigs.k8s.io/yaml" // must use "sigs.k8s.io/yaml"
)

// AWS_K8S_TESTER_EC2_PREFIX is the environment variable prefix used for "ec2config".
const AWS_K8S_TESTER_EC2_PREFIX = "AWS_K8S_TESTER_EC2_"

const (
	// AMITypeAL2ARM64 is the AMI type for Amazon Linux 2 AMI.
	// https://aws.amazon.com/ec2/graviton/
	AMITypeAL2ARM64 = "AL2_arm_64"
	// AMITypeBottleRocketCPU is the AMI type for Bottlerocket OS.
	// https://github.com/bottlerocket-os/bottlerocket
	AMITypeBottleRocketCPU = "BOTTLEROCKET_x86_64"
	// AMITypeAL2X8664 is the AMI type for Amazon Linux 2 AMI.
	AMITypeAL2X8664 = "AL2_x86_64"
	// AMITypeAL2X8664GPU is the AMI type for Amazon Linux 2 AMI with GPU.
	AMITypeAL2X8664GPU = "AL2_x86_64_GPU"

	// AMITypeOther is defined for all other AMI types.
	AMITypeOther = "OTHER"

	AMITypeWindowsServerCore2019X8664 = "WINDOWS_SERVER_CORE_2019_x86_64"

	// DefaultNodeInstanceTypeCPUARM is the default EC2 instance type for CPU worker node.
	// https://aws.amazon.com/ec2/instance-types/m6/
	DefaultNodeInstanceTypeCPUARM = "m6g.xlarge"
	// DefaultNodeInstanceTypeCPU is the default EC2 instance type for CPU worker node.
	DefaultNodeInstanceTypeCPU = "c5.xlarge"
	// DefaultNodeInstanceTypeGPU is the default EC2 instance type for GPU worker node.
	DefaultNodeInstanceTypeGPU = "p3.8xlarge"

	// DefaultNodeVolumeSize is the default EC2 instance volume size for a worker node.
	DefaultNodeVolumeSize = 40

	// ASGsMaxLimit is the maximum number of "Managed Node Group"s per a EKS cluster.
	ASGsMaxLimit = 10
	// ASGMaxLimit is the maximum number of nodes per a "Managed Node Group".
	ASGMaxLimit = 100
)

// Config defines EC2 configuration.
type Config struct {
	mu *sync.RWMutex

	// Up is true if the cluster is up.
	Up              bool               `json:"up"`
	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`
	// StatusCurrent represents the current status of the cluster.
	StatusCurrent string `json:"status-current"`
	// Status represents the status of the cluster.
	Status           []Status          `json:"status"`
	DeletedResources map[string]string `json:"deleted-resources"`

	// Name is the cluster name.
	// If empty, deployer auto-populates it.
	Name string `json:"name"`
	// Partition is the AWS partition for EC2 deployment region.
	// If empty, set default partition "aws".
	Partition string `json:"partition"`
	// Region is the AWS geographic area for EC2 deployment.
	// If empty, set default region.
	Region string `json:"region"`
	// AvailabilityZoneNames lists the availability zones for the specified region.
	// ref. https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeAvailabilityZones.html
	AvailabilityZoneNames []string `json:"availability-zone-names,omitempty" read-only:"true"`

	// ConfigPath is the configuration file path.
	// Deployer is expected to update this file with latest status.
	ConfigPath string `json:"config-path,omitempty"`

	// AWSAccountID is the account ID of the eks tester caller session.
	AWSAccountID string `json:"aws-account-id" read-only:"true"`
	// AWSUserID is the user ID of the eks tester caller session.
	AWSUserID string `json:"aws-user-id" read-only:"true"`
	// AWSIAMRoleARN is the user IAM Role ARN of the eks tester caller session.
	AWSIAMRoleARN string `json:"aws-iam-role-arn" read-only:"true"`
	// AWSCredentialPath is automatically set via AWS SDK Go.
	// And to be mounted as a volume as 'Secret' object.
	AWSCredentialPath string `json:"aws-credential-path" read-only:"true"`

	// LogColor is true to output logs in color.
	LogColor bool `json:"log-color"`
	// LogColorOverride is not empty to override "LogColor" setting.
	// If not empty, the automatic color check is not even run and use this value instead.
	// For instance, github action worker might not support color device,
	// thus exiting color check with the exit code 1.
	// Useful to output in color in HTML based log outputs (e.g., Prow).
	// Useful to skip terminal color check when there is no color device (e.g., Github action worker).
	LogColorOverride string `json:"log-color-override"`

	// LogLevel configures log level. Only supports debug, info, warn, error, panic, or fatal. Default 'info'.
	LogLevel string `json:"log-level"`
	// LogOutputs is a list of log outputs. Valid values are 'default', 'stderr', 'stdout', or file names.
	// Logs are appended to the existing file, if any.
	// Multiple values are accepted. If empty, it sets to 'default', which outputs to stderr.
	// See https://pkg.go.dev/go.uber.org/zap#Open and https://pkg.go.dev/go.uber.org/zap#Config for more details.
	LogOutputs []string `json:"log-outputs,omitempty"`

	// OnFailureDelete is true to delete all resources on creation fail.
	OnFailureDelete bool `json:"on-failure-delete"`
	// OnFailureDeleteWaitSeconds is the seconds to wait before deleting
	// all resources on creation fail.
	OnFailureDeleteWaitSeconds uint64 `json:"on-failure-delete-wait-seconds"`

	S3   *S3   `json:"s3"`
	Role *Role `json:"role"`
	VPC  *VPC  `json:"vpc"`

	// RemoteAccessKeyCreate is true to create the remote SSH access private key.
	RemoteAccessKeyCreate bool `json:"remote-access-key-create"`
	// RemoteAccessKeyName is the remote SSH access private key name.
	RemoteAccessKeyName string `json:"remote-access-key-name"`
	// RemoteAccessPrivateKeyPath is the remote SSH access private key path.
	RemoteAccessPrivateKeyPath string `json:"remote-access-private-key-path"`
	// RemoteAccessCommandsOutputPath is the output path for ssh commands.
	RemoteAccessCommandsOutputPath string `json:"remote-access-commands-output-path,omitempty"`

	// ASGsFetchLogs is true to fetch logs from remote nodes using SSH.
	ASGsFetchLogs bool `json:"asgs-fetch-logs"`
	// ASGsLogsDir is set to specify the target directory to store all remote log files.
	// If empty, it stores in the same directory as "ConfigPath".
	ASGsLogsDir string `json:"asgs-logs-dir,omitempty"`
	// ASGs is a map from each ASG name to EC2 ASG.
	ASGs map[string]ASG `json:"asgs"`

	// TotalNodes is the total number of nodes from all ASGs.
	TotalNodes int32 `json:"total-nodes" read-only:"true"`
}

type S3 struct {
	// BucketCreate is true to auto-create S3 bucket.
	BucketCreate bool `json:"bucket-create"`
	// BucketCreateKeep is true to not delete auto-created S3 bucket.
	// The created S3 bucket is kept.
	BucketCreateKeep bool `json:"bucket-create-keep"`
	// BucketName is the name of cluster S3.
	BucketName string `json:"bucket-name"`
	// BucketLifecycleExpirationDays is expiration in days for the lifecycle of the object.
	BucketLifecycleExpirationDays int64 `json:"bucket-lifecycle-expiration-days"`
	// Dir is the S3 directory to store all test results.
	// It is under the bucket "eksconfig.Config.S3BucketName".
	Dir string `json:"dir"`
}

func getDefaultS3() *S3 {
	return &S3{
		BucketName:                    "",
		BucketCreate:                  true,
		BucketCreateKeep:              true,
		BucketLifecycleExpirationDays: 0,
	}
}

type Role struct {
	// Name is the name of cluster role.
	Name string `json:"name"`
	// Create is true to auto-create and delete cluster role.
	Create bool `json:"create"`
	// ARN is the role ARN that EKS uses to create AWS resources for Kubernetes.
	// By default, it's empty which triggers tester to create one.
	ARN string `json:"arn"`

	// ServicePrincipals is the EKS Role Service Principals
	ServicePrincipals []string `json:"service-principals"`
	// ManagedPolicyARNs is EKS Role managed policy ARNs.
	ManagedPolicyARNs []string `json:"managed-policy-arns"`

	// PolicyName is the name of the policy.
	PolicyName string `json:"policy-name" read-only:"true"`
	// PolicyARN is the attached policy ARN.
	PolicyARN string `json:"policy-arn" read-only:"true"`

	// InstanceProfileName is the instance profile name for the node group.
	InstanceProfileName string `json:"instance-profile-name" read-only:"true"`
	// InstanceProfileARN is the instance profile ARN for the node group.
	InstanceProfileARN string `json:"instance-profile-arn" read-only:"true"`
}

func getDefaultRole() *Role {
	return &Role{
		Create: true,
		ServicePrincipals: []string{
			"ec2.amazonaws.com",
		},
		ManagedPolicyARNs: []string{
			"arn:aws:iam::aws:policy/AmazonEC2FullAccess",
			"arn:aws:iam::aws:policy/AmazonSSMFullAccess",
			"arn:aws:iam::aws:policy/AmazonS3FullAccess",
		},
	}
}

type VPC struct {
	// Create is true to auto-create and delete VPC.
	Create bool `json:"create"`
	// ID is the VPC ID for cluster creation.
	// If not empty, VPC is reused and not deleted.
	// If empty, VPC is created anew and deleted on cluster deletion.
	ID              string `json:"id"`
	SecurityGroupID string `json:"security-group-id" read-only:"true"`

	// CIDRs is the list of CIDR blocks with IP range (CIDR notation) for the primary VPC Block.
	// Must be a valid RFC 1918 CIDR range.
	CIDRs []string `json:"cidrs"`

	// PublicSubnetCIDRs is the CIDR blocks for public subnets.
	PublicSubnetCIDRs                    []string `json:"public-subnet-cidrs"`
	PublicSubnetIDs                      []string `json:"public-subnet-ids" read-only:"true"`
	InternetGatewayID                    string   `json:"internet-gateway-id" read-only:"true"`
	PublicRouteTableID                   string   `json:"public-route-table-id" read-only:"true"`
	PublicSubnetRouteTableAssociationIDs []string `json:"public-subnet-route-table-association-ids" read-only:"true"`
	EIPAllocationIDs                     []string `json:"eip-allocation-ids" read-only:"true"`
	NATGatewayIDs                        []string `json:"nat-gateway-ids" read-only:"true"`

	// PrivateSubnetCIDRs is the CIDR blocks for private subnets.
	PrivateSubnetCIDRs                    []string `json:"private-subnet-cidrs,omitempty"`
	PrivateSubnetIDs                      []string `json:"private-subnet-ids" read-only:"true"`
	PrivateRouteTableIDs                  []string `json:"private-route-table-ids" read-only:"true"`
	PrivateSubnetRouteTableAssociationIDs []string `json:"private-subnet-route-table-association-ids" read-only:"true"`

	// DHCPOptionsDomainName is used to complete unqualified DNS hostnames for VPC.
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-dhcp-options.html
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/cluster-endpoint.html
	DHCPOptionsDomainName string `json:"dhcp-options-domain-name,omitempty"`
	// DHCPOptionsDomainNameServers is a list of strings.
	// The IPv4 addresses of up to four domain name servers, or AmazonProvidedDNS, for VPC.
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-dhcp-options.html
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/cluster-endpoint.html
	DHCPOptionsDomainNameServers []string `json:"dhcp-options-domain-name-servers,omitempty"`
	DHCPOptionsID                string   `json:"dhcp-options-id,omitempty" read-only:"true"`
}

func getDefaultVPC() *VPC {
	return &VPC{
		Create: true,
		CIDRs: []string{
			"10.0.0.0/16",
			"10.1.0.0/16",
			"10.2.0.0/16",
			"10.3.0.0/16",
		},
		PublicSubnetCIDRs: []string{
			"10.0.0.0/16",
			"10.1.0.0/16",
			"10.2.0.0/16",
		},
		PrivateSubnetCIDRs: []string{
			"10.3.0.0/17",
			"10.3.128.0/17",
		},
	}
}

func (c Config) Colorize(input string) string {
	colorize := colorstring.Colorize{
		Colors:  colorstring.DefaultColors,
		Disable: !c.LogColor,
		Reset:   true,
	}
	return colorize.Color(input)
}

// Status is the status.
type Status struct {
	Time   time.Time `json:"time"`
	Status string    `json:"status"`
}

// TODO: asg status
const StatusDELETEDORNOTEXIST = "DELETED/NOT-EXIST"

// RecordStatus records cluster status.
func (cfg *Config) RecordStatus(status string) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	cfg.StatusCurrent = status
	switch status {
	case StatusDELETEDORNOTEXIST:
		cfg.Up = false
	case "TODO/active":
		cfg.Up = true
	}

	sv := Status{Time: time.Now(), Status: status}
	n := len(cfg.Status)
	if n == 0 {
		cfg.Status = []Status{sv}
		cfg.unsafeSync()
		return
	}

	copied := make([]Status, n+1)
	copy(copied[1:], cfg.Status)
	copied[0] = sv
	cfg.Status = copied
	cfg.unsafeSync()
}

// ASG represents one ASG.
type ASG struct {
	// Name is the ASG name.
	Name string `json:"name"`

	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// RemoteAccessUserName is the user name used for running init scripts or SSH access.
	RemoteAccessUserName string `json:"remote-access-user-name"`

	// SSM defines SSM command parameters.
	SSM *SSM `json:"ssm"`

	// TODO: support bootstrap arguments
	// ref. https://github.com/awslabs/amazon-eks-ami/blob/master/amazon-eks-nodegroup.yaml

	// AMIType is the AMI type for the node group.
	// Allowed values are BOTTLEROCKET_x86_64, AL2_x86_64 and AL2_x86_64_GPU.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/launch-workers.html
	// ref. https://github.com/awslabs/amazon-eks-ami/blob/master/amazon-eks-nodegroup.yaml
	AMIType string `json:"ami-type,omitempty"`
	// ImageID is the Amazon Machine Image (AMI).
	// This value overrides any AWS Systems Manager Parameter Store value.
	// NOTE: THIS FIELD IS SET TO EMPTY IF "ImageIDSSMParameter" IS NOT EMPTY.
	ImageID string `json:"image-id"`
	// ImageIDSSMParameter is the AWS Systems Manager Parameter Store
	// parameter of the AMI ID.
	ImageIDSSMParameter string `json:"image-id-ssm-parameter"`

	// InstanceType is the EC2 instance type.
	InstanceType string `json:"instance-type"`

	// VolumeSize is the size of the default volume, in GiB.
	//
	// Constraints: 1-16384 for General Purpose SSD (gp2), 4-16384 for Provisioned
	// IOPS SSD (io1), 500-16384 for Throughput Optimized HDD (st1), 500-16384 for
	// Cold HDD (sc1), and 1-1024 for Magnetic (standard) volumes. If you specify
	// a snapshot, the volume size must be equal to or larger than the snapshot
	// size.
	//
	// Default: If you're creating the volume from a snapshot and don't specify
	// a volume size, the default is the snapshot size.
	VolumeSize int32 `json:"volume-size"`

	// VolumeType is the type of volume for the underlying EBS volume.
	// See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-volume-types.html
	VolumeType aws_ec2_v2_types.VolumeType `json:"volume-type"`

	// ASGMinSize is the minimum size of ASG.
	ASGMinSize int32 `json:"asg-min-size,omitempty"`
	// ASGMaxSize is the maximum size of ASG.
	ASGMaxSize int32 `json:"asg-max-size,omitempty"`
	// ASGDesiredCapacity is the desired capacity of ASG.
	ASGDesiredCapacity int32 `json:"asg-desired-capacity,omitempty"`

	// Instances is a map from instance ID to instance.
	Instances map[string]Instance `json:"instanaces" read-only:"true"`
	// Logs maps each instance ID to a list of log file paths fetched via SSH access.
	Logs map[string][]string `json:"logs" read-only:"true"`

	// LaunchTemplateName is the name of the launch template.
	LaunchTemplateName string `json:"launch-template-name" read-only:"true"`
}

type SSM struct {
	// DocumentCreate is true to auto-create and delete SSM document.
	DocumentCreate bool `json:"document-create"`
	// DocumentName is the name of SSM document.
	DocumentName string `json:"document-name"`
	// DocumentCommands is the commands for SSM document.
	// Only used if SSM doc is created.
	DocumentCommands string `json:"document-commands"`
	// DocumentExecutionTimeoutSeconds is the SSM document execution timeout in seconds.
	DocumentExecutionTimeoutSeconds int      `json:"document-execution-timeout-in-seconds"`
	DocumentCommandIDs              []string `json:"document-command-ids" read-only:"true"`
}

// Instance represents an EC2 instance.
type Instance struct {
	Architecture          string               `json:"architecture"`
	ImageID               string               `json:"image-id"`
	IAMInstanceProfile    IAMInstanceProfile   `json:"iam-instance-profile"`
	InstanceID            string               `json:"instance-id"`
	InstanceType          string               `json:"instance-type"`
	KeyName               string               `json:"key-name"`
	Placement             Placement            `json:"placement"`
	PrivateDNSName        string               `json:"private-dns-name"`
	PrivateIP             string               `json:"private-ip"`
	PublicDNSName         string               `json:"public-dns-name"`
	PublicIP              string               `json:"public-ip"`
	State                 State                `json:"state"`
	StateReason           StateReason          `json:"state-reason"`
	StateTransitionReason string               `json:"state-transition-reason"`
	SubnetID              string               `json:"subnet-id"`
	VPCID                 string               `json:"vpc-id"`
	CPUOptions            CPUOptions           `json:"cpu-options"`
	BlockDeviceMappings   []BlockDeviceMapping `json:"block-device-mappings"`
	EBSOptimized          bool                 `json:"ebs-optimized"`
	RootDeviceName        string               `json:"root-device-name"`
	RootDeviceType        string               `json:"root-device-type"`
	SecurityGroups        []SecurityGroup      `json:"security-groups"`
	LaunchTime            time.Time            `json:"launch-time"`
	RemoteAccessUserName  string               `json:"remote-access-user-name"`
	Hypervisor            string               `json:"hypervisor"`
	VirtualizationType    string               `json:"virtualization-type"`
}

// IAMInstanceProfile is the IAM instance profile.
type IAMInstanceProfile struct {
	// ARN is the Amazon Resource Name (ARN) of the instance profile.
	ARN string `json:"arn"`
	// ID is the ID of the instance profile.
	ID string `json:"id"`
}

// CPUOptions represents the CPU of an EC2 instance.
type CPUOptions struct {
	// CoreCount is the number of CPU cores for the instance.
	CoreCount int32 `json:"core-count"`
	// ThreadsPerCore is the number of threads per CPU core.
	ThreadsPerCore int32 `json:"threads-per-core"`
}

// Placement defines EC2 placement.
type Placement struct {
	AvailabilityZone string `json:"availability-zone"`
	Tenancy          string `json:"tenancy"`
}

// State defines an EC2 state.
type State struct {
	Code int32  `json:"code"`
	Name string `json:"name"`
}

// StateReason represents the EC2 state reason.
type StateReason struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// BlockDeviceMapping defines a block device mapping.
type BlockDeviceMapping struct {
	DeviceName string `json:"device-name"`
	EBS        EBS    `json:"ebs"`
}

// EBS defines an EBS volume.
type EBS struct {
	DeleteOnTermination bool   `json:"delete-on-termination"`
	Status              string `json:"status"`
	VolumeID            string `json:"volume-id"`
}

// SecurityGroup defines a security group.
type SecurityGroup struct {
	GroupName string `json:"group-name"`
	GroupID   string `json:"group-id"`
}

// Load loads configuration from YAML.
// Useful when injecting shared configuration via ConfigMap.
//
// Example usage:
//
//  import "github.com/aws/aws-k8s-tester/eksconfig"
//  cfg := eksconfig.Load("test.yaml")
//  err := cfg.ValidateAndSetDefaults()
//
// Do not set default values in this function.
// "ValidateAndSetDefaults" must be called separately,
// to prevent overwriting previous data when loaded from disks.
func Load(p string) (cfg *Config, err error) {
	var d []byte
	d, err = ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}
	cfg = new(Config)
	if err = yaml.Unmarshal(d, cfg, yaml.DisallowUnknownFields); err != nil {
		return nil, err
	}

	cfg.mu = new(sync.RWMutex)
	if cfg.ConfigPath != p {
		cfg.ConfigPath = p
	}
	var ap string
	ap, err = filepath.Abs(p)
	if err != nil {
		return nil, err
	}
	cfg.ConfigPath = ap
	cfg.unsafeSync()

	return cfg, nil
}

// Sync persists current configuration and states to disk.
func (cfg *Config) Sync() (err error) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	return cfg.unsafeSync()
}

func (cfg *Config) unsafeSync() (err error) {
	var p string
	if cfg.ConfigPath != "" && !filepath.IsAbs(cfg.ConfigPath) {
		p, err = filepath.Abs(cfg.ConfigPath)
		if err != nil {
			return fmt.Errorf("failed to 'filepath.Abs(%s)' %v", cfg.ConfigPath, err)
		}
		cfg.ConfigPath = p
	}
	var d []byte
	d, err = yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to 'yaml.Marshal' %v", err)
	}

	err = ioutil.WriteFile(cfg.ConfigPath, d, 0600)
	if err != nil {
		return fmt.Errorf("failed to write file %q (%v)", cfg.ConfigPath, err)
	}
	err = ioutil.WriteFile(cfg.RemoteAccessCommandsOutputPath, []byte(cmdTop+cfg.unsafeSSHCommands()), 0600)
	if err != nil {
		return fmt.Errorf("failed to write file %q (%v)", cfg.RemoteAccessCommandsOutputPath, err)
	}

	return nil
}

const cmdTop = `#!/bin/bash
set -e
set -x

`

// SSHCommands returns the SSH commands.
func (cfg *Config) SSHCommands() string {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	return cfg.unsafeSSHCommands()
}

func (cfg *Config) unsafeSSHCommands() (s string) {
	if len(cfg.ASGs) == 0 {
		return ""
	}

	buf := bytes.NewBuffer(nil)
	buf.WriteByte('\n')

	for name, cur := range cfg.ASGs {
		buf.WriteString("ASG name \"" + name + "\":\n")
		buf.WriteString(cur.SSHCommands(cfg.Region, cfg.RemoteAccessPrivateKeyPath, cur.RemoteAccessUserName))
		buf.WriteString("\n\n")
	}
	return buf.String()
}

// SSHCommands returns the SSH commands.
func (asg *ASG) SSHCommands(region string, keyPath string, userName string) (s string) {
	if len(asg.Instances) == 0 {
		return fmt.Sprintf("# empty ASG %q\n", asg.Name)
	}
	s = fmt.Sprintf(`
# change SSH key permission
chmod 400 %s
`, keyPath)
	for _, v := range asg.Instances {
		s += fmt.Sprintf(`# SSH into the remote machine (instance ID %q, public IP %q, private IP %q, public DNS %q)
ssh -o "StrictHostKeyChecking no" -i %s %s@%s
# download to local machine
scp -i %s %s@%s:REMOTE_FILE_PATH LOCAL_FILE_PATH
scp -i %s -r %s@%s:REMOTE_DIRECTORY_PATH LOCAL_DIRECTORY_PATH
# upload to remote machine
scp -i %s LOCAL_FILE_PATH %s@%s:REMOTE_FILE_PATH
scp -i %s -r LOCAL_DIRECTORY_PATH %s@%s:REMOTE_DIRECTORY_PATH
# SSM session (requires SSM agent)
aws ssm --region %s start-session --target %s


`,
			v.InstanceID, v.PublicIP, v.PrivateIP, v.PublicDNSName,
			keyPath, userName, v.PublicDNSName,
			keyPath, userName, v.PublicDNSName,
			keyPath, userName, v.PublicDNSName,
			keyPath, userName, v.PublicDNSName,
			keyPath, userName, v.PublicDNSName,
			region, v.InstanceID,
		)
	}

	return s
}

// ConvertInstance converts "aws ec2 describe-instances" to "config.Instance".
func ConvertInstance(iv aws_ec2_v2_types.Instance) (instance Instance) {
	instance = Instance{
		Architecture:          fmt.Sprint(iv.Architecture),
		ImageID:               aws_v2.ToString(iv.ImageId),
		InstanceID:            aws_v2.ToString(iv.InstanceId),
		InstanceType:          fmt.Sprint(iv.InstanceType),
		KeyName:               aws_v2.ToString(iv.KeyName),
		PrivateDNSName:        aws_v2.ToString(iv.PrivateDnsName),
		PrivateIP:             aws_v2.ToString(iv.PrivateIpAddress),
		PublicDNSName:         aws_v2.ToString(iv.PublicDnsName),
		PublicIP:              aws_v2.ToString(iv.PublicIpAddress),
		StateTransitionReason: aws_v2.ToString(iv.StateTransitionReason),
		SubnetID:              aws_v2.ToString(iv.SubnetId),
		VPCID:                 aws_v2.ToString(iv.VpcId),
		BlockDeviceMappings:   make([]BlockDeviceMapping, len(iv.BlockDeviceMappings)),
		EBSOptimized:          aws_v2.ToBool(iv.EbsOptimized),
		RootDeviceName:        aws_v2.ToString(iv.RootDeviceName),
		RootDeviceType:        fmt.Sprint(iv.RootDeviceType),
		SecurityGroups:        make([]SecurityGroup, len(iv.SecurityGroups)),
		LaunchTime:            aws_v2.ToTime(iv.LaunchTime),
		Hypervisor:            fmt.Sprint(iv.Hypervisor),
		VirtualizationType:    fmt.Sprint(iv.VirtualizationType),
	}
	for j := range iv.BlockDeviceMappings {
		instance.BlockDeviceMappings[j] = BlockDeviceMapping{
			DeviceName: aws_v2.ToString(iv.BlockDeviceMappings[j].DeviceName),
			EBS: EBS{
				DeleteOnTermination: aws_v2.ToBool(iv.BlockDeviceMappings[j].Ebs.DeleteOnTermination),
				Status:              fmt.Sprint(iv.BlockDeviceMappings[j].Ebs.Status),
				VolumeID:            aws_v2.ToString(iv.BlockDeviceMappings[j].Ebs.VolumeId),
			},
		}
	}
	for j := range iv.SecurityGroups {
		instance.SecurityGroups[j] = SecurityGroup{
			GroupName: aws_v2.ToString(iv.SecurityGroups[j].GroupName),
			GroupID:   aws_v2.ToString(iv.SecurityGroups[j].GroupId),
		}
	}
	if iv.IamInstanceProfile != nil {
		instance.IAMInstanceProfile = IAMInstanceProfile{
			ARN: aws_v2.ToString(iv.IamInstanceProfile.Arn),
			ID:  aws_v2.ToString(iv.IamInstanceProfile.Id),
		}
	}
	if iv.Placement != nil {
		instance.Placement = Placement{
			AvailabilityZone: aws_v2.ToString(iv.Placement.AvailabilityZone),
			Tenancy:          fmt.Sprint(iv.Placement.Tenancy),
		}
	}
	if iv.State != nil {
		instance.State = State{
			Code: aws_v2.ToInt32(iv.State.Code),
			Name: fmt.Sprint(iv.State.Name),
		}
	}
	if iv.StateReason != nil {
		instance.StateReason = StateReason{
			Code:    aws_v2.ToString(iv.StateReason.Code),
			Message: aws_v2.ToString(iv.StateReason.Message),
		}
	}
	if iv.CpuOptions != nil {
		instance.CPUOptions = CPUOptions{
			CoreCount:      aws_v2.ToInt32(iv.CpuOptions.CoreCount),
			ThreadsPerCore: aws_v2.ToInt32(iv.CpuOptions.ThreadsPerCore),
		}
	}
	return instance
}
