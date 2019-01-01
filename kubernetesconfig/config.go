// Package kubernetesconfig defines Kubernetes configuration.
package kubernetesconfig

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"text/template"
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

	KubeletMasterNodes     *Kubelet                `json:"kubelet-master-nodes"`
	KubeletWorkerNodes     *Kubelet                `json:"kubelet-worker-nodes"`
	KubeProxyMasterNodes   *KubeProxy              `json:"kube-proxy-master-nodes"`
	KubeProxyWorkerNodes   *KubeProxy              `json:"kube-proxy-worker-nodes"`
	Kubectl                *Kubectl                `json:"kubectl"`
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

type Kubelet struct {
	Path           string `json:"path"`
	DownloadURL    string `json:"download-url"`
	VersionCommand string `json:"version-command"`

	AllowPrivileged         bool   `json:"allow-privileged" kubelet:"allow-privileged"`
	AnonymousAuth           bool   `json:"anonymous-auth" kubelet:"anonymous-auth"`
	CgroupRoot              string `json:"cgroup-root" kubelet:"cgroup-root"`
	ClientCAFile            string `json:"client-ca-file" kubelet:"client-ca-file"`
	CloudProvider           string `json:"cloud-provider" kubelet:"cloud-provider"`
	ClusterDNS              string `json:"cluster-dns" kubelet:"cluster-dns"`
	ClusterDomain           string `json:"cluster-domain" kubelet:"cluster-domain"`
	EnableDebuggingHandlers bool   `json:"enable-debugging-handlers" kubelet:"enable-debugging-handlers"`
	EvictionHard            string `json:"eviction-hard" kubelet:"eviction-hard"`
	FeatureGates            string `json:"feature-gates" kubelet:"feature-gates"`
	HostnameOverride        string `json:"hostname-override" kubelet:"hostname-override"`
	Kubeconfig              string `json:"kubeconfig" kubelet:"kubeconfig"`
	NetworkPluginMTU        int64  `json:"network-plugin-mtu" kubelet:"network-plugin-mtu"`
	NetworkPlugin           string `json:"network-plugin" kubelet:"network-plugin"`
	NodeLabels              string `json:"node-labels" kubelet:"node-labels"`
	NonMasqueradeCIDR       string `json:"non-masquerade-cidr" kubelet:"non-masquerade-cidr"`
	PodInfraContainerImage  string `json:"pod-infra-container-image" kubelet:"pod-infra-container-image"`
	PodManifestPath         string `json:"pod-manifest-path" kubelet:"pod-manifest-path"`
	RegisterSchedulable     bool   `json:"register-schedulable" kubelet:"register-schedulable"`
	RegisterWithTaints      string `json:"register-with-taints" kubelet:"register-with-taints"`
	V                       int    `json:"v" kubelet:"v"`
	CNIBinDir               string `json:"cni-bin-dir" kubelet:"cni-bin-dir"`
	CNIConfDir              string `json:"cni-conf-dir" kubelet:"cni-conf-dir"`
}

// KubeProxy defines kube-proxy configuration.
// Reference: https://godoc.org/k8s.io/kube-proxy/config/v1alpha1#KubeProxyConfiguration.
type KubeProxy struct {
	// Image is the container image name and tag for kube-proxy to run as a static pod.
	Image string `json:"image"`

	ClusterCIDR         string `json:"cluster-cidr" kube-proxy:"cluster-cidr"`
	ConntrackMaxPerCore int64  `json:"conntrack-max-per-core" kube-proxy:"conntrack-max-per-core"`
	HostnameOverride    string `json:"hostname-override" kube-proxy:"hostname-override"`
	Kubeconfig          string `json:"kubeconfig" kube-proxy:"kubeconfig"`
	Master              string `json:"master" kube-proxy:"master"`
	OOMScoreAdj         int    `json:"oom-score-adj" kube-proxy:"oom-score-adj"`
	ResourceContainer   string `json:"resource-container" kube-proxy:"resource-container" allow-empty:"true"`
}

type Kubectl struct {
	Path           string `json:"path"`
	DownloadURL    string `json:"download-url"`
	VersionCommand string `json:"version-command"`
}

type KubeAPIServer struct {
	// Image is the container image name and tag for kube-apiserver to run as a static pod.
	Image string `json:"image"`

	AllowPrivileged                 bool   `json:"allow-privileged" kube-apiserver:"allow-privileged"`
	AnonymousAuth                   bool   `json:"anonymous-auth" kube-apiserver:"anonymous-auth"`
	APIServerCount                  int    `json:"apiserver-count" kube-apiserver:"apiserver-count"`
	AuthorizationMode               string `json:"authorization-mode" kube-apiserver:"authorization-mode"`
	BasicAuthFile                   string `json:"basic-auth-file" kube-apiserver:"basic-auth-file"`
	BindAddress                     string `json:"bind-address" kube-apiserver:"bind-address"`
	ClientCAFile                    string `json:"client-ca-file" kube-apiserver:"client-ca-file"`
	CloudProvider                   string `json:"cloud-provider" kube-apiserver:"cloud-provider"`
	EnableAdmissionPlugins          string `json:"enable-admission-plugins" kube-apiserver:"enable-admission-plugins"`
	EtcdServersOverrides            string `json:"etcd-servers-overrides" kube-apiserver:"etcd-servers-overrides"`
	EtcdServers                     string `json:"etcd-servers" kube-apiserver:"etcd-servers"`
	InsecureBindAddress             string `json:"insecure-bind-address" kube-apiserver:"insecure-bind-address"`
	InsecurePort                    int    `json:"insecure-port" kube-apiserver:"insecure-port"`
	KubeletClientCertificate        string `json:"kubelet-client-certificate" kube-apiserver:"kubelet-client-certificate"`
	KubeletClientKey                string `json:"kubelet-client-key" kube-apiserver:"kubelet-client-key"`
	KubeletPreferredAddressTypes    string `json:"kubelet-preferred-address-types" kube-apiserver:"kubelet-preferred-address-types"`
	ProxyClientCertFile             string `json:"proxy-client-cert-file" kube-apiserver:"proxy-client-cert-file"`
	ProxyClientKeyFile              string `json:"proxy-client-key-file" kube-apiserver:"proxy-client-key-file"`
	RequestHeaderAllowedNames       string `json:"request-header-allowed-names" kube-apiserver:"requestheader-allowed-names"`
	RequestHeaderClientCAFile       string `json:"request-header-client-ca-file" kube-apiserver:"requestheader-client-ca-file"`
	RequestHeaderExtraHeadersPrefix string `json:"request-header-extra-headers-prefix" kube-apiserver:"requestheader-extra-headers-prefix"`
	RequestHeaderGroupHeaders       string `json:"request-header-group-headers" kube-apiserver:"requestheader-group-headers"`
	RequestHeaderUsernameHeaders    string `json:"request-header-username-headers" kube-apiserver:"requestheader-username-headers"`
	SecurePort                      int    `json:"secure-port" kube-apiserver:"secure-port"`
	ServiceClusterIPRange           string `json:"service-cluster-ip-range" kube-apiserver:"service-cluster-ip-range"`
	StorageBackend                  string `json:"storage-backend" kube-apiserver:"storage-backend"`
	TLSCertFile                     string `json:"tls-cert-file" kube-apiserver:"tls-cert-file"`
	TLSPrivateKeyFile               string `json:"tls-private-key-file" kube-apiserver:"tls-private-key-file"`
	TokenAuthFile                   string `json:"token-auth-file" kube-apiserver:"token-auth-file"`
	V                               int    `json:"v" kube-apiserver:"v"`
}

type KubeControllerManager struct {
	// Image is the container image name and tag for kube-controller-manager to run as a static pod.
	Image string `json:"image"`

	AllocateNodeCIDRs               bool   `json:"allocate-node-cidrs" kube-controller-manager:"allocate-node-cidrs"`
	AttachDetachReconcileSyncPeriod string `json:"attach-detach-reconcile-sync-period" kube-controller-manager:"attach-detach-reconcile-sync-period"`
	CloudProvider                   string `json:"cloud-provider" kube-controller-manager:"cloud-provider"`
	ClusterCIDR                     string `json:"cluster-cidr" kube-controller-manager:"cluster-cidr"`
	ClusterName                     string `json:"cluster-name" kube-controller-manager:"cluster-name"`
	ClusterSigningCertFile          string `json:"cluster-signing-cert-file" kube-controller-manager:"cluster-signing-cert-file"`
	ClusterSigningKeyFile           string `json:"cluster-signing-key-file" kube-controller-manager:"cluster-signing-key-file"`
	ConfigureCloudRoutes            bool   `json:"configure-cloud-routes" kube-controller-manager:"configure-cloud-routes"`
	Kubeconfig                      string `json:"kubeconfig" kube-controller-manager:"kubeconfig"`
	LeaderElect                     bool   `json:"leader-elect" kube-controller-manager:"leader-elect"`
	RootCAFile                      string `json:"root-ca-file" kube-controller-manager:"root-ca-file"`
	ServiceAccountPrivateKeyFile    string `json:"service-account-private-key-file" kube-controller-manager:"service-account-private-key-file"`
	UseServiceAccountCredentials    bool   `json:"use-service-account-credentials" kube-controller-manager:"use-service-account-credentials"`
	V                               int    `json:"v" kube-controller-manager:"v"`
}

type KubeScheduler struct {
	// Image is the container image name and tag for kube-scheduler to run as a static pod.
	Image string `json:"image"`

	Kubeconfig  string `json:"kubeconfig" kube-scheduler:"kubeconfig"`
	LeaderElect bool   `json:"leader-elect" kube-scheduler:"leader-elect"`
}

type CloudControllerManager struct {
	// Image is the container image name and tag for cloud-controller-manager to run as a static pod.
	Image string `json:"image"`
}

// NewDefault returns a copy of the default configuration.
func NewDefault() *Config {
	vv := defaultConfig
	return &vv
}

func init() {
	defaultConfig.Tag = genTag()
	defaultConfig.ClusterName = defaultConfig.Tag + "-" + randString(7)
	defaultConfig.LoadBalancerName = defaultConfig.ClusterName + "-lb"

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
	return fmt.Sprintf("a8t-k8s-%d%02d%02d", now.Year(), now.Month(), now.Day())
}

var defaultConfig = Config{
	WaitBeforeDown: time.Minute,
	Down:           true,

	KubeletMasterNodes: &Kubelet{
		Path:           "/usr/bin/kubelet",
		DownloadURL:    "https://storage.googleapis.com/kubernetes-release/release/v1.13.1/bin/linux/amd64/kubelet",
		VersionCommand: "/usr/bin/kubelet --version",

		AllowPrivileged:         true,
		AnonymousAuth:           false,
		CgroupRoot:              "/",
		ClientCAFile:            "/srv/kubernetes/ca.crt",
		CloudProvider:           "aws",
		ClusterDNS:              "100.64.0.10", // ??
		ClusterDomain:           "cluster.local",
		EnableDebuggingHandlers: true,
		EvictionHard:            "memory.available<100Mi,nodefs.available<10%,nodefs.inodesFree<5%,imagefs.available<10%,imagefs.inodesFree<5%",
		FeatureGates:            "ExperimentalCriticalPodAnnotation=true",
		HostnameOverride:        "PRIVATE_DNS",
		Kubeconfig:              "/var/lib/kubelet/kubeconfig",
		NetworkPluginMTU:        9001,
		NetworkPlugin:           "kubenet",
		NodeLabels:              "aws-k8s-tester.k8s.io/instancegroup=master-us-west-2a,kubernetes.io/role=master,node-role.kubernetes.io/master=",
		NonMasqueradeCIDR:       "100.64.0.0/10",
		PodInfraContainerImage:  "k8s.gcr.io/pause-amd64:3.0",
		PodManifestPath:         "/etc/kubernetes/manifests",
		RegisterSchedulable:     true,
		RegisterWithTaints:      "node-role.kubernetes.io/master=:NoSchedule",
		V:                       2,
		CNIBinDir:               "/opt/cni/bin/",
		CNIConfDir:              "/etc/cni/net.d/",
	},
	KubeletWorkerNodes: &Kubelet{
		Path:           "/usr/bin/kubelet",
		DownloadURL:    "https://storage.googleapis.com/kubernetes-release/release/v1.13.1/bin/linux/amd64/kubelet",
		VersionCommand: "/usr/bin/kubelet --version",

		AllowPrivileged:         true,
		AnonymousAuth:           false,
		CgroupRoot:              "/",
		ClientCAFile:            "/srv/kubernetes/ca.crt",
		CloudProvider:           "aws",
		ClusterDNS:              "100.64.0.10", // ??
		ClusterDomain:           "cluster.local",
		EnableDebuggingHandlers: true,
		EvictionHard:            "memory.available<100Mi,nodefs.available<10%,nodefs.inodesFree<5%,imagefs.available<10%,imagefs.inodesFree<5%",
		FeatureGates:            "ExperimentalCriticalPodAnnotation=true",
		HostnameOverride:        "PRIVATE_DNS",
		Kubeconfig:              "/var/lib/kubelet/kubeconfig",
		NetworkPluginMTU:        9001,
		NetworkPlugin:           "kubenet",
		NodeLabels:              "aws-k8s-tester.k8s.io/instancegroup=nodes,kubernetes.io/role=node,node-role.kubernetes.io/node=",
		NonMasqueradeCIDR:       "100.64.0.0/10",
		PodInfraContainerImage:  "k8s.gcr.io/pause-amd64:3.0",
		PodManifestPath:         "/etc/kubernetes/manifests",
		RegisterSchedulable:     true,
		V:                       2,
		CNIBinDir:               "/opt/cni/bin/",
		CNIConfDir:              "/etc/cni/net.d/",
	},

	KubeProxyMasterNodes: &KubeProxy{
		Image: "k8s.gcr.io/kube-proxy:v1.13.1",
	},
	KubeProxyWorkerNodes: &KubeProxy{
		Image: "k8s.gcr.io/kube-proxy:v1.13.1",
	},

	Kubectl: &Kubectl{
		Path:           "/usr/bin/kubectl",
		DownloadURL:    "https://storage.googleapis.com/kubernetes-release/release/v1.13.1/bin/linux/amd64/kubectl",
		VersionCommand: "/usr/bin/kubectl version --client",
	},

	KubeAPIServer: &KubeAPIServer{
		AllowPrivileged:                 true,
		AnonymousAuth:                   false,
		APIServerCount:                  1,
		AuthorizationMode:               "RBAC",
		BasicAuthFile:                   "/srv/kubernetes/basic_auth.csv",
		BindAddress:                     "0.0.0.0",
		ClientCAFile:                    "/srv/kubernetes/ca.crt",
		CloudProvider:                   "aws",
		EnableAdmissionPlugins:          "Initializers,NamespaceLifecycle,LimitRanger,ServiceAccount,PersistentVolumeLabel,DefaultStorageClass,DefaultTolerationSeconds,MutatingAdmissionWebhook,ValidatingAdmissionWebhook,NodeRestriction,ResourceQuota",
		EtcdServersOverrides:            "/events#http://127.0.0.1:4002",
		EtcdServers:                     "http://127.0.0.1:4001",
		InsecureBindAddress:             "127.0.0.1",
		InsecurePort:                    8080,
		KubeletClientCertificate:        "/srv/kubernetes/kubelet-api.pem",
		KubeletClientKey:                "/srv/kubernetes/kubelet-api-key.pem",
		KubeletPreferredAddressTypes:    "InternalIP,Hostname,ExternalIP",
		ProxyClientCertFile:             "/srv/kubernetes/apiserver-aggregator.cert",
		ProxyClientKeyFile:              "/srv/kubernetes/apiserver-aggregator.key",
		RequestHeaderAllowedNames:       "aggregator",
		RequestHeaderClientCAFile:       "/srv/kubernetes/apiserver-aggregator-ca.cert",
		RequestHeaderExtraHeadersPrefix: "X-Remote-Extra-",
		RequestHeaderGroupHeaders:       "X-Remote-Group",
		RequestHeaderUsernameHeaders:    "X-Remote-User",
		SecurePort:                      443,
		ServiceClusterIPRange:           "100.64.0.0/13",
		StorageBackend:                  "etcd3",
		TLSCertFile:                     "/srv/kubernetes/server.cert",
		TLSPrivateKeyFile:               "/srv/kubernetes/server.key",
		TokenAuthFile:                   "/srv/kubernetes/known_tokens.csv",
		V:                               2,
	},
	KubeControllerManager: &KubeControllerManager{
		AllocateNodeCIDRs:               true,
		AttachDetachReconcileSyncPeriod: "1m0s",
		CloudProvider:                   "aws",
		ClusterCIDR:                     "100.96.0.0/11",
		ClusterName:                     "leegyuho-kops.k8s.local",
		ClusterSigningCertFile:          "/srv/kubernetes/ca.crt",
		ClusterSigningKeyFile:           "/srv/kubernetes/ca.key",
		ConfigureCloudRoutes:            true,
		Kubeconfig:                      "/var/lib/kube-controller-manager/kubeconfig",
		LeaderElect:                     true,
		RootCAFile:                      "/srv/kubernetes/ca.crt",
		ServiceAccountPrivateKeyFile:    "/srv/kubernetes/server.key",
		UseServiceAccountCredentials:    true,
		V:                               2,
	},
	KubeScheduler: &KubeScheduler{
		Image:       "k8s.gcr.io/kube-apiserver:v1.13.1",
		Kubeconfig:  "/var/lib/kube-scheduler/kubeconfig",
		LeaderElect: true,
	},
	CloudControllerManager: &CloudControllerManager{},

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
	envPfxKubeProxyMasterNodes   = envPfx + "KUBE_PROXY_MASTER_NODES_"
	envPfxKubeProxyWorkerNodes   = envPfx + "KUBE_PROXY_WORKER_NODES_"
	envPfxKubectl                = envPfx + "KUBECTL_"
	envPfxKubeletMasterNodes     = envPfx + "KUBELET_MASTER_NODES_"
	envPfxKubeletWorkerNodes     = envPfx + "KUBELET_WORKER_NODES_"
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

	kubeletMasterNodes := *cc.KubeletMasterNodes
	tpKubeletMasterNodes, vvKubeletMasterNodes := reflect.TypeOf(&kubeletMasterNodes).Elem(), reflect.ValueOf(&kubeletMasterNodes).Elem()
	for i := 0; i < tpKubeletMasterNodes.NumField(); i++ {
		jv := tpKubeletMasterNodes.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.Replace(jv, "-", "_", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := envPfxKubeletMasterNodes + jv
		if os.Getenv(env) == "" {
			continue
		}
		sv := os.Getenv(env)

		switch vvKubeletMasterNodes.Field(i).Type().Kind() {
		case reflect.String:
			vvKubeletMasterNodes.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeletMasterNodes.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			// if tpKubeletMasterNodes.Field(i).Name { continue }
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeletMasterNodes.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeletMasterNodes.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeletMasterNodes.Field(i).SetFloat(fv)

		case reflect.Slice:
			ss := strings.Split(sv, ",")
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for i := range ss {
				slice.Index(i).SetString(ss[i])
			}
			vvKubeletMasterNodes.Field(i).Set(slice)

		default:
			return fmt.Errorf("%q (%v) is not supported as an env", env, vvKubeletMasterNodes.Field(i).Type())
		}
	}
	cc.KubeletMasterNodes = &kubeletMasterNodes

	kubeletWorkerNodes := *cc.KubeletWorkerNodes
	tpKubeletWorkerNodes, vvKubeletWorkerNodes := reflect.TypeOf(&kubeletWorkerNodes).Elem(), reflect.ValueOf(&kubeletWorkerNodes).Elem()
	for i := 0; i < tpKubeletWorkerNodes.NumField(); i++ {
		jv := tpKubeletWorkerNodes.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.Replace(jv, "-", "_", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := envPfxKubeletWorkerNodes + jv
		if os.Getenv(env) == "" {
			continue
		}
		sv := os.Getenv(env)

		switch vvKubeletWorkerNodes.Field(i).Type().Kind() {
		case reflect.String:
			vvKubeletWorkerNodes.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeletWorkerNodes.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			// if tpKubeletWorkerNodes.Field(i).Name { continue }
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeletWorkerNodes.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeletWorkerNodes.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeletWorkerNodes.Field(i).SetFloat(fv)

		case reflect.Slice:
			ss := strings.Split(sv, ",")
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for i := range ss {
				slice.Index(i).SetString(ss[i])
			}
			vvKubeletWorkerNodes.Field(i).Set(slice)

		default:
			return fmt.Errorf("%q (%v) is not supported as an env", env, vvKubeletWorkerNodes.Field(i).Type())
		}
	}
	cc.KubeletWorkerNodes = &kubeletWorkerNodes

	kubeProxyMasterNodes := *cc.KubeProxyMasterNodes
	tpKubeProxyMasterNodes, vvKubeProxyMasterNodes := reflect.TypeOf(&kubeProxyMasterNodes).Elem(), reflect.ValueOf(&kubeProxyMasterNodes).Elem()
	for i := 0; i < tpKubeProxyMasterNodes.NumField(); i++ {
		jv := tpKubeProxyMasterNodes.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.Replace(jv, "-", "_", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := envPfxKubeProxyMasterNodes + jv
		if os.Getenv(env) == "" {
			continue
		}
		sv := os.Getenv(env)

		switch vvKubeProxyMasterNodes.Field(i).Type().Kind() {
		case reflect.String:
			vvKubeProxyMasterNodes.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeProxyMasterNodes.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			// if tpKubeProxyMasterNodes.Field(i).Name { continue }
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeProxyMasterNodes.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeProxyMasterNodes.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeProxyMasterNodes.Field(i).SetFloat(fv)

		case reflect.Slice:
			ss := strings.Split(sv, ",")
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for i := range ss {
				slice.Index(i).SetString(ss[i])
			}
			vvKubeProxyMasterNodes.Field(i).Set(slice)

		default:
			return fmt.Errorf("%q (%v) is not supported as an env", env, vvKubeProxyMasterNodes.Field(i).Type())
		}
	}
	cc.KubeProxyMasterNodes = &kubeProxyMasterNodes

	kubeProxyWorkerNodes := *cc.KubeProxyWorkerNodes
	tpKubeProxyWorkerNodes, vvKubeProxyWorkerNodes := reflect.TypeOf(&kubeProxyWorkerNodes).Elem(), reflect.ValueOf(&kubeProxyWorkerNodes).Elem()
	for i := 0; i < tpKubeProxyWorkerNodes.NumField(); i++ {
		jv := tpKubeProxyWorkerNodes.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.Replace(jv, "-", "_", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := envPfxKubeProxyWorkerNodes + jv
		if os.Getenv(env) == "" {
			continue
		}
		sv := os.Getenv(env)

		switch vvKubeProxyWorkerNodes.Field(i).Type().Kind() {
		case reflect.String:
			vvKubeProxyWorkerNodes.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeProxyWorkerNodes.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			// if tpKubeProxyWorkerNodes.Field(i).Name { continue }
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeProxyWorkerNodes.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeProxyWorkerNodes.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vvKubeProxyWorkerNodes.Field(i).SetFloat(fv)

		case reflect.Slice:
			ss := strings.Split(sv, ",")
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for i := range ss {
				slice.Index(i).SetString(ss[i])
			}
			vvKubeProxyWorkerNodes.Field(i).Set(slice)

		default:
			return fmt.Errorf("%q (%v) is not supported as an env", env, vvKubeProxyWorkerNodes.Field(i).Type())
		}
	}
	cc.KubeProxyWorkerNodes = &kubeProxyWorkerNodes

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

	// to prevent "ValidationError: LoadBalancer name cannot be longer than 32 characters"
	if len(cfg.LoadBalancerName) > 31 {
		cfg.LoadBalancerName = cfg.LoadBalancerName[len(cfg.LoadBalancerName)-31:]
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
			Path:        cfg.KubeletMasterNodes.Path,
			DownloadURL: cfg.KubeletMasterNodes.DownloadURL,
			DownloadCommand: fmt.Sprintf(
				"sudo rm -f %s && sudo curl --silent -L --remote-name-all %s -o %s && sudo chmod +x %s && %s",
				cfg.KubeletMasterNodes.Path, cfg.KubeletMasterNodes.DownloadURL, cfg.KubeletMasterNodes.Path, cfg.KubeletMasterNodes.Path, cfg.KubeletMasterNodes.VersionCommand,
			),
			VersionCommand: cfg.KubeletMasterNodes.VersionCommand,
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
	}
}

// DownloadsWorker returns all download commands for Kubernetes worker.
func (cfg *Config) DownloadsWorker() (ds []Download) {
	return []Download{
		{
			Path:        cfg.KubeletWorkerNodes.Path,
			DownloadURL: cfg.KubeletWorkerNodes.DownloadURL,
			DownloadCommand: fmt.Sprintf(
				"sudo rm -f %s && sudo curl --silent -L --remote-name-all %s -o %s && sudo chmod +x %s && %s",
				cfg.KubeletWorkerNodes.Path, cfg.KubeletWorkerNodes.DownloadURL, cfg.KubeletWorkerNodes.Path, cfg.KubeletWorkerNodes.Path, cfg.KubeletWorkerNodes.VersionCommand,
			),
			VersionCommand: cfg.KubeletWorkerNodes.VersionCommand,
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
	}
}

// Service returns a script to configure Kubernetes Kubelet systemd service file.
func (kb *Kubelet) Service() (s string, err error) {
	tpl := template.Must(template.New("kubeletTemplate").Parse(kubeletTemplate))
	buf := bytes.NewBuffer(nil)
	kv := kubeletTemplateInfo{KubeletPath: kb.Path}
	if err := tpl.Execute(buf, kv); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type kubeletTemplateInfo struct {
	KubeletPath string
}

const kubeletTemplate = `#!/usr/bin/env bash

sudo systemctl stop kubelet.service || true

sudo mkdir -p /etc/sysconfig/
sudo rm -f /etc/sysconfig/kubelet
sudo touch /etc/sysconfig/kubelet

sudo rm -rf /var/lib/kubelet/
sudo mkdir -p /var/lib/kubelet/
sudo rm -f /var/lib/kubelet/kubeconfig

sudo rm -rf /srv/kubernetes/
sudo mkdir -p /srv/kubernetes/

sudo rm -rf /etc/kubernetes/manifests/
sudo mkdir -p /etc/kubernetes/manifests/

sudo rm -rf /opt/cni/bin/
sudo mkdir -p /opt/cni/bin/

sudo rm -rf /etc/cni/net.d/
sudo mkdir -p /etc/cni/net.d/

rm -f /tmp/kubelet.service
cat <<EOF > /tmp/kubelet.service
[Unit]
Description=kubelet: The Kubernetes Node Agent
Documentation=http://kubernetes.io/docs/
After=docker.service

[Service]
EnvironmentFile=/etc/sysconfig/kubelet
ExecStart={{ .KubeletPath }} "\$DAEMON_ARGS"
Restart=always
RestartSec=2s
StartLimitInterval=0
KillMode=process
User=root

[Install]
WantedBy=multi-user.target
EOF
cat /tmp/kubelet.service

sudo mkdir -p /etc/systemd/system/kubelet.service.d
sudo cp /tmp/kubelet.service /etc/systemd/system/kubelet.service

sudo systemctl daemon-reload
sudo systemctl cat kubelet.service
`

// Flags returns the list of "kubelet" flags.
// Make sure to validate the configuration with "ValidateAndSetDefaults".
func (kb *Kubelet) Flags() (flags []string, err error) {
	tp, vv := reflect.TypeOf(kb).Elem(), reflect.ValueOf(kb).Elem()
	for i := 0; i < tp.NumField(); i++ {
		k := tp.Field(i).Tag.Get("kubelet")
		if k == "" {
			continue
		}

		switch vv.Field(i).Type().Kind() {
		case reflect.String:
			if vv.Field(i).String() != "" {
				flags = append(flags, fmt.Sprintf("--%s=%s", k, vv.Field(i).String()))
			}

		case reflect.Int, reflect.Int32, reflect.Int64:
			flags = append(flags, fmt.Sprintf("--%s=%d", k, vv.Field(i).Int()))

		case reflect.Bool:
			flags = append(flags, fmt.Sprintf("--%s=%v", k, vv.Field(i).Bool()))

		default:
			return nil, fmt.Errorf("unknown %q", k)
		}
	}
	return flags, nil
}

func (kb *Kubelet) Sysconfig() (s string, err error) {
	var fs []string
	fs, err = kb.Flags()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`DAEMON_ARGS="%s"
HOME="/root"
`, strings.Join(fs, " ")), nil
}

// Flags returns the list of "kube-proxy" flags.
// Make sure to validate the configuration with "ValidateAndSetDefaults".
func (kb *KubeProxy) Flags() (flags []string, err error) {
	tp, vv := reflect.TypeOf(kb).Elem(), reflect.ValueOf(kb).Elem()
	for i := 0; i < tp.NumField(); i++ {
		k := tp.Field(i).Tag.Get("kube-proxy")
		if k == "" {
			continue
		}
		allowEmpty := tp.Field(i).Tag.Get("allow-empty") == "true"

		switch vv.Field(i).Type().Kind() {
		case reflect.String:
			if vv.Field(i).String() != "" {
				flags = append(flags, fmt.Sprintf("--%s=%s", k, vv.Field(i).String()))
			} else if allowEmpty {
				// e.g. handle --resource-container=""
				flags = append(flags, fmt.Sprintf(`--%s=""`, k))
			}

		case reflect.Int, reflect.Int32, reflect.Int64:
			flags = append(flags, fmt.Sprintf("--%s=%d", k, vv.Field(i).Int()))

		case reflect.Bool:
			flags = append(flags, fmt.Sprintf("--%s=%v", k, vv.Field(i).Bool()))

		default:
			return nil, fmt.Errorf("unknown %q", k)
		}
	}
	return flags, nil
}

// Flags returns the list of "kube-controller-manager" flags.
// Make sure to validate the configuration with "ValidateAndSetDefaults".
func (kb *KubeControllerManager) Flags() (flags []string, err error) {
	tp, vv := reflect.TypeOf(kb).Elem(), reflect.ValueOf(kb).Elem()
	for i := 0; i < tp.NumField(); i++ {
		k := tp.Field(i).Tag.Get("kube-controller-manager")
		if k == "" {
			continue
		}

		switch vv.Field(i).Type().Kind() {
		case reflect.String:
			if vv.Field(i).String() != "" {
				flags = append(flags, fmt.Sprintf("--%s=%s", k, vv.Field(i).String()))
			}

		case reflect.Int, reflect.Int32, reflect.Int64:
			flags = append(flags, fmt.Sprintf("--%s=%d", k, vv.Field(i).Int()))

		case reflect.Bool:
			flags = append(flags, fmt.Sprintf("--%s=%v", k, vv.Field(i).Bool()))

		default:
			return nil, fmt.Errorf("unknown %q", k)
		}
	}
	return flags, nil
}

// Flags returns the list of "kube-scheduler" flags.
// Make sure to validate the configuration with "ValidateAndSetDefaults".
func (kb *KubeScheduler) Flags() (flags []string, err error) {
	tp, vv := reflect.TypeOf(kb).Elem(), reflect.ValueOf(kb).Elem()
	for i := 0; i < tp.NumField(); i++ {
		k := tp.Field(i).Tag.Get("kube-scheduler")
		if k == "" {
			continue
		}

		switch vv.Field(i).Type().Kind() {
		case reflect.String:
			if vv.Field(i).String() != "" {
				flags = append(flags, fmt.Sprintf("--%s=%s", k, vv.Field(i).String()))
			}

		case reflect.Int, reflect.Int32, reflect.Int64:
			flags = append(flags, fmt.Sprintf("--%s=%d", k, vv.Field(i).Int()))

		case reflect.Bool:
			flags = append(flags, fmt.Sprintf("--%s=%v", k, vv.Field(i).Bool()))

		default:
			return nil, fmt.Errorf("unknown %q", k)
		}
	}
	return flags, nil
}

// Flags returns the list of "kube-apiserver" flags.
// Make sure to validate the configuration with "ValidateAndSetDefaults".
func (kb *KubeAPIServer) Flags() (flags []string, err error) {
	tp, vv := reflect.TypeOf(kb).Elem(), reflect.ValueOf(kb).Elem()
	for i := 0; i < tp.NumField(); i++ {
		k := tp.Field(i).Tag.Get("kube-apiserver")
		if k == "" {
			continue
		}

		switch vv.Field(i).Type().Kind() {
		case reflect.String:
			if vv.Field(i).String() != "" {
				flags = append(flags, fmt.Sprintf("--%s=%s", k, vv.Field(i).String()))
			}

		case reflect.Int, reflect.Int32, reflect.Int64:
			flags = append(flags, fmt.Sprintf("--%s=%d", k, vv.Field(i).Int()))

		case reflect.Bool:
			flags = append(flags, fmt.Sprintf("--%s=%v", k, vv.Field(i).Bool()))

		default:
			return nil, fmt.Errorf("unknown %q", k)
		}
	}
	return flags, nil
}

/*
KubeProxyWorkerNodes: &KubeProxy{
	Path:           "/usr/bin/kube-proxy",
	DownloadURL:    "https://storage.googleapis.com/kubernetes-release/release/v1.13.1/bin/linux/amd64/kube-proxy",
	VersionCommand: "/usr/bin/kube-proxy --version",
},
Kubectl: &Kubectl{
	Path:           "/usr/bin/kubectl",
	DownloadURL:    "https://storage.googleapis.com/kubernetes-release/release/v1.13.1/bin/linux/amd64/kubectl",
	VersionCommand: "/usr/bin/kubectl version --client",
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
*/
