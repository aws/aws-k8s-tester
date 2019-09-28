// Package eksconfig defines EKS test configuration.
package eksconfig

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/pkg/awsapi"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/yaml"
)

// Config defines EKS test configuration.
type Config struct {
	// Tag is the tag used for S3 bucket name.
	// If empty, deployer auto-populates it.
	Tag string `json:"tag,omitempty"`
	// ClusterName is the cluster name.
	// If empty, deployer auto-populates it.
	ClusterName string `json:"cluster-name,omitempty"`

	// EKSTags defines EKS create cluster tags.
	EKSTags map[string]string `json:"eks-tags,omitempty"`
	// EKSRequestHeader defines EKS create cluster request header.
	EKSRequestHeader map[string]string `json:"eks-request-header,omitempty"`
	// EKSResolverURL defines an AWS resolver endpoint for EKS.
	// Must be left empty to use production EKS service.
	EKSResolverURL string `json:"eks-resolver-url"`
	// EKSSigningName is the EKS create request signing name.
	EKSSigningName string `json:"eks-signing-name"`

	// AWSK8sTesterPath is the path to download the "aws-k8s-tester".
	// This is required for Kubernetes kubetest plugin.
	AWSK8sTesterPath string `json:"aws-k8s-tester-path,omitempty"`
	// AWSK8sTesterDownloadURL is the download URL to download "aws-k8s-tester" binary from.
	// It's only used for "kubetest" deployer interface.
	AWSK8sTesterDownloadURL string `json:"aws-k8s-tester-download-url,omitempty"`
	// KubectlPath is the path to download the "kubectl".
	KubectlPath string `json:"kubectl-path,omitempty"`
	// KubectlDownloadURL is the download URL to download "kubectl" binary from.
	// https://docs.aws.amazon.com/eks/latest/userguide/install-kubectl.html
	KubectlDownloadURL string `json:"kubectl-download-url,omitempty"`
	// AWSIAMAuthenticatorPath is the path to download the "aws-iam-authenticator".
	// This is required for Kubernetes kubetest plugin.
	AWSIAMAuthenticatorPath string `json:"aws-iam-authenticator-path,omitempty"`
	// AWSIAMAuthenticatorDownloadURL is the download URL to download "aws-iam-authenticator" binary from.
	AWSIAMAuthenticatorDownloadURL string `json:"aws-iam-authenticator-download-url,omitempty"`

	// ConfigPath is the configuration file path.
	// Must be left empty, and let deployer auto-populate this field.
	// Deployer is expected to update this file with latest status,
	// and to make a backup of original configuration
	// with the filename suffix ".backup.yaml" in the same directory.
	ConfigPath string `json:"config-path,omitempty"`
	// ConfigPathBucket is the path inside S3 bucket.
	ConfigPathBucket string `json:"config-path-bucket,omitempty"` // read-only to user
	ConfigPathURL    string `json:"config-path-url,omitempty"`    // read-only to user

	// KubeConfigPath is the file path of KUBECONFIG for the EKS cluster.
	// If empty, auto-generate one.
	// Deployer is expected to delete this on cluster tear down.
	KubeConfigPath string `json:"kubeconfig-path,omitempty"` // read-only to user
	// KubeConfigPathBucket is the path inside S3 bucket.
	KubeConfigPathBucket string `json:"kubeconfig-path-bucket,omitempty"` // read-only to user
	KubeConfigPathURL    string `json:"kubeconfig-path-url,omitempty"`    // read-only to user

	// DestroyAfterCreate is true to automatically tear down cluster.
	DestroyAfterCreate bool `json:"destroy-after-create"`
	// DestroyWaitTime is the duration to sleep before cluster tear down.
	// Be ignored if "DestroyAfterCreate" is false.
	DestroyWaitTime time.Duration `json:"destroy-wait-time,omitempty"`

	// AWSAccountID is the AWS account ID.
	AWSAccountID string `json:"aws-account-id,omitempty"`
	// AWSCredentialToMountPath is the file path to AWS credential.
	// Required for AWS ALB Ingress Controller deployments and other AWS specific tests.
	// If not empty, deployer is expected to mount the file as a secret object "aws-cred-aws-k8s-tester",
	// to the path "/etc/aws-cred-aws-k8s-tester/aws-cred-aws-k8s-tester", under "kube-system" namespace.
	// Path must be an absolute path, although it will try to parse '~/.aws' or '${HOME}/.aws'.
	// If "AWS_SHARED_CREDENTIALS_FILE" is specified, this field will overwritten.
	AWSCredentialToMountPath string `json:"aws-credential-to-mount-path,omitempty"`
	// AWSRegion is the AWS geographic area for EKS deployment.
	// If empty, set default region.
	AWSRegion string `json:"aws-region,omitempty"`

	// EnableWorkerNodeSSH is true to enable SSH access to worker nodes.
	EnableWorkerNodeSSH bool `json:"enable-worker-node-ssh"`
	// EnableWorkerNodeHA is true to use all 3 subnets to create worker nodes.
	// Note that at least 2 subnets are required for EKS cluster.
	EnableWorkerNodeHA bool `json:"enable-worker-node-ha"`
	// EnableWorkerNodePrivilegedPortAccess is true to allow control plane to
	// talk to worker nodes through their privileged ports (i.e ports 1-1024).
	EnableWorkerNodePrivilegedPortAccess bool `json:"enable-worker-node-privileged-port-access"`

	// VPCID is the VPC ID.
	VPCID string `json:"vpc-id"`
	// SubnetIDs is the subnet IDs.
	SubnetIDs []string `json:"subnet-ids"`
	// SecurityGroupID is the default security group ID.
	SecurityGroupID string `json:"security-group-id"`

	// WorkerNodeCFTemplatePath is the file path of worker node template.
	// If empty, download https://raw.githubusercontent.com/awslabs/amazon-eks-ami/master/amazon-eks-nodegroup.yaml.
	WorkerNodeCFTemplatePath string `json:"worker-node-cf-template-path,omitempty"`
	// WorkerNodeCFTemplateAdditionalParameterKeys defines a list of additional cloudformation parameter keys.
	WorkerNodeCFTemplateAdditionalParameterKeys []string `json:"worker-node-cf-template-additional-parameter-keys,omitempty"`
	// WorkerNodePrivateKeyPath is the file path to store node group key pair private key.
	// Thus, deployer must delete the private key right after node group creation.
	// MAKE SURE PRIVATE KEY NEVER GETS UPLOADED TO CLOUD STORAGE AND DELETE AFTER USE!!!
	WorkerNodePrivateKeyPath string `json:"worker-node-private-key-path"`
	// WorkerNodeAMIType is either "amazon-linux-2" or "amazon-linux-2-gpu".
	// Be ignored if "WorkerNodeAMIID" is specified.
	// Must be non-empty if "WorkerNodeAMIID" is NOT specified.
	WorkerNodeAMIType string `json:"worker-node-ami-type"`
	// WorkerNodeUserName is the user name for worker node SSH access.
	WorkerNodeUserName string `json:"worker-node-user-name"`
	// WorkerNodeAMIID is the Amazon EKS worker node AMI ID for the specified Region.
	// Reference https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html.
	// Leave empty to auto-populate from SSM parameter.
	WorkerNodeAMIID string `json:"worker-node-ami-id"`
	// WorkerNodeAMIName is the name of the worker node AMI.
	// Leave empty to auto-populate from SSM parameter.
	WorkerNodeAMIName string `json:"worker-node-ami-name"`
	// WorkerNodeInstanceType is the EC2 instance type for worker nodes.
	WorkerNodeInstanceType string `json:"worker-node-instance-type"`
	// WorkerNodeASGMin is the minimum number of nodes in worker node ASG.
	WorkerNodeASGMin int `json:"worker-node-asg-min,omitempty"`
	// WorkerNodeASGMax is the maximum number of nodes in worker node ASG.
	WorkerNodeASGMax int `json:"worker-node-asg-max,omitempty"`
	// WorkerNodeASGDesiredCapacity is the desired capacity of Node Group ASG.
	WorkerNodeASGDesiredCapacity int `json:"worker-node-asg-desired-capacity,omitempty"`

	// WorkerNodeVolumeSizeGB is the maximum number of nodes in worker node ASG.
	// If empty, set default value.
	WorkerNodeVolumeSizeGB int `json:"worker-node-volume-size-gb,omitempty"`

	// KubernetesVersion is the version of Kubernetes cluster.
	// If empty, set default version.
	KubernetesVersion string `json:"kubernetes-version,omitempty"`
	// PlatformVersion is the platform version of EKS.
	// Read-only to user.
	PlatformVersion string `json:"platform-version,omitempty"`

	// LogLevel configures log level. Only supports debug, info, warn, error, panic, or fatal. Default 'info'.
	LogLevel string `json:"log-level"`
	// LogOutputs is a list of log outputs. Valid values are 'default', 'stderr', 'stdout', or file names.
	// Logs are appended to the existing file, if any.
	// Multiple values are accepted. If empty, it sets to 'default', which outputs to stderr.
	// See https://godoc.org/go.uber.org/zap#Open and https://godoc.org/go.uber.org/zap#Config for more details.
	LogOutputs []string `json:"log-outputs,omitempty"`
	// LogOutputToUploadPath is the aws-k8s-tester log file path to upload to cloud storage.
	// Must be left empty.
	// This will be overwritten by cluster name.
	LogOutputToUploadPath       string `json:"log-output-to-upload-path,omitempty"`
	LogOutputToUploadPathBucket string `json:"log-output-to-upload-path-bucket,omitempty"`
	LogOutputToUploadPathURL    string `json:"log-output-to-upload-path-url,omitempty"`

	// LogAccess is true to enable AWS API access logs (e.g. ALB access logs).
	// Automatically uploaded to S3 bucket named by cluster name.
	// https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-access-logs.html
	// https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/ingress-resources.md
	LogAccess bool `json:"log-access"`

	// UploadTesterLogs is true to auto-upload log files.
	UploadTesterLogs bool `json:"upload-tester-logs"`
	// UploadKubeConfig is true to auto-upload KUBECONFIG file.
	UploadKubeConfig bool `json:"upload-kubeconfig"`
	// UploadWorkerNodeLogs is true to auto-upload worker node log files.
	UploadWorkerNodeLogs bool `json:"upload-worker-node-logs"`
	// UploadBucketExpireDays is the number of days for a S3 bucket to expire.
	// Set 0 to not expire.
	UploadBucketExpireDays int `json:"upload-bucket-expire-days"`

	// UpdatedAt is the timestamp when the configuration has been updated.
	// Read only to 'Config' struct users.
	UpdatedAt time.Time `json:"updated-at,omitempty"` // read-only to user

	// ClusterState is the EKS status state.
	// Deployer is expected to keep this in sync.
	// Read-only to kubetest.
	ClusterState *ClusterState `json:"cluster-state,omitempty"`

	// CFStackVPCName is the name of VPC cloudformation stack.
	// Read-only. Only used to create a new one.
	CFStackVPCName string `json:"cf-stack-vpc-name,omitempty"`
	// CFStackVPCStatus is the last cloudformation status of VPC stack.
	CFStackVPCStatus string `json:"cf-stack-vpc-status,omitempty"`
	// CFStackVPCParameterVPCBlock is CIDR range for the VPC.
	// This should be a valid private (RFC 1918) CIDR range.
	CFStackVPCParameterVPCBlock      string `json:"cf-stack-vpc-parameter-vpc-block"`
	CFStackVPCParameterSubnet01Block string `json:"cf-stack-vpc-parameter-subnet-01-block"`
	CFStackVPCParameterSubnet02Block string `json:"cf-stack-vpc-parameter-subnet-02-block"`
	CFStackVPCParameterSubnet03Block string `json:"cf-stack-vpc-parameter-subnet-03-block"`
}

// ClusterState contains EKS cluster specific states.
// Deployer is expected to write and read this.
// Read-only to kubetest.
type ClusterState struct {
	// ClusterARN is the cluster ARN.
	ClusterARN string `json:"cluster-arn,omitempty"`

	// Status is the cluster status from EKS API.
	// It's either CREATING, ACTIVE, DELETING, FAILED, "DELETE_COMPLETE".
	// Reference: https://docs.aws.amazon.com/eks/latest/APIReference/API_Cluster.html#AmazonEKS-Type-Cluster-status.
	Status string `json:"status,omitempty"` // read-only to user

	StatusRoleCreated       bool `json:"status-role-created"`        // read-only to user
	StatusPolicyAttached    bool `json:"status-policy-attached"`     // read-only to user
	StatusVPCCreated        bool `json:"status-vpc-created"`         // read-only to user
	StatusClusterCreated    bool `json:"status-cluster-created"`     // read-only to user
	StatusKeyPairCreated    bool `json:"status-key-pair-created"`    // read-only to user
	StatusWorkerNodeCreated bool `json:"status-worker-node-created"` // read-only to user

	// Created is the timestamp of cluster creation.
	Created time.Time `json:"created,omitempty"` // read-only to user

	// UpTook is total duration that took to set up cluster up and running.
	// Does not include sub-project resource creation (e.g. ALB Ingress Controller).
	UpTook string        `json:"up-took,omitempty"` // read-only to user
	upTook time.Duration // read-only to user

	// ServiceRoleWithPolicyName is the name of the EKS cluster service role with policy.
	// Prefixed with cluster name and suffixed with 'SERVICE-ROLE'.
	ServiceRoleWithPolicyName string `json:"service-role-with-policy-name,omitempty"`
	// ServiceRolePolicies is the list of policy ARNs to create cluster service role with.
	ServiceRolePolicies []string `json:"service-role-policies,omitempty"`
	// ServiceRoleWithPolicyARN is the ARN of the created cluster service role.
	ServiceRoleWithPolicyARN string `json:"service-role-with-policy-arn,omitempty"`

	// Endpoint is the cluster endpoint of the EKS cluster, required for KUBECONFIG write.
	Endpoint string `json:"endpoint,omitempty"`
	// CA is the EKS cluster CA, required for KUBECONFIG write.
	CA string `json:"ca,omitempty"`
	// CADecoded is the decoded EKS cluster CA, required for k8s.io/client-go.
	CADecoded string `json:"ca-decoded,omitempty"`

	// WorkerNodeGroupStatus is the status Kubernetes worker node group.
	// "READY" when they successfully join the EKS cluster as worker nodes.
	WorkerNodeGroupStatus string `json:"worker-node-group-status,omitempty"`
	// WorkerNodes is a list of worker nodes.
	WorkerNodes map[string]ec2config.Instance `json:"worker-nodes,omitempty"`

	// WorkerNodeLogs is a list of worker node log file paths, fetched via SSH.
	WorkerNodeLogs map[string]string `json:"worker-node-logs,omitempty"`

	// CFStackWorkerNodeGroupName is the name of cloudformation stack for worker node group.
	CFStackWorkerNodeGroupName string `json:"cf-stack-worker-node-group-name,omitempty"`
	// CFStackWorkerNodeGroupStatus is the last cloudformation status of node group stack.
	CFStackWorkerNodeGroupStatus string `json:"cf-stack-worker-node-group-status,omitempty"`
	// CFStackWorkerNodeGroupKeyPairName is required for node group creation.
	CFStackWorkerNodeGroupKeyPairName string `json:"cf-stack-worker-node-group-key-pair-name,omitempty"`
	// CFStackWorkerNodeGroupSecurityGroupID is the security group ID
	// that worker node cloudformation stack created.
	CFStackWorkerNodeGroupSecurityGroupID string `json:"cf-stack-worker-node-group-security-group-id,omitempty"`
	// CFStackWorkerNodeGroupAutoScalingGroupName is the name of worker node auto scaling group.
	CFStackWorkerNodeGroupAutoScalingGroupName string `json:"cf-stack-worker-node-group-auto-scaling-group-name,omitempty"`

	// CFStackWorkerNodeGroupWorkerNodeInstanceRoleARN is the ARN of NodeInstance role of node group.
	// Required to enable worker nodes to join cluster.
	// Update this after creating node group stack
	CFStackWorkerNodeGroupWorkerNodeInstanceRoleARN string `json:"cf-stack-worker-node-group-worker-node-instance-role-arn,omitempty"`
}

// NewDefault returns a copy of the default configuration.
func NewDefault() *Config {
	vv := defaultConfig
	return &vv
}

// genTag generates a tag for cluster name, CloudFormation, and S3 bucket.
// Note that this would be used as S3 bucket name to upload tester logs.
func genTag() string {
	// use UTC time for everything
	now := time.Now().UTC()
	return fmt.Sprintf("eks-%d%02d%02d%02d", now.Year(), int(now.Month()), now.Day(), now.Hour())
}

func init() {
	if runtime.GOOS == "darwin" {
		defaultConfig.AWSK8sTesterDownloadURL = strings.Replace(defaultConfig.AWSK8sTesterDownloadURL, "linux", "darwin", -1)
		defaultConfig.KubectlDownloadURL = strings.Replace(defaultConfig.KubectlDownloadURL, "linux", "darwin", -1)
		defaultConfig.AWSIAMAuthenticatorDownloadURL = strings.Replace(defaultConfig.AWSIAMAuthenticatorDownloadURL, "linux", "darwin", -1)
		defaultConfig.WorkerNodePrivateKeyPath = filepath.Join(os.TempDir(), randString(12)+".insecure.key")
	}
	sshDir := filepath.Join(homedir.HomeDir(), ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		panic(fmt.Errorf("failed to mkdir %q (%v)", sshDir, err))
	}
}

// defaultConfig is the default configuration.
//  - empty string creates a non-nil object for pointer-type field
//  - omitting an entire field returns nil value
//  - make sure to check both
var defaultConfig = Config{
	EKSResolverURL: "",
	EKSSigningName: "eks",

	// https://github.com/aws/aws-k8s-tester/releases
	AWSK8sTesterDownloadURL: "https://github.com/aws/aws-k8s-tester/releases/download/v0.4.0/aws-k8s-tester-v0.4.0-linux-amd64",
	AWSK8sTesterPath:        "/tmp/aws-k8s-tester/aws-k8s-tester",

	// https://docs.aws.amazon.com/eks/latest/userguide/install-kubectl.html
	KubectlDownloadURL: "https://amazon-eks.s3-us-west-2.amazonaws.com/1.14.6/2019-08-22/bin/linux/amd64/kubectl",
	KubectlPath:        "/tmp/aws-k8s-tester/kubectl",

	KubeConfigPath: "/tmp/aws-k8s-tester/kubeconfig",

	AWSIAMAuthenticatorDownloadURL: "https://amazon-eks.s3-us-west-2.amazonaws.com/1.13.7/2019-06-11/bin/linux/amd64/aws-iam-authenticator",
	AWSIAMAuthenticatorPath:        "/tmp/aws-k8s-tester/aws-iam-authenticator",

	DestroyAfterCreate: false,
	DestroyWaitTime:    time.Minute,

	AWSAccountID: "",

	// to be overwritten by AWS_SHARED_CREDENTIALS_FILE
	AWSCredentialToMountPath: filepath.Join(homedir.HomeDir(), ".aws", "credentials"),
	AWSRegion:                "us-west-2",

	EnableWorkerNodeHA:                   true,
	EnableWorkerNodeSSH:                  true,
	EnableWorkerNodePrivilegedPortAccess: true,

	// keep in-sync with the default value in https://godoc.org/k8s.io/kubernetes/test/e2e/framework#GetSigner
	WorkerNodePrivateKeyPath:     filepath.Join(homedir.HomeDir(), ".ssh", "kube_aws_rsa"),
	WorkerNodeAMIType:            "amazon-linux-2",
	WorkerNodeUserName:           "ec2-user",
	WorkerNodeInstanceType:       "m3.xlarge",
	WorkerNodeASGMin:             1,
	WorkerNodeASGMax:             1,
	WorkerNodeASGDesiredCapacity: 1,
	WorkerNodeVolumeSizeGB:       20,

	KubernetesVersion: "1.14",

	LogLevel: logutil.DefaultLogLevel,
	// default, stderr, stdout, or file name
	// log file named with cluster name will be added automatically
	LogOutputs:             []string{"stderr"},
	LogAccess:              false,
	UploadTesterLogs:       false,
	UploadKubeConfig:       false,
	UploadWorkerNodeLogs:   false,
	UploadBucketExpireDays: 2,

	ClusterState: &ClusterState{},
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

	if cfg.ClusterState == nil {
		cfg.ClusterState = &ClusterState{}
	}

	if cfg.ConfigPath != p {
		cfg.ConfigPath = p
	}
	cfg.ConfigPath, err = filepath.Abs(p)
	if err != nil {
		return nil, err
	}

	if cfg.ClusterState.UpTook != "" {
		cfg.ClusterState.upTook, err = time.ParseDuration(cfg.ClusterState.UpTook)
		if err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

// Sync persists current configuration and states to disk.
func (cfg *Config) Sync() (err error) {
	if !filepath.IsAbs(cfg.ConfigPath) {
		cfg.ConfigPath, err = filepath.Abs(cfg.ConfigPath)
		if err != nil {
			return err
		}
	}

	cfg.UpdatedAt = time.Now().UTC()
	var d []byte
	d, err = yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(cfg.ConfigPath, d, 0600)
}

// ValidateAndSetDefaults returns an error for invalid configurations.
// And updates empty fields with default values.
// At the end, it writes populated YAML to aws-k8s-tester config path.
func (cfg *Config) ValidateAndSetDefaults() error {
	if len(cfg.LogOutputs) == 0 {
		return errors.New("LogOutputs is not empty")
	}
	if cfg.KubernetesVersion == "" {
		return errors.New("KubernetesVersion is empty")
	}
	if cfg.AWSRegion == "" {
		return errors.New("AWSRegion is empty")
	}
	if _, ok := awsapi.RegionToAiport[cfg.AWSRegion]; !ok {
		return fmt.Errorf("%q not found", cfg.AWSRegion)
	}
	if cfg.Tag == "" {
		cfg.Tag = genTag()
	}
	if cfg.ClusterName == "" {
		airport := awsapi.RegionToAiport[cfg.AWSRegion]
		cfg.ClusterName = cfg.Tag + "-" + strings.ToLower(airport) + "-" + cfg.AWSRegion + "-" + randString(5)
	}

	if cfg.EKSResolverURL != "" {
		switch cfg.EKSResolverURL {
		case "https://api.beta.us-west-2.wesley.amazonaws.com":
		}
	}

	if cfg.WorkerNodeUserName == "" {
		return errors.New("WorkerNodeUserName is empty")
	}

	// expect auto-populate from SSM
	if cfg.WorkerNodeAMIID == "" {
		if cfg.WorkerNodeAMIName != "" {
			return errors.New("WorkerNodeAMIID is not specified; but WorkerNodeAMIName is specified")
		}
		switch cfg.WorkerNodeAMIType {
		case "amazon-linux-2":
			if cfg.WorkerNodeUserName != "ec2-user" {
				return fmt.Errorf("unexpected WorkerNodeUserName %q", cfg.WorkerNodeUserName)
			}
		case "amazon-linux-2-gpu":
			if cfg.WorkerNodeUserName != "ec2-user" {
				return fmt.Errorf("unexpected WorkerNodeUserName %q", cfg.WorkerNodeUserName)
			}
		default:
			return fmt.Errorf("WorkerNodeAMIID is not specified; but unknown WorkerNodeAMIType %q", cfg.WorkerNodeAMIType)
		}
	}

	if cfg.WorkerNodeInstanceType == "" {
		return errors.New("WorkerNodeInstanceType is not specified")
	}
	if cfg.WorkerNodeASGMin == 0 {
		return errors.New("WorkerNodeASGMin is not specified")
	}
	if cfg.WorkerNodeASGMax == 0 {
		return errors.New("WorkerNodeASGMax is not specified")
	}
	if !checkWorkderNodeASG(cfg.WorkerNodeASGMin, cfg.WorkerNodeASGMax) {
		return fmt.Errorf("EKS WorkderNodeASG %d and %d is not valid", cfg.WorkerNodeASGMin, cfg.WorkerNodeASGMax)
	}
	if cfg.WorkerNodeVolumeSizeGB == 0 {
		cfg.WorkerNodeVolumeSizeGB = defaultWorkderNodeVolumeSizeGB
	}

	// resources created from aws-k8s-tester always follow
	// the same naming convention
	cfg.ClusterState.ServiceRoleWithPolicyName = genServiceRoleWithPolicy(cfg.ClusterName)
	cfg.ClusterState.ServiceRolePolicies = []string{serviceRolePolicyARNCluster, serviceRolePolicyARNService}
	cfg.CFStackVPCName = genCFStackVPC(cfg.ClusterName)
	if cfg.CFStackVPCParameterVPCBlock != "" ||
		cfg.CFStackVPCParameterSubnet01Block != "" ||
		cfg.CFStackVPCParameterSubnet02Block != "" ||
		cfg.CFStackVPCParameterSubnet03Block != "" {
		if cfg.CFStackVPCParameterVPCBlock == "" ||
			cfg.CFStackVPCParameterSubnet01Block == "" ||
			cfg.CFStackVPCParameterSubnet02Block == "" ||
			cfg.CFStackVPCParameterSubnet03Block == "" {
			return fmt.Errorf("CFStackVPC parameters must be all empty or non-empty (got %q | %q | %q | %q)",
				cfg.CFStackVPCParameterVPCBlock,
				cfg.CFStackVPCParameterSubnet01Block,
				cfg.CFStackVPCParameterSubnet02Block,
				cfg.CFStackVPCParameterSubnet03Block,
			)
		}
	}
	cfg.ClusterState.CFStackWorkerNodeGroupKeyPairName = genNodeGroupKeyPairName(cfg.ClusterName)
	// SECURITY NOTE: MAKE SURE PRIVATE KEY NEVER GETS UPLOADED TO CLOUD STORAGE AND DELETE AFTER USE!!!
	if cfg.WorkerNodePrivateKeyPath == "" {
		cfg.WorkerNodePrivateKeyPath = filepath.Join(
			os.TempDir(),
			cfg.ClusterState.CFStackWorkerNodeGroupKeyPairName+".private.key",
		)
	}
	cfg.ClusterState.CFStackWorkerNodeGroupName = genCFStackWorkerNodeGroup(cfg.ClusterName)

	////////////////////////////////////////////////////////////////////////
	// populate all paths on disks and on remote storage
	if cfg.ConfigPath == "" {
		f, err := ioutil.TempFile(os.TempDir(), "eks")
		if err != nil {
			return err
		}
		cfg.ConfigPath, _ = filepath.Abs(f.Name())
		f.Close()
		os.RemoveAll(cfg.ConfigPath)
	}
	cfg.ConfigPathBucket = filepath.Join(cfg.ClusterName, "eks.yaml")

	cfg.LogOutputToUploadPath = filepath.Join(os.TempDir(), fmt.Sprintf("%s.log", cfg.ClusterName))
	logOutputExist := false
	for _, lv := range cfg.LogOutputs {
		if cfg.LogOutputToUploadPath == lv {
			logOutputExist = true
			break
		}
	}
	if !logOutputExist {
		// auto-insert generated log output paths to zap logger output list
		cfg.LogOutputs = append(cfg.LogOutputs, cfg.LogOutputToUploadPath)
	}
	cfg.LogOutputToUploadPathBucket = filepath.Join(cfg.ClusterName, "eks.log")

	cfg.KubeConfigPathBucket = filepath.Join(cfg.ClusterName, "kubeconfig")
	////////////////////////////////////////////////////////////////////////

	if cfg.AWSCredentialToMountPath != "" && os.Getenv("AWS_SHARED_CREDENTIALS_FILE") == "" {
		p := cfg.AWSCredentialToMountPath
		if filepath.IsAbs(p) && !exist(p) {
			// TODO: if defined, overwrite/create credential file from environmental variables
			if os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
				return fmt.Errorf("'AWS_ACCESS_KEY_ID' and 'AWS_SECRET_ACCESS_KEY' are defined but not 'AWS_SHARED_CREDENTIALS_FILE', please write those environmental variables to the file %q, and try again", cfg.AWSCredentialToMountPath)
			}
			return fmt.Errorf("AWSCredentialToMountPath or 'AWS_SHARED_CREDENTIALS_FILE' %q does not exist on disk", cfg.AWSCredentialToMountPath)
		}

		// expand manually
		if strings.HasPrefix(p, "~/.aws") ||
			strings.HasPrefix(p, "$HOME/.aws") ||
			strings.HasPrefix(p, "${HOME}/.aws") {
			p = filepath.Join(homedir.HomeDir(), ".aws", filepath.Base(p))
		}
		if !exist(p) {
			// TODO: if defined, overwrite/create credential file from environmental variables
			if os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
				return fmt.Errorf("'AWS_ACCESS_KEY_ID' and 'AWS_SECRET_ACCESS_KEY' are defined but not 'AWS_SHARED_CREDENTIALS_FILE', please write those environmental variables to the file %q, and try again", cfg.AWSCredentialToMountPath)
			}
			return fmt.Errorf("AWSCredentialToMountPath or 'AWS_SHARED_CREDENTIALS_FILE' %q does not exist", p)
		}
		cfg.AWSCredentialToMountPath = p
	}

	// overwrite "AWSCredentialToMountPath" from env "AWS_SHARED_CREDENTIALS_FILE"
	if os.Getenv("AWS_SHARED_CREDENTIALS_FILE") != "" {
		p := os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
		var err error
		p, err = filepath.Abs(p)
		if err != nil {
			return fmt.Errorf("failed to expand AWS_SHARED_CREDENTIALS_FILE %q (%v)", p, err)
		}
		if !exist(p) {
			return fmt.Errorf("AWS_SHARED_CREDENTIALS_FILE %q does not exist", p)
		}
		cfg.AWSCredentialToMountPath = p
	}

	return cfg.Sync()
}

// SetClusterUpTook updates 'ClusterUpTook' field.
func (cfg *Config) SetClusterUpTook(d time.Duration) {
	cfg.ClusterState.upTook = d
	cfg.ClusterState.UpTook = d.String()
}

const envPfx = "AWS_K8S_TESTER_EKS_"

// UpdateFromEnvs updates fields from environmental variables.
func (cfg *Config) UpdateFromEnvs() error {
	cc := *cfg

	tp, vv := reflect.TypeOf(&cc).Elem(), reflect.ValueOf(&cc).Elem()
	for i := 0; i < tp.NumField(); i++ {
		jv := tp.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.Replace(jv, "-", "_", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := envPfx + jv
		if os.Getenv(env) == "" {
			continue
		}
		sv := os.Getenv(env)
		fieldName := tp.Field(i).Name

		switch vv.Field(i).Type().Kind() {
		case reflect.String:
			vv.Field(i).SetString(sv)

		case reflect.Map:
			switch fieldName {
			case "EKSTags",
				"EKSRequestHeader":
				vv.Field(i).Set(reflect.ValueOf(make(map[string]string)))
				for _, pair := range strings.Split(sv, ",") {
					fields := strings.Split(pair, "=")
					if len(fields) != 2 {
						return fmt.Errorf("map %q has unexpected format (e.g. should be 'a=b;c;d,e=f'", sv)
					}
					vv.Field(i).SetMapIndex(reflect.ValueOf(fields[0]), reflect.ValueOf(fields[1]))
				}

			default:
				return fmt.Errorf("parsing field name %q not supported", fieldName)
			}

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			if fieldName == "DestroyWaitTime" {
				dv, err := time.ParseDuration(sv)
				if err != nil {
					return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
				}
				vv.Field(i).SetInt(int64(dv))
				continue
			}
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv.Field(i).SetFloat(fv)

		case reflect.Slice:
			ss := strings.Split(sv, ",")
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for i := range ss {
				slice.Index(i).SetString(ss[i])
			}
			vv.Field(i).Set(slice)

		default:
			return fmt.Errorf("%q (%v) is not supported as an env", env, vv.Field(i).Type())
		}
	}
	*cfg = cc

	return nil
}

// SSHCommands returns the SSH commands.
func (cfg *Config) SSHCommands() (s string) {
	if cfg.ClusterState == nil || len(cfg.ClusterState.WorkerNodes) == 0 {
		return ""
	}
	ec := &ec2config.Config{
		UserName:  cfg.WorkerNodeUserName,
		KeyPath:   cfg.WorkerNodePrivateKeyPath,
		Instances: cfg.ClusterState.WorkerNodes,
	}
	return ec.SSHCommands()
}

// KubectlCommands returns the SSH commands.
func (cfg *Config) KubectlCommands() (s string) {
	if cfg.KubeConfigPath == "" {
		return ""
	}
	tpl := template.Must(template.New("kubectlCmdTmpl").Parse(kubectlCmdTmpl))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, struct {
		KubeConfigPath    string
		KubernetesVersion string
	}{
		cfg.KubeConfigPath,
		cfg.KubernetesVersion,
	}); err != nil {
		return ""
	}
	return buf.String()
}

const kubectlCmdTmpl = `# to test
KUBECTL_CMD="kubectl --kubeconfig={{ .KubeConfigPath }}"
${KUBECTL_CMD} cluster-info
${KUBECTL_CMD} get all --all-namespaces
${KUBECTL_CMD} get nodes
${KUBECTL_CMD} get psp
${KUBECTL_CMD} get pods -n kube-system
${KUBECTL_CMD} get ds -n kube-system

go get -v -u github.com/heptio/sonobuoy
sonobuoy delete --wait --kubeconfig={{ .KubeConfigPath }}

sonobuoy run \
  --mode Quick \
  --wait \
  --kube-conformance-image gcr.io/heptio-images/kube-conformance:v{{ .KubernetesVersion }}.0 \
  --kubeconfig={{ .KubeConfigPath }}

sonobuoy run \
  --wait \
  --kube-conformance-image gcr.io/heptio-images/kube-conformance:v{{ .KubernetesVersion }}.0 \
  --kubeconfig={{ .KubeConfigPath }}

sonobuoy status --kubeconfig={{ .KubeConfigPath }}

results=$(sonobuoy retrieve --kubeconfig={{ .KubeConfigPath }})
sonobuoy e2e --kubeconfig={{ .KubeConfigPath }} $results --show all
sonobuoy e2e --kubeconfig={{ .KubeConfigPath }} $results
`

func checkWorkderNodeASG(min, max int) (ok bool) {
	if min == 0 || max == 0 {
		return false
	}
	if min > max {
		return false
	}
	return true
}

const (
	serviceRolePolicyARNService = "arn:aws:iam::aws:policy/AmazonEKSServicePolicy"
	serviceRolePolicyARNCluster = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
)

func genServiceRoleWithPolicy(clusterName string) string {
	return fmt.Sprintf("%s-SERVICE-ROLE", clusterName)
}

func genCFStackVPC(clusterName string) string {
	return fmt.Sprintf("%s-VPC-STACK", clusterName)
}

func genNodeGroupKeyPairName(clusterName string) string {
	return fmt.Sprintf("%s-KEY-PAIR", clusterName)
}

func genCFStackWorkerNodeGroup(clusterName string) string {
	return fmt.Sprintf("%s-NODE-GROUP-STACK", clusterName)
}

// defaultWorkderNodeVolumeSizeGB is the default EKS worker node volume size in gigabytes.
// https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html
const defaultWorkderNodeVolumeSizeGB = 20

func exist(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}

const ll = "0123456789abcdefghijklmnopqrstuvwxyz"

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		rand.Seed(time.Now().UTC().UnixNano())
		b[i] = ll[rand.Intn(len(ll))]
	}
	return string(b)
}
