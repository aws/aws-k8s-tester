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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/mitchellh/colorstring"
	"sigs.k8s.io/yaml" // must use "sigs.k8s.io/yaml"
)

// AWS_K8S_TESTER_EC2_PREFIX is the environment variable prefix used for "ec2config".
const AWS_K8S_TESTER_EC2_PREFIX = "AWS_K8S_TESTER_EC2_"

const (
	// AMITypeBottleRocketCPU is the AMI type for Bottlerocket OS.
	// https://github.com/bottlerocket-os/bottlerocket
	AMITypeBottleRocketCPU = "BOTTLEROCKET_x86_64"
	// AMITypeAL2X8664 is the AMI type for Amazon Linux 2 AMI.
	AMITypeAL2X8664 = "AL2_x86_64"
	// AMITypeAL2X8664GPU is the AMI type for Amazon Linux 2 AMI with GPU.
	AMITypeAL2X8664GPU = "AL2_x86_64_GPU"

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
	Status []Status `json:"status"`

	// Name is the cluster name.
	// If empty, deployer auto-populates it.
	Name string `json:"name"`
	// Partition is the AWS partition for EC2 deployment region.
	// If empty, set default partition "aws".
	Partition string `json:"partition"`
	// Region is the AWS geographic area for EC2 deployment.
	// If empty, set default region.
	Region string `json:"region"`

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
	// LogColorOverride is true to use "LogColor" setting
	// even if the current terminal does not support color outputs.
	// Useful to output in color in HTML based log outputs (e.g. Prow).
	LogColorOverride bool `json:"log-color-override"`
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

	// S3BucketCreate is true to auto-create S3 bucket.
	S3BucketCreate bool `json:"s3-bucket-create"`
	// S3BucketCreateKeep is true to not delete auto-created S3 bucket.
	// The created S3 bucket is kept.
	S3BucketCreateKeep bool `json:"s3-bucket-create-keep"`
	// S3BucketName is the name of cluster S3.
	S3BucketName string `json:"s3-bucket-name"`
	// S3BucketLifecycleExpirationDays is expiration in days for the lifecycle of the object.
	S3BucketLifecycleExpirationDays int64 `json:"s3-bucket-lifecycle-expiration-days"`

	// S3Dir is the S3 directory to store all test results.
	// It is under the bucket "eksconfig.Config.S3BucketName".
	S3Dir string `json:"s3-dir"`

	// RoleName is the name of cluster role.
	RoleName string `json:"role-name"`
	// RoleCreate is true to auto-create and delete cluster role.
	RoleCreate bool `json:"role-create"`
	// RoleARN is the role ARN that EC2 uses to create AWS resources for Kubernetes.
	// By default, it's empty which triggers tester to create one.
	RoleARN string `json:"role-arn"`
	// RoleServicePrincipals is the EC2 Role Service Principals
	RoleServicePrincipals []string `json:"role-service-principals"`
	// RoleManagedPolicyARNs is EC2 Role managed policy ARNs.
	RoleManagedPolicyARNs []string `json:"role-managed-policy-arns"`
	RoleCFNStackID        string   `json:"role-cfn-stack-id" read-only:"true"`
	RoleCFNStackYAMLPath  string   `json:"role-cfn-stack-yaml-path" read-only:"true"`
	RoleCFNStackYAMLS3Key string   `json:"role-cfn-stack-yaml-s3-key" read-only:"true"`

	// VPCCreate is true to auto-create and delete VPC.
	VPCCreate bool `json:"vpc-create"`
	// VPCID is the VPC ID for cluster creation.
	// If not empty, VPC is reused and not deleted.
	// If empty, VPC is created anew and deleted on cluster deletion.
	VPCID                string `json:"vpc-id"`
	VPCCFNStackID        string `json:"vpc-cfn-stack-id" read-only:"true"`
	VPCCFNStackYAMLPath  string `json:"vpc-cfn-stack-yaml-path" read-only:"true"`
	VPCCFNStackYAMLS3Key string `json:"vpc-cfn-stack-yaml-s3-key" read-only:"true"`
	// SSHIngressIPv4Range is the IP range for SSH inbound traffic.
	SSHIngressIPv4Range string `json:"ssh-ingress-ipv4-range"`
	// VpcCIDR is the IP range (CIDR notation) for VPC, must be a valid private
	// (RFC 1918) CIDR range.
	VPCCIDR string `json:"vpc-cidr,omitempty"`
	// PublicSubnetCIDR1 is the CIDR Block for subnet 1 within the VPC.
	PublicSubnetCIDR1 string `json:"public-subnet-cidr-1,omitempty"`
	// PublicSubnetCIDR2 is the CIDR Block for subnet 2 within the VPC.
	PublicSubnetCIDR2 string `json:"public-subnet-cidr-2,omitempty"`
	// PublicSubnetCIDR3 is the CIDR Block for subnet 3 within the VPC.
	PublicSubnetCIDR3 string `json:"public-subnet-cidr-3,omitempty"`
	// PrivateSubnetCIDR1 is the CIDR Block for subnet 1 within the VPC.
	PrivateSubnetCIDR1 string `json:"private-subnet-cidr-1,omitempty"`
	// PrivateSubnetCIDR2 is the CIDR Block for subnet 2 within the VPC.
	PrivateSubnetCIDR2 string `json:"private-subnet-cidr-2,omitempty"`
	// PublicSubnetIDs is the list of all public subnets in the VPC.
	PublicSubnetIDs []string `json:"public-subnet-ids" read-only:"true"`
	// PrivateSubnetIDs is the list of all private subnets in the VPC.
	PrivateSubnetIDs []string `json:"private-subnet-ids" read-only:"true"`

	// DHCPOptionsDomainName is used to complete unqualified DNS hostnames for VPC.
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-dhcp-options.html
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/cluster-endpoint.html
	DHCPOptionsDomainName string `json:"dhcp-options-domain-name"`
	// DHCPOptionsDomainNameServers is a list of strings.
	// The IPv4 addresses of up to four domain name servers, or AmazonProvidedDNS, for VPC.
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-dhcp-options.html
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/cluster-endpoint.html
	DHCPOptionsDomainNameServers []string `json:"dhcp-options-domain-name-servers"`

	// SecurityGroupID is the security group ID for the VPC.
	SecurityGroupID string `json:"security-group-id" read-only:"true"`

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

	ASGCFNStackID        string `json:"asg-cfn-stack-id" read-only:"true"`
	ASGCFNStackYAMLPath  string `json:"asg-cfn-stack-yaml-path" read-only:"true"`
	ASGCFNStackYAMLS3Key string `json:"asg-cfn-stack-yaml-s3-key" read-only:"true"`

	TimeFrameCreate timeutil.TimeFrame `json:"time-frame-create" read-only:"true"`
	TimeFrameDelete timeutil.TimeFrame `json:"time-frame-delete" read-only:"true"`

	// RemoteAccessUserName is the user name used for running init scripts or SSH access.
	RemoteAccessUserName string `json:"remote-access-user-name"`

	// SSMDocumentCreate is true to auto-create and delete SSM document.
	SSMDocumentCreate bool `json:"ssm-document-create"`
	// SSMDocumentName is the name of SSM document.
	SSMDocumentName string `json:"ssm-document-name"`
	// SSMDocumentCFNStackName is the name of SSM document CFN stack.
	SSMDocumentCFNStackName string `json:"ssm-document-cfn-stack-name"`
	// SSMDocumentCommands is the commands for SSM document.
	// Only used if SSM doc is created.
	SSMDocumentCommands string `json:"ssm-document-commands"`
	// SSMDocumentExecutionTimeoutSeconds is the SSM document execution timeout in seconds.
	SSMDocumentExecutionTimeoutSeconds int      `json:"ssm-document-execution-timeout-in-seconds"`
	SSMDocumentCFNStackID              string   `json:"ssm-document-cfn-stack-id" read-only:"true"`
	SSMDocumentCFNStackYAMLPath        string   `json:"ssm-document-cfn-stack-yaml-path" read-only:"true"`
	SSMDocumentCFNStackYAMLS3Key       string   `json:"ssm-document-cfn-stack-yaml-s3-key" read-only:"true"`
	SSMDocumentCommandIDs              []string `json:"ssm-document-command-ids" read-only:"true"`

	// TODO: support bootstrap arguments
	// ref. https://github.com/awslabs/amazon-eks-ami/blob/master/amazon-eks-nodegroup.yaml

	// AMIType is the AMI type for the node group.
	// Allowed values are BOTTLEROCKET_x86_64, AL2_x86_64 and AL2_x86_64_GPU.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/launch-workers.html
	// ref. https://github.com/awslabs/amazon-eks-ami/blob/master/amazon-eks-nodegroup.yaml
	AMIType string `json:"ami-type,omitempty"`
	// ImageID is the Amazon Machine Image (AMI).
	// This value overrides any AWS Systems Manager Parameter Store value.
	ImageID string `json:"image-id"`
	// ImageIDSSMParameter is the AWS Systems Manager Parameter Store
	// parameter of the AMI ID.
	ImageIDSSMParameter string `json:"image-id-ssm-parameter"`

	// InstanceTypes is the list of EC2 instance types.
	InstanceTypes []string `json:"instance-types"`
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
	VolumeSize int64 `json:"volume-size"`

	// ASGMinSize is the minimum size of ASG.
	ASGMinSize int64 `json:"asg-min-size,omitempty"`
	// ASGMaxSize is the maximum size of ASG.
	ASGMaxSize int64 `json:"asg-max-size,omitempty"`
	// ASGDesiredCapacity is the desired capacity of ASG.
	ASGDesiredCapacity int64 `json:"asg-desired-capacity,omitempty"`

	// Instances is a map from instance ID to instance.
	Instances map[string]Instance `json:"instanaces" read-only:"true"`
	// Logs maps each instance ID to a list of log file paths fetched via SSH access.
	Logs map[string][]string `json:"logs" read-only:"true"`
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
	CoreCount int64 `json:"core-count"`
	// ThreadsPerCore is the number of threads per CPU core.
	ThreadsPerCore int64 `json:"threads-per-core"`
}

// Placement defines EC2 placement.
type Placement struct {
	AvailabilityZone string `json:"availability-zone"`
	Tenancy          string `json:"tenancy"`
}

// State defines an EC2 state.
type State struct {
	Code int64  `json:"code"`
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
	if err = yaml.Unmarshal(d, cfg); err != nil {
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
func ConvertInstance(iv *ec2.Instance) (instance Instance) {
	instance = Instance{
		Architecture:          aws.StringValue(iv.Architecture),
		ImageID:               aws.StringValue(iv.ImageId),
		InstanceID:            aws.StringValue(iv.InstanceId),
		InstanceType:          aws.StringValue(iv.InstanceType),
		KeyName:               aws.StringValue(iv.KeyName),
		PrivateDNSName:        aws.StringValue(iv.PrivateDnsName),
		PrivateIP:             aws.StringValue(iv.PrivateIpAddress),
		PublicDNSName:         aws.StringValue(iv.PublicDnsName),
		PublicIP:              aws.StringValue(iv.PublicIpAddress),
		StateTransitionReason: aws.StringValue(iv.StateTransitionReason),
		SubnetID:              aws.StringValue(iv.SubnetId),
		VPCID:                 aws.StringValue(iv.VpcId),
		BlockDeviceMappings:   make([]BlockDeviceMapping, len(iv.BlockDeviceMappings)),
		EBSOptimized:          aws.BoolValue(iv.EbsOptimized),
		RootDeviceName:        aws.StringValue(iv.RootDeviceName),
		RootDeviceType:        aws.StringValue(iv.RootDeviceType),
		SecurityGroups:        make([]SecurityGroup, len(iv.SecurityGroups)),
		LaunchTime:            aws.TimeValue(iv.LaunchTime),
		Hypervisor:            aws.StringValue(iv.Hypervisor),
		VirtualizationType:    aws.StringValue(iv.VirtualizationType),
	}
	for j := range iv.BlockDeviceMappings {
		instance.BlockDeviceMappings[j] = BlockDeviceMapping{
			DeviceName: aws.StringValue(iv.BlockDeviceMappings[j].DeviceName),
			EBS: EBS{
				DeleteOnTermination: aws.BoolValue(iv.BlockDeviceMappings[j].Ebs.DeleteOnTermination),
				Status:              aws.StringValue(iv.BlockDeviceMappings[j].Ebs.Status),
				VolumeID:            aws.StringValue(iv.BlockDeviceMappings[j].Ebs.VolumeId),
			},
		}
	}
	for j := range iv.SecurityGroups {
		instance.SecurityGroups[j] = SecurityGroup{
			GroupName: aws.StringValue(iv.SecurityGroups[j].GroupName),
			GroupID:   aws.StringValue(iv.SecurityGroups[j].GroupId),
		}
	}
	if iv.IamInstanceProfile != nil {
		instance.IAMInstanceProfile = IAMInstanceProfile{
			ARN: aws.StringValue(iv.IamInstanceProfile.Arn),
			ID:  aws.StringValue(iv.IamInstanceProfile.Id),
		}
	}
	if iv.Placement != nil {
		instance.Placement = Placement{
			AvailabilityZone: aws.StringValue(iv.Placement.AvailabilityZone),
			Tenancy:          aws.StringValue(iv.Placement.Tenancy),
		}
	}
	if iv.State != nil {
		instance.State = State{
			Code: aws.Int64Value(iv.State.Code),
			Name: aws.StringValue(iv.State.Name),
		}
	}
	if iv.StateReason != nil {
		instance.StateReason = StateReason{
			Code:    aws.StringValue(iv.StateReason.Code),
			Message: aws.StringValue(iv.StateReason.Message),
		}
	}
	if iv.CpuOptions != nil {
		instance.CPUOptions = CPUOptions{
			CoreCount:      aws.Int64Value(iv.CpuOptions.CoreCount),
			ThreadsPerCore: aws.Int64Value(iv.CpuOptions.ThreadsPerCore),
		}
	}
	return instance
}
