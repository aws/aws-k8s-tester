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
	// Tag is the tag used for S3 bucket name.
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

	// KubeProxyPath is the path to download the "kubectl".
	KubeProxyPath string `json:"kube-proxy-path,omitempty"`
	// KubeProxyDownloadURL is the download URL to download "kube-proxy" binary from.
	KubeProxyDownloadURL string `json:"kube-proxy-download-url,omitempty"`
	// KubectlPath is the path to download the "kubectl".
	KubectlPath string `json:"kubectl-path,omitempty"`
	// KubectlDownloadURL is the download URL to download "kubectl" binary from.
	KubectlDownloadURL string `json:"kubectl-download-url,omitempty"`
	// KubeletPath is the path to download the "kubelet".
	KubeletPath string `json:"kubelet-path,omitempty"`
	// KubeletDownloadURL is the download URL to download "kubelet" binary from.
	KubeletDownloadURL string `json:"kubelet-download-url,omitempty"`
	// KubeAPIServerPath is the path to download the "kube-apiserver".
	KubeAPIServerPath string `json:"kube-apiserver-path,omitempty"`
	// KubeAPIServerDownloadURL is the download URL to download "kube-apiserver" binary from.
	KubeAPIServerDownloadURL string `json:"kube-apiserver-download-url,omitempty"`
	// KubeControllerManagerPath is the path to download the "kube-controller-manager".
	KubeControllerManagerPath string `json:"kube-controller-manager-path,omitempty"`
	// KubeControllerManagerDownloadURL is the download URL to download "kube-controller-manager" binary from.
	KubeControllerManagerDownloadURL string `json:"kube-controller-manager-download-url,omitempty"`
	// KubeSchedulerPath is the path to download the "kube-scheduler".
	KubeSchedulerPath string `json:"kube-scheduler-path,omitempty"`
	// KubeSchedulerDownloadURL is the download URL to download "kube-scheduler" binary from.
	KubeSchedulerDownloadURL string `json:"kube-scheduler-download-url,omitempty"`
	// CloudControllerManagerPath is the path to download the Kubernetes "cloud-controller-manager".
	CloudControllerManagerPath string `json:"cloud-controller-manager-path,omitempty"`
	// CloudControllerManagerDownloadURL is the download URL to download Kubernetes "cloud-controller-manager" binary from.
	CloudControllerManagerDownloadURL string `json:"cloud-controller-manager-download-url,omitempty"`

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
	// EC2MasterNodes defines ec2 instance configuration for Kubernetes master components.
	EC2MasterNodes *ec2config.Config `json:"ec2-master-nodes"`
	// EC2WorkerNodes defines ec2 instance configuration for Kubernetes worker nodes.
	EC2WorkerNodes *ec2config.Config `json:"ec2-worker-nodes"`

	// TestTimeout is the test operation timeout.
	TestTimeout time.Duration `json:"test-timeout,omitempty"`
}

// NewDefault returns a copy of the default configuration.
func NewDefault() *Config {
	vv := defaultConfig
	return &vv
}

func init() {
	defaultConfig.Tag = genTag()
	defaultConfig.ClusterName = defaultConfig.Tag + "-" + randString(5)

	defaultConfig.ETCDNodes.EC2.Tag = defaultConfig.Tag + "-etcd-nodes"
	defaultConfig.ETCDNodes.EC2.ClusterName = defaultConfig.ClusterName + "-etcd-nodes"
	defaultConfig.ETCDNodes.EC2Bastion.Tag = defaultConfig.Tag + "-etcd-bastion-nodes"
	defaultConfig.ETCDNodes.EC2Bastion.ClusterName = defaultConfig.ClusterName + "-etcd-bastion-nodes"
	defaultConfig.EC2MasterNodes.Tag = defaultConfig.Tag + "-master-nodes"
	defaultConfig.EC2MasterNodes.ClusterName = defaultConfig.ClusterName + "-master-nodes"
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

	KubeProxyPath:                     "/usr/bin/kube-proxy",
	KubeProxyDownloadURL:              "https://storage.googleapis.com/kubernetes-release/release/v1.13.1/bin/linux/amd64/kube-proxy",
	KubectlPath:                       "/usr/bin/kubectl",
	KubectlDownloadURL:                "https://storage.googleapis.com/kubernetes-release/release/v1.13.1/bin/linux/amd64/kubectl",
	KubeletPath:                       "/usr/bin/kubelet",
	KubeletDownloadURL:                "https://storage.googleapis.com/kubernetes-release/release/v1.13.1/bin/linux/amd64/kubelet",
	KubeAPIServerPath:                 "/usr/bin/kube-apiserver",
	KubeAPIServerDownloadURL:          "https://storage.googleapis.com/kubernetes-release/release/v1.13.1/bin/linux/amd64/kube-apiserver",
	KubeControllerManagerPath:         "/usr/bin/kube-controller-manager",
	KubeControllerManagerDownloadURL:  "https://storage.googleapis.com/kubernetes-release/release/v1.13.1/bin/linux/amd64/kube-controller-manager",
	KubeSchedulerPath:                 "/usr/bin/kube-scheduler",
	KubeSchedulerDownloadURL:          "https://storage.googleapis.com/kubernetes-release/release/v1.13.1/bin/linux/amd64/kube-scheduler",
	CloudControllerManagerPath:        "/usr/bin/cloud-controller-manager",
	CloudControllerManagerDownloadURL: "https://storage.googleapis.com/kubernetes-release/release/v1.13.1/bin/linux/amd64/cloud-controller-manager",

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
	envPfx            = "AWS_K8S_TESTER_KUBERNETES_"
	envPfxMasterNodes = "AWS_K8S_TESTER_EC2_MASTER_NODES_"
	envPfxWorkerNodes = "AWS_K8S_TESTER_EC2_WORKER_NODES_"
)

// UpdateFromEnvs updates fields from environmental variables.
func (cfg *Config) UpdateFromEnvs() error {
	if err := cfg.ETCDNodes.UpdateFromEnvs(); err != nil {
		return err
	}
	cfg.EC2MasterNodes.EnvPrefix = envPfxMasterNodes
	cfg.EC2WorkerNodes.EnvPrefix = envPfxWorkerNodes
	if err := cfg.EC2MasterNodes.UpdateFromEnvs(); err != nil {
		return err
	}
	if err := cfg.EC2WorkerNodes.UpdateFromEnvs(); err != nil {
		return err
	}
	cc := *cfg

	tp1, vv1 := reflect.TypeOf(&cc).Elem(), reflect.ValueOf(&cc).Elem()
	for i := 0; i < tp1.NumField(); i++ {
		jv := tp1.Field(i).Tag.Get("json")
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

		switch vv1.Field(i).Type().Kind() {
		case reflect.String:
			vv1.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv1.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			if tp1.Field(i).Name == "WaitBeforeDown" ||
				tp1.Field(i).Name == "TestTimeout" {
				dv, err := time.ParseDuration(sv)
				if err != nil {
					return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
				}
				vv1.Field(i).SetInt(int64(dv))
				continue
			}
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv1.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv1.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv1.Field(i).SetFloat(fv)

		case reflect.Slice:
			ss := strings.Split(sv, ",")
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for i := range ss {
				slice.Index(i).SetString(ss[i])
			}
			vv1.Field(i).Set(slice)

		default:
			return fmt.Errorf("%q (%v) is not supported as an env", env, vv1.Field(i).Type())
		}
	}
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
