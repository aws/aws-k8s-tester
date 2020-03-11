// Package eksconfig defines EKS test configuration.
package eksconfig

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	awseks "github.com/aws/aws-sdk-go/service/eks"
	"sigs.k8s.io/yaml"
)

// Config defines EKS configuration.
type Config struct {
	mu sync.RWMutex

	// ConfigPath is the configuration file path.
	// Deployer is expected to update this file with latest status.
	ConfigPath string `json:"config-path,omitempty"`
	// KubectlCommandsOutputPath is the output path for kubectl commands.
	KubectlCommandsOutputPath string `json:"kubectl-commands-output-path,omitempty"`
	// RemoteAccessCommandsOutputPath is the output path for ssh commands.
	RemoteAccessCommandsOutputPath string `json:"remote-access-commands-output-path,omitempty"`

	// Region is the AWS geographic area for EKS deployment.
	// If empty, set default region.
	Region string `json:"region,omitempty"`
	// Name is the cluster name.
	// If empty, deployer auto-populates it.
	Name string `json:"name,omitempty"`

	// LogLevel configures log level. Only supports debug, info, warn, error, panic, or fatal. Default 'info'.
	LogLevel string `json:"log-level"`
	// LogOutputs is a list of log outputs. Valid values are 'default', 'stderr', 'stdout', or file names.
	// Logs are appended to the existing file, if any.
	// Multiple values are accepted. If empty, it sets to 'default', which outputs to stderr.
	// See https://pkg.go.dev/go.uber.org/zap#Open and https://pkg.go.dev/go.uber.org/zap#Config for more details.
	LogOutputs []string `json:"log-outputs,omitempty"`

	// AWSCLIPath is the path for AWS CLI path.
	AWSCLIPath string `json:"aws-cli-path,omitempty"`

	// KubectlPath is the path to download the "kubectl".
	KubectlPath string `json:"kubectl-path,omitempty"`
	// KubectlDownloadURL is the download URL to download "kubectl" binary from.
	// https://docs.aws.amazon.com/eks/latest/userguide/install-kubectl.html
	KubectlDownloadURL string `json:"kubectl-download-url,omitempty"`
	// KubeConfigPath is the file path of KUBECONFIG for the EKS cluster.
	// If empty, auto-generate one.
	// Deployer is expected to delete this on cluster tear down.
	KubeConfigPath string `json:"kubeconfig-path,omitempty"`

	// AWSIAMAuthenticatorPath is the path to aws-iam-authenticator.
	AWSIAMAuthenticatorPath string `json:"aws-iam-authenticator-path,omitempty"`
	// AWSIAMAuthenticatorDownloadURL is the download URL to download "aws-iam-authenticator" binary from.
	AWSIAMAuthenticatorDownloadURL string `json:"aws-iam-authenticator-download-url,omitempty"`

	// OnFailureDelete is true to delete all resources on creation fail.
	OnFailureDelete bool `json:"on-failure-delete"`
	// OnFailureDeleteWaitSeconds is the seconds to wait before deleting
	// all resources on creation fail.
	OnFailureDeleteWaitSeconds uint64 `json:"on-failure-delete-wait-seconds"`

	// CommandAfterCreateCluster is the command to execute after creating clusters.
	CommandAfterCreateCluster           string `json:"command-after-create-cluster,omitempty"`
	CommandAfterCreateClusterOutputPath string `json:"command-after-create-cluster-output-path,omitempty" read-only:"true"`
	// CommandAfterCreateAddOns is the command to execute after creating clusters and add-ons.
	CommandAfterCreateAddOns           string `json:"command-after-create-add-ons,omitempty"`
	CommandAfterCreateAddOnsOutputPath string `json:"command-after-create-add-ons-output-path,omitempty" read-only:"true"`

	// Parameters defines EKS "cluster" creation parameters.
	// It's ok to leave any parameters empty.
	// If empty, it will use default values.
	Parameters *Parameters `json:"parameters,omitempty"`

	// RemoteAccessKeyCreate is true to create the remote SSH access private key.
	RemoteAccessKeyCreate bool `json:"remote-access-key-create"`
	// RemoteAccessKeyName is the key name for node group SSH EC2 key pair.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	RemoteAccessKeyName string `json:"remote-access-key-name,omitempty"`
	// RemoteAccessPrivateKeyPath is the file path to store node group key pair private key.
	// Thus, deployer must delete the private key right after node group creation.
	// MAKE SURE PRIVATE KEY NEVER GETS UPLOADED TO CLOUD STORAGE AND DELETE AFTER USE!!!
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	RemoteAccessPrivateKeyPath string `json:"remote-access-private-key-path,omitempty"`

	// AddOnManagedNodeGroups defines EKS "Managed Node Group"
	// creation parameters. If empty, it will use default values.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	AddOnManagedNodeGroups *AddOnManagedNodeGroups `json:"add-on-managed-node-groups,omitempty"`

	// AddOnNLBHelloWorld defines parameters for EKS cluster
	// add-on NLB hello-world service.
	AddOnNLBHelloWorld *AddOnNLBHelloWorld `json:"add-on-nlb-hello-world,omitempty"`
	// AddOnALB2048 defines parameters for EKS cluster
	// add-on ALB 2048 service.
	AddOnALB2048 *AddOnALB2048 `json:"add-on-alb-2048,omitempty"`
	// AddOnJobPi defines parameters for EKS cluster
	// add-on Job with pi Perl command.
	AddOnJobPi *AddOnJobPi `json:"add-on-job-pi,omitempty"`
	// AddOnJobEcho defines parameters for EKS cluster
	// add-on Job with echo.
	AddOnJobEcho *AddOnJobEcho `json:"add-on-job-echo,omitempty"`
	// AddOnCronJob defines parameters for EKS cluster
	// add-on with CronJob.
	AddOnCronJob *AddOnCronJob `json:"add-on-cron-job,omitempty"`
	// AddOnSecrets defines parameters for EKS cluster
	// add-on "Secrets".
	AddOnSecrets *AddOnSecrets `json:"add-on-secrets,omitempty"`
	// AddOnIRSA defines parameters for EKS cluster
	// add-on "IAM Roles for Service Accounts (IRSA)".
	AddOnIRSA *AddOnIRSA `json:"add-on-irsa,omitempty"`
	// AddOnFargate defines parameters for EKS cluster
	// add-on "EKS on Fargate".
	AddOnFargate *AddOnFargate `json:"add-on-fargate,omitempty"`
	// AddOnAppMesh defines parameters for EKS cluster
	// add-on "EKS App Mesh Integration".
	AddOnAppMesh *AddOnAppMesh `json:"add-on-app-mesh,omitempty"`

	// Status represents the current status of AWS resources.
	// Status is read-only.
	// Status cannot be configured via environmental variables.
	Status *Status `json:"status,omitempty" read-only:"true"`
	// StatusManagedNodeGroups represents EKS "Managed Node Group" status.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	StatusManagedNodeGroups *StatusManagedNodeGroups `json:"status-managed-node-groups" read-only:"true"`
}

// Parameters defines parameters for EKS "cluster" creation.
type Parameters struct {
	// RoleName is the name of cluster role.
	RoleName string `json:"role-name"`
	// RoleCreate is true to auto-create and delete cluster role.
	RoleCreate bool `json:"role-create"`
	// RoleARN is the role ARN that EKS uses to create AWS resources for Kubernetes.
	// By default, it's empty which triggers tester to create one.
	RoleARN string `json:"role-arn"`
	// RoleServicePrincipals is the EKS Role Service Principals
	RoleServicePrincipals []string `json:"role-service-principals"`
	// RoleManagedPolicyARNs is EKS Role managed policy ARNs.
	RoleManagedPolicyARNs []string `json:"role-managed-policy-arns"`
	RoleCFNStackID        string   `json:"role-cfn-stack-id" read-only:"true"`

	// Tags defines EKS create cluster tags.
	Tags map[string]string `json:"tags"`
	// RequestHeaderKey defines EKS create cluster request header key.
	RequestHeaderKey string `json:"request-header-key"`
	// RequestHeaderValue defines EKS create cluster request header value.
	RequestHeaderValue string `json:"request-header-value"`

	// ResolverURL defines an AWS resolver endpoint for EKS API.
	// Must be left empty to use production EKS service.
	ResolverURL string `json:"resolver-url"`
	// SigningName is the EKS create request signing name.
	SigningName string `json:"signing-name"`

	// VPCCreate is true to auto-create and delete VPC.
	VPCCreate bool `json:"vpc-create"`
	// VPCID is the VPC ID for cluster creation.
	// If not empty, VPC is reused and not deleted.
	// If empty, VPC is created anew and deleted on cluster deletion.
	VPCID         string `json:"vpc-id"`
	VPCCFNStackID string `json:"vpc-cfn-stack-id" read-only:"true"`
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
	// ControlPlaneSecurityGroupID is the security group ID for the cluster control
	// plane communication with worker nodes.
	ControlPlaneSecurityGroupID string `json:"control-plane-security-group-id" read-only:"true"`

	// Version is the version of EKS Kubernetes "cluster".
	// If empty, set default version.
	Version      string  `json:"version"`
	VersionValue float64 `json:"version-value" read-only:"true"`

	// EncryptionCMKCreate is true to auto-create and delete KMS CMK
	// for encryption feature.
	EncryptionCMKCreate bool `json:"encryption-cmk-create"`
	// EncryptionCMKARN is the KMS CMK ARN for encryption feature.
	// If not empty, the cluster is created with encryption feature
	// enabled.
	EncryptionCMKARN string `json:"encryption-cmk-arn"`
}

func (cfg *Config) IsAddOnManagedNodeGroupsEnabled() bool {
	return cfg.AddOnManagedNodeGroups != nil && cfg.AddOnManagedNodeGroups.Enable
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

	// RemoteAccessUserName is the user name for managed node group SSH access.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	RemoteAccessUserName string `json:"remote-access-user-name,omitempty"`

	// LogsDir is set to specify the target directory to store all remote log files.
	// If empty, it stores in the same directory as "ConfigPath".
	LogsDir string `json:"logs-dir,omitempty"`
	// MNGs maps from EKS Managed Node Group name to "MNG".
	MNGs map[string]MNG `json:"mngs,omitempty"`
}

// MNG represents parameters for one EKS "Managed Node Group".
type MNG struct {
	// Name is the name of the managed node group.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	Name string `json:"name,omitempty"`
	// Tags defines EKS managed node group create tags.
	Tags map[string]string `json:"tags,omitempty"`
	// ReleaseVersion is the AMI version of the Amazon EKS-optimized AMI for the node group.
	// The version may differ from EKS "cluster" version.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
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
}

func (cfg *Config) IsAddOnNLBHelloWorldEnabled() bool {
	return cfg.AddOnNLBHelloWorld != nil && cfg.AddOnNLBHelloWorld.Enable
}

// AddOnNLBHelloWorld defines parameters for EKS cluster
// add-on NLB hello-world service.
type AddOnNLBHelloWorld struct {
	// Enable is 'true' to create this add-on.
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

	// DeploymentReplicas is the number of replicas to deploy using "Deployment" object.
	DeploymentReplicas int32 `json:"deployment-replicas"`
	// Namespace is the namespace to create "NLB" objects in.
	Namespace string `json:"namespace"`

	// NLBARN is the ARN of the NLB created from the service.
	NLBARN string `json:"nlb-arn" read-only:"true"`
	// NLBName is the name of the NLB created from the service.
	NLBName string `json:"nlb-name" read-only:"true"`
	// URL is the host name for hello-world service.
	URL string `json:"url" read-only:"true"`
}

func (cfg *Config) IsAddOnALB2048Enabled() bool {
	return cfg.AddOnALB2048 != nil && cfg.AddOnALB2048.Enable
}

// AddOnALB2048 defines parameters for EKS cluster
// add-on ALB 2048 service.
type AddOnALB2048 struct {
	// Enable is 'true' to create this add-on.
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

	// Namespace is the namespace to create "ALB" objects in.
	Namespace string `json:"namespace"`
	// DeploymentReplicasALB is the number of ALB replicas to deploy using "Deployment" object.
	DeploymentReplicasALB int32 `json:"deployment-replicas-alb"`
	// DeploymentReplicas2048 is the number of 2048 replicas to deploy using "Deployment" object.
	DeploymentReplicas2048 int32 `json:"deployment-replicas-2048"`

	// ALBARN is the ARN of the ALB created from the service.
	ALBARN string `json:"alb-arn" read-only:"true"`
	// ALBName is the name of the ALB created from the service.
	ALBName string `json:"alb-name" read-only:"true"`
	// URL is the URL for ALB 2048 Service.
	URL string `json:"url" read-only:"true"`
}

func (cfg *Config) IsAddOnJobPiEnabled() bool {
	return cfg.AddOnJobPi != nil && cfg.AddOnJobPi.Enable
}

// AddOnJobPi defines parameters for EKS cluster
// add-on Job with Perl.
type AddOnJobPi struct {
	// Enable is 'true' to create this add-on.
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

	// Namespace is the namespace to create "Job" objects in.
	Namespace string `json:"namespace"`

	// Completes is the desired number of successfully finished pods.
	Completes int `json:"completes"`
	// Parallels is the the maximum desired number of pods the
	// job should run at any given time.
	Parallels int `json:"parallels"`
}

func (cfg *Config) IsAddOnJobEchoEnabled() bool {
	return cfg.AddOnJobEcho != nil && cfg.AddOnJobEcho.Enable
}

// AddOnJobEcho defines parameters for EKS cluster
// add-on Job with echo.
type AddOnJobEcho struct {
	// Enable is 'true' to create this add-on.
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

	// Namespace is the namespace to create "Job" objects in.
	Namespace string `json:"namespace"`

	// Completes is the desired number of successfully finished pods.
	Completes int `json:"completes"`
	// Parallels is the the maximum desired number of pods the
	// job should run at any given time.
	Parallels int `json:"parallels"`
	// EchoSize is the job object size in bytes.
	// "Request entity too large: limit is 3145728" (3.1 MB).
	// "The Job "echo" is invalid: metadata.annotations:
	// Too long: must have at most 262144 characters". (0.26 MB)
	EchoSize int `json:"echo-size"`
}

func (cfg *Config) IsAddOnCronJobEnabled() bool {
	return cfg.AddOnCronJob != nil && cfg.AddOnCronJob.Enable
}

// AddOnCronJob defines parameters for EKS cluster
// add-on with CronJob.
type AddOnCronJob struct {
	// Enable is 'true' to create this add-on.
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

	// Namespace is the namespace to create "Job" objects in.
	Namespace string `json:"namespace"`

	// Schedule is the cron schedule (e.g. "*/1 * * * *").
	Schedule string `json:"schedule"`
	// Completes is the desired number of successfully finished pods.
	Completes int `json:"completes"`
	// Parallels is the the maximum desired number of pods the
	// job should run at any given time.
	Parallels int `json:"parallels"`
	// SuccessfulJobsHistoryLimit is the number of successful finished
	// jobs to retain. Defaults to 3.
	SuccessfulJobsHistoryLimit int32 `json:"successful-jobs-history-limit"`
	// FailedJobsHistoryLimit is the number of failed finished jobs
	// to retain. Defaults to 1.
	FailedJobsHistoryLimit int32 `json:"failed-jobs-history-limit"`

	// EchoSize is the job object size in bytes.
	// "Request entity too large: limit is 3145728" (3.1 MB).
	// "The Job "echo" is invalid: metadata.annotations:
	// Too long: must have at most 262144 characters". (0.26 MB)
	EchoSize int `json:"echo-size"`
}

func (cfg *Config) IsAddOnSecretsEnabled() bool {
	return cfg.AddOnSecrets != nil && cfg.AddOnSecrets.Enable
}

// AddOnSecrets defines parameters for EKS cluster
// add-on "Secrets".
type AddOnSecrets struct {
	// Enable is 'true' to create this add-on.
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

	// Namespace is the namespace to create "Secret" and "Pod" objects in.
	Namespace string `json:"namespace"`

	// Objects is the number of "Secret" objects to write/read.
	Objects int `json:"objects"`
	// Size is the "Secret" value size in bytes.
	Size int `json:"size"`

	// SecretQPS is the number of "Secret" create requests to send
	// per second. Requests may be throttled by kube-apiserver.
	// Default rate limits from kube-apiserver are:
	// FLAG: --max-mutating-requests-inflight="200"
	// FLAG: --max-requests-inflight="400"
	SecretQPS uint `json:"secret-qps"`
	// SecretBurst is the number of "Secret" create requests that
	// a client can make in excess of the rate specified by the limiter.
	// Requests may be throttled by kube-apiserver.
	// Default rate limits from kube-apiserver are:
	// FLAG: --max-mutating-requests-inflight="200"
	// FLAG: --max-requests-inflight="400"
	SecretBurst uint `json:"secret-burst"`
	// CreatedSecretNames is the list of created "Secret" object names.
	CreatedSecretNames []string `json:"created-secret-names" read-only:"true"`

	// PodQPS is the number of "Pod" create requests to send
	// per second. Requests may be throttled by kube-apiserver.
	// Default rate limits from kube-apiserver are:
	// FLAG: --max-mutating-requests-inflight="200"
	// FLAG: --max-requests-inflight="400"
	PodQPS uint `json:"pod-qps"`
	// PodBurst is the number of "Pod" create requests that
	// a client can make in excess of the rate specified by the limiter.
	// Requests may be throttled by kube-apiserver.
	// Default rate limits from kube-apiserver are:
	// FLAG: --max-mutating-requests-inflight="200"
	// FLAG: --max-requests-inflight="400"
	PodBurst uint `json:"pod-burst"`
	// CreatedPodNames is the list of created "Pod" object names.
	CreatedPodNames []string `json:"created-pod-names" read-only:"true"`

	// WritesResultPath is the CSV file path to output Secret writes test results.
	WritesResultPath string `json:"writes-result-path"`
	// ReadsResultPath is the CSV file path to output Secret reads test results.
	ReadsResultPath string `json:"reads-result-path"`
}

func (cfg *Config) IsAddOnIRSAEnabled() bool {
	return cfg.AddOnIRSA != nil && cfg.AddOnIRSA.Enable
}

// AddOnIRSA defines parameters for EKS cluster
// add-on "IAM Roles for Service Accounts (IRSA)".
type AddOnIRSA struct {
	// Enable is 'true' to create this add-on.
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

	// Namespace is the namespace to create "Secret" and "Pod" objects in.
	Namespace string `json:"namespace"`

	// RoleName is the role name for IRSA.
	RoleName string `json:"role-name"`
	// RoleARN is the role ARN for IRSA.
	RoleARN string `json:"role-arn"`
	// RoleManagedPolicyARNs is IRSA role managed policy ARNs.
	// ref. https://aws.amazon.com/blogs/opensource/introducing-fine-grained-iam-roles-service-accounts/
	RoleManagedPolicyARNs []string `json:"role-managed-policy-arns"`
	RoleCFNStackID        string   `json:"role-cfn-stack-id" read-only:"true"`

	// ServiceAccountName is the ServiceAccount name.
	ServiceAccountName string `json:"service-account-name"`
	// ConfigMapName is the ConfigMap name.
	ConfigMapName string `json:"config-map-name"`
	// ConfigMapScriptFileName is the ConfigMap script name.
	ConfigMapScriptFileName string `json:"config-map-script-file-name"`
	// S3BucketName is the S3 bucket for IRSA tests.
	S3BucketName string `json:"s3-bucket-name"`
	// S3Key is the S3 key to write for IRSA tests.
	S3Key string `json:"s3-key"`

	// DeploymentName is the Deployment name.
	DeploymentName string `json:"deployment-name"`
	// DeploymentReplicas is the number of Deployment replicas.
	DeploymentReplicas int32 `json:"deployment-replicas"`
	// DeploymentResultPath is the output of "Deployment" run.
	DeploymentResultPath string `json:"deployment-result-path"`
	// DeploymentTook is the duration that took for Deployment resource.
	DeploymentTook time.Duration `json:"deployment-took,omitempty" read-only:"true"`
	// DeploymentTookString is the duration that took for Deployment resource.
	DeploymentTookString string `json:"deployment-took-string,omitempty" read-only:"true"`
}

func (cfg *Config) IsAddOnFargateEnabled() bool {
	return cfg.AddOnFargate != nil && cfg.AddOnFargate.Enable
}

// AddOnFargate defines parameters for EKS cluster
// add-on "EKS on Fargate".
type AddOnFargate struct {
	// Enable is 'true' to create this add-on.
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

	// Namespace is the namespace to create "Secret" and "Pod" objects in.
	Namespace string `json:"namespace"`

	// RoleName is the role name for Fargate.
	RoleName string `json:"role-name"`
	// RoleCreate is true to auto-create and delete role.
	RoleCreate bool `json:"role-create"`
	// RoleARN is the role ARN for Fargate.
	RoleARN string `json:"role-arn"`
	// RoleServicePrincipals is the Fargate role Service Principals
	RoleServicePrincipals []string `json:"role-service-principals"`
	// RoleManagedPolicyARNs is Fargate role managed policy ARNs.
	RoleManagedPolicyARNs []string `json:"role-managed-policy-arns"`
	RoleCFNStackID        string   `json:"role-cfn-stack-id" read-only:"true"`

	// ProfileName is the profile name for Fargate.
	ProfileName string `json:"profile-name"`
	// SecretName is the secret name for Fargate.
	SecretName string `json:"secret-name"`
	// PodName is the name of the Fargate Pod.
	PodName string `json:"pod-name"`
	// ContainerName is the name of the Fargate container.
	ContainerName string `json:"container-name"`
}

func (cfg *Config) IsAddOnAppMeshEnabled() bool {
	return cfg.AddOnAppMesh != nil && cfg.AddOnAppMesh.Enable
}

// AddOnAppMesh defines parameters for EKS cluster
// add-on "EKS App Mesh Integration".
type AddOnAppMesh struct {
	// Enable is 'true' to create this add-on.
	Enable bool `json:"enable"`

	// Namespace is the namespace to create "AppMesh" controller/injector.
	Namespace string `json:"namespace"`

	// The image of appMesh controller
	ControllerImage string `json:"controller-image"`

	// The image of appMesh injector
	InjectorImage string `json:"injector-image"`

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

	// AddOnStackARN is the arn of cloudFormation to create the resource.
	AddOnCFNStackARN string `json:"add-on-cfn-stack-arn,omitempty" read-only:"true"`
}

// Status represents the current status of AWS resources.
// Read-only. Cannot be configured via environmental variables.
type Status struct {
	// Up is true if the cluster is up.
	Up bool `json:"up"`

	// CreateTook is the duration that took to create the resource.
	CreateTook time.Duration `json:"create-took,omitempty" read-only:"true"`
	// CreateTookString is the duration that took to create the resource.
	CreateTookString string `json:"create-took-string,omitempty" read-only:"true"`
	// DeleteTook is the duration that took to create the resource.
	DeleteTook time.Duration `json:"delete-took,omitempty" read-only:"true"`
	// DeleteTookString is the duration that took to create the resource.
	DeleteTookString string `json:"delete-took-string,omitempty" read-only:"true"`

	// AWSAccountID is the account ID of the eks tester caller session.
	AWSAccountID string `json:"aws-account-id"`
	// AWSUserID is the user ID of the eks tester caller session.
	AWSUserID string `json:"aws-user-id"`
	// AWSIAMRoleARN is the user IAM Role ARN of the eks tester caller session.
	AWSIAMRoleARN string `json:"aws-iam-role-arn"`
	// AWSCredentialPath is automatically set via AWS SDK Go.
	// And to be mounted as a volume as 'Secret' object.
	AWSCredentialPath string `json:"aws-credential-path"`

	ClusterCFNStackID string `json:"cluster-cfn-stack-id"`
	ClusterARN        string `json:"cluster-arn"`
	// ClusterAPIServerEndpoint is the cluster endpoint of the EKS cluster,
	// required for KUBECONFIG write.
	ClusterAPIServerEndpoint string `json:"cluster-api-server-endpoint"`
	// ClusterOIDCIssuerURL is the issuer URL for the OpenID Connect
	// (https://openid.net/connect/) identity provider .
	ClusterOIDCIssuerURL string `json:"cluster-oidc-issuer-url"`
	// ClusterOIDCIssuerHostPath is the issuer host path.
	ClusterOIDCIssuerHostPath string `json:"cluster-oidc-issuer-host-path"`
	// ClusterOIDCIssuerARN is the issuer ARN for the OpenID Connect
	// (https://openid.net/connect/) identity provider .
	ClusterOIDCIssuerARN string `json:"cluster-oidc-issuer-arn"`
	// ClusterOIDCIssuerCAThumbprint is the issuer CA thumbprint.
	ClusterOIDCIssuerCAThumbprint string `json:"cluster-oidc-issuer-ca-thumbprint"`

	// ClusterCA is the EKS cluster CA, required for KUBECONFIG write.
	ClusterCA string `json:"cluster-ca"`
	// ClusterCADecoded is the decoded EKS cluster CA, required for k8s.io/client-go.
	ClusterCADecoded string `json:"cluster-ca-decoded"`

	// ClusterStatusCurrent represents the current status of the cluster.
	ClusterStatusCurrent string `json:"cluster-status-current"`
	// ClusterStatus represents the status of the cluster.
	ClusterStatus []ClusterStatus `json:"cluster-status"`
}

// ClusterStatus represents the cluster status.
type ClusterStatus struct {
	Time   time.Time `json:"time"`
	Status string    `json:"status"`
}

// ClusterStatusDELETEDORNOTEXIST defines the cluster status when the cluster is not found.
//
// ref. https://docs.aws.amazon.com/eks/latest/APIReference/API_Cluster.html#AmazonEKS-Type-Cluster-status
//
//  CREATING
//  ACTIVE
//  UPDATING
//  DELETING
//  FAILED
//
const ClusterStatusDELETEDORNOTEXIST = "DELETED/NOT-EXIST"

// RecordStatus records cluster status.
func (cfg *Config) RecordStatus(status string) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	cfg.Status.ClusterStatusCurrent = status
	switch status {
	case ClusterStatusDELETEDORNOTEXIST:
		cfg.Status.Up = false
	case awseks.ClusterStatusActive:
		cfg.Status.Up = true
	}

	sv := ClusterStatus{Time: time.Now(), Status: status}
	n := len(cfg.Status.ClusterStatus)
	if n == 0 {
		cfg.Status.ClusterStatus = []ClusterStatus{sv}
		cfg.unsafeSync()
		return
	}

	copied := make([]ClusterStatus, n+1)
	copy(copied[1:], cfg.Status.ClusterStatus)
	copied[0] = sv
	cfg.Status.ClusterStatus = copied
	cfg.unsafeSync()
}

// StatusManagedNodeGroups represents the status of EKS "Managed Node Group".
// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
type StatusManagedNodeGroups struct {
	// CreateTook is the duration that took to create the resource.
	CreateTook time.Duration `json:"create-took,omitempty" read-only:"true"`
	// CreateTookString is the duration that took to create the resource.
	CreateTookString string `json:"create-took-string,omitempty" read-only:"true"`
	// DeleteTook is the duration that took to create the resource.
	DeleteTook time.Duration `json:"delete-took,omitempty" read-only:"true"`
	// DeleteTookString is the duration that took to create the resource.
	DeleteTookString string `json:"delete-took-string,omitempty" read-only:"true"`

	// NvidiaDriverInstalled is true if nvidia driver has been installed.
	NvidiaDriverInstalled bool `json:"nvidia-driver-installed"`
	// Nodes maps from EKS "Managed Node Group" name to its status.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	Nodes map[string]StatusManagedNodeGroup `json:"nodes"`
}

// StatusManagedNodeGroup represents the status of EKS "Managed Node Group".
// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
type StatusManagedNodeGroup struct {
	// CreateRequested is true if "CreateNodegroupRequest" has been sent.
	CreateRequested bool `json:"create-requested"`
	// CFNStackID is the CloudFormation stack ID for a managed node group.
	CFNStackID string `json:"cfn-stack-id"`

	RemoteAccessSecurityGroupID string `json:"remote-access-security-group-id"`

	// PhysicalID is the Physical ID for the created "AWS::EKS::Nodegroup".
	PhysicalID string `json:"physical-id"`

	// Status is the current status of EKS "Managed Node Group".
	Status string `json:"status"`

	// Instances maps an instance ID to an EC2 instance object for the node group.
	Instances map[string]ec2config.Instance `json:"instances"`
	// Logs maps each instance ID to a list of log file paths fetched via SSH access.
	Logs map[string][]string `json:"logs"`
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
	if cfg.KubeConfigPath != "" && !filepath.IsAbs(cfg.KubeConfigPath) {
		p, err = filepath.Abs(cfg.KubeConfigPath)
		if err != nil {
			return fmt.Errorf("failed to 'filepath.Abs(%s)' %v", cfg.KubeConfigPath, err)
		}
		cfg.KubeConfigPath = p
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

	err = ioutil.WriteFile(cfg.KubectlCommandsOutputPath, []byte(cmdTop+cfg.KubectlCommands()), 0600)
	if err != nil {
		return fmt.Errorf("failed to write file %q (%v)", cfg.KubectlCommandsOutputPath, err)
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

// KubectlCommand returns the kubectl command.
func (cfg *Config) KubectlCommand() string {
	return fmt.Sprintf("%s --kubeconfig=%s", cfg.KubectlPath, cfg.KubeConfigPath)
}

// KubectlCommands returns the various kubectl commands.
func (cfg *Config) KubectlCommands() (s string) {
	if cfg.KubeConfigPath == "" {
		return ""
	}
	tpl := template.Must(template.New("kubectlCmdTmpl").Parse(kubectlCmdTmpl))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, struct {
		KubectlPath    string
		KubeConfigPath string
		KubectlCmd     string
		Version        string
	}{
		cfg.KubectlPath,
		cfg.KubeConfigPath,
		fmt.Sprintf("%s --kubeconfig=%s", cfg.KubectlPath, cfg.KubeConfigPath),
		cfg.Parameters.Version,
	}); err != nil {
		return ""
	}
	return buf.String()
}

const kubectlCmdTmpl = `# kubectl commands
export KUBECONFIG={{ .KubeConfigPath }}
export KUBECTL="{{ .KubectlCmd }}"
${KUBECTL} version
${KUBECTL} cluster-info

export KUBECONFIG={{ .KubeConfigPath }}
export KUBECTL="{{ .KubectlCmd }}"
${KUBECTL} get cs
${KUBECTL} get nodes

export KUBECONFIG={{ .KubeConfigPath }}
export KUBECTL="{{ .KubectlCmd }}"
${KUBECTL} --namespace=kube-system get pods

export KUBECONFIG={{ .KubeConfigPath }}
export KUBECTL="{{ .KubectlCmd }}"
${KUBECTL} get pods

export KUBECONFIG={{ .KubeConfigPath }}
export KUBECTL="{{ .KubectlCmd }}"
${KUBECTL} --namespace=kube-system get ds

export KUBECONFIG={{ .KubeConfigPath }}
export KUBECTL="{{ .KubectlCmd }}"
${KUBECTL} get secrets --all-namespaces

export KUBECONFIG={{ .KubeConfigPath }}
export KUBECTL="{{ .KubectlCmd }}"
${KUBECTL} get configmap --all-namespaces

export KUBECONFIG={{ .KubeConfigPath }}
export KUBECTL="{{ .KubectlCmd }}"
${KUBECTL} get all --all-namespaces

# sonobuoy commands
go get -v -u github.com/heptio/sonobuoy

sonobuoy delete --wait --kubeconfig={{ .KubeConfigPath }}
sonobuoy run \
  --mode Quick \
  --wait \
  --kube-conformance-image gcr.io/heptio-images/kube-conformance:v{{ .Version }}.0 \
  --kubeconfig={{ .KubeConfigPath }}

sonobuoy delete --wait --kubeconfig={{ .KubeConfigPath }}
sonobuoy run \
  --wait \
  --kube-conformance-image gcr.io/heptio-images/kube-conformance:v{{ .Version }}.0 \
  --kubeconfig={{ .KubeConfigPath }}

sonobuoy status --kubeconfig={{ .KubeConfigPath }}

results=$(sonobuoy retrieve --kubeconfig={{ .KubeConfigPath }})
sonobuoy e2e --kubeconfig={{ .KubeConfigPath }} $results --show all
sonobuoy e2e --kubeconfig={{ .KubeConfigPath }} $results
`

// SSHCommands returns the SSH commands.
func (cfg *Config) SSHCommands() string {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	return cfg.unsafeSSHCommands()
}

func (cfg *Config) unsafeSSHCommands() (s string) {
	if cfg.StatusManagedNodeGroups == nil {
		return ""
	}
	if len(cfg.StatusManagedNodeGroups.Nodes) == 0 {
		return ""
	}
	buf := bytes.NewBuffer(nil)
	for name, ng := range cfg.StatusManagedNodeGroups.Nodes {
		buf.WriteString("ASG name \"" + name + "\":\n")
		asg := &ec2config.ASG{
			Name:      name,
			Instances: ng.Instances,
		}
		buf.WriteString(asg.SSHCommands(cfg.Region, cfg.RemoteAccessPrivateKeyPath, cfg.AddOnManagedNodeGroups.RemoteAccessUserName))
		buf.WriteString("\n")
	}
	return buf.String()
}
