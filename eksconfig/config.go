// Package eksconfig defines EKS test configuration.
package eksconfig

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"sigs.k8s.io/yaml" // must use "sigs.k8s.io/yaml"
)

// AWS_K8S_TESTER_EKS_PREFIX is the environment variable prefix used for "eksconfig".
const AWS_K8S_TESTER_EKS_PREFIX = "AWS_K8S_TESTER_EKS_"

// Config defines EKS configuration.
type Config struct {
	mu *sync.RWMutex

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
	// This field is used for "eks/cluster-loader" tester. Configure accordingly.
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
	// This field is used for "eks/cluster-loader" tester. Configure accordingly.
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
	// This field is used for "eks/cluster-loader" tester. Configure accordingly.
	// Rate limit is done via "k8s.io/client-go/util/flowcontrol.NewTokenBucketRateLimiter".
	ClientBurst int `json:"client-burst"`
	// ClientTimeout is the client timeout.
	ClientTimeout       time.Duration `json:"client-timeout"`
	ClientTimeoutString string        `json:"client-timeout-string,omitempty" read-only:"true"`

	// AddOnNodeGroups defines EKS "Node Group"
	// creation parameters.
	AddOnNodeGroups *AddOnNodeGroups `json:"add-on-node-groups,omitempty"`
	// AddOnManagedNodeGroups defines EKS "Managed Node Group"
	// creation parameters. If empty, it will use default values.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	AddOnManagedNodeGroups *AddOnManagedNodeGroups `json:"add-on-managed-node-groups,omitempty"`

	// AddOnCSIEBS defines parameters for EKS cluster
	// add-on AWS EBS CSI Driver.
	AddOnCSIEBS *AddOnCSIEBS `json:"add-on-csi-ebs,omitempty"`
	// AddOnKubernetesDashboard defines parameters for EKS cluster
	// add-on Dashboard.
	AddOnKubernetesDashboard *AddOnKubernetesDashboard `json:"add-on-kubernetes-dashboard,omitempty"`
	// AddOnPrometheusGrafana defines parameters for EKS cluster
	// add-on Prometheus/Grafana.
	AddOnPrometheusGrafana *AddOnPrometheusGrafana `json:"add-on-prometheus-grafana,omitempty"`

	// AddOnNLBHelloWorld defines parameters for EKS cluster
	// add-on NLB hello-world service.
	AddOnNLBHelloWorld *AddOnNLBHelloWorld `json:"add-on-nlb-hello-world,omitempty"`
	// AddOnALB2048 defines parameters for EKS cluster
	// add-on ALB 2048 service.
	AddOnALB2048 *AddOnALB2048 `json:"add-on-alb-2048,omitempty"`
	// AddOnAppMesh defines parameters for EKS cluster
	// add-on "EKS App Mesh Integration".
	AddOnAppMesh *AddOnAppMesh `json:"add-on-app-mesh,omitempty"`
	// AddOnJobsPi defines parameters for EKS cluster
	// add-on Job with pi Perl command.
	AddOnJobsPi *AddOnJobsPi `json:"add-on-jobs-pi,omitempty"`
	// AddOnJobsEcho defines parameters for EKS cluster
	// add-on Job with echo.
	AddOnJobsEcho *AddOnJobsEcho `json:"add-on-jobs-echo,omitempty"`
	// AddOnCronJobs defines parameters for EKS cluster
	// add-on with CronJob.
	AddOnCronJobs *AddOnCronJobs `json:"add-on-cron-jobs,omitempty"`
	// AddOnCSRs defines parameters for EKS cluster
	// add-on with CSRs.
	AddOnCSRs *AddOnCSRs `json:"add-on-csrs,omitempty"`
	// AddOnConfigMaps defines parameters for EKS cluster
	// add-on with ConfigMap.
	AddOnConfigMaps *AddOnConfigMaps `json:"add-on-config-maps,omitempty"`
	// AddOnSecrets defines parameters for EKS cluster
	// add-on "Secrets".
	AddOnSecrets *AddOnSecrets `json:"add-on-secrets,omitempty"`
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

	// AddOnConformance defines parameters for EKS cluster
	// add-on local Hollow Nodes.
	AddOnHollowNodesLocal *AddOnHollowNodesLocal `json:"add-on-hollow-nodes-local,omitempty"`
	// AddOnConformance defines parameters for EKS cluster
	// add-on remote Hollow Nodes.
	AddOnHollowNodesRemote *AddOnHollowNodesRemote `json:"add-on-hollow-nodes-remote,omitempty"`

	// AddOnClusterLoaderLocal defines parameters for EKS cluster
	// add-on local Cluster Loader.
	AddOnClusterLoaderLocal *AddOnClusterLoaderLocal `json:"add-on-cluster-loader-local,omitempty"`
	// AddOnClusterLoaderRemote defines parameters for EKS cluster
	// add-on remote Cluster Loader.
	AddOnClusterLoaderRemote *AddOnClusterLoaderRemote `json:"add-on-cluster-loader-remote,omitempty"`

	// AddOnConformance defines parameters for EKS cluster
	// add-on Conformance.
	AddOnConformance *AddOnConformance `json:"add-on-conformance,omitempty"`

	// Status represents the current status of AWS resources.
	// Status is read-only.
	// Status cannot be configured via environmental variables.
	Status *Status `json:"status,omitempty" read-only:"true"`
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

	return cfg.unsafeSync()
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
			buf.WriteString("\n")
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
			buf.WriteString("\n")
		}
	}

	return buf.String()
}
