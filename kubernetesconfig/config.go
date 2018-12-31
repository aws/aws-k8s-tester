// Package kubernetesconfig defines Kubernetes configuration.
package kubernetesconfig

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/etcdconfig"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/yaml"
)

// Config defines kubeadm test configuration.
type Config struct {
	// Tag is the tag used for S3 bucket name and load balancer name.
	// If empty, deployer auto-populates it.
	Tag string `json:"tag,omitempty"`
	// ClusterName is the cluster name.
	// If empty, deployer auto-populates it.
	ClusterName string `json:"cluster-name,omitempty"`

	// WaitBeforeDown is the duration to sleep before cluster tear down.
	WaitBeforeDown time.Duration `json:"wait-before-down,omitempty"`
	// Down is true to automatically tear down cluster in "test".
	// Deployer implementation should not call "Down" inside "Up" method.
	// This is meant to be used as a flag for test.
	Down bool `json:"down"`

	KubeProxy              *KubeProxy              `json:"kube-proxy"`
	Kubectl                *Kubectl                `json:"kubectl"`
	Kubelet                *Kubelet                `json:"kubelet"`
	KubeAPIServer          *KubeAPIServer          `json:"kube-apiserver"`
	KubeControllerManager  *KubeControllerManager  `json:"kube-controller-manager"`
	KubeScheduler          *KubeScheduler          `json:"kube-scheduler"`
	CloudControllerManager *CloudControllerManager `json:"cloud-controller-manager"`

	// ConfigPath is the configuration file path.
	// Must be left empty, and let deployer auto-populate this field.
	// Deployer is expected to update this file with latest status,
	// and to make a backup of original configuration
	// with the filename suffix ".backup.yaml" in the same directory.
	ConfigPath string `json:"config-path,omitempty"`
	// ConfigPathBucket is the path inside S3 bucket.
	ConfigPathBucket string `json:"config-path-bucket,omitempty"`
	ConfigPathURL    string `json:"config-path-url,omitempty"`

	// KubeConfigPath is the file path of KUBECONFIG for the kubeadm cluster.
	// If empty, auto-generate one.
	// Deployer is expected to delete this on cluster tear down.
	KubeConfigPath string `json:"kubeconfig-path,omitempty"` // read-only to user
	// KubeConfigPathBucket is the path inside S3 bucket.
	KubeConfigPathBucket string `json:"kubeconfig-path-bucket,omitempty"` // read-only to user
	KubeConfigPathURL    string `json:"kubeconfig-path-url,omitempty"`    // read-only to user

	// AWSRegion is the AWS region.
	AWSRegion string `json:"aws-region,omitempty"`

	// LogDebug is true to enable debug level logging.
	LogDebug bool `json:"log-debug"`
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

	// LogsMasterNodes is a list of master node log file paths, fetched via SSH.
	LogsMasterNodes map[string]string `json:"logs-master-nodes,omitempty"`
	// LogsWorkerNodes is a list of worker node log file paths, fetched via SSH.
	LogsWorkerNodes map[string]string `json:"logs-worker-nodes,omitempty"`

	// UploadTesterLogs is true to auto-upload log files.
	UploadTesterLogs bool `json:"upload-tester-logs"`
	// UploadKubeConfig is true to auto-upload KUBECONFIG file.
	UploadKubeConfig bool `json:"upload-kubeconfig"`
	// UploadBucketExpireDays is the number of days for a S3 bucket to expire.
	// Set 0 to not expire.
	UploadBucketExpireDays int `json:"upload-bucket-expire-days"`

	// ETCDNodes defines etcd cluster.
	ETCDNodes *etcdconfig.Config `json:"etcd-nodes"`
	// ETCDNodesCreated is true to indicate that etcd nodes have been created,
	// thus needs clean-up on test complete.
	ETCDNodesCreated bool `json:"etcd-nodes-created"`
	// EC2MasterNodes defines ec2 instance configuration for Kubernetes master components.
	EC2MasterNodes *ec2config.Config `json:"ec2-master-nodes"`
	// EC2MasterNodesCreated is true to indicate that master nodes have been created,
	// thus needs clean-up on test complete.
	EC2MasterNodesCreated bool `json:"ec2-master-nodes-created"`
	// EC2WorkerNodes defines ec2 instance configuration for Kubernetes worker nodes.
	EC2WorkerNodes *ec2config.Config `json:"ec2-worker-nodes"`
	// EC2WorkerNodesCreated is true to indicate that worker nodes have been created,
	// thus needs clean-up on test complete.
	EC2WorkerNodesCreated bool `json:"ec2-worker-nodes-created"`

	// LoadBalancerName is the name of the load balancer.
	LoadBalancerName string `json:"load-balancer-name,omitempty"`
	// LoadBalancerDNSName is the DNS name output from load balancer creation.
	LoadBalancerDNSName string `json:"load-balancer-dns-name,omitempty"`
	// LoadBalancerCreated is true to indicate that load balancer has been created,
	// thus needs clean-up on test complete.
	LoadBalancerCreated bool `json:"load-balancer-created"`
	// LoadBalancerRegistered is true to indicate that load balancer has registered EC2 instances,
	// thus needs de-registration on test complete.
	LoadBalancerRegistered bool `json:"load-balancer-registered"`

	// TestTimeout is the test operation timeout.
	TestTimeout time.Duration `json:"test-timeout,omitempty"`
}

type KubeProxy struct {
	Path           string `json:"path"`
	DownloadURL    string `json:"download-url"`
	VersionCommand string `json:"version-command"`
}

type Kubectl struct {
	Path           string `json:"path"`
	DownloadURL    string `json:"download-url"`
	VersionCommand string `json:"version-command"`
}

type Kubelet struct {
	Path           string `json:"path"`
	DownloadURL    string `json:"download-url"`
	VersionCommand string `json:"version-command"`
}

type KubeAPIServer struct {
	Path           string `json:"path"`
	DownloadURL    string `json:"download-url"`
	VersionCommand string `json:"version-command"`
}

type KubeControllerManager struct {
	Path           string `json:"path"`
	DownloadURL    string `json:"download-url"`
	VersionCommand string `json:"version-command"`
}

type KubeScheduler struct {
	Path           string `json:"path"`
	DownloadURL    string `json:"download-url"`
	VersionCommand string `json:"version-command"`
}

type CloudControllerManager struct {
	Path           string `json:"path"`
	DownloadURL    string `json:"download-url"`
	VersionCommand string `json:"version-command"`
}

// NewDefault returns a copy of the default configuration.
func NewDefault() *Config {
	vv := defaultConfig
	return &vv
}

func init() {
	defaultConfig.Tag = genTag()
	defaultConfig.LoadBalancerName = defaultConfig.Tag + "-lb"
	defaultConfig.ClusterName = defaultConfig.Tag + "-" + randString(5)

	defaultConfig.ETCDNodes.EC2.AWSRegion = defaultConfig.AWSRegion
	defaultConfig.ETCDNodes.EC2.Tag = defaultConfig.Tag + "-etcd-nodes"
	defaultConfig.ETCDNodes.EC2.ClusterName = defaultConfig.ClusterName + "-etcd-nodes"
	defaultConfig.ETCDNodes.EC2Bastion.Tag = defaultConfig.Tag + "-etcd-bastion-nodes"
	defaultConfig.ETCDNodes.EC2Bastion.ClusterName = defaultConfig.ClusterName + "-etcd-bastion-nodes"
	defaultConfig.EC2MasterNodes.AWSRegion = defaultConfig.AWSRegion
	defaultConfig.EC2MasterNodes.Tag = defaultConfig.Tag + "-master-nodes"
	defaultConfig.EC2MasterNodes.ClusterName = defaultConfig.ClusterName + "-master-nodes"
	defaultConfig.EC2WorkerNodes.AWSRegion = defaultConfig.AWSRegion
	defaultConfig.EC2WorkerNodes.Tag = defaultConfig.Tag + "-worker-nodes"
	defaultConfig.EC2WorkerNodes.ClusterName = defaultConfig.ClusterName + "-worker-nodes"

	// TODO: use single node cluster for now
	defaultConfig.ETCDNodes.ClusterSize = 1

	// keep in-sync with the default value in https://godoc.org/k8s.io/kubernetes/test/e2e/framework#GetSigner
	defaultConfig.EC2MasterNodes.KeyPath = filepath.Join(homedir.HomeDir(), ".ssh", "kube_aws_rsa")
	defaultConfig.ETCDNodes.EC2.KeyPath = defaultConfig.EC2MasterNodes.KeyPath
	defaultConfig.ETCDNodes.EC2Bastion.KeyPath = defaultConfig.EC2MasterNodes.KeyPath
	defaultConfig.EC2WorkerNodes.KeyPath = defaultConfig.EC2MasterNodes.KeyPath

	defaultConfig.EC2MasterNodes.ClusterSize = 1
	defaultConfig.EC2MasterNodes.Wait = true
	defaultConfig.EC2MasterNodes.IngressRulesTCP = map[string]string{
		"22":          "0.0.0.0/0",      // SSH
		"6443":        "192.168.0.0/16", // Kubernetes API server
		"2379-2380":   "192.168.0.0/16", // etcd server client API
		"10250":       "192.168.0.0/16", // Kubelet API
		"10251":       "192.168.0.0/16", // kube-scheduler
		"10252":       "192.168.0.0/16", // kube-controller-manager
		"30000-32767": "192.168.0.0/16", // NodePort Services
	}

	defaultConfig.EC2WorkerNodes.ClusterSize = 1
	defaultConfig.EC2WorkerNodes.Wait = true
	defaultConfig.EC2WorkerNodes.IngressRulesTCP = map[string]string{
		"22":          "0.0.0.0/0",      // SSH
		"30000-32767": "192.168.0.0/16", // NodePort Services
	}

	// package "internal/ec2" defaults
	// Amazon Linux 2 AMI (HVM), SSD Volume Type
	// ImageID:  "ami-01bbe152bf19d0289"
	// UserName: "ec2-user"
	defaultConfig.EC2MasterNodes.Plugins = []string{
		"update-amazon-linux-2",
		"install-start-docker-amazon-linux-2",
		"install-kubernetes-amazon-linux-2",
	}
	defaultConfig.EC2WorkerNodes.Plugins = []string{
		"update-amazon-linux-2",
		"install-start-docker-amazon-linux-2",
		"install-kubernetes-amazon-linux-2",
	}
}

var masterNodesPorts = []string{
	"22",          // SSH
	"6443",        // Kubernetes API server
	"2379-2380",   // etcd server client API
	"10250",       // Kubelet API
	"10251",       // kube-scheduler
	"10252",       // kube-controller-manager
	"30000-32767", // NodePort Services
}

var workerNodesPorts = []string{
	"22",          // SSH
	"30000-32767", // NodePort Services
}

// genTag generates a tag for cluster name, CloudFormation, and S3 bucket.
// Note that this would be used as S3 bucket name to upload tester logs.
func genTag() string {
	// use UTC time for everything
	now := time.Now().UTC()
	return fmt.Sprintf("awsk8stester-kubernetes-%d%02d%02d", now.Year(), now.Month(), now.Day())
}

var defaultConfig = Config{
	WaitBeforeDown: time.Minute,
	Down:           true,

	KubeProxy: &KubeProxy{
		Path:           "/usr/bin/kube-proxy",
		DownloadURL:    "https://storage.googleapis.com/kubernetes-release/release/v1.13.1/bin/linux/amd64/kube-proxy",
		VersionCommand: "/usr/bin/kube-proxy --version",
	},
	Kubectl: &Kubectl{
		Path:           "/usr/bin/kubectl",
		DownloadURL:    "https://storage.googleapis.com/kubernetes-release/release/v1.13.1/bin/linux/amd64/kubectl",
		VersionCommand: "/usr/bin/kubectl version --client",
	},
	Kubelet: &Kubelet{
		Path:           "/usr/bin/kubelet",
		DownloadURL:    "https://storage.googleapis.com/kubernetes-release/release/v1.13.1/bin/linux/amd64/kubelet",
		VersionCommand: "/usr/bin/kubelet --version",
	},
	KubeAPIServer: &KubeAPIServer{
		Path:           "/usr/bin/kube-apiserver",
		DownloadURL:    "https://storage.googleapis.com/kubernetes-release/release/v1.13.1/bin/linux/amd64/kube-apiserver",
		VersionCommand: "/usr/bin/kube-apiserver --version",
	},
	KubeControllerManager: &KubeControllerManager{
		Path:           "/usr/bin/kube-controller-manager",
		DownloadURL:    "https://storage.googleapis.com/kubernetes-release/release/v1.13.1/bin/linux/amd64/kube-controller-manager",
		VersionCommand: "/usr/bin/kube-controller-manager --version",
	},
	KubeScheduler: &KubeScheduler{
		Path:           "/usr/bin/kube-scheduler",
		DownloadURL:    "https://storage.googleapis.com/kubernetes-release/release/v1.13.1/bin/linux/amd64/kube-scheduler",
		VersionCommand: "/usr/bin/kube-scheduler --version",
	},
	CloudControllerManager: &CloudControllerManager{
		Path:           "/usr/bin/cloud-controller-manager",
		DownloadURL:    "https://storage.googleapis.com/kubernetes-release/release/v1.13.1/bin/linux/amd64/cloud-controller-manager",
		VersionCommand: "/usr/bin/cloud-controller-manager --version",
	},

	AWSRegion: "us-west-2",

	LogDebug: false,
	// default, stderr, stdout, or file name
	// log file named with cluster name will be added automatically
	LogOutputs:             []string{"stderr"},
	UploadTesterLogs:       false,
	UploadKubeConfig:       false,
	UploadBucketExpireDays: 2,

	KubeConfigPath: "/tmp/aws-k8s-tester/kubeconfig",

	ETCDNodes:      etcdconfig.NewDefault(),
	EC2MasterNodes: ec2config.NewDefault(),
	EC2WorkerNodes: ec2config.NewDefault(),

	TestTimeout: 10 * time.Second,
}

// Load loads configuration from YAML.
// Useful when injecting shared configuration via ConfigMap.
//
// Example usage:
//
//  import "github.com/aws/aws-k8s-tester/kubernetesconfig"
//  cfg := kubernetesconfig.Load("test.yaml")
//  p, err := cfg.BackupConfig()
//  err = cfg.ValidateAndSetDefaults()
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
			return err
		}
	}
	var d []byte
	d, err = yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(cfg.ConfigPath, d, 0600)
}

// BackupConfig stores the original aws-k8s-tester configuration
// file to backup, suffixed with ".backup.yaml".
// Otherwise, deployer will overwrite its state back to YAML.
// Useful when the original configuration would be reused
// for other tests.
func (cfg *Config) BackupConfig() (p string, err error) {
	var d []byte
	d, err = ioutil.ReadFile(cfg.ConfigPath)
	if err != nil {
		return "", err
	}
	p = fmt.Sprintf("%s.%X.backup.yaml",
		cfg.ConfigPath,
		time.Now().UTC().UnixNano(),
	)
	return p, ioutil.WriteFile(p, d, 0600)
}

const (
	envPfx                       = "AWS_K8S_TESTER_KUBERNETES_"
	envPfxKubeProxy              = envPfx + "KUBE_PROXY_"
	envPfxKubectl                = envPfx + "KUBECTL_"
	envPfxKubelet                = envPfx + "KUBELET_"
	envPfxKubeAPIServer          = envPfx + "KUBE_APISERVER_"
	envPfxKubeControllerManager  = envPfx + "KUBE_CONTROLLER_MANAGER_"
	envPfxKubeScheduler          = envPfx + "KUBE_SCHEDULER_"
	envPfxCloudControllerManager = envPfx + "CLOUD_CONTROLLER_MANAGER_"
	envPfxMasterNodes            = "AWS_K8S_TESTER_EC2_MASTER_NODES_"
	envPfxWorkerNodes            = "AWS_K8S_TESTER_EC2_WORKER_NODES_"
)

// UpdateFromEnvs updates fields from environmental variables.
func (cfg *Config) UpdateFromEnvs() error {
	cfg.ETCDNodes.EC2.AWSRegion = cfg.AWSRegion
	cfg.EC2MasterNodes.AWSRegion = cfg.AWSRegion
	cfg.EC2WorkerNodes.AWSRegion = cfg.AWSRegion

	if err := cfg.ETCDNodes.UpdateFromEnvs(); err != nil {
		return err
	}

	cfg.EC2MasterNodes.EnvPrefix = envPfxMasterNodes
	if err := cfg.EC2MasterNodes.UpdateFromEnvs(); err != nil {
		return err
	}

	cfg.EC2WorkerNodes.EnvPrefix = envPfxWorkerNodes
	if err := cfg.EC2WorkerNodes.UpdateFromEnvs(); err != nil {
		return err
	}

	cc := *cfg

	tpTop, vvTop := reflect.TypeOf(&cc).Elem(), reflect.ValueOf(&cc).Elem()
	for i := 0; i < tpTop.NumField(); i++ {
		jv := tpTop.Field(i).Tag.Get("json")
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

		switch vvTop.Field(i).Type().Kind() {
		case reflect.String:
			vvTop.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvTop.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			if tpTop.Field(i).Name == "WaitBeforeDown" ||
				tpTop.Field(i).Name == "TestTimeout" {
				dv, err := time.ParseDuration(sv)
				if err != nil {
					return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
				}
				vvTop.Field(i).SetInt(int64(dv))
				continue
			}
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvTop.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvTop.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvTop.Field(i).SetFloat(fv)

		case reflect.Slice:
			ss := strings.Split(sv, ",")
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for i := range ss {
				slice.Index(i).SetString(ss[i])
			}
			vvTop.Field(i).Set(slice)

		default:
			return fmt.Errorf("%q (%v) is not supported as an env", env, vvTop.Field(i).Type())
		}
	}

	kubeProxy := *cc.KubeProxy
	tpKubeProxy, vvKubeProxy := reflect.TypeOf(&kubeProxy).Elem(), reflect.ValueOf(&kubeProxy).Elem()
	for i := 0; i < tpKubeProxy.NumField(); i++ {
		jv := tpKubeProxy.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.Replace(jv, "-", "_", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := envPfxKubeProxy + jv
		if os.Getenv(env) == "" {
			continue
		}
		sv := os.Getenv(env)

		switch vvKubeProxy.Field(i).Type().Kind() {
		case reflect.String:
			vvKubeProxy.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeProxy.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			// if tpKubeProxy.Field(i).Name { continue }
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeProxy.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeProxy.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeProxy.Field(i).SetFloat(fv)

		case reflect.Slice:
			ss := strings.Split(sv, ",")
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for i := range ss {
				slice.Index(i).SetString(ss[i])
			}
			vvKubeProxy.Field(i).Set(slice)

		default:
			return fmt.Errorf("%q (%v) is not supported as an env", env, vvKubeProxy.Field(i).Type())
		}
	}
	cc.KubeProxy = &kubeProxy

	kubectl := *cc.Kubectl
	tpKubectl, vvKubectl := reflect.TypeOf(&kubectl).Elem(), reflect.ValueOf(&kubectl).Elem()
	for i := 0; i < tpKubectl.NumField(); i++ {
		jv := tpKubectl.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.Replace(jv, "-", "_", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := envPfxKubectl + jv
		if os.Getenv(env) == "" {
			continue
		}
		sv := os.Getenv(env)

		switch vvKubectl.Field(i).Type().Kind() {
		case reflect.String:
			vvKubectl.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubectl.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			// if tpKubectl.Field(i).Name { continue }
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubectl.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubectl.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubectl.Field(i).SetFloat(fv)

		case reflect.Slice:
			ss := strings.Split(sv, ",")
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for i := range ss {
				slice.Index(i).SetString(ss[i])
			}
			vvKubectl.Field(i).Set(slice)

		default:
			return fmt.Errorf("%q (%v) is not supported as an env", env, vvKubectl.Field(i).Type())
		}
	}
	cc.Kubectl = &kubectl

	kubelet := *cc.Kubelet
	tpKubelet, vvKubelet := reflect.TypeOf(&kubelet).Elem(), reflect.ValueOf(&kubelet).Elem()
	for i := 0; i < tpKubelet.NumField(); i++ {
		jv := tpKubelet.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.Replace(jv, "-", "_", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := envPfxKubelet + jv
		if os.Getenv(env) == "" {
			continue
		}
		sv := os.Getenv(env)

		switch vvKubelet.Field(i).Type().Kind() {
		case reflect.String:
			vvKubelet.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubelet.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			// if tpKubelet.Field(i).Name { continue }
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubelet.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubelet.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubelet.Field(i).SetFloat(fv)

		case reflect.Slice:
			ss := strings.Split(sv, ",")
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for i := range ss {
				slice.Index(i).SetString(ss[i])
			}
			vvKubelet.Field(i).Set(slice)

		default:
			return fmt.Errorf("%q (%v) is not supported as an env", env, vvKubelet.Field(i).Type())
		}
	}
	cc.Kubelet = &kubelet

	kubeAPIServer := *cc.KubeAPIServer
	tpKubeAPIServer, vvKubeAPIServer := reflect.TypeOf(&kubeAPIServer).Elem(), reflect.ValueOf(&kubeAPIServer).Elem()
	for i := 0; i < tpKubeAPIServer.NumField(); i++ {
		jv := tpKubeAPIServer.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.Replace(jv, "-", "_", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := envPfxKubeAPIServer + jv
		if os.Getenv(env) == "" {
			continue
		}
		sv := os.Getenv(env)

		switch vvKubeAPIServer.Field(i).Type().Kind() {
		case reflect.String:
			vvKubeAPIServer.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeAPIServer.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			// if tpKubeAPIServer.Field(i).Name { continue }
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeAPIServer.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeAPIServer.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeAPIServer.Field(i).SetFloat(fv)

		case reflect.Slice:
			ss := strings.Split(sv, ",")
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for i := range ss {
				slice.Index(i).SetString(ss[i])
			}
			vvKubeAPIServer.Field(i).Set(slice)

		default:
			return fmt.Errorf("%q (%v) is not supported as an env", env, vvKubeAPIServer.Field(i).Type())
		}
	}
	cc.KubeAPIServer = &kubeAPIServer

	kubeControllerManager := *cc.KubeControllerManager
	tpKubeControllerManager, vvKubeControllerManager := reflect.TypeOf(&kubeControllerManager).Elem(), reflect.ValueOf(&kubeControllerManager).Elem()
	for i := 0; i < tpKubeControllerManager.NumField(); i++ {
		jv := tpKubeControllerManager.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.Replace(jv, "-", "_", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := envPfxKubeControllerManager + jv
		if os.Getenv(env) == "" {
			continue
		}
		sv := os.Getenv(env)

		switch vvKubeControllerManager.Field(i).Type().Kind() {
		case reflect.String:
			vvKubeControllerManager.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeControllerManager.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			// if tpKubeControllerManager.Field(i).Name { continue }
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeControllerManager.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeControllerManager.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeControllerManager.Field(i).SetFloat(fv)

		case reflect.Slice:
			ss := strings.Split(sv, ",")
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for i := range ss {
				slice.Index(i).SetString(ss[i])
			}
			vvKubeControllerManager.Field(i).Set(slice)

		default:
			return fmt.Errorf("%q (%v) is not supported as an env", env, vvKubeControllerManager.Field(i).Type())
		}
	}
	cc.KubeControllerManager = &kubeControllerManager

	kubeScheduler := *cc.KubeScheduler
	tpKubeScheduler, vvKubeScheduler := reflect.TypeOf(&kubeScheduler).Elem(), reflect.ValueOf(&kubeScheduler).Elem()
	for i := 0; i < tpKubeScheduler.NumField(); i++ {
		jv := tpKubeScheduler.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.Replace(jv, "-", "_", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := envPfxKubeScheduler + jv
		if os.Getenv(env) == "" {
			continue
		}
		sv := os.Getenv(env)

		switch vvKubeScheduler.Field(i).Type().Kind() {
		case reflect.String:
			vvKubeScheduler.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeScheduler.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			// if tpKubeScheduler.Field(i).Name { continue }
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeScheduler.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeScheduler.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeScheduler.Field(i).SetFloat(fv)

		case reflect.Slice:
			ss := strings.Split(sv, ",")
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for i := range ss {
				slice.Index(i).SetString(ss[i])
			}
			vvKubeScheduler.Field(i).Set(slice)

		default:
			return fmt.Errorf("%q (%v) is not supported as an env", env, vvKubeScheduler.Field(i).Type())
		}
	}
	cc.KubeScheduler = &kubeScheduler

	cloudControllerManager := *cc.CloudControllerManager
	tpCloudControllerManager, vvCloudControllerManager := reflect.TypeOf(&cloudControllerManager).Elem(), reflect.ValueOf(&cloudControllerManager).Elem()
	for i := 0; i < tpCloudControllerManager.NumField(); i++ {
		jv := tpCloudControllerManager.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.Replace(jv, "-", "_", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := envPfxCloudControllerManager + jv
		if os.Getenv(env) == "" {
			continue
		}
		sv := os.Getenv(env)

		switch vvCloudControllerManager.Field(i).Type().Kind() {
		case reflect.String:
			vvCloudControllerManager.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvCloudControllerManager.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			// if tpCloudControllerManager.Field(i).Name { continue }
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvCloudControllerManager.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvCloudControllerManager.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvCloudControllerManager.Field(i).SetFloat(fv)

		case reflect.Slice:
			ss := strings.Split(sv, ",")
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for i := range ss {
				slice.Index(i).SetString(ss[i])
			}
			vvCloudControllerManager.Field(i).Set(slice)

		default:
			return fmt.Errorf("%q (%v) is not supported as an env", env, vvCloudControllerManager.Field(i).Type())
		}
	}
	cc.CloudControllerManager = &cloudControllerManager

	*cfg = cc
	return nil
}

// ValidateAndSetDefaults returns an error for invalid configurations.
// And updates empty fields with default values.
// At the end, it writes populated YAML to aws-k8s-tester config path.
func (cfg *Config) ValidateAndSetDefaults() (err error) {
	if cfg.EC2MasterNodes == nil {
		return errors.New("EC2MasterNodes configuration not found")
	}
	if err = cfg.EC2MasterNodes.ValidateAndSetDefaults(); err != nil {
		return err
	}
	for _, p := range masterNodesPorts {
		_, ok := cfg.EC2MasterNodes.IngressRulesTCP[p]
		if !ok {
			return fmt.Errorf("master node expects port %q but not found from %v", p, cfg.EC2MasterNodes.IngressRulesTCP)
		}
	}
	if cfg.EC2WorkerNodes == nil {
		return errors.New("EC2WorkerNodes configuration not found")
	}

	// let master node EC2 deployer create SSH key
	// and share the same SSH key for master and worker nodes
	cfg.ETCDNodes.EC2.KeyName = cfg.EC2MasterNodes.KeyName
	cfg.ETCDNodes.EC2.KeyPath = cfg.EC2MasterNodes.KeyPath
	cfg.ETCDNodes.EC2.KeyCreateSkip = true
	cfg.ETCDNodes.EC2.KeyCreated = false
	cfg.ETCDNodes.EC2Bastion.KeyName = cfg.EC2MasterNodes.KeyName
	cfg.ETCDNodes.EC2Bastion.KeyPath = cfg.EC2MasterNodes.KeyPath
	cfg.ETCDNodes.EC2Bastion.KeyCreateSkip = true
	cfg.ETCDNodes.EC2Bastion.KeyCreated = false
	cfg.EC2WorkerNodes.KeyName = cfg.EC2MasterNodes.KeyName
	cfg.EC2WorkerNodes.KeyPath = cfg.EC2MasterNodes.KeyPath
	cfg.EC2WorkerNodes.KeyCreateSkip = true
	cfg.EC2WorkerNodes.KeyCreated = false

	if cfg.ETCDNodes == nil {
		return errors.New("ETCDNodes configuration not found")
	}
	if err = cfg.ETCDNodes.ValidateAndSetDefaults(); err != nil {
		return err
	}

	if err = cfg.EC2WorkerNodes.ValidateAndSetDefaults(); err != nil {
		return err
	}
	for _, p := range workerNodesPorts {
		_, ok := cfg.EC2WorkerNodes.IngressRulesTCP[p]
		if !ok {
			return fmt.Errorf("worker node expects port %q but not found from %v", p, cfg.EC2WorkerNodes.IngressRulesTCP)
		}
	}

	okAMZLnx, okDocker, okKubernetes := false, false, false
	for _, v := range cfg.EC2MasterNodes.Plugins {
		if v == "update-amazon-linux-2" {
			okAMZLnx = true
			continue
		}
		if strings.HasPrefix(v, "install-start-docker-amazon-linux-2") {
			okDocker = true
			continue
		}
		if strings.HasPrefix(v, "install-kubernetes-amazon-linux-2") {
			okKubernetes = true
			continue
		}
	}
	if !okAMZLnx {
		return errors.New("EC2MasterNodes Plugin 'update-amazon-linux-2' not found")
	}
	if !okDocker {
		return errors.New("EC2MasterNodes Plugin 'install-start-docker-amazon-linux-2' not found")
	}
	if !okKubernetes {
		return errors.New("EC2MasterNodes Plugin 'install-kubernetes-amazon-linux-2' not found")
	}
	okAMZLnx, okDocker, okKubernetes = false, false, false
	for _, v := range cfg.EC2WorkerNodes.Plugins {
		if v == "update-amazon-linux-2" {
			okAMZLnx = true
			continue
		}
		if strings.HasPrefix(v, "install-start-docker-amazon-linux-2") {
			okDocker = true
			continue
		}
		if strings.HasPrefix(v, "install-kubernetes-amazon-linux-2") {
			okKubernetes = true
			continue
		}
	}
	if !okAMZLnx {
		return errors.New("EC2WorkerNodes Plugin 'update-amazon-linux-2' not found")
	}
	if !okDocker {
		return errors.New("EC2WorkerNodes Plugin 'install-start-docker-amazon-linux-2' not found")
	}
	if !okKubernetes {
		return errors.New("EC2MasterNodes Plugin 'install-kubernetes-amazon-linux-2' not found")
	}

	if !cfg.EC2MasterNodes.Wait {
		return errors.New("Set EC2MasterNodes Wait to true")
	}
	if cfg.EC2MasterNodes.UserName != "ec2-user" {
		return fmt.Errorf("EC2MasterNodes.UserName expected 'ec2-user' user name, got %q", cfg.EC2MasterNodes.UserName)
	}
	if !cfg.EC2WorkerNodes.Wait {
		return errors.New("Set EC2WorkerNodes Wait to true")
	}
	if cfg.EC2WorkerNodes.UserName != "ec2-user" {
		return fmt.Errorf("EC2WorkerNodes.UserName expected 'ec2-user' user name, got %q", cfg.EC2WorkerNodes.UserName)
	}

	if cfg.Tag == "" {
		return errors.New("Tag is empty")
	}
	if cfg.ClusterName == "" {
		return errors.New("ClusterName is empty")
	}

	// populate all paths on disks and on remote storage
	if cfg.ConfigPath == "" {
		f, err := ioutil.TempFile(os.TempDir(), "awsk8stester-kubernetesconfig")
		if err != nil {
			return err
		}
		cfg.ConfigPath, _ = filepath.Abs(f.Name())
		f.Close()
		os.RemoveAll(cfg.ConfigPath)
	}
	cfg.ConfigPathBucket = filepath.Join(cfg.ClusterName, "awsk8stester-kubernetesconfig.yaml")

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
	cfg.LogOutputToUploadPathBucket = filepath.Join(cfg.ClusterName, "awsk8stester-kubernetes.log")

	cfg.KubeConfigPathBucket = filepath.Join(cfg.ClusterName, "kubeconfig")

	return cfg.Sync()
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

// Download contains fields for downloading a Kubernetes component.
type Download struct {
	Path            string
	DownloadURL     string
	DownloadCommand string
	VersionCommand  string
}

// DownloadsMaster returns all download commands for Kubernetes master.
func (cfg *Config) DownloadsMaster() []Download {
	return []Download{
		{
			Path:        cfg.KubeProxy.Path,
			DownloadURL: cfg.KubeProxy.DownloadURL,
			DownloadCommand: fmt.Sprintf(
				"sudo rm -f %s && sudo curl --silent -L --remote-name-all %s -o %s && sudo chmod +x %s && %s",
				cfg.KubeProxy.Path, cfg.KubeProxy.DownloadURL, cfg.KubeProxy.Path, cfg.KubeProxy.Path, cfg.KubeProxy.VersionCommand,
			),
			VersionCommand: cfg.KubeProxy.VersionCommand,
		},
		{
			Path:        cfg.Kubectl.Path,
			DownloadURL: cfg.Kubectl.DownloadURL,
			DownloadCommand: fmt.Sprintf(
				"sudo rm -f %s && sudo curl --silent -L --remote-name-all %s -o %s && sudo chmod +x %s && %s",
				cfg.Kubectl.Path, cfg.Kubectl.DownloadURL, cfg.Kubectl.Path, cfg.Kubectl.Path, cfg.Kubectl.VersionCommand,
			),
			VersionCommand: cfg.Kubectl.VersionCommand,
		},
		{
			Path:        cfg.Kubelet.Path,
			DownloadURL: cfg.Kubelet.DownloadURL,
			DownloadCommand: fmt.Sprintf(
				"sudo rm -f %s && sudo curl --silent -L --remote-name-all %s -o %s && sudo chmod +x %s && %s",
				cfg.Kubelet.Path, cfg.Kubelet.DownloadURL, cfg.Kubelet.Path, cfg.Kubelet.Path, cfg.Kubelet.VersionCommand,
			),
			VersionCommand: cfg.Kubelet.VersionCommand,
		},
		{
			Path:        cfg.KubeAPIServer.Path,
			DownloadURL: cfg.KubeAPIServer.DownloadURL,
			DownloadCommand: fmt.Sprintf(
				"sudo rm -f %s && sudo curl --silent -L --remote-name-all %s -o %s && sudo chmod +x %s && %s",
				cfg.KubeAPIServer.Path, cfg.KubeAPIServer.DownloadURL, cfg.KubeAPIServer.Path, cfg.KubeAPIServer.Path, cfg.KubeAPIServer.VersionCommand,
			),
			VersionCommand: cfg.KubeAPIServer.VersionCommand,
		},
		{
			Path:        cfg.KubeControllerManager.Path,
			DownloadURL: cfg.KubeControllerManager.DownloadURL,
			DownloadCommand: fmt.Sprintf(
				"sudo rm -f %s && sudo curl --silent -L --remote-name-all %s -o %s && sudo chmod +x %s && %s",
				cfg.KubeControllerManager.Path, cfg.KubeControllerManager.DownloadURL, cfg.KubeControllerManager.Path, cfg.KubeControllerManager.Path, cfg.KubeControllerManager.VersionCommand,
			),
			VersionCommand: cfg.KubeControllerManager.VersionCommand,
		},
		{
			Path:        cfg.KubeScheduler.Path,
			DownloadURL: cfg.KubeScheduler.DownloadURL,
			DownloadCommand: fmt.Sprintf(
				"sudo rm -f %s && sudo curl --silent -L --remote-name-all %s -o %s && sudo chmod +x %s && %s",
				cfg.KubeScheduler.Path, cfg.KubeScheduler.DownloadURL, cfg.KubeScheduler.Path, cfg.KubeScheduler.Path, cfg.KubeScheduler.VersionCommand,
			),
			VersionCommand: cfg.KubeScheduler.VersionCommand,
		},
		{
			Path:        cfg.CloudControllerManager.Path,
			DownloadURL: cfg.CloudControllerManager.DownloadURL,
			DownloadCommand: fmt.Sprintf(
				"sudo rm -f %s && sudo curl --silent -L --remote-name-all %s -o %s && sudo chmod +x %s && %s",
				cfg.CloudControllerManager.Path, cfg.CloudControllerManager.DownloadURL, cfg.CloudControllerManager.Path, cfg.CloudControllerManager.Path, cfg.CloudControllerManager.VersionCommand,
			),
			VersionCommand: cfg.CloudControllerManager.VersionCommand,
		},
	}
}

// DownloadsWorker returns all download commands for Kubernetes worker.
func (cfg *Config) DownloadsWorker() (ds []Download) {
	for _, v := range cfg.DownloadsMaster() {
		if strings.HasSuffix(v.Path, "kube-apiserver") {
			continue
		}
		ds = append(ds, v)
	}
	return ds
}
