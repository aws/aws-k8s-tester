// Package eksconfig defines EKS test configuration.
package eksconfig

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-k8s-tester/pkg/terminal"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/mitchellh/colorstring"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml" // must use "sigs.k8s.io/yaml"
)

// AWS_K8S_TESTER_EKS_PREFIX is the environment variable prefix used for "eksconfig".
const AWS_K8S_TESTER_EKS_PREFIX = "AWS_K8S_TESTER_EKS_"

// EnvironmentVariablePrefixParameters is the environment variable prefix used for "eksconfig".
const EnvironmentVariablePrefixParameters = AWS_K8S_TESTER_EKS_PREFIX + "PARAMETERS_"

// Config defines EKS configuration.
type Config struct {
	mu *sync.RWMutex

	// TODO, Migrate metadata fields to here
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Name is the cluster name.
	// If empty, deployer auto-populates it.
	Name string `json:"name"`
	// Partition is the AWS partition for EKS deployment region.
	// If empty, set default partition "aws".
	Partition string `json:"partition"`
	// Region is the AWS geographic area for EKS deployment.
	// If empty, set default region.
	Region string `json:"region"`

	// ConfigPath is the configuration file path.
	// Deployer is expected to update this file with latest status.
	ConfigPath string `json:"config-path,omitempty"`
	// KubectlCommandsOutputPath is the output path for kubectl commands.
	KubectlCommandsOutputPath string `json:"kubectl-commands-output-path,omitempty"`
	// RemoteAccessCommandsOutputPath is the output path for ssh commands.
	RemoteAccessCommandsOutputPath string `json:"remote-access-commands-output-path,omitempty"`

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

	// AWSCLIPath is the path for AWS CLI path.
	// Required for 'aws eks update-kubeconfig'.
	AWSCLIPath string `json:"aws-cli-path,omitempty"`

	// KubectlPath is the path to download the "kubectl".
	KubectlPath string `json:"kubectl-path,omitempty"`
	// KubectlDownloadURL is the download URL to download "kubectl" binary from.
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
	// Currently supported variables are:
	//  - "GetRef.Name" for cluster name
	//  - "GetRef.ClusterARN" for cluster ARN
	CommandAfterCreateCluster              string        `json:"command-after-create-cluster"`
	CommandAfterCreateClusterOutputPath    string        `json:"command-after-create-cluster-output-path" read-only:"true"`
	CommandAfterCreateClusterTimeout       time.Duration `json:"command-after-create-cluster-timeout"`
	CommandAfterCreateClusterTimeoutString string        `json:"command-after-create-cluster-timeout-string" read-only:"true"`
	// CommandAfterCreateAddOns is the command to execute after creating clusters and add-ons.
	// Currently supported variables are:
	//  - "GetRef.Name" for cluster name
	//  - "GetRef.ClusterARN" for cluster ARN
	CommandAfterCreateAddOns              string        `json:"command-after-create-add-ons"`
	CommandAfterCreateAddOnsOutputPath    string        `json:"command-after-create-add-ons-output-path" read-only:"true"`
	CommandAfterCreateAddOnsTimeout       time.Duration `json:"command-after-create-add-ons-timeout"`
	CommandAfterCreateAddOnsTimeoutString string        `json:"command-after-create-add-ons-timeout-string" read-only:"true"`

	// S3BucketCreate is true to auto-create S3 bucket.
	S3BucketCreate bool `json:"s3-bucket-create"`
	// S3BucketCreateKeep is true to not delete auto-created S3 bucket.
	// The created S3 bucket is kept.
	S3BucketCreateKeep bool `json:"s3-bucket-create-keep"`
	// S3BucketName is the name of cluster S3.
	S3BucketName string `json:"s3-bucket-name"`
	// S3BucketLifecycleExpirationDays is expiration in days for the lifecycle of the object.
	S3BucketLifecycleExpirationDays int64 `json:"s3-bucket-lifecycle-expiration-days"`

	// CWNamespace is the CloudWatch namespace to put metric datum.
	CWNamespace string `json:"cw-namespace"`

	// SkipDeleteClusterAndNodes is true to skip EKS "cluster" and "nodes" deletion.
	// The created EKS "cluster" and all resources created "before" cluster are kept.
	// For example, CMK key, VPC, IAM role are not deleted if cluster is to be kept.
	// All node groups and managed node groups are kept.
	// Use this to use existing clusters to create/delete add-ons.
	SkipDeleteClusterAndNodes bool `json:"skip-delete-cluster-and-nodes"`

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

	// Clients is the number of kubernetes clients to create.
	// Default is 1.
	// This field is used for "eks/stresser" tester. Configure accordingly.
	// Rate limit is done via "k8s.io/client-go/util/flowcontrol.NewTokenBucketRateLimiter".
	Clients int `json:"clients"`
	// ClientQPS is the QPS for kubernetes client.
	// To use while talking with kubernetes apiserver.
	//
	// Kubernetes client DefaultQPS is 5.
	// Kubernetes client DefaultBurst is 10.
	// ref. https://github.com/kubernetes/kubernetes/blob/4d0e86f0b8d1eae00a202009858c8739e4c9402e/staging/src/k8s.io/client-go/rest/config.go#L43-L46
	//
	// kube-apiserver default inflight requests limits are:
	// FLAG: --max-mutating-requests-inflight="200"
	// FLAG: --max-requests-inflight="400"
	// ref. https://github.com/kubernetes/kubernetes/blob/4d0e86f0b8d1eae00a202009858c8739e4c9402e/staging/src/k8s.io/apiserver/pkg/server/config.go#L300-L301
	//
	// This field is used for "eks/stresser" tester. Configure accordingly.
	// Rate limit is done via "k8s.io/client-go/util/flowcontrol.NewTokenBucketRateLimiter".
	ClientQPS float32 `json:"client-qps"`
	// ClientBurst is the burst for kubernetes client.
	// To use while talking with kubernetes apiserver
	//
	// Kubernetes client DefaultQPS is 5.
	// Kubernetes client DefaultBurst is 10.
	// ref. https://github.com/kubernetes/kubernetes/blob/4d0e86f0b8d1eae00a202009858c8739e4c9402e/staging/src/k8s.io/client-go/rest/config.go#L43-L46
	//
	// kube-apiserver default inflight requests limits are:
	// FLAG: --max-mutating-requests-inflight="200"
	// FLAG: --max-requests-inflight="400"
	// ref. https://github.com/kubernetes/kubernetes/blob/4d0e86f0b8d1eae00a202009858c8739e4c9402e/staging/src/k8s.io/apiserver/pkg/server/config.go#L300-L301
	//
	// This field is used for "eks/stresser" tester. Configure accordingly.
	// Rate limit is done via "k8s.io/client-go/util/flowcontrol.NewTokenBucketRateLimiter".
	ClientBurst int `json:"client-burst"`
	// ClientTimeout is the client timeout.
	ClientTimeout       time.Duration `json:"client-timeout"`
	ClientTimeoutString string        `json:"client-timeout-string,omitempty" read-only:"true"`

	// AddOnCNIVPC defines parameters for https://github.com/aws/amazon-vpc-cni-k8s.
	AddOnCNIVPC *AddOnCNIVPC `json:"add-on-cni-vpc"`
	// AddOnNodeGroups defines EKS "Node Group"
	// creation parameters.
	AddOnNodeGroups *AddOnNodeGroups `json:"add-on-node-groups,omitempty"`
	// AddOnManagedNodeGroups defines EKS "Managed Node Group"
	// creation parameters. If empty, it will use default values.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	AddOnManagedNodeGroups *AddOnManagedNodeGroups `json:"add-on-managed-node-groups,omitempty"`

	// TotalNodes is the total number of nodes from all node groups.
	TotalNodes       int64 `json:"total-nodes" read-only:"true"`
	TotalHollowNodes int64 `json:"total-hollow-nodes" read-only:"true"`

	// AddOnCWAgent defines parameters for EKS cluster
	// add-on Fluentd.
	AddOnCWAgent *AddOnCWAgent `json:"add-on-cw-agent,omitempty"`
	// AddOnFluentd defines parameters for EKS cluster
	// add-on Fluentd.
	AddOnFluentd *AddOnFluentd `json:"add-on-fluentd,omitempty"`
	// AddOnMetricsServer defines parameters for EKS cluster
	// add-on metrics server.
	AddOnMetricsServer *AddOnMetricsServer `json:"add-on-metrics-server,omitempty"`

	// AddOnConformance defines parameters for EKS cluster
	// add-on Conformance.
	AddOnConformance *AddOnConformance `json:"add-on-conformance,omitempty"`

	// AddOnAppMesh defines parameters for EKS cluster
	// add-on "EKS App Mesh Integration".
	AddOnAppMesh *AddOnAppMesh `json:"add-on-app-mesh,omitempty"`
	// AddOnCSIEBS defines parameters for EKS cluster
	// add-on AWS EBS CSI Driver.
	AddOnCSIEBS *AddOnCSIEBS `json:"add-on-csi-ebs,omitempty"`
	// AddOnKubernetesDashboard defines parameters for EKS cluster
	// add-on Dashboard.
	AddOnKubernetesDashboard *AddOnKubernetesDashboard `json:"add-on-kubernetes-dashboard,omitempty"`
	// AddOnPrometheusGrafana defines parameters for EKS cluster
	// add-on Prometheus/Grafana.
	AddOnPrometheusGrafana *AddOnPrometheusGrafana `json:"add-on-prometheus-grafana,omitempty"`

	// AddOnPHPApache defines parameters for EKS cluster
	// add-on PHP Apache.
	AddOnPHPApache *AddOnPHPApache `json:"add-on-php-apache,omitempty"`
	// AddOnNLBHelloWorld defines parameters for EKS cluster
	// add-on NLB hello-world service.
	AddOnNLBHelloWorld *AddOnNLBHelloWorld `json:"add-on-nlb-hello-world,omitempty"`
	// AddOnNLBGuestbook defines parameters for EKS cluster
	// add-on NLB guestbook service.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/eks-guestbook.html
	AddOnNLBGuestbook *AddOnNLBGuestbook `json:"add-on-nlb-guestbook,omitempty"`
	// AddOnALB2048 defines parameters for EKS cluster
	// add-on ALB 2048 service.
	AddOnALB2048 *AddOnALB2048 `json:"add-on-alb-2048,omitempty"`
	// AddOnJobsPi defines parameters for EKS cluster
	// add-on Job with pi Perl command.
	AddOnJobsPi *AddOnJobsPi `json:"add-on-jobs-pi,omitempty"`
	// AddOnJobsEcho defines parameters for EKS cluster
	// add-on Job with echo.
	AddOnJobsEcho *AddOnJobsEcho `json:"add-on-jobs-echo,omitempty"`
	// AddOnCronJobs defines parameters for EKS cluster
	// add-on with CronJob.
	AddOnCronJobs *AddOnCronJobs `json:"add-on-cron-jobs,omitempty"`

	// AddOnCSRsLocal defines parameters for EKS cluster
	// add-on "CertificateSigningRequest" local.
	// It generates loads from the local host machine.
	AddOnCSRsLocal *AddOnCSRsLocal `json:"add-on-csrs-local,omitempty"`
	// AddOnCSRsRemote defines parameters for EKS cluster
	// add-on "CertificateSigningRequest" remote.
	// It generates loads from the remote workers (Pod) in the cluster.
	AddOnCSRsRemote *AddOnCSRsRemote `json:"add-on-csrs-remote,omitempty"`

	// AddOnConfigmapsLocal defines parameters for EKS cluster
	// add-on "ConfigMap" local.
	// It generates loads from the local host machine.
	AddOnConfigmapsLocal *AddOnConfigmapsLocal `json:"add-on-configmaps-local,omitempty"`
	// AddOnConfigmapsRemote defines parameters for EKS cluster
	// add-on "ConfigMap" remote.
	// It generates loads from the remote workers (Pod) in the cluster.
	AddOnConfigmapsRemote *AddOnConfigmapsRemote `json:"add-on-configmaps-remote,omitempty"`

	// AddOnSecretsLocal defines parameters for EKS cluster
	// add-on "Secrets" local.
	// It generates loads from the local host machine.
	AddOnSecretsLocal *AddOnSecretsLocal `json:"add-on-secrets-local,omitempty"`
	// AddOnSecretsRemote defines parameters for EKS cluster
	// add-on "Secrets" remote.
	// It generates loads from the remote workers (Pod) in the cluster.
	AddOnSecretsRemote *AddOnSecretsRemote `json:"add-on-secrets-remote,omitempty"`

	// AddOnFargate defines parameters for EKS cluster
	// add-on "EKS on Fargate".
	AddOnFargate *AddOnFargate `json:"add-on-fargate,omitempty"`
	// AddOnIRSA defines parameters for EKS cluster
	// add-on "IAM Roles for Service Accounts (IRSA)".
	AddOnIRSA *AddOnIRSA `json:"add-on-irsa,omitempty"`
	// AddOnIRSAFargate defines parameters for EKS cluster
	// add-on "IAM Roles for Service Accounts (IRSA)" with Fargate.
	AddOnIRSAFargate *AddOnIRSAFargate `json:"add-on-irsa-fargate,omitempty"`
	// AddOnWordpress defines parameters for EKS cluster
	// add-on WordPress.
	AddOnWordpress *AddOnWordpress `json:"add-on-wordpress,omitempty"`
	// AddOnJupyterHub defines parameters for EKS cluster
	// add-on JupyterHub.
	AddOnJupyterHub *AddOnJupyterHub `json:"add-on-jupyter-hub,omitempty"`
	// AddOnKubeflow defines parameters for EKS cluster
	// add-on Kubeflow.
	AddOnKubeflow *AddOnKubeflow `json:"add-on-kubeflow,omitempty"`
	// AddOnCUDAVectorAdd defines parameters for EKS cluster
	// add-on cuda-vector-add.
	AddOnCUDAVectorAdd *AddOnCUDAVectorAdd `json:"add-on-cuda-vector-add,omitempty"`

	// AddOnClusterLoaderLocal defines parameters for EKS cluster
	// add-on cluster loader local.
	// It generates loads from the local host machine.
	// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2
	AddOnClusterLoaderLocal *AddOnClusterLoaderLocal `json:"add-on-cluster-loader-local,omitempty"`
	// AddOnClusterLoaderRemote defines parameters for EKS cluster
	// add-on cluster loader remote.
	// It generates loads from the remote host machine.
	// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2
	AddOnClusterLoaderRemote *AddOnClusterLoaderRemote `json:"add-on-cluster-loader-remote,omitempty"`

	// AddOnHollowNodesLocal defines parameters for EKS cluster
	// add-on hollow nodes local.
	// It generates loads from the local host machine.
	AddOnHollowNodesLocal *AddOnHollowNodesLocal `json:"add-on-hollow-nodes-local,omitempty"`
	// AddOnHollowNodesRemote defines parameters for EKS cluster
	// add-on hollow nodes remote.
	// It generates loads from the remote workers (Pod) in the cluster.
	AddOnHollowNodesRemote *AddOnHollowNodesRemote `json:"add-on-hollow-nodes-remote,omitempty"`

	// AddOnStresserLocal defines parameters for EKS cluster
	// add-on cluster loader local.
	// It generates loads from the local host machine.
	// ref. https://github.com/kubernetes/perf-tests
	AddOnStresserLocal *AddOnStresserLocal `json:"add-on-stresser-local,omitempty"`
	// AddOnStresserRemote defines parameters for EKS cluster
	// add-on cluster loader remote.
	// It generates loads from the remote workers (Pod) in the cluster.
	// ref. https://github.com/kubernetes/perf-tests
	AddOnStresserRemote *AddOnStresserRemote `json:"add-on-stresser-remote,omitempty"`
	// AddOnStresserRemoteV2 defines parameters for EKS cluster
	// add-on cluster loader remote v2.
	// It generates loads from the remote workers (Pod) in the cluster.
	// ref. https://github.com/kubernetes/perf-tests
	AddOnStresserRemoteV2 *AddOnStresserRemoteV2 `json:"add-on-stresser-remote-v2,omitempty"`

	// AddOnClusterVersionUpgrade defines parameters
	// for EKS cluster version upgrade add-on.
	AddOnClusterVersionUpgrade *AddOnClusterVersionUpgrade `json:"add-on-cluster-version-upgrade,omitempty"`

	// AddOnAmiSoftLockupIssue454 defines parameters
	// for testing the AMI soft lockup issue.
	AddOnAmiSoftLockupIssue454 *AddOnAmiSoftLockupIssue454 `json:"add-on-ami-soft-lockup-issue-454,omitempty"`

	// Spec contains addons and other configuration
	// Note: New addons should be implemented inside spec
	Spec Spec `json:"spec,omitempty"`

	// Status represents the current status of AWS resources.
	// Status is read-only.
	// Status cannot be configured via environmental variables.
	Status *Status `json:"status,omitempty" read-only:"true"`
}

// Colorize prints colorized input, if color output is supported.
func (c Config) Colorize(input string) string {
	colorize := colorstring.Colorize{
		Colors:  colorstring.DefaultColors,
		Disable: !c.LogColor,
		Reset:   true,
	}
	return colorize.Color(input)
}

// Parameters defines EKS "cluster" creation parameters.
// It's ok to leave any parameters empty.
// If empty, it will use default values.
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
	RoleCFNStackYAMLPath  string   `json:"role-cfn-stack-yaml-path" read-only:"true"`
	RoleCFNStackYAMLS3Key string   `json:"role-cfn-stack-yaml-s3-key" read-only:"true"`

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
	VPCID                string `json:"vpc-id"`
	VPCCFNStackID        string `json:"vpc-cfn-stack-id" read-only:"true"`
	VPCCFNStackYAMLPath  string `json:"vpc-cfn-stack-yaml-path" read-only:"true"`
	VPCCFNStackYAMLS3Key string `json:"vpc-cfn-stack-yaml-s3-key" read-only:"true"`
	// VpcBlock1 is the IP range (CIDR notation) for the primary VPC Block, must be a valid private
	// (RFC 1918) CIDR range.
	VPCBlock1 string `json:"vpc-cidr-block1,omitempty"`
	// VpcBlock2 is the IP range (CIDR notation) for the secondary VPC Block, must be a valid private
	// (RFC 1918) CIDR range.
	VPCBlock2 string `json:"vpc-cidr-block2,omitempty"`
	// VpcBlock3 is the IP range (CIDR notation) for the secondary VPC Block, must be a valid private
	// (RFC 1918) CIDR range.
	VPCBlock3 string `json:"vpc-cidr-block3,omitempty"`
	// VpcBlock4 is the IP range (CIDR notation) for the secondary VPC Block, must be a valid private
	// (RFC 1918) CIDR range.
	VPCBlock4 string `json:"vpc-cidr-block4,omitempty"`
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

	// EKS internal only
	// If empty, use default kube-controller-manager and kube-scheduler qps and burst
	// ref. https://kubernetes.io/docs/reference/command-line-tools-reference/kube-controller-manager/
	// ref. https://kubernetes.io/docs/reference/command-line-tools-reference/kube-scheduler/

	// KubeAPIServerMaxRequestsInflight is the EKS kube-apiserver max-requests-inflight
	// The maximum number of non-mutating requests in flight at a given time. When the server exceeds this, it rejects requests. Zero for no limit.
	// --max-requests-inflight int     Default: 400
	KubeAPIServerMaxRequestsInflight string `json:"kube-apiserver-max-requests-inflight"`
	// KubeControllerManagerQPS is the EKS kube-controller-manager qps
	// --kube-api-qps float32     Default: 20
	KubeControllerManagerQPS string `json:"kube-controller-manager-qps,omitempty"`
	// KubeControllerManagerBurst is the EKS kube-controller-manager burst
	// --kube-api-burst int32     Default: 30
	KubeControllerManagerBurst string `json:"kube-controller-manager-burst,omitempty"`
	// KubeSchedulerQPS is the internal EKS kube-scheduler qps
	// --kube-api-qps float32     Default: 50
	KubeSchedulerQPS string `json:"kube-scheduler-qps,omitempty"`
	// KubeSchedulerBurst is the internal EKS kube-scheduler burst
	// --kube-api-burst int32     Default: 100
	KubeSchedulerBurst string `json:"kube-scheduler-burst,omitempty"`
	// FEUpdateMasterFlagsURL is the internal EKS update master flags endpoint
	FEUpdateMasterFlagsURL string `json:"fe-update-master-flags-url,omitempty"`
}

func getDefaultParameters() *Parameters {
	return &Parameters{
		RoleCreate:          true,
		VPCCreate:           true,
		VPCBlock1:           "10.0.0.0/16",
		VPCBlock2:           "10.1.0.0/16",
		VPCBlock3:           "10.2.0.0/16",
		VPCBlock4:           "10.3.0.0/16",
		PublicSubnetCIDR1:   "10.0.0.0/16",
		PublicSubnetCIDR2:   "10.1.0.0/16",
		PublicSubnetCIDR3:   "10.2.0.0/16",
		PrivateSubnetCIDR1:  "10.3.0.0/17",
		PrivateSubnetCIDR2:  "10.3.128.0/17",
		SigningName:         "eks",
		Version:             "1.18",
		EncryptionCMKCreate: true,
	}
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
	if serr := cfg.unsafeSync(); serr != nil {
		fmt.Fprintf(os.Stderr, "[WARN] failed to sync config files %v\n", serr)
	}

	return cfg, nil
}

// EvaluateCommandRefs updates "CommandAfterCreateCluster" and "CommandAfterCreateAddOns".
// currently, only support "GetRef.Name" and "GetRef.ClusterARN"
func (cfg *Config) EvaluateCommandRefs() error {
	cfg.mu.Lock()
	err := cfg.evaluateCommandRefs()
	cfg.mu.Unlock()
	return err
}

func (cfg *Config) evaluateCommandRefs() error {
	if cfg.CommandAfterCreateCluster != "" {
		ss := strings.Split(cfg.CommandAfterCreateCluster, " ")
		p, err := exec.LookPath(ss[0])
		if err != nil {
			return fmt.Errorf("%q does not exist (%v)", ss[0], err)
		}
		ss[0] = p
		cfg.CommandAfterCreateCluster = strings.Join(ss, " ")
	}

	if cfg.CommandAfterCreateAddOns != "" {
		ss := strings.Split(cfg.CommandAfterCreateAddOns, " ")
		p, err := exec.LookPath(ss[0])
		if err != nil {
			return fmt.Errorf("%q does not exist (%v)", ss[0], err)
		}
		ss[0] = p
		cfg.CommandAfterCreateAddOns = strings.Join(ss, " ")
	}

	if cfg.Name != "" && strings.Contains(cfg.CommandAfterCreateCluster, "GetRef.Name") {
		cfg.CommandAfterCreateCluster = strings.ReplaceAll(cfg.CommandAfterCreateCluster, "GetRef.Name", cfg.Name)
	}
	if cfg.Status != nil && cfg.Status.ClusterARN != "" && strings.Contains(cfg.CommandAfterCreateCluster, "GetRef.ClusterARN") {
		cfg.CommandAfterCreateCluster = strings.ReplaceAll(cfg.CommandAfterCreateCluster, "GetRef.ClusterARN", cfg.Status.ClusterARN)
	}

	if cfg.Name != "" && strings.Contains(cfg.CommandAfterCreateAddOns, "GetRef.Name") {
		cfg.CommandAfterCreateAddOns = strings.ReplaceAll(cfg.CommandAfterCreateAddOns, "GetRef.Name", cfg.Name)
	}
	if cfg.Status != nil && cfg.Status.ClusterARN != "" && strings.Contains(cfg.CommandAfterCreateAddOns, "GetRef.ClusterARN") {
		cfg.CommandAfterCreateAddOns = strings.ReplaceAll(cfg.CommandAfterCreateAddOns, "GetRef.ClusterARN", cfg.Status.ClusterARN)
	}
	if serr := cfg.unsafeSync(); serr != nil {
		fmt.Fprintf(os.Stderr, "[WARN] failed to sync config files %v\n", serr)
	}

	return nil
}

// Sync persists current configuration and states to disk.
// Every call overwrites the previous contents if any.
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

	if cfg.RemoteAccessCommandsOutputPath != "" {
		err = ioutil.WriteFile(cfg.RemoteAccessCommandsOutputPath, []byte(cmdTop+cfg.unsafeSSHCommands()), 0600)
		if err != nil {
			return fmt.Errorf("failed to write RemoteAccessCommandsOutputPath %q (%v)", cfg.RemoteAccessCommandsOutputPath, err)
		}
	}

	if cfg.KubectlCommandsOutputPath != "" {
		err = ioutil.WriteFile(cfg.KubectlCommandsOutputPath, []byte(cmdTop+cfg.KubectlCommands()), 0600)
		if err != nil {
			return fmt.Errorf("failed to write KubectlCommandsOutputPath %q (%v)", cfg.KubectlCommandsOutputPath, err)
		}
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
	tpl := template.Must(template.New("kubectlTmpl").Parse(kubectlTmpl))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, struct {
		KubeConfigPath                         string
		KubectlCommand                         string
		KubernetesDashboardEnabled             bool
		KubernetesDashboardURL                 string
		KubernetesDashboardAuthenticationToken string
	}{
		cfg.KubeConfigPath,
		cfg.KubectlCommand(),
		cfg.IsEnabledAddOnKubernetesDashboard(),
		cfg.getAddOnKubernetesDashboardURL(),
		cfg.getAddOnKubernetesDashboardAuthenticationToken(),
	}); err != nil {
		return ""
	}
	return buf.String()
}

const kubectlTmpl = `
###########################
# kubectl commands
export KUBECONFIG={{ .KubeConfigPath }}
export KUBECTL="{{ .KubectlCommand }}"

{{ .KubectlCommand }} version
{{ .KubectlCommand }} cluster-info
{{ .KubectlCommand }} get cs
{{ .KubectlCommand }} --namespace=kube-system get pods
{{ .KubectlCommand }} --namespace=kube-system get ds
{{ .KubectlCommand }} get pods
{{ .KubectlCommand }} get csr -o=yaml
{{ .KubectlCommand }} get nodes --show-labels -o=wide
{{ .KubectlCommand }} get nodes -o=wide
###########################
{{ if .KubernetesDashboardEnabled }}
###########################
{{ .KubectlCommand }} proxy

# Kubernetes Dashboard URL:
{{ .KubernetesDashboardURL }}

# Kubernetes Dashboard Authentication Token:
{{ .KubernetesDashboardAuthenticationToken }}
###########################
{{ end }}
`

// SSHCommands returns the SSH commands.
func (cfg *Config) SSHCommands() string {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	return cfg.unsafeSSHCommands()
}

func (cfg *Config) unsafeSSHCommands() (s string) {
	buf := bytes.NewBuffer(nil)
	buf.WriteByte('\n')

	if cfg.AddOnNodeGroups != nil && cfg.AddOnNodeGroups.Enable {
		for name, cur := range cfg.AddOnNodeGroups.ASGs {
			if len(cur.Instances) == 0 {
				buf.WriteString(fmt.Sprintf("no ASG instances found for node group %s\n", name))
				continue
			}
			buf.WriteString("ASG name from node group \"" + name + "\":\n")
			asg := &ec2config.ASG{
				Name:      name,
				Instances: cur.Instances,
			}
			buf.WriteString(asg.SSHCommands(cfg.Region, cfg.RemoteAccessPrivateKeyPath, cfg.AddOnNodeGroups.ASGs[name].RemoteAccessUserName))
			buf.WriteString("\n\n")
		}
	}

	if cfg.AddOnManagedNodeGroups != nil && cfg.AddOnManagedNodeGroups.Enable {
		for name, cur := range cfg.AddOnManagedNodeGroups.MNGs {
			if len(cur.Instances) == 0 {
				buf.WriteString(fmt.Sprintf("no ASG instances found for managed node group %s\n", name))
				continue
			}
			buf.WriteString("ASG name from managed node group \"" + name + "\":\n")
			asg := &ec2config.ASG{
				Name:      name,
				Instances: cur.Instances,
			}
			buf.WriteString(asg.SSHCommands(cfg.Region, cfg.RemoteAccessPrivateKeyPath, cfg.AddOnManagedNodeGroups.MNGs[name].RemoteAccessUserName))
			buf.WriteString("\n\n")
		}
	}

	return buf.String()
}

const (
	// DefaultClients is the default number of clients to create.
	DefaultClients = 2
	// DefaultClientQPS is the default client QPS.
	DefaultClientQPS float32 = 10
	// DefaultClientBurst is the default client burst.
	DefaultClientBurst = 20
	// DefaultClientTimeout is the default client timeout.
	DefaultClientTimeout = 15 * time.Second

	DefaultCommandAfterCreateClusterTimeout = 3 * time.Minute
	DefaultCommandAfterCreateAddOnsTimeout  = 3 * time.Minute

	// DefaultNodeInstanceTypeCPU is the default EC2 instance type for CPU worker node.
	DefaultNodeInstanceTypeCPU = "c5.xlarge"
	// DefaultNodeInstanceTypeGPU is the default EC2 instance type for GPU worker node.
	DefaultNodeInstanceTypeGPU = "p3.8xlarge"

	// DefaultNodeVolumeSize is the default EC2 instance volume size for a worker node.
	DefaultNodeVolumeSize = 40

	// NGsMaxLimit is the maximum number of "Node Group"s per a EKS cluster.
	NGsMaxLimit = 10
	// NGMaxLimit is the maximum number of nodes per a "Node Group".
	NGMaxLimit = 5000

	// MNGsMaxLimit is the maximum number of "Managed Node Group"s per a EKS cluster.
	MNGsMaxLimit = 10
	// MNGMaxLimit is the maximum number of nodes per a "Managed Node Group".
	MNGMaxLimit = 100
)

// NewDefault returns a default configuration.
//  - empty string creates a non-nil object for pointer-type field
//  - omitting an entire field returns nil value
//  - make sure to check both
func NewDefault() *Config {
	name := fmt.Sprintf("eks-%s-%s", getTS()[:10], randutil.String(12))
	if v := os.Getenv(AWS_K8S_TESTER_EKS_PREFIX + "NAME"); v != "" {
		name = v
	}

	cfg := Config{
		mu: new(sync.RWMutex),

		Name:      name,
		Partition: endpoints.AwsPartitionID,
		Region:    endpoints.UsWest2RegionID,

		// to be auto-generated
		ConfigPath:                "",
		KubectlCommandsOutputPath: "",
		KubeConfigPath:            "",
		AWSCLIPath:                "",

		LogColor:         true,
		LogColorOverride: false,
		LogLevel:         logutil.DefaultLogLevel,
		// default, stderr, stdout, or file name
		// log file named with cluster name will be added automatically
		LogOutputs: []string{"stderr"},

		// https://github.com/kubernetes/kubernetes/tags
		// https://kubernetes.io/docs/tasks/tools/install-kubectl/
		// https://docs.aws.amazon.com/eks/latest/userguide/install-kubectl.html
		KubectlPath:        "/tmp/kubectl-test-v1.18.9",
		KubectlDownloadURL: "https://storage.googleapis.com/kubernetes-release/release/v1.18.9/bin/linux/amd64/kubectl",

		OnFailureDelete:            true,
		OnFailureDeleteWaitSeconds: 120,

		S3BucketName:                    "",
		S3BucketCreate:                  true,
		S3BucketCreateKeep:              true,
		S3BucketLifecycleExpirationDays: 0,

		CWNamespace: "aws-k8s-tester-eks",

		SkipDeleteClusterAndNodes: false,
		Parameters:                getDefaultParameters(),

		RemoteAccessKeyCreate: true,
		// keep in-sync with the default value in https://pkg.go.dev/k8s.io/kubernetes/test/e2e/framework#GetSigner
		// RemoteAccessPrivateKeyPath: filepath.Join(homedir.HomeDir(), ".ssh", "kube_aws_rsa"),
		RemoteAccessPrivateKeyPath: filepath.Join(os.TempDir(), randutil.String(15)+".insecure.key"),

		// Kubernetes client DefaultQPS is 5.
		// Kubernetes client DefaultBurst is 10.
		// ref. https://github.com/kubernetes/kubernetes/blob/4d0e86f0b8d1eae00a202009858c8739e4c9402e/staging/src/k8s.io/client-go/rest/config.go#L43-L46
		//
		// kube-apiserver default inflight requests limits are:
		// FLAG: --max-mutating-requests-inflight="200"
		// FLAG: --max-requests-inflight="400"
		// ref. https://github.com/kubernetes/kubernetes/blob/4d0e86f0b8d1eae00a202009858c8739e4c9402e/staging/src/k8s.io/apiserver/pkg/server/config.go#L300-L301
		//
		Clients:     DefaultClients,
		ClientQPS:   DefaultClientQPS,
		ClientBurst: DefaultClientBurst,

		AddOnCNIVPC:            getDefaultAddOnCNIVPC(),
		AddOnNodeGroups:        getDefaultAddOnNodeGroups(name),
		AddOnManagedNodeGroups: getDefaultAddOnManagedNodeGroups(name),

		AddOnCWAgent:       getDefaultAddOnCWAgent(),
		AddOnFluentd:       getDefaultAddOnFluentd(),
		AddOnMetricsServer: getDefaultAddOnMetricsServer(),

		AddOnConformance: getDefaultAddOnConformance(),

		AddOnAppMesh:               getDefaultAddOnAppMesh(),
		AddOnCSIEBS:                getDefaultAddOnCSIEBS(),
		AddOnKubernetesDashboard:   getDefaultAddOnKubernetesDashboard(),
		AddOnPrometheusGrafana:     getDefaultAddOnPrometheusGrafana(),
		AddOnPHPApache:             getDefaultAddOnPHPApache(),
		AddOnNLBHelloWorld:         getDefaultAddOnNLBHelloWorld(),
		AddOnNLBGuestbook:          getDefaultAddOnNLBGuestbook(),
		AddOnALB2048:               getDefaultAddOnALB2048(),
		AddOnJobsPi:                getDefaultAddOnJobsPi(),
		AddOnJobsEcho:              getDefaultAddOnJobsEcho(),
		AddOnCronJobs:              getDefaultAddOnCronJobs(),
		AddOnCSRsLocal:             getDefaultAddOnCSRsLocal(),
		AddOnCSRsRemote:            getDefaultAddOnCSRsRemote(),
		AddOnConfigmapsLocal:       getDefaultAddOnConfigmapsLocal(),
		AddOnConfigmapsRemote:      getDefaultAddOnConfigmapsRemote(),
		AddOnSecretsLocal:          getDefaultAddOnSecretsLocal(),
		AddOnSecretsRemote:         getDefaultAddOnSecretsRemote(),
		AddOnFargate:               getDefaultAddOnFargate(),
		AddOnIRSA:                  getDefaultAddOnIRSA(),
		AddOnIRSAFargate:           getDefaultAddOnIRSAFargate(),
		AddOnWordpress:             getDefaultAddOnWordpress(),
		AddOnJupyterHub:            getDefaultAddOnJupyterHub(),
		AddOnKubeflow:              getDefaultAddOnKubeflow(),
		AddOnCUDAVectorAdd:         getDefaultAddOnCUDAVectorAdd(),
		AddOnClusterLoaderLocal:    getDefaultAddOnClusterLoaderLocal(),
		AddOnClusterLoaderRemote:   getDefaultAddOnClusterLoaderRemote(),
		AddOnHollowNodesLocal:      getDefaultAddOnHollowNodesLocal(),
		AddOnHollowNodesRemote:     getDefaultAddOnHollowNodesRemote(),
		AddOnStresserLocal:         getDefaultAddOnStresserLocal(),
		AddOnStresserRemote:        getDefaultAddOnStresserRemote(),
		AddOnStresserRemoteV2:      getDefaultAddOnStresserRemoteV2(),
		AddOnClusterVersionUpgrade: getDefaultAddOnClusterVersionUpgrade(),
		AddOnAmiSoftLockupIssue454: getDefaultAddOnAmiSoftLockupIssue454(),

		// read-only
		Status: &Status{
			Up:                   false,
			PrivateDNSToNodeInfo: make(map[string]NodeInfo),
		},
	}

	// https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-welcome.html
	// pip3 install awscli --no-cache-dir --upgrade
	var err error
	cfg.AWSCLIPath, err = exec.LookPath("aws")
	if err != nil {
		panic(fmt.Errorf("aws CLI is not installed (%v); required for 'aws eks update-kubeconfig'", err))
	}

	if runtime.GOOS == "darwin" {
		cfg.KubectlDownloadURL = strings.Replace(cfg.KubectlDownloadURL, "linux", "darwin", -1)
	}

	return &cfg
}

// ValidateAndSetDefaults returns an error for invalid configurations.
// And updates empty fields with default values.
// At the end, it writes populated YAML to aws-k8s-tester config path.
// "read-only" fields cannot be set, causing errors.
func (cfg *Config) ValidateAndSetDefaults() error {
	if cfg.mu == nil {
		cfg.mu = new(sync.RWMutex)
	}
	cfg.mu.Lock()
	defer func() {
		if serr := cfg.unsafeSync(); serr != nil {
			fmt.Fprintf(os.Stderr, "[WARN] failed to sync config files %v\n", serr)
		}
		cfg.mu.Unlock()
	}()

	// Generically defaults and validates addons that are members of cfg.Spec
	spec := reflect.ValueOf(cfg.Spec)
	for i := 0; i < spec.NumField(); i++ {
		// Skip if the field does not implement Addon or is Nil
		if addon, ok := spec.Field(i).Interface().(Addon); ok && !reflect.ValueOf(addon).IsNil() {
			addon.Default(cfg)
			if err := addon.Validate(cfg); err != nil {
				return fmt.Errorf("Failed to validate %s, %v", reflect.ValueOf(addon).Type(), err)
			}
		}
	}

	if err := cfg.validateConfig(); err != nil {
		return fmt.Errorf("validateConfig failed [%v]", err)
	}
	if err := cfg.validateParameters(); err != nil {
		return fmt.Errorf("validateParameters failed [%v]", err)
	}
	if err := cfg.validateAddOnNodeGroups(); err != nil {
		return fmt.Errorf("validateAddOnNodeGroups failed [%v]", err)
	}
	if err := cfg.validateAddOnManagedNodeGroups(); err != nil {
		return fmt.Errorf("validateAddOnManagedNodeGroups failed [%v]", err)
	}

	if err := cfg.validateAddOnCNIVPC(); err != nil {
		return fmt.Errorf("validateAddOnCNIVPC failed [%v]", err)
	}

	total := int64(0)
	if cfg.IsEnabledAddOnNodeGroups() {
		for _, cur := range cfg.AddOnNodeGroups.ASGs {
			total += cur.ASGDesiredCapacity
		}
	}
	if cfg.IsEnabledAddOnManagedNodeGroups() {
		for _, cur := range cfg.AddOnManagedNodeGroups.MNGs {
			total += int64(cur.ASGDesiredCapacity)
		}
	}
	cfg.TotalNodes = total

	totalHollowNodes := int64(0)
	if cfg.IsEnabledAddOnHollowNodesLocal() {
		totalHollowNodes += int64(cfg.AddOnHollowNodesLocal.Nodes)
	}
	if cfg.IsEnabledAddOnHollowNodesRemote() {
		totalHollowNodes += int64(cfg.AddOnHollowNodesRemote.Nodes) * int64(cfg.AddOnHollowNodesRemote.NodeGroups)
	}
	cfg.TotalHollowNodes = totalHollowNodes

	if err := cfg.validateAddOnCWAgent(); err != nil {
		return fmt.Errorf("validateAddOnCWAgent failed [%v]", err)
	}
	if err := cfg.validateAddOnFluentd(); err != nil {
		return fmt.Errorf("validateAddOnFluentd failed [%v]", err)
	}
	if err := cfg.validateAddOnMetricsServer(); err != nil {
		return fmt.Errorf("validateAddOnMetricsServer failed [%v]", err)
	}

	if err := cfg.validateAddOnConformance(); err != nil {
		return fmt.Errorf("validateAddOnConformance failed [%v]", err)
	}

	if err := cfg.validateAddOnAppMesh(); err != nil {
		return fmt.Errorf("validateAddOnAppMesh failed [%v]", err)
	}
	if err := cfg.validateAddOnCSIEBS(); err != nil {
		return fmt.Errorf("validateAddOnCSIEBS failed [%v]", err)
	}

	if err := cfg.validateAddOnKubernetesDashboard(); err != nil {
		return fmt.Errorf("validateAddOnKubernetesDashboard failed [%v]", err)
	}
	if err := cfg.validateAddOnPrometheusGrafana(); err != nil {
		return fmt.Errorf("validateAddOnPrometheusGrafana failed [%v]", err)
	}
	if err := cfg.validateAddOnPHPApache(); err != nil {
		return fmt.Errorf("validateAddOnPHPApache failed [%v]", err)
	}
	if err := cfg.validateAddOnNLBHelloWorld(); err != nil {
		return fmt.Errorf("validateAddOnNLBHelloWorld failed [%v]", err)
	}
	if err := cfg.validateAddOnNLBGuestbook(); err != nil {
		return fmt.Errorf("validateAddOnNLBGuestbook failed [%v]", err)
	}
	if err := cfg.validateAddOnALB2048(); err != nil {
		return fmt.Errorf("validateAddOnALB2048 failed [%v]", err)
	}
	if err := cfg.validateAddOnJobsPi(); err != nil {
		return fmt.Errorf("validateAddOnJobsPi failed [%v]", err)
	}
	if err := cfg.validateAddOnJobsEcho(); err != nil {
		return fmt.Errorf("validateAddOnJobsEcho failed [%v]", err)
	}
	if err := cfg.validateAddOnCronJobs(); err != nil {
		return fmt.Errorf("validateAddOnCronJobs failed [%v]", err)
	}

	if err := cfg.validateAddOnCSRsLocal(); err != nil {
		return fmt.Errorf("validateAddOnCSRsLocal failed [%v]", err)
	}
	if err := cfg.validateAddOnCSRsRemote(); err != nil {
		return fmt.Errorf("validateAddOnCSRsRemote failed [%v]", err)
	}

	if err := cfg.validateAddOnConfigmapsLocal(); err != nil {
		return fmt.Errorf("validateAddOnConfigmapsLocal failed [%v]", err)
	}
	if err := cfg.validateAddOnConfigmapsRemote(); err != nil {
		return fmt.Errorf("validateAddOnConfigmapsRemote failed [%v]", err)
	}

	if err := cfg.validateAddOnSecretsLocal(); err != nil {
		return fmt.Errorf("validateAddOnSecretsLocal failed [%v]", err)
	}
	if err := cfg.validateAddOnSecretsRemote(); err != nil {
		return fmt.Errorf("validateAddOnSecretsRemote failed [%v]", err)
	}

	if err := cfg.validateAddOnFargate(); err != nil {
		return fmt.Errorf("validateAddOnFargate failed [%v]", err)
	}
	if err := cfg.validateAddOnIRSA(); err != nil {
		return fmt.Errorf("validateAddOnIRSA failed [%v]", err)
	}
	if err := cfg.validateAddOnIRSAFargate(); err != nil {
		return fmt.Errorf("validateIRSAAddOnFargate failed [%v]", err)
	}
	if err := cfg.validateAddOnWordpress(); err != nil {
		return fmt.Errorf("validateAddOnWordpress failed [%v]", err)
	}
	if err := cfg.validateAddOnJupyterHub(); err != nil {
		return fmt.Errorf("validateAddOnJupyterHub failed [%v]", err)
	}
	if err := cfg.validateAddOnKubeflow(); err != nil {
		return fmt.Errorf("validateAddOnKubeflow failed [%v]", err)
	}
	if err := cfg.validateAddOnCUDAVectorAdd(); err != nil {
		return fmt.Errorf("validateAddOnCUDAVectorAdd failed [%v]", err)
	}

	if err := cfg.validateAddOnClusterLoaderLocal(); err != nil {
		return fmt.Errorf("validateAddOnClusterLoaderLocal failed [%v]", err)
	}
	if err := cfg.validateAddOnClusterLoaderRemote(); err != nil {
		return fmt.Errorf("validateAddOnClusterLoaderRemote failed [%v]", err)
	}

	if err := cfg.validateAddOnHollowNodesLocal(); err != nil {
		return fmt.Errorf("validateAddOnHollowNodesLocal failed [%v]", err)
	}
	if err := cfg.validateAddOnHollowNodesRemote(); err != nil {
		return fmt.Errorf("validateAddOnHollowNodesRemote failed [%v]", err)
	}

	if err := cfg.validateAddOnStresserLocal(); err != nil {
		return fmt.Errorf("validateAddOnStresserLocal failed [%v]", err)
	}
	if err := cfg.validateAddOnStresserRemote(); err != nil {
		return fmt.Errorf("validateAddOnStresserRemote failed [%v]", err)
	}
	if err := cfg.validateAddOnStresserRemoteV2(); err != nil {
		return fmt.Errorf("validateAddOnStresserRemoteV2 failed [%v]", err)
	}

	if err := cfg.validateAddOnClusterVersionUpgrade(); err != nil {
		return fmt.Errorf("validateAddOnClusterVersionUpgrade failed [%v]", err)
	}

	if err := cfg.validateAddOnAmiSoftLockupIssue454(); err != nil {
		return fmt.Errorf("validateAddOnClusterVersionUpgrade failed [%v]", err)
	}

	return nil
}

func (cfg *Config) validateConfig() error {
	if len(cfg.Name) == 0 {
		return errors.New("Name is empty")
	}
	if cfg.Name != strings.ToLower(cfg.Name) {
		return fmt.Errorf("Name %q must be in lower-case", cfg.Name)
	}

	var partition endpoints.Partition
	switch cfg.Partition {
	case endpoints.AwsPartitionID:
		partition = endpoints.AwsPartition()
	case endpoints.AwsCnPartitionID:
		partition = endpoints.AwsCnPartition()
	case endpoints.AwsUsGovPartitionID:
		partition = endpoints.AwsUsGovPartition()
	case endpoints.AwsIsoPartitionID:
		partition = endpoints.AwsIsoPartition()
	case endpoints.AwsIsoBPartitionID:
		partition = endpoints.AwsIsoBPartition()
	default:
		return fmt.Errorf("unknown partition %q", cfg.Partition)
	}
	regions := partition.Regions()
	if _, ok := regions[cfg.Region]; !ok {
		// we will get this error when the Go AWS SDK is not updated to support a new region
		fmt.Fprintf(os.Stderr, "[WARN] region %q for partition %q not found in %+v", cfg.Region, cfg.Partition, regions)
	}

	_, cerr := terminal.IsColor()
	if cfg.LogColor && !cfg.LogColorOverride && cerr != nil {
		cfg.LogColor = false
	}
	if len(cfg.LogOutputs) == 0 {
		return errors.New("LogOutputs is not empty")
	}

	if cfg.Clients == 0 {
		cfg.Clients = DefaultClients
	}
	if cfg.ClientQPS == 0 {
		cfg.ClientQPS = DefaultClientQPS
	}
	if cfg.ClientBurst == 0 {
		cfg.ClientBurst = DefaultClientBurst
	}
	if cfg.ClientTimeout == time.Duration(0) {
		cfg.ClientTimeout = DefaultClientTimeout
	}
	cfg.ClientTimeoutString = cfg.ClientTimeout.String()

	if cfg.ConfigPath == "" {
		rootDir, err := os.Getwd()
		if err != nil {
			rootDir = filepath.Join(os.TempDir(), cfg.Name)
			if err := os.MkdirAll(rootDir, 0700); err != nil {
				return err
			}
		}
		cfg.ConfigPath = filepath.Join(rootDir, cfg.Name+".yaml")
		var p string
		p, err = filepath.Abs(cfg.ConfigPath)
		if err != nil {
			panic(err)
		}
		cfg.ConfigPath = p
	}
	if err := os.MkdirAll(filepath.Dir(cfg.ConfigPath), 0700); err != nil {
		return err
	}
	if err := fileutil.IsDirWriteable(filepath.Dir(cfg.ConfigPath)); err != nil {
		return err
	}

	if len(cfg.LogOutputs) == 1 && (cfg.LogOutputs[0] == "stderr" || cfg.LogOutputs[0] == "stdout") {
		cfg.LogOutputs = append(cfg.LogOutputs, strings.ReplaceAll(cfg.ConfigPath, ".yaml", "")+".log")
	}
	logFilePath := ""
	for _, fpath := range cfg.LogOutputs {
		if filepath.Ext(fpath) == ".log" {
			logFilePath = fpath
			break
		}
	}
	if logFilePath == "" {
		return fmt.Errorf("*.log file not found in %q", cfg.LogOutputs)
	}

	if cfg.KubectlCommandsOutputPath == "" {
		cfg.KubectlCommandsOutputPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".kubectl.sh"
	}
	if filepath.Ext(cfg.KubectlCommandsOutputPath) != ".sh" {
		cfg.KubectlCommandsOutputPath = cfg.KubectlCommandsOutputPath + ".sh"
	}
	if err := fileutil.IsDirWriteable(filepath.Dir(cfg.KubectlCommandsOutputPath)); err != nil {
		return err
	}
	if cfg.RemoteAccessCommandsOutputPath == "" {
		cfg.RemoteAccessCommandsOutputPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".ssh.sh"
	}
	if filepath.Ext(cfg.RemoteAccessCommandsOutputPath) != ".sh" {
		cfg.RemoteAccessCommandsOutputPath = cfg.RemoteAccessCommandsOutputPath + ".sh"
	}
	if err := fileutil.IsDirWriteable(filepath.Dir(cfg.RemoteAccessCommandsOutputPath)); err != nil {
		return err
	}

	if cfg.CommandAfterCreateClusterOutputPath == "" {
		cfg.CommandAfterCreateClusterOutputPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".after-create-cluster.out.log"
	}
	if filepath.Ext(cfg.CommandAfterCreateClusterOutputPath) != ".log" {
		cfg.CommandAfterCreateClusterOutputPath = cfg.CommandAfterCreateClusterOutputPath + ".log"
	}
	if cfg.CommandAfterCreateClusterTimeout == time.Duration(0) {
		cfg.CommandAfterCreateClusterTimeout = DefaultCommandAfterCreateClusterTimeout
	}
	if err := fileutil.IsDirWriteable(filepath.Dir(cfg.CommandAfterCreateClusterOutputPath)); err != nil {
		return err
	}
	cfg.CommandAfterCreateClusterTimeoutString = cfg.CommandAfterCreateClusterTimeout.String()

	if cfg.CommandAfterCreateAddOnsOutputPath == "" {
		cfg.CommandAfterCreateAddOnsOutputPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".after-create-add-ons.out.log"
	}
	if filepath.Ext(cfg.CommandAfterCreateAddOnsOutputPath) != ".log" {
		cfg.CommandAfterCreateAddOnsOutputPath = cfg.CommandAfterCreateAddOnsOutputPath + ".log"
	}
	if err := fileutil.IsDirWriteable(filepath.Dir(cfg.CommandAfterCreateAddOnsOutputPath)); err != nil {
		return err
	}
	if cfg.CommandAfterCreateAddOnsTimeout == time.Duration(0) {
		cfg.CommandAfterCreateAddOnsTimeout = DefaultCommandAfterCreateAddOnsTimeout
	}
	cfg.CommandAfterCreateAddOnsTimeoutString = cfg.CommandAfterCreateAddOnsTimeout.String()

	if cfg.KubeConfigPath == "" {
		cfg.KubeConfigPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".kubeconfig.yaml"
	}
	if err := fileutil.IsDirWriteable(filepath.Dir(cfg.KubeConfigPath)); err != nil {
		return err
	}

	if cfg.KubectlPath == "" && cfg.KubectlDownloadURL == "" {
		return errors.New("empty KubectlPath and KubectlDownloadURL")
	}
	if !strings.ContainsAny(cfg.KubectlDownloadURL, runtime.GOOS) {
		return fmt.Errorf("kubectl-download-url %q build OS mismatch, expected %q", cfg.KubectlDownloadURL, runtime.GOOS)
	}

	if err := cfg.evaluateCommandRefs(); err != nil {
		return err
	}

	switch cfg.S3BucketCreate {
	case true: // need create one, or already created
		if cfg.S3BucketName == "" {
			cfg.S3BucketName = cfg.Name + "-s3-bucket"
		}
		if cfg.S3BucketLifecycleExpirationDays > 0 && cfg.S3BucketLifecycleExpirationDays < 3 {
			cfg.S3BucketLifecycleExpirationDays = 3
		}
	case false: // use existing one
		if cfg.S3BucketName == "" {
			return errors.New("empty S3BucketName")
		}
	}

	if cfg.CWNamespace == "" {
		cfg.CWNamespace = "aws-k8s-tester-eks"
	}

	if cfg.Status == nil {
		cfg.Status = &Status{
			Up:                   false,
			PrivateDNSToNodeInfo: make(map[string]NodeInfo),
		}
	}
	if cfg.Status.ClusterCFNStackYAMLPath == "" {
		cfg.Status.ClusterCFNStackYAMLPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".cluster.cfn.yaml"
	}
	if cfg.Status.ClusterCFNStackYAMLS3Key == "" {
		cfg.Status.ClusterCFNStackYAMLS3Key = path.Join(cfg.Name, path.Base(cfg.Status.ClusterCFNStackYAMLPath))
	}
	return nil
}

func (cfg *Config) validateParameters() error {
	if cfg.Parameters.Version == "" {
		return errors.New("empty Parameters.Version")
	}
	var err error
	cfg.Parameters.VersionValue, err = strconv.ParseFloat(cfg.Parameters.Version, 64)
	if err != nil {
		return fmt.Errorf("cannot parse Parameters.Version %q (%v)", cfg.Parameters.Version, err)
	}

	if cfg.Parameters.RoleCFNStackYAMLPath == "" {
		cfg.Parameters.RoleCFNStackYAMLPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".role.cfn.yaml"
	}
	if cfg.Parameters.RoleCFNStackYAMLS3Key == "" {
		cfg.Parameters.RoleCFNStackYAMLS3Key = path.Join(cfg.Name, path.Base(cfg.Parameters.RoleCFNStackYAMLPath))
	}
	switch cfg.Parameters.RoleCreate {
	case true: // need create one, or already created
		if cfg.Parameters.RoleName == "" {
			cfg.Parameters.RoleName = cfg.Name + "-role"
		}
		if cfg.Parameters.RoleARN != "" {
			// just ignore...
			// could be populated from previous run
			// do not error, so long as RoleCreate false, role won't be deleted
		}

	case false: // use existing one
		if cfg.Parameters.RoleARN == "" {
			return fmt.Errorf("Parameters.RoleCreate false; expect non-empty RoleARN but got %q", cfg.Parameters.RoleARN)
		}
		if cfg.Parameters.RoleName == "" {
			cfg.Parameters.RoleName = getNameFromARN(cfg.Parameters.RoleARN)
		}
		if len(cfg.Parameters.RoleManagedPolicyARNs) > 0 {
			return fmt.Errorf("Parameters.RoleCreate false; expect empty RoleManagedPolicyARNs but got %q", cfg.Parameters.RoleManagedPolicyARNs)
		}
		if len(cfg.Parameters.RoleServicePrincipals) > 0 {
			return fmt.Errorf("Parameters.RoleCreate false; expect empty RoleServicePrincipals but got %q", cfg.Parameters.RoleServicePrincipals)
		}
	}

	if cfg.Parameters.VPCCFNStackYAMLPath == "" {
		cfg.Parameters.VPCCFNStackYAMLPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".vpc.cfn.yaml"
	}
	if cfg.Parameters.VPCCFNStackYAMLS3Key == "" {
		cfg.Parameters.VPCCFNStackYAMLS3Key = path.Join(cfg.Name, path.Base(cfg.Parameters.VPCCFNStackYAMLPath))
	}
	switch cfg.Parameters.VPCCreate {
	case true: // need create one, or already created
		if cfg.Parameters.VPCID != "" {
			// just ignore...
			// could be populated from previous run
			// do not error, so long as VPCCreate false, VPC won't be deleted
		}
	case false: // use existing one
		if cfg.Parameters.VPCID == "" {
			return fmt.Errorf("Parameters.RoleCreate false; expect non-empty VPCID but got %q", cfg.Parameters.VPCID)
		}
	}

	switch cfg.Parameters.EncryptionCMKCreate {
	case true: // need create one, or already created
		if cfg.Parameters.EncryptionCMKARN != "" {
			// just ignore...
			// could be populated from previous run
			// do not error, so long as EncryptionCMKCreate false, CMK won't be deleted
		}
	case false: // use existing one
		if cfg.Parameters.EncryptionCMKARN == "" {
			// return fmt.Errorf("Parameters.EncryptionCMKCreate false; expect non-empty EncryptionCMKARN but got %q", cfg.Parameters.EncryptionCMKARN)
		}
	}

	switch {
	case cfg.Parameters.VPCBlock1 != "":
		if cfg.Parameters.PublicSubnetCIDR1 == "" {
			return fmt.Errorf("empty Parameters.PublicSubnetCIDR1 when VPCBlock1 is %q", cfg.Parameters.VPCBlock1)
		}
	case cfg.Parameters.VPCBlock2 != "":
		if cfg.Parameters.PublicSubnetCIDR2 == "" {
			return fmt.Errorf("empty Parameters.PublicSubnetCIDR2 when VPCBlock2 is %q", cfg.Parameters.VPCBlock2)
		}
	case cfg.Parameters.VPCBlock3 != "":
		if cfg.Parameters.PublicSubnetCIDR3 == "" {
			return fmt.Errorf("empty Parameters.PublicSubnetCIDR3 when VPCBlock3 is %q", cfg.Parameters.VPCBlock3)
		}
	case cfg.Parameters.VPCBlock4 != "":
		switch {
		case cfg.Parameters.PrivateSubnetCIDR1 == "":
			return fmt.Errorf("empty Parameters.PrivateSubnetCIDR1 when VPCBlock4 is %q", cfg.Parameters.VPCBlock4)
		case cfg.Parameters.PrivateSubnetCIDR2 == "":
			return fmt.Errorf("empty Parameters.PrivateSubnetCIDR2 when VPCBlock4 is %q", cfg.Parameters.VPCBlock4)
		}

	case cfg.Parameters.VPCBlock1 == "":
		if cfg.Parameters.PublicSubnetCIDR1 != "" {
			return fmt.Errorf("non-empty Parameters.PublicSubnetCIDR1 %q when VPCBlock1 is empty", cfg.Parameters.PublicSubnetCIDR1)
		}
	case cfg.Parameters.VPCBlock2 == "":
		if cfg.Parameters.PublicSubnetCIDR2 != "" {
			return fmt.Errorf("non-empty Parameters.PublicSubnetCIDR2 %q when VPCBlock2 is empty", cfg.Parameters.PublicSubnetCIDR2)
		}
	case cfg.Parameters.VPCBlock3 == "":
		if cfg.Parameters.PublicSubnetCIDR3 != "" {
			return fmt.Errorf("non-empty Parameters.PublicSubnetCIDR3 %q when VPCBlock3 is empty", cfg.Parameters.PublicSubnetCIDR3)
		}
	case cfg.Parameters.VPCBlock4 == "":
		switch {
		case cfg.Parameters.PrivateSubnetCIDR1 != "":
			return fmt.Errorf("non-empty Parameters.PrivateSubnetCIDR1 %q when VPCBlock4 is empty", cfg.Parameters.PrivateSubnetCIDR1)
		case cfg.Parameters.PrivateSubnetCIDR2 != "":
			return fmt.Errorf("non-empty Parameters.PrivateSubnetCIDR2 %q when VPCBlock4 is empty", cfg.Parameters.PrivateSubnetCIDR1)
		}
	}

	switch cfg.RemoteAccessKeyCreate {
	case true: // need create one, or already created
		if cfg.RemoteAccessKeyName == "" {
			cfg.RemoteAccessKeyName = cfg.Name + "-remote-access-key"
		}
		if cfg.RemoteAccessPrivateKeyPath == "" {
			cfg.RemoteAccessPrivateKeyPath = filepath.Join(os.TempDir(), randutil.String(10)+".insecure.key")
		}

	case false: // use existing one
		if cfg.RemoteAccessKeyName == "" {
			return fmt.Errorf("RemoteAccessKeyCreate false; expect non-empty RemoteAccessKeyName but got %q", cfg.RemoteAccessKeyName)
		}
		if cfg.RemoteAccessPrivateKeyPath == "" {
			return fmt.Errorf("RemoteAccessKeyCreate false; expect non-empty RemoteAccessPrivateKeyPath but got %q", cfg.RemoteAccessPrivateKeyPath)
		}
		if !fileutil.Exist(cfg.RemoteAccessPrivateKeyPath) {
			return fmt.Errorf("RemoteAccessPrivateKeyPath %q does not exist", cfg.RemoteAccessPrivateKeyPath)
		}
	}
	keyDir := filepath.Dir(cfg.RemoteAccessPrivateKeyPath)
	if err := fileutil.IsDirWriteable(keyDir); err != nil {
		return err
	}

	return nil
}

// only letters and numbers
var regex = regexp.MustCompile("[^a-zA-Z0-9]+")

// get "role-eks" from "arn:aws:iam::123:role/role-eks"
func getNameFromARN(arn string) string {
	if ss := strings.Split(arn, "/"); len(ss) > 0 {
		arn = ss[len(ss)-1]
	}
	return arn
}

func getTS() string {
	now := time.Now()
	return fmt.Sprintf(
		"%04d%02d%02d%02d%02d",
		now.Year(),
		int(now.Month()),
		now.Day(),
		now.Hour(),
		now.Second(),
	)
}
