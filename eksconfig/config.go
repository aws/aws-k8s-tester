// Package eksconfig defines EKS test configuration.
package eksconfig

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
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
	// ConfigPath is the configuration file path.
	// Deployer is expected to update this file with latest status.
	ConfigPath string `json:"config-path,omitempty"`
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
	// See https://godoc.org/go.uber.org/zap#Open and https://godoc.org/go.uber.org/zap#Config for more details.
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

	// Parameters defines EKS cluster creation parameters.
	// It's ok to leave any parameters empty.
	// If empty, it will use default values.
	Parameters *Parameters `json:"parameters,omitempty"`

	// Status represents the current status of AWS resources.
	// Status is read-only.
	// Status cannot be configured via environmental variables.
	Status *Status `json:"status,omitempty"`

	AddOnNLBHelloWorld *AddOnNLBHelloWorld `json:"add-on-nlb-hello-world,omitempty"`
	AddOnALB2048       *AddOnALB2048       `json:"add-on-alb-2048,omitempty"`
	AddOnJobPerl       *AddOnJobPerl       `json:"add-on-job-perl,omitempty"`
	AddOnJobEcho       *AddOnJobEcho       `json:"add-on-job-echo,omitempty"`
}

// Parameters defines parameters for EKS cluster creation.
type Parameters struct {
	// ClusterRoleServicePrincipals is the EKS Role Service Principals
	ClusterRoleServicePrincipals []string `json:"cluster-role-service-principals,omitempty"`
	// ClusterRoleManagedPolicyARNs is EKS Role managed policy ARNs.
	ClusterRoleManagedPolicyARNs []string `json:"cluster-role-managed-policy-arns,omitempty"`
	// ClusterRoleARN is the role ARN that EKS uses to create AWS resources for Kubernetes.
	// By default, it's empty which triggers tester to create one.
	ClusterRoleARN string `json:"cluster-role-arn,omitempty"`

	// ClusterTags defines EKS create cluster tags.
	ClusterTags map[string]string `json:"cluster-tags,omitempty"`
	// ClusterRequestHeaderKey defines EKS create cluster request header key.
	ClusterRequestHeaderKey string `json:"cluster-request-header-key,omitempty"`
	// ClusterRequestHeaderValue defines EKS create cluster request header value.
	ClusterRequestHeaderValue string `json:"cluster-request-header-value,omitempty"`

	// ClusterResolverURL defines an AWS resolver endpoint for EKS API.
	// Must be left empty to use production EKS service.
	ClusterResolverURL string `json:"cluster-resolver-url"`
	// ClusterSigningName is the EKS create request signing name.
	ClusterSigningName string `json:"cluster-signing-name"`

	// VpcCIDR is the IP range (CIDR notation) for VPC, must be a valid private (RFC 1918) CIDR range.
	VPCCIDR string `json:"vpc-cidr,omitempty"`
	// PrivateSubnetCIDR1 is the CIDR Block for subnet 1 within the VPC.
	PrivateSubnetCIDR1 string `json:"private-subnet-cidr-1,omitempty"`
	// PrivateSubnetCIDR2 is the CIDR Block for subnet 2 within the VPC.
	PrivateSubnetCIDR2 string `json:"private-subnet-cidr-2,omitempty"`
	// PrivateSubnetCIDR3 is the CIDR Block for subnet 3 within the VPC.
	PrivateSubnetCIDR3 string `json:"private-subnet-cidr-3,omitempty"`

	// PrivateSubnetIDs is the list of all private subnets in the VPC.
	// By default, it's empty which triggers tester to create a VPC.
	// This must be from the same VPC that configures 'SecurityGroupIDs'.
	PrivateSubnetIDs []string `json:"private-subnet-ids,omitempty"`
	// ControlPlaneSecurityGroupID is the security group ID for the cluster control
	// plane communication with worker nodes
	// By default, it's empty which triggers tester to create a VPC.
	// This must be from the same VPC that configures 'PrivateSubnetIDs'.
	ControlPlaneSecurityGroupID string `json:"control-plane-security-group-id,omitempty"`

	// Version is the version of Kubernetes cluster.
	// If empty, set default version.
	Version string `json:"version,omitempty"`

	// ManagedNodeGroupCreate is true to auto-create a managed node group.
	ManagedNodeGroupCreate bool `json:"managed-node-group-create"`

	// ManagedNodeGroupRoleName is the name of the managed node group.
	ManagedNodeGroupRoleName string `json:"managed-node-group-role-name,omitempty"`
	// ManagedNodeGroupRoleServicePrincipals is the node group Service Principals
	ManagedNodeGroupRoleServicePrincipals []string `json:"managed-node-group-role-service-principals,omitempty"`
	// ManagedNodeGroupRoleManagedPolicyARNs is node group managed policy ARNs.
	ManagedNodeGroupRoleManagedPolicyARNs []string `json:"managed-node-group-role-managed-policy-arns,omitempty"`
	// ManagedNodeGroupRoleARN is the role ARN that EKS managed node group uses to create AWS resources for Kubernetes.
	// By default, it's empty which triggers tester to create one.
	ManagedNodeGroupRoleARN string `json:"managed-node-group-role-arn,omitempty"`

	// ManagedNodeGroupTags defines EKS managed node group create tags.
	ManagedNodeGroupTags map[string]string `json:"managed-node-group-tags,omitempty"`
	// ManagedNodeGroupRequestHeaderKey defines EKS managed node group create cluster request header key.
	ManagedNodeGroupRequestHeaderKey string `json:"managed-node-group-request-header-key,omitempty"`
	// ManagedNodeGroupRequestHeaderValue defines EKS managed node group create cluster request header value.
	ManagedNodeGroupRequestHeaderValue string `json:"managed-node-group-request-header-value,omitempty"`

	// ManagedNodeGroupResolverURL defines an AWS resolver endpoint for EKS API.
	// Must be left empty to use production EKS managed node group service.
	ManagedNodeGroupResolverURL string `json:"managed-node-group-resolver-url"`
	// ManagedNodeGroupSigningName is the EKS managed node group create request signing name.
	ManagedNodeGroupSigningName string `json:"managed-node-group-signing-name"`

	// ManagedNodeGroupName is the name of the managed node group.
	ManagedNodeGroupName string `json:"managed-node-group-name,omitempty"`
	// ManagedNodeGroupSSHKeyPairName is the key name for node group SSH EC2 key pair.
	ManagedNodeGroupSSHKeyPairName string `json:"managed-node-group-ssh-key-pair-name,omitempty"`
	// ManagedNodeGroupRemoteAccessPrivateKeyPath is the file path to store node group key pair private key.
	// Thus, deployer must delete the private key right after node group creation.
	// MAKE SURE PRIVATE KEY NEVER GETS UPLOADED TO CLOUD STORAGE AND DELETE AFTER USE!!!
	ManagedNodeGroupRemoteAccessPrivateKeyPath string `json:"managed-node-group-remote-access-private-key-path,omitempty"`
	// ManagedNodeGroupRemoteAccessUserName is the user name for managed node group SSH access.
	ManagedNodeGroupRemoteAccessUserName string `json:"managed-node-group-remote-access-user-name,omitempty"`
	// ManagedNodeGroupAMIType is the AMI type for the node group.
	ManagedNodeGroupAMIType string `json:"managed-node-group-ami-type,omitempty"`
	// ManagedNodeGroupASGMinSize is the minimum size of Node Group Auto Scaling Group.
	ManagedNodeGroupASGMinSize int `json:"managed-node-group-asg-min-size,omitempty"`
	// ManagedNodeGroupASGMaxSize is the maximum size of Node Group Auto Scaling Group.
	ManagedNodeGroupASGMaxSize int `json:"managed-node-group-asg-max-size,omitempty"`
	// ManagedNodeGroupASGDesiredCapacity is the desired capacity of Node Group ASG.
	ManagedNodeGroupASGDesiredCapacity int `json:"managed-node-group-asg-desired-capacity,omitempty"`
	// ManagedNodeGroupInstanceTypes is the EC2 instance types for the node instances.
	ManagedNodeGroupInstanceTypes []string `json:"managed-node-group-instance-types,omitempty"`
	// ManagedNodeGroupVolumeSize is the node volume size.
	ManagedNodeGroupVolumeSize int `json:"managed-node-group-volume-size,omitempty"`
}

// Status represents the current status of AWS resources.
// Read-only. Cannot be configured via environmental variables.
type Status struct {
	// Up is true if the cluster is up.
	Up bool `json:"up"`

	AWSAccountID string `json:"aws-account-id"`

	// AWSCredentialPath is automatically set via AWS SDK Go.
	// And to be mounted as a volume as 'Secret' object.
	AWSCredentialPath string `json:"aws-credential-path"`

	ClusterRoleCFNStackID string `json:"cluster-role-cfn-stack-id"`
	ClusterRoleARN        string `json:"cluster-role-arn"`
	ClusterRoleName       string `json:"cluster-role-name"`

	VPCCFNStackID               string   `json:"vpc-cfn-stack-id"`
	VPCID                       string   `json:"vpc-id"`
	PrivateSubnetIDs            []string `json:"private-subnet-ids"`
	ControlPlaneSecurityGroupID string   `json:"control-plane-security-group-id"`

	ClusterCFNStackID string `json:"cluster-cfn-stack-id"`
	ClusterARN        string `json:"cluster-arn"`
	// ClusterAPIServerEndpoint is the cluster endpoint of the EKS cluster, required for KUBECONFIG write.
	ClusterAPIServerEndpoint string `json:"cluster-api-server-endpoint"`
	// ClusterOIDCIssuer is the issuer URL for the OpenID Connect
	// (https://openid.net/connect/) identity provider .
	ClusterOIDCIssuer string `json:"cluster-oidc-issuer"`
	// ClusterCA is the EKS cluster CA, required for KUBECONFIG write.
	ClusterCA string `json:"cluster-ca"`
	// ClusterCADecoded is the decoded EKS cluster CA, required for k8s.io/client-go.
	ClusterCADecoded string `json:"cluster-ca-decoded"`

	ClusterStatus string `json:"cluster-status"`

	// ManagedNodeGroupRoleCFNStackID is the CloudFormation stack ID for a managed node group role.
	ManagedNodeGroupRoleCFNStackID string `json:"managed-node-group-role-cfn-stack-id"`

	// ManagedNodeGroupCFNStackID is the CloudFormation stack ID for a managed node group.
	ManagedNodeGroupCFNStackID                  string `json:"managed-node-group-cfn-stack-id"`
	ManagedNodeGroupRemoteAccessSecurityGroupID string `json:"managed-node-group-remote-access-security-group-id"`

	// ManagedNodeGroupID is the Physical ID for the created "AWS::EKS::Nodegroup".
	ManagedNodeGroupID string `json:"managed-node-group-id"`
	// ManagedNodeGroups maps each Auto Scaling Group to a set of latest EC2 nodes.
	ManagedNodeGroups map[string]NodeGroup `json:"managed-node-groups"`
	// ManagedNodeGroupsLogs maps each instance ID to a list of log file paths fetched via SSH access.
	ManagedNodeGroupsLogs map[string][]string `json:"managed-node-groups-logs"`

	ManagedNodeGroupStatus string `json:"managed-node-group-status"`
}

// NodeGroup is a set of EC2 instances in EC2 Auto Scaling Group.
type NodeGroup struct {
	// Instances maps an instance ID to an EC2 instance object.
	Instances map[string]ec2config.Instance `json:"instances"`
}

// AddOnNLBHelloWorld defines parameters for EKS cluster add-on NLB hello-world service.
type AddOnNLBHelloWorld struct {
	Enable bool   `json:"enable"`
	URL    string `json:"url" read-only:"true"`
}

// AddOnALB2048 defines parameters for EKS cluster add-on ALB 2048 service.
type AddOnALB2048 struct {
	Enable bool `json:"enable"`

	// PolicyCFNStackID is the CloudFormation stack ID
	// for ALB Ingress Controller IAM policy.
	PolicyCFNStackID string `json:"policy-cfn-stack-id" read-only:"true"`
	PolicyName       string `json:"policy-name"`

	// URL is the URL for ALB 2048 Service.
	URL string `json:"url" read-only:"true"`
}

// AddOnJobPerl defines parameters for EKS cluster add-on Job with Perl.
type AddOnJobPerl struct {
	Enable    bool `json:"enable"`
	Completes int  `json:"completes"`
	Parallels int  `json:"parallels"`
}

// AddOnJobEcho defines parameters for EKS cluster add-on Job with echo.
type AddOnJobEcho struct {
	Enable    bool `json:"enable"`
	Completes int  `json:"completes"`
	Parallels int  `json:"parallels"`
	// Size is the job object size.
	// "Request entity too large: limit is 3145728" (3.1 MB).
	// "The Job "echo" is invalid: metadata.annotations: Too long: must have at most 262144 characters". (0.26 MB)
	Size int `json:"size"`
}

// NewDefault returns a copy of the default configuration.
func NewDefault() *Config {
	vv := defaultConfig
	return &vv
}

func init() {
	// https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-welcome.html
	// pip3 install awscli --no-cache-dir --upgrade
	var err error
	defaultConfig.AWSCLIPath, err = exec.LookPath("aws")
	if err != nil {
		panic(fmt.Errorf("aws CLI is not installed (%v)", err))
	}

	if runtime.GOOS == "darwin" {
		defaultConfig.KubectlDownloadURL = strings.Replace(defaultConfig.KubectlDownloadURL, "linux", "darwin", -1)
		defaultConfig.Parameters.ManagedNodeGroupRemoteAccessPrivateKeyPath = filepath.Join(os.TempDir(), randString(12)+".insecure.key")
	}

	if err := os.MkdirAll(filepath.Dir(defaultConfig.KubectlPath), 0700); err != nil {
		panic(fmt.Errorf("could not create %q (%v)", filepath.Dir(defaultConfig.KubectlPath), err))
	}
}

// defaultConfig is the default configuration.
//  - empty string creates a non-nil object for pointer-type field
//  - omitting an entire field returns nil value
//  - make sure to check both
//
// MAKE SURE TO SYNC THE DEFAULT VALUES in "eks" templates
//
var defaultConfig = Config{
	Region: "us-west-2",

	LogLevel: logutil.DefaultLogLevel,
	// default, stderr, stdout, or file name
	// log file named with cluster name will be added automatically
	LogOutputs: []string{"stderr"},

	// https://docs.aws.amazon.com/eks/latest/userguide/install-kubectl.html
	// KubectlDownloadURL: "https://amazon-eks.s3-us-west-2.amazonaws.com/1.14.6/2019-08-22/bin/linux/amd64/kubectl",

	// https://kubernetes.io/docs/tasks/tools/install-kubectl/
	KubectlDownloadURL: "https://storage.googleapis.com/kubernetes-release/release/v1.14.10/bin/linux/amd64/kubectl",

	KubectlPath:    "/tmp/aws-k8s-tester/kubectl",
	KubeConfigPath: "/tmp/aws-k8s-tester/kubeconfig",

	Parameters: &Parameters{
		ClusterSigningName:          "eks",
		ManagedNodeGroupSigningName: "eks",

		Version: "1.14",

		ManagedNodeGroupRoleServicePrincipals: []string{
			"ec2.amazonaws.com",
			"eks.amazonaws.com",
		},
		ManagedNodeGroupRoleManagedPolicyARNs: []string{
			"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
			"arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
			"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
		},

		ManagedNodeGroupCreate: true,

		// Allowed values are AL2_x86_64 and AL2_x86_64_GPU
		ManagedNodeGroupAMIType:            "AL2_x86_64",
		ManagedNodeGroupASGMinSize:         3,
		ManagedNodeGroupASGMaxSize:         3,
		ManagedNodeGroupASGDesiredCapacity: 3,
		ManagedNodeGroupInstanceTypes:      []string{"c5.xlarge"},
		ManagedNodeGroupVolumeSize:         40,

		// keep in-sync with the default value in https://godoc.org/k8s.io/kubernetes/test/e2e/framework#GetSigner
		ManagedNodeGroupRemoteAccessPrivateKeyPath: filepath.Join(homedir.HomeDir(), ".ssh", "kube_aws_rsa"),
		ManagedNodeGroupRemoteAccessUserName:       "ec2-user",
	},

	Status: &Status{Up: false},

	AddOnNLBHelloWorld: &AddOnNLBHelloWorld{
		Enable: true,
	},
	AddOnALB2048: &AddOnALB2048{
		Enable: true,
	},
	AddOnJobPerl: &AddOnJobPerl{
		Enable:    true,
		Completes: 30,
		Parallels: 10,
	},
	AddOnJobEcho: &AddOnJobEcho{
		Enable:    true,
		Completes: 1000, // create 100 MB of data total
		Parallels: 100,
		Size:      100 * 1024, // 100 KB
	},

	// default config generates about 550 MB of cluster data
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
	cfg.ConfigPath, err = filepath.Abs(p)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// Sync persists current configuration and states to disk.
func (cfg *Config) Sync() (err error) {
	if !filepath.IsAbs(cfg.ConfigPath) {
		cfg.ConfigPath, err = filepath.Abs(cfg.ConfigPath)
		if err != nil {
			return fmt.Errorf("failed to 'filepath.Abs(%s)' %v", cfg.ConfigPath, err)
		}
	}
	if !filepath.IsAbs(cfg.KubeConfigPath) {
		cfg.KubeConfigPath, err = filepath.Abs(cfg.KubeConfigPath)
		if err != nil {
			return fmt.Errorf("failed to 'filepath.Abs(%s)' %v", cfg.KubeConfigPath, err)
		}
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
	return err
}

// genTag generates a tag for cluster name, CloudFormation, and S3 bucket.
// Note that this would be used as S3 bucket name to upload tester logs.
func genTag() string {
	// use UTC time for everything
	now := time.Now()
	return fmt.Sprintf("eks-%d%02d%02d%02d", now.Year(), int(now.Month()), now.Day(), now.Hour())
}

// ValidateAndSetDefaults returns an error for invalid configurations.
// And updates empty fields with default values.
// At the end, it writes populated YAML to aws-k8s-tester config path.
func (cfg *Config) ValidateAndSetDefaults() error {
	if len(cfg.LogOutputs) == 0 {
		return errors.New("LogOutputs is not empty")
	}

	if _, ok := awsapi.RegionToAiport[cfg.Region]; !ok {
		return fmt.Errorf("region %q not found", cfg.Region)
	}

	if cfg.Name == "" {
		now := time.Now()
		cfg.Name = fmt.Sprintf(
			"eks-%d%02d%02d%02d-%s-%s-%s",
			now.Year(),
			int(now.Month()),
			now.Day(),
			now.Hour(),
			strings.ToLower(awsapi.RegionToAiport[cfg.Region]),
			cfg.Region,
			randString(5),
		)
	}

	if cfg.ConfigPath == "" {
		rootDir := filepath.Join(os.TempDir(), cfg.Name)
		if err := os.MkdirAll(rootDir, 0700); err != nil {
			return err
		}
		f, err := ioutil.TempFile(rootDir, cfg.Name+"-config-yaml")
		if err != nil {
			return err
		}
		cfg.ConfigPath, _ = filepath.Abs(f.Name())
		f.Close()
		os.RemoveAll(cfg.ConfigPath)
	}

	if !strings.Contains(cfg.KubectlDownloadURL, runtime.GOOS) {
		return fmt.Errorf("kubectl-download-url %q build OS mismatch, expected %q", cfg.KubectlDownloadURL, runtime.GOOS)
	}

	// validate role-related
	if (len(cfg.Parameters.ClusterRoleServicePrincipals) > 0 || len(cfg.Parameters.ClusterRoleManagedPolicyARNs) > 0) && cfg.Parameters.ClusterRoleARN != "" {
		return fmt.Errorf("non-empty Parameters.ClusterRoleServicePrincipals %+v or Parameters.ClusterRoleManagedPolicyARNs %+v, but got Parameters.ClusterRoleARN %q",
			cfg.Parameters.ClusterRoleServicePrincipals,
			cfg.Parameters.ClusterRoleManagedPolicyARNs,
			cfg.Parameters.ClusterRoleARN,
		)
	}
	if cfg.Status.ClusterRoleCFNStackID != "" {
		if cfg.Status.ClusterRoleName == "" {
			return fmt.Errorf("non-empty Status.ClusterRoleCFNStackID %q, but empty Status.ClusterRoleName",
				cfg.Status.ClusterRoleCFNStackID,
			)
		}
		if cfg.Status.ClusterRoleARN == "" {
			return fmt.Errorf("non-empty Status.ClusterRoleCFNStackID %q, but empty Status.ClusterRoleARN",
				cfg.Status.ClusterRoleCFNStackID,
			)
		}
	}
	if cfg.Status.ClusterRoleName != "" {
		if cfg.Status.ClusterRoleARN == "" {
			return fmt.Errorf("non-empty Status.ClusterRoleName %q, but empty Status.ClusterRoleARN",
				cfg.Status.ClusterRoleName,
			)
		}
	}
	if cfg.Status.ClusterRoleARN != "" {
		if cfg.Status.ClusterRoleName == "" {
			return fmt.Errorf("non-empty Status.ClusterRoleARN %q, but empty Status.ClusterRoleName",
				cfg.Status.ClusterRoleARN,
			)
		}
	}

	// validate VPC-related
	if cfg.Parameters.VPCCIDR != "" {
		if cfg.Parameters.PrivateSubnetCIDR1 == "" {
			return fmt.Errorf("non-empty Parameters.VPCCIDR %q, but got empty Parameters.PrivateSubnetCIDR1", cfg.Parameters.VPCCIDR)
		}
		if cfg.Parameters.PrivateSubnetCIDR2 == "" {
			return fmt.Errorf("non-empty Parameters.VPCCIDR %q, but got empty Parameters.PrivateSubnetCIDR2", cfg.Parameters.VPCCIDR)
		}
		if cfg.Parameters.PrivateSubnetCIDR3 == "" {
			return fmt.Errorf("non-empty Parameters.VPCCIDR %q, but got empty Parameters.PrivateSubnetCIDR3", cfg.Parameters.VPCCIDR)
		}
	}
	if cfg.Parameters.PrivateSubnetCIDR1 != "" {
		if cfg.Parameters.VPCCIDR == "" {
			return fmt.Errorf("non-empty Parameters.PrivateSubnetCIDR1 %q, but got empty Parameters.VPCCIDR", cfg.Parameters.PrivateSubnetCIDR1)
		}
	}
	if cfg.Parameters.PrivateSubnetCIDR2 != "" {
		if cfg.Parameters.VPCCIDR == "" {
			return fmt.Errorf("non-empty Parameters.PrivateSubnetCIDR2 %q, but got empty Parameters.VPCCIDR", cfg.Parameters.PrivateSubnetCIDR2)
		}
	}
	if cfg.Parameters.PrivateSubnetCIDR3 != "" {
		if cfg.Parameters.VPCCIDR == "" {
			return fmt.Errorf("non-empty Parameters.PrivateSubnetCIDR3 %q, but got empty Parameters.VPCCIDR", cfg.Parameters.PrivateSubnetCIDR3)
		}
	}
	if cfg.Status.VPCCFNStackID != "" {
		if cfg.Status.VPCID == "" {
			return fmt.Errorf("non-empty Status.VPCCFNStackID %q, but empty Status.VPCID",
				cfg.Status.VPCCFNStackID,
			)
		}
		if len(cfg.Status.PrivateSubnetIDs) == 0 {
			return fmt.Errorf("non-empty Status.ClusterRoleCFNStackID %q, but empty Status.PrivateSubnetIDs",
				cfg.Status.ClusterRoleCFNStackID,
			)
		}
		if cfg.Status.ControlPlaneSecurityGroupID == "" {
			return fmt.Errorf("non-empty Status.ClusterRoleCFNStackID %q, but empty Status.ControlPlaneSecurityGroupID",
				cfg.Status.ClusterRoleCFNStackID,
			)
		}
	}
	if cfg.Status.VPCID != "" {
		if cfg.Status.VPCCFNStackID == "" {
			return fmt.Errorf("non-empty Status.VPCID %q, but empty Status.VPCCFNStackID",
				cfg.Status.VPCID,
			)
		}
	}
	if len(cfg.Status.PrivateSubnetIDs) > 0 {
		if cfg.Status.ControlPlaneSecurityGroupID == "" {
			return fmt.Errorf("non-empty Status.PrivateSubnetIDs %+v, but empty Status.ControlPlaneSecurityGroupID",
				cfg.Status.PrivateSubnetIDs,
			)
		}
	}
	if cfg.Status.ControlPlaneSecurityGroupID != "" {
		if len(cfg.Status.PrivateSubnetIDs) == 0 {
			return fmt.Errorf("non-empty Status.ControlPlaneSecurityGroupID %q, but empty Status.PrivateSubnetIDs",
				cfg.Status.ControlPlaneSecurityGroupID,
			)
		}
	}

	// validate cluster-related
	if cfg.Parameters.Version == "" {
		return errors.New("empty Parameters.Version")
	}
	if len(cfg.Parameters.PrivateSubnetIDs) == 0 && cfg.Parameters.ControlPlaneSecurityGroupID != "" {
		return fmt.Errorf("empty Parameters.PrivateSubnetIDs, non-empty Parameters.ControlPlaneSecurityGroupID %q", cfg.Parameters.ControlPlaneSecurityGroupID)
	}
	if len(cfg.Parameters.PrivateSubnetIDs) > 0 && cfg.Parameters.ControlPlaneSecurityGroupID != "" {
		return fmt.Errorf("non-empty Parameters.PrivateSubnetIDs %+v, but empty Parameters.ControlPlaneSecurityGroupID", cfg.Parameters.PrivateSubnetIDs)
	}
	if cfg.Status.ClusterCFNStackID != "" {
		if cfg.Status.ClusterARN == "" {
			return fmt.Errorf("non-empty Status.ClusterCFNStackID %q, but empty Status.ClusterARN", cfg.Status.ClusterCFNStackID)
		}
		if cfg.Status.ClusterCA == "" {
			return fmt.Errorf("non-empty Status.ClusterCFNStackID %q, but empty Status.ClusterCA", cfg.Status.ClusterCFNStackID)
		}
		if cfg.Status.ClusterCADecoded == "" {
			return fmt.Errorf("non-empty Status.ClusterCFNStackID %q, but empty Status.ClusterCADecoded", cfg.Status.ClusterCFNStackID)
		}
	}
	if cfg.Status.ClusterARN != "" {
		if cfg.Status.ClusterCA == "" {
			return fmt.Errorf("non-empty Status.ClusterARN %q, but empty Status.ClusterCA", cfg.Status.ClusterARN)
		}
		if cfg.Status.ClusterCADecoded == "" {
			return fmt.Errorf("non-empty Status.ClusterARN %q, but empty Status.ClusterCADecoded", cfg.Status.ClusterARN)
		}
	}

	// validate node group related
	if !cfg.Parameters.ManagedNodeGroupCreate {
		if cfg.AddOnNLBHelloWorld.Enable {
			return fmt.Errorf("Parameters.ManagedNodeGroupCreate false, but got AddOnNLBHelloWorld.Enable %v", cfg.AddOnNLBHelloWorld.Enable)
		}
		if cfg.AddOnALB2048.Enable {
			return fmt.Errorf("Parameters.ManagedNodeGroupCreate false, but got AddOnALB2048.Enable %v", cfg.AddOnALB2048.Enable)
		}
		if cfg.AddOnJobPerl.Enable {
			return fmt.Errorf("Parameters.ManagedNodeGroupCreate false, but got AddOnJobPerl.Enable %v", cfg.AddOnJobPerl.Enable)
		}
		if cfg.AddOnJobEcho.Enable {
			return fmt.Errorf("Parameters.ManagedNodeGroupCreate false, but got AddOnJobEcho.Enable %v", cfg.AddOnJobEcho.Enable)
		}
	}
	if cfg.Parameters.ManagedNodeGroupRoleName == "" {
		cfg.Parameters.ManagedNodeGroupRoleName = cfg.Name + "-managed-node-group-role"
	}
	if cfg.Parameters.ManagedNodeGroupName == "" {
		cfg.Parameters.ManagedNodeGroupName = cfg.Name + "-managed-node-group"
	}
	if cfg.Parameters.ManagedNodeGroupSSHKeyPairName == "" {
		cfg.Parameters.ManagedNodeGroupSSHKeyPairName = cfg.Name + "-ssh-key-pair"
	}
	if cfg.Parameters.ManagedNodeGroupRemoteAccessPrivateKeyPath == "" {
		return errors.New("empty Parameters.ManagedNodeGroupRemoteAccessPrivateKeyPath")
	}
	if cfg.Parameters.ManagedNodeGroupRemoteAccessUserName == "" {
		return errors.New("empty Parameters.ManagedNodeGroupRemoteAccessUserName")
	}

	if cfg.AddOnJobEcho.Size > 250000 {
		return fmt.Errorf("echo size limit is 0.25 MB, got %d", cfg.AddOnJobEcho.Size)
	}

	return cfg.Sync()
}

const (
	// EnvironmentVariablePrefix is the environment variable prefix used for setting configuration.
	EnvironmentVariablePrefix                   = "AWS_K8S_TESTER_EKS_"
	EnvironmentVariablePrefixParameters         = "AWS_K8S_TESTER_EKS_PARAMETERS_"
	EnvironmentVariablePrefixAddOnNLBHelloWorld = "AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_"
	EnvironmentVariablePrefixAddOnALB2048       = "AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_"
	EnvironmentVariablePrefixAddOnJobPerl       = "AWS_K8S_TESTER_EKS_ADD_ON_JOB_PERL_"
	EnvironmentVariablePrefixAddOnJobEcho       = "AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_"
)

// UpdateFromEnvs updates fields from environmental variables.
// Empty values are ignored.
func (cfg *Config) UpdateFromEnvs() error {
	copied := *cfg

	tp, vv := reflect.TypeOf(&copied).Elem(), reflect.ValueOf(&copied).Elem()
	for i := 0; i < tp.NumField(); i++ {
		jv := tp.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.Replace(jv, "-", "_", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := EnvironmentVariablePrefix + jv
		sv := os.Getenv(env)
		if sv == "" {
			continue
		}
		if tp.Field(i).Tag.Get("read-only") == "true" {
			continue
		}
		fieldName := tp.Field(i).Name

		switch vv.Field(i).Type().Kind() {
		case reflect.String:
			vv.Field(i).SetString(sv)

		case reflect.Map:
			return fmt.Errorf("parsing field name %q not supported", fieldName)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
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
			if len(ss) < 1 {
				continue
			}
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for j := range ss {
				slice.Index(j).SetString(ss[j])
			}
			vv.Field(i).Set(slice)

		default:
			return fmt.Errorf("%q (type %v) is not supported as an env", env, vv.Field(i).Type())
		}
	}
	*cfg = copied

	fieldParameters := *copied.Parameters
	tpParameters, vvParameters := reflect.TypeOf(&fieldParameters).Elem(), reflect.ValueOf(&fieldParameters).Elem()
	for i := 0; i < tpParameters.NumField(); i++ {
		jv := tpParameters.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := EnvironmentVariablePrefixParameters + jv
		sv := os.Getenv(env)
		if sv == "" {
			continue
		}
		fieldName := tpParameters.Field(i).Name

		switch vvParameters.Field(i).Type().Kind() {
		case reflect.String:
			vvParameters.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvParameters.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvParameters.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvParameters.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvParameters.Field(i).SetFloat(fv)

		case reflect.Slice:
			ss := strings.Split(sv, ",")
			if len(ss) < 1 {
				continue
			}
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for j := range ss {
				slice.Index(j).SetString(ss[j])
			}
			vvParameters.Field(i).Set(slice)

		case reflect.Map:
			switch fieldName {
			case "ClusterTags",
				"ManagedNodeGroupTags":
				vvParameters.Field(i).Set(reflect.ValueOf(make(map[string]string)))
				for _, pair := range strings.Split(sv, ",") {
					fields := strings.Split(pair, "=")
					if len(fields) != 2 {
						return fmt.Errorf("map %q has unexpected format (e.g. should be 'a=b;c;d,e=f'", sv)
					}
					vvParameters.Field(i).SetMapIndex(reflect.ValueOf(fields[0]), reflect.ValueOf(fields[1]))
				}

			default:
				return fmt.Errorf("parsing field name %q not supported", fieldName)
			}

		default:
			return fmt.Errorf("%q (type %v) is not supported as an env", env, vvParameters.Field(i).Type())
		}
	}
	cfg.Parameters = &fieldParameters

	// DO NOT SET cfg.Status since it's read-only

	fieldAddOnNLBHelloWorld := *copied.AddOnNLBHelloWorld
	tpAddOnNLBHelloWorld, vvAddOnNLBHelloWorld := reflect.TypeOf(&fieldAddOnNLBHelloWorld).Elem(), reflect.ValueOf(&fieldAddOnNLBHelloWorld).Elem()
	for i := 0; i < tpAddOnNLBHelloWorld.NumField(); i++ {
		jv := tpAddOnNLBHelloWorld.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := EnvironmentVariablePrefixAddOnNLBHelloWorld + jv
		sv := os.Getenv(env)
		if sv == "" {
			continue
		}
		if tpAddOnNLBHelloWorld.Field(i).Tag.Get("read-only") == "true" {
			continue
		}

		switch vvAddOnNLBHelloWorld.Field(i).Type().Kind() {
		case reflect.String:
			vvAddOnNLBHelloWorld.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvAddOnNLBHelloWorld.Field(i).SetBool(bb)

		default:
			return fmt.Errorf("%q (type %v) is not supported as an env", env, vvAddOnNLBHelloWorld.Field(i).Type())
		}
	}
	cfg.AddOnNLBHelloWorld = &fieldAddOnNLBHelloWorld

	fieldAddOnALB2048 := *copied.AddOnALB2048
	tpAddOnALB2048, vvAddOnALB2048 := reflect.TypeOf(&fieldAddOnALB2048).Elem(), reflect.ValueOf(&fieldAddOnALB2048).Elem()
	for i := 0; i < tpAddOnALB2048.NumField(); i++ {
		jv := tpAddOnALB2048.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := EnvironmentVariablePrefixAddOnALB2048 + jv
		sv := os.Getenv(env)
		if sv == "" {
			continue
		}
		if tpAddOnALB2048.Field(i).Tag.Get("read-only") == "true" {
			continue
		}

		switch vvAddOnALB2048.Field(i).Type().Kind() {
		case reflect.String:
			vvAddOnALB2048.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvAddOnALB2048.Field(i).SetBool(bb)

		default:
			return fmt.Errorf("%q (type %v) is not supported as an env", env, vvAddOnALB2048.Field(i).Type())
		}
	}
	cfg.AddOnALB2048 = &fieldAddOnALB2048

	fieldAddOnJobPerl := *copied.AddOnJobPerl
	tpAddOnJobPerl, vvAddOnJobPerl := reflect.TypeOf(&fieldAddOnJobPerl).Elem(), reflect.ValueOf(&fieldAddOnJobPerl).Elem()
	for i := 0; i < tpAddOnJobPerl.NumField(); i++ {
		jv := tpAddOnJobPerl.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := EnvironmentVariablePrefixAddOnJobPerl + jv
		sv := os.Getenv(env)
		if sv == "" {
			continue
		}
		if tpAddOnJobPerl.Field(i).Tag.Get("read-only") == "true" {
			continue
		}

		switch vvAddOnJobPerl.Field(i).Type().Kind() {
		case reflect.String:
			vvAddOnJobPerl.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvAddOnJobPerl.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvAddOnJobPerl.Field(i).SetInt(iv)

		default:
			return fmt.Errorf("%q (type %v) is not supported as an env", env, vvAddOnJobPerl.Field(i).Type())
		}
	}
	cfg.AddOnJobPerl = &fieldAddOnJobPerl

	fieldAddOnJobEcho := *copied.AddOnJobEcho
	tpAddOnJobEcho, vvAddOnJobEcho := reflect.TypeOf(&fieldAddOnJobEcho).Elem(), reflect.ValueOf(&fieldAddOnJobEcho).Elem()
	for i := 0; i < tpAddOnJobEcho.NumField(); i++ {
		jv := tpAddOnJobEcho.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := EnvironmentVariablePrefixAddOnJobEcho + jv
		sv := os.Getenv(env)
		if sv == "" {
			continue
		}
		if tpAddOnJobEcho.Field(i).Tag.Get("read-only") == "true" {
			continue
		}

		switch vvAddOnJobEcho.Field(i).Type().Kind() {
		case reflect.String:
			vvAddOnJobEcho.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvAddOnJobEcho.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvAddOnJobEcho.Field(i).SetInt(iv)

		default:
			return fmt.Errorf("%q (type %v) is not supported as an env", env, vvAddOnJobEcho.Field(i).Type())
		}
	}
	cfg.AddOnJobEcho = &fieldAddOnJobEcho

	return nil
}

// KubectlCommands returns the SSH commands.
func (cfg *Config) KubectlCommands() (s string) {
	if cfg.KubeConfigPath == "" {
		return ""
	}
	tpl := template.Must(template.New("kubectlCmdTmpl").Parse(kubectlCmdTmpl))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, struct {
		KubeConfigPath string
		Version        string
	}{
		cfg.KubeConfigPath,
		cfg.Parameters.Version,
	}); err != nil {
		return ""
	}
	return buf.String()
}

const kubectlCmdTmpl = `# kubectl commands
kubectl --kubeconfig={{ .KubeConfigPath }} version
kubectl --kubeconfig={{ .KubeConfigPath }} cluster-info
kubectl --kubeconfig={{ .KubeConfigPath }} get cs
kubectl --kubeconfig={{ .KubeConfigPath }} get nodes
kubectl --kubeconfig={{ .KubeConfigPath }} get pods --namespace kube-system
kubectl --kubeconfig={{ .KubeConfigPath }} get ds --namespace kube-system
kubectl --kubeconfig={{ .KubeConfigPath }} get secrets --all-namespaces
kubectl --kubeconfig={{ .KubeConfigPath }} get configmap --all-namespaces
kubectl --kubeconfig={{ .KubeConfigPath }} get all --all-namespaces

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
func (cfg *Config) SSHCommands() (s string) {
	if len(cfg.Status.ManagedNodeGroups) == 0 {
		return ""
	}
	buf := bytes.NewBuffer(nil)
	for name, ng := range cfg.Status.ManagedNodeGroups {
		buf.WriteString(fmt.Sprintf("ASG %q instance:\n", name))
		ec := &ec2config.Config{
			UserName:  cfg.Parameters.ManagedNodeGroupRemoteAccessUserName,
			KeyPath:   cfg.Parameters.ManagedNodeGroupRemoteAccessPrivateKeyPath,
			Instances: ng.Instances,
		}
		buf.WriteString(ec.SSHCommands())
		buf.WriteString("\n")
	}
	return buf.String()
}

const ll = "0123456789abcdefghijklmnopqrstuvwxyz"

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		rand.Seed(time.Now().UnixNano())
		b[i] = ll[rand.Intn(len(ll))]
	}
	return string(b)
}
