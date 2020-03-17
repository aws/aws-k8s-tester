// Package ec2config defines EC2 configuration.
package ec2config

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"sigs.k8s.io/yaml"
)

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

	// Name is the name of EC2 tester.
	Name string `json:"name"`
	// ConfigPath is the configuration file path.
	// Deployer is expected to update this file with latest status.
	ConfigPath string `json:"config-path,omitempty"`
	// Region is the AWS geographic area for EC2 deployment.
	// If empty, set default region.
	Region string `json:"region,omitempty"`

	// AWSAccountID is the account ID of the eks tester caller session.
	AWSAccountID string `json:"aws-account-id" read-only:"true"`
	// AWSUserID is the user ID of the eks tester caller session.
	AWSUserID string `json:"aws-user-id" read-only:"true"`
	// AWSIAMRoleARN is the user IAM Role ARN of the eks tester caller session.
	AWSIAMRoleARN string `json:"aws-iam-role-arn" read-only:"true"`
	// AWSCredentialPath is automatically set via AWS SDK Go.
	// And to be mounted as a volume as 'Secret' object.
	AWSCredentialPath string `json:"aws-credential-path" read-only:"true"`

	// CreateTook is the duration that took to create the resource.
	CreateTook time.Duration `json:"create-took,omitempty" read-only:"true"`
	// CreateTookString is the duration that took to create the resource.
	CreateTookString string `json:"create-took-string,omitempty" read-only:"true"`
	// DeleteTook is the duration that took to create the resource.
	DeleteTook time.Duration `json:"delete-took,omitempty" read-only:"true"`
	// DeleteTookString is the duration that took to create the resource.
	DeleteTookString string `json:"delete-took-string,omitempty" read-only:"true"`

	// LogLevel configures log level. Only supports debug, info, warn, error, panic, or fatal. Default 'info'.
	LogLevel string `json:"log-level"`
	// LogOutputs is a list of log outputs. Valid values are 'default', 'stderr', 'stdout', or file names.
	// Logs are appended to the existing file, if any.
	// Multiple values are accepted. If empty, it sets to 'default', which outputs to stderr.
	// See https://pkg.go.dev/go.uber.org/zap#Open and https://pkg.go.dev/go.uber.org/zap#Config for more details.
	LogOutputs []string `json:"log-outputs,omitempty"`

	// Up is true if the cluster is up.
	Up bool `json:"up"`
	// StatusCurrent represents the current status of the cluster.
	StatusCurrent string `json:"status-current"`
	// Status represents the status of the cluster.
	Status []Status `json:"status"`

	// OnFailureDelete is true to delete all resources on creation fail.
	OnFailureDelete bool `json:"on-failure-delete"`
	// OnFailureDeleteWaitSeconds is the seconds to wait before deleting
	// all resources on creation fail.
	OnFailureDeleteWaitSeconds uint64 `json:"on-failure-delete-wait-seconds"`

	// S3BucketName is the name of cluster S3.
	S3BucketName string `json:"s3-bucket-name"`
	// S3BucketCreate is true to auto-create S3 bucket.
	S3BucketCreate bool `json:"s3-bucket-create"`
	// S3BucketLifecycleExpirationDays is expiration in days for the lifecycle of the object
	S3BucketLifecycleExpirationDays int64 `json:"s3-bucket-lifecycle-expiration-days"`

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

	// VPCCreate is true to auto-create and delete VPC.
	VPCCreate bool `json:"vpc-create"`
	// VPCID is the VPC ID for cluster creation.
	// If not empty, VPC is reused and not deleted.
	// If empty, VPC is created anew and deleted on cluster deletion.
	VPCID         string `json:"vpc-id"`
	VPCCFNStackID string `json:"vpc-cfn-stack-id" read-only:"true"`
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
	Name          string `json:"name"`
	ASGCFNStackID string `json:"asg-cfn-stack-id" read-only:"true"`

	// CreateTook is the duration that took to create the resource.
	CreateTook time.Duration `json:"create-took,omitempty" read-only:"true"`
	// CreateTookString is the duration that took to create the resource.
	CreateTookString string `json:"create-took-string,omitempty" read-only:"true"`
	// DeleteTook is the duration that took to create the resource.
	DeleteTook time.Duration `json:"delete-took,omitempty" read-only:"true"`
	// DeleteTookString is the duration that took to create the resource.
	DeleteTookString string `json:"delete-took-string,omitempty" read-only:"true"`

	// RemoteAccessUserName is the user name used for running init scripts or SSH access.
	RemoteAccessUserName string `json:"remote-access-user-name"`

	// SSMDocumentName is the name of SSM document.
	SSMDocumentName string `json:"ssm-document-name"`
	// SSMDocumentCreate is true to auto-create and delete SSM document.
	SSMDocumentCreate bool `json:"ssm-document-create"`
	// SSMDocumentCommands is the commands for SSM document.
	// Only used if SSM doc is created.
	SSMDocumentCommands string `json:"ssm-document-commands"`
	// SSMDocumentExecutionTimeoutSeconds is the SSM document execution timeout in seconds.
	SSMDocumentExecutionTimeoutSeconds int    `json:"ssm-document-execution-timeout-in-seconds"`
	SSMDocumentCFNStackID              string `json:"ssm-document-cfn-stack-id" read-only:"true"`
	SSMDocumentCommandID               string `json:"ssm-document-command-id" read-only:"true"`

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
	ImageID              string               `json:"image-id"`
	InstanceID           string               `json:"instance-id"`
	InstanceType         string               `json:"instance-type"`
	KeyName              string               `json:"key-name"`
	Placement            Placement            `json:"placement"`
	PrivateDNSName       string               `json:"private-dns-name"`
	PrivateIP            string               `json:"private-ip"`
	PublicDNSName        string               `json:"public-dns-name"`
	PublicIP             string               `json:"public-ip"`
	State                State                `json:"state"`
	SubnetID             string               `json:"subnet-id"`
	VPCID                string               `json:"vpc-id"`
	BlockDeviceMappings  []BlockDeviceMapping `json:"block-device-mappings"`
	EBSOptimized         bool                 `json:"ebs-optimized"`
	RootDeviceName       string               `json:"root-device-name"`
	RootDeviceType       string               `json:"root-device-type"`
	SecurityGroups       []SecurityGroup      `json:"security-groups"`
	LaunchTime           time.Time            `json:"launch-time"`
	RemoteAccessUserName string               `json:"remote-access-user-name"`
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
	for name, asg := range cfg.ASGs {
		buf.WriteString("ASG name \"" + name + "\":\n")
		buf.WriteString(asg.SSHCommands(cfg.Region, cfg.RemoteAccessPrivateKeyPath, asg.RemoteAccessUserName))
		buf.WriteString("\n")
	}
	return buf.String()
}

// SSHCommands returns the SSH commands.
func (asg *ASG) SSHCommands(region string, keyPath string, userName string) (s string) {
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

	return s + "\n"
}

// ConvertInstance converts "aws ec2 describe-instances" to "config.Instance".
func ConvertInstance(iv *ec2.Instance) (instance Instance) {
	instance = Instance{
		ImageID:      *iv.ImageId,
		InstanceID:   *iv.InstanceId,
		InstanceType: *iv.InstanceType,
		KeyName:      *iv.KeyName,
		Placement: Placement{
			AvailabilityZone: *iv.Placement.AvailabilityZone,
			Tenancy:          *iv.Placement.Tenancy,
		},
		PrivateDNSName: *iv.PrivateDnsName,
		PrivateIP:      *iv.PrivateIpAddress,
		State: State{
			Code: *iv.State.Code,
			Name: *iv.State.Name,
		},
		SubnetID:            *iv.SubnetId,
		VPCID:               *iv.VpcId,
		BlockDeviceMappings: make([]BlockDeviceMapping, len(iv.BlockDeviceMappings)),
		EBSOptimized:        *iv.EbsOptimized,
		RootDeviceName:      *iv.RootDeviceName,
		RootDeviceType:      *iv.RootDeviceType,
		SecurityGroups:      make([]SecurityGroup, len(iv.SecurityGroups)),
		LaunchTime:          *iv.LaunchTime,
	}
	if iv.PublicDnsName != nil {
		instance.PublicDNSName = *iv.PublicDnsName
	}
	if iv.PublicIpAddress != nil {
		instance.PublicIP = *iv.PublicIpAddress
	}
	for j := range iv.BlockDeviceMappings {
		instance.BlockDeviceMappings[j] = BlockDeviceMapping{
			DeviceName: *iv.BlockDeviceMappings[j].DeviceName,
			EBS: EBS{
				DeleteOnTermination: *iv.BlockDeviceMappings[j].Ebs.DeleteOnTermination,
				Status:              *iv.BlockDeviceMappings[j].Ebs.Status,
				VolumeID:            *iv.BlockDeviceMappings[j].Ebs.VolumeId,
			},
		}
	}
	for j := range iv.SecurityGroups {
		instance.SecurityGroups[j] = SecurityGroup{
			GroupName: *iv.SecurityGroups[j].GroupName,
			GroupID:   *iv.SecurityGroups[j].GroupId,
		}
	}
	return instance
}
