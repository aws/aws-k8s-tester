package eksconfig

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/pkg/aws"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-sdk-go/service/eks"
	"k8s.io/client-go/util/homedir"
)

const (
	// DefaultClients is the default number of clients to create.
	DefaultClients = 3
	// DefaultClientQPS is the default client QPS.
	DefaultClientQPS float32 = 5.0
	// DefaultClientBurst is the default client burst.
	DefaultClientBurst = 10
	// DefaultClientTimeout is the default client timeout.
	DefaultClientTimeout = 30 * time.Second

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
	NGMaxLimit = 300

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
	cfg := Config{
		mu: new(sync.RWMutex),

		Name: fmt.Sprintf("eks-%s-%s", getTS()[:10], randString(12)),

		// to be auto-generated
		ConfigPath:                "",
		KubectlCommandsOutputPath: "",
		KubeConfigPath:            "",
		AWSCLIPath:                "",

		Region: "us-west-2",

		LogLevel: logutil.DefaultLogLevel,
		// default, stderr, stdout, or file name
		// log file named with cluster name will be added automatically
		LogOutputs: []string{"stderr"},

		KubectlPath: "/tmp/kubectl-test-v1.16.9",
		// https://github.com/kubernetes/kubernetes/tags
		// https://kubernetes.io/docs/tasks/tools/install-kubectl/
		// https://docs.aws.amazon.com/eks/latest/userguide/install-kubectl.html
		KubectlDownloadURL: "https://storage.googleapis.com/kubernetes-release/release/v1.16.9/bin/linux/amd64/kubectl",

		OnFailureDelete:            true,
		OnFailureDeleteWaitSeconds: 120,

		S3BucketName:                    "",
		S3BucketCreate:                  false,
		S3BucketLifecycleExpirationDays: 0,

		Parameters: &Parameters{
			RoleCreate:          true,
			VPCCreate:           true,
			SigningName:         "eks",
			Version:             "1.15",
			EncryptionCMKCreate: true,
		},

		RemoteAccessKeyCreate: true,
		// keep in-sync with the default value in https://pkg.go.dev/k8s.io/kubernetes/test/e2e/framework#GetSigner
		RemoteAccessPrivateKeyPath: filepath.Join(homedir.HomeDir(), ".ssh", "kube_aws_rsa"),

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

		AddOnNodeGroups: &AddOnNodeGroups{
			Enable:     false,
			FetchLogs:  true,
			RoleCreate: true,
			LogsDir:    "", // to be auto-generated
		},

		AddOnManagedNodeGroups: &AddOnManagedNodeGroups{
			Enable:      false,
			FetchLogs:   true,
			SigningName: "eks",
			RoleCreate:  true,
			LogsDir:     "", // to be auto-generated
		},

		AddOnCSIEBS: &AddOnCSIEBS{
			Enable: false,
			// https://github.com/kubernetes-sigs/aws-ebs-csi-driver#deploy-driver
			ChartRepoURL: "https://github.com/kubernetes-sigs/aws-ebs-csi-driver/releases/download/v0.5.0/helm-chart.tgz",
		},

		AddOnNLBHelloWorld: &AddOnNLBHelloWorld{
			Enable:             false,
			DeploymentReplicas: 3,
		},

		AddOnALB2048: &AddOnALB2048{
			Enable:                 false,
			DeploymentReplicasALB:  3,
			DeploymentReplicas2048: 3,
		},

		AddOnJobsPi: &AddOnJobsPi{
			Enable:    false,
			Completes: 30,
			Parallels: 10,
		},

		AddOnJobsEcho: &AddOnJobsEcho{
			Enable:    false,
			Completes: 10,
			Parallels: 10,
			EchoSize:  100 * 1024, // 100 KB

			// writes total 100 MB data to etcd
			// Completes: 1000,
			// Parallels: 100,
			// EchoSize: 100 * 1024, // 100 KB
		},

		AddOnCronJobs: &AddOnCronJobs{
			Enable:                     false,
			Schedule:                   "*/10 * * * *", // every 10-min
			Completes:                  10,
			Parallels:                  10,
			SuccessfulJobsHistoryLimit: 3,
			FailedJobsHistoryLimit:     1,
			EchoSize:                   100 * 1024, // 100 KB
		},

		AddOnCSRs: &AddOnCSRs{
			Enable: false,

			InitialRequestConditionType: "",

			Objects: 10,

			// writes total 5 MB data to etcd
			// Objects: 1000,
		},

		AddOnConfigMaps: &AddOnConfigMaps{
			Enable:  false,
			Objects: 10,
			Size:    10 * 1024, // 10 KB

			// writes total 300 MB data to etcd
			// Objects: 1000,
			// Size: 300000, // 0.3 MB
		},

		AddOnSecrets: &AddOnSecrets{
			Enable:  false,
			Objects: 10,
			Size:    10 * 1024, // 10 KB

			// writes total 100 MB for "Secret" objects,
			// plus "Pod" objects, writes total 330 MB to etcd
			//
			// with 3 nodes, takes about 1.5 hour for all
			// these "Pod"s to complete
			//
			// Objects:     10000,
			// Size:        10 * 1024, // 10 KB
		},

		AddOnIRSA: &AddOnIRSA{
			Enable:             false,
			DeploymentReplicas: 10,
		},

		AddOnFargate: &AddOnFargate{
			Enable:     false,
			RoleCreate: true,
		},

		AddOnIRSAFargate: &AddOnIRSAFargate{
			Enable: false,
		},

		AddOnAppMesh: &AddOnAppMesh{
			Enable: false,
		},

		AddOnKubernetesDashboard: &AddOnKubernetesDashboard{
			Enable: false,
			URL:    defaultKubernetesDashboardURL,
		},

		AddOnPrometheusGrafana: &AddOnPrometheusGrafana{
			Enable:               false,
			GrafanaAdminUserName: "admin",
			GrafanaAdminPassword: "",
		},

		AddOnWordpress: &AddOnWordpress{
			Enable:   false,
			UserName: "user",
			Password: "",
		},

		AddOnJupyterHub: &AddOnJupyterHub{
			Enable: false,
		},

		AddOnKubeflow: &AddOnKubeflow{
			Enable:           false,
			KfctlPath:        "/tmp/kfctl-test-v1.0.2",
			KfctlDownloadURL: "https://github.com/kubeflow/kfctl/releases/download/v1.0.2/kfctl_v1.0.2-0-ga476281_linux.tar.gz",
		},

		AddOnClusterLoader: &AddOnClusterLoader{
			Enable: false,
		},

		// read-only
		Status: &Status{Up: false},
	}

	if name := os.Getenv(EnvironmentVariablePrefix + "NAME"); name != "" {
		cfg.Name = name
	}

	// https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-welcome.html
	// pip3 install awscli --no-cache-dir --upgrade
	var err error
	cfg.AWSCLIPath, err = exec.LookPath("aws")
	if err != nil {
		panic(fmt.Errorf("aws CLI is not installed (%v)", err))
	}

	if runtime.GOOS == "darwin" {
		cfg.KubectlDownloadURL = strings.Replace(cfg.KubectlDownloadURL, "linux", "darwin", -1)
		if cfg.IsEnabledAddOnKubeflow() {
			cfg.AddOnKubeflow.KfctlDownloadURL = strings.Replace(cfg.AddOnKubeflow.KfctlDownloadURL, "linux", "darwin", -1)
		}
		cfg.RemoteAccessPrivateKeyPath = filepath.Join(os.TempDir(), randString(10)+".insecure.key")
	}

	cfg.AddOnNodeGroups.ASGs = map[string]ASG{
		cfg.Name + "-ng-asg-cpu": ASG{
			ASG: ec2config.ASG{
				Name:                 cfg.Name + "-ng-asg-cpu",
				RemoteAccessUserName: "ec2-user", // assume Amazon Linux 2
				AMIType:              eks.AMITypesAl2X8664,
				ImageIDSSMParameter:  "/aws/service/eks/optimized-ami/1.15/amazon-linux-2/recommended/image_id",
				ASGMinSize:           1,
				ASGMaxSize:           1,
				ASGDesiredCapacity:   1,
				InstanceTypes:        []string{DefaultNodeInstanceTypeCPU},
				VolumeSize:           DefaultNodeVolumeSize,
			},
			KubeletExtraArgs: "",
		},
	}
	cfg.AddOnManagedNodeGroups.MNGs = map[string]MNG{
		cfg.Name + "-mng-cpu": {
			Name:                 cfg.Name + "-mng-cpu",
			RemoteAccessUserName: "ec2-user", // assume Amazon Linux 2
			ReleaseVersion:       "",         // to be auto-filled by EKS API
			AMIType:              eks.AMITypesAl2X8664,
			ASGMinSize:           2,
			ASGMaxSize:           2,
			ASGDesiredCapacity:   2,
			InstanceTypes:        []string{DefaultNodeInstanceTypeCPU},
			VolumeSize:           DefaultNodeVolumeSize,
		},
	}

	return &cfg
}

// ValidateAndSetDefaults returns an error for invalid configurations.
// And updates empty fields with default values.
// At the end, it writes populated YAML to aws-k8s-tester config path.
func (cfg *Config) ValidateAndSetDefaults() error {
	if cfg.mu == nil {
		cfg.mu = new(sync.RWMutex)
	}
	cfg.mu.Lock()
	defer func() {
		cfg.unsafeSync()
		cfg.mu.Unlock()
	}()

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
	if err := cfg.validateAddOnCSIEBS(); err != nil {
		return fmt.Errorf("validateAddOnCSIEBS failed [%v]", err)
	}
	if err := cfg.validateAddOnNLBHelloWorld(); err != nil {
		return fmt.Errorf("validateAddOnNLBHelloWorld failed [%v]", err)
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
	if err := cfg.validateAddOnCSRs(); err != nil {
		return fmt.Errorf("validateAddOnCSRs failed [%v]", err)
	}
	if err := cfg.validateAddOnConfigMaps(); err != nil {
		return fmt.Errorf("validateAddOnConfigMaps failed [%v]", err)
	}
	if err := cfg.validateAddOnSecrets(); err != nil {
		return fmt.Errorf("validateAddOnSecrets failed [%v]", err)
	}
	if err := cfg.validateAddOnIRSA(); err != nil {
		return fmt.Errorf("validateAddOnIRSA failed [%v]", err)
	}
	if err := cfg.validateAddOnFargate(); err != nil {
		return fmt.Errorf("validateAddOnFargate failed [%v]", err)
	}
	if err := cfg.validateAddOnIRSAFargate(); err != nil {
		return fmt.Errorf("validateIRSAAddOnFargate failed [%v]", err)
	}
	if err := cfg.validateAddOnAppMesh(); err != nil {
		return fmt.Errorf("validateAddOnAppMesh failed [%v]", err)
	}
	if err := cfg.validateAddOnKubernetesDashboard(); err != nil {
		return fmt.Errorf("validateAddOnKubernetesDashboard failed [%v]", err)
	}
	if err := cfg.validateAddOnPrometheusGrafana(); err != nil {
		return fmt.Errorf("validateAddOnPrometheusGrafana failed [%v]", err)
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
	if err := cfg.validateAddOnClusterLoader(); err != nil {
		return fmt.Errorf("validateAddOnClusterLoader failed [%v]", err)
	}

	return nil
}

func (cfg *Config) validateConfig() error {
	if _, ok := aws.RegionToAiport[cfg.Region]; !ok {
		return fmt.Errorf("region %q not found", cfg.Region)
	}
	if len(cfg.Name) == 0 {
		return errors.New("Name is empty")
	}
	if cfg.Name != strings.ToLower(cfg.Name) {
		return fmt.Errorf("Name %q must be in lower-case", cfg.Name)
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

	if len(cfg.LogOutputs) == 1 && (cfg.LogOutputs[0] == "stderr" || cfg.LogOutputs[0] == "stdout") {
		cfg.LogOutputs = append(cfg.LogOutputs, cfg.ConfigPath+".log")
	}

	if cfg.KubectlCommandsOutputPath == "" {
		cfg.KubectlCommandsOutputPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".kubectl.sh"
	}
	if filepath.Ext(cfg.KubectlCommandsOutputPath) != ".sh" {
		cfg.KubectlCommandsOutputPath = cfg.KubectlCommandsOutputPath + ".sh"
	}
	if cfg.RemoteAccessCommandsOutputPath == "" {
		cfg.RemoteAccessCommandsOutputPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".ssh.sh"
	}
	if filepath.Ext(cfg.RemoteAccessCommandsOutputPath) != ".sh" {
		cfg.RemoteAccessCommandsOutputPath = cfg.RemoteAccessCommandsOutputPath + ".sh"
	}

	if filepath.Ext(cfg.CommandAfterCreateClusterOutputPath) != ".log" {
		cfg.CommandAfterCreateClusterOutputPath = cfg.CommandAfterCreateClusterOutputPath + ".log"
	}
	if cfg.CommandAfterCreateClusterOutputPath == "" {
		cfg.CommandAfterCreateClusterOutputPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".after-create-cluster.out.log"
	}
	if cfg.CommandAfterCreateClusterTimeout == time.Duration(0) {
		cfg.CommandAfterCreateClusterTimeout = DefaultCommandAfterCreateClusterTimeout
	}
	cfg.CommandAfterCreateClusterTimeoutString = cfg.CommandAfterCreateClusterTimeout.String()

	if filepath.Ext(cfg.CommandAfterCreateAddOnsOutputPath) != ".log" {
		cfg.CommandAfterCreateAddOnsOutputPath = cfg.CommandAfterCreateAddOnsOutputPath + ".log"
	}
	if cfg.CommandAfterCreateAddOnsOutputPath == "" {
		cfg.CommandAfterCreateAddOnsOutputPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".after-create-add-ons.out.log"
	}
	if cfg.CommandAfterCreateAddOnsTimeout == time.Duration(0) {
		cfg.CommandAfterCreateAddOnsTimeout = DefaultCommandAfterCreateAddOnsTimeout
	}
	cfg.CommandAfterCreateAddOnsTimeoutString = cfg.CommandAfterCreateAddOnsTimeout.String()

	if cfg.KubeConfigPath == "" {
		cfg.KubeConfigPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".kubeconfig.yaml"
	}

	if !strings.Contains(cfg.KubectlDownloadURL, runtime.GOOS) {
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

	switch cfg.Parameters.RoleCreate {
	case true: // need create one, or already created
		if cfg.Parameters.RoleName == "" {
			cfg.Parameters.RoleName = cfg.Name + "-role-cluster"
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
	case cfg.Parameters.VPCCIDR != "":
		switch {
		case cfg.Parameters.PublicSubnetCIDR1 == "":
			return fmt.Errorf("empty Parameters.PublicSubnetCIDR1 when VPCCIDR is %q", cfg.Parameters.VPCCIDR)
		case cfg.Parameters.PublicSubnetCIDR2 == "":
			return fmt.Errorf("empty Parameters.PublicSubnetCIDR2 when VPCCIDR is %q", cfg.Parameters.VPCCIDR)
		case cfg.Parameters.PublicSubnetCIDR3 == "":
			return fmt.Errorf("empty Parameters.PublicSubnetCIDR3 when VPCCIDR is %q", cfg.Parameters.VPCCIDR)
		case cfg.Parameters.PrivateSubnetCIDR1 == "":
			return fmt.Errorf("empty Parameters.PrivateSubnetCIDR1 when VPCCIDR is %q", cfg.Parameters.VPCCIDR)
		case cfg.Parameters.PrivateSubnetCIDR2 == "":
			return fmt.Errorf("empty Parameters.PrivateSubnetCIDR2 when VPCCIDR is %q", cfg.Parameters.VPCCIDR)
		}

	case cfg.Parameters.VPCCIDR == "":
		switch {
		case cfg.Parameters.PublicSubnetCIDR1 != "":
			return fmt.Errorf("non-empty Parameters.PublicSubnetCIDR1 %q when VPCCIDR is empty", cfg.Parameters.PublicSubnetCIDR1)
		case cfg.Parameters.PublicSubnetCIDR2 != "":
			return fmt.Errorf("non-empty Parameters.PublicSubnetCIDR2 %q when VPCCIDR is empty", cfg.Parameters.PublicSubnetCIDR2)
		case cfg.Parameters.PublicSubnetCIDR3 != "":
			return fmt.Errorf("non-empty Parameters.PublicSubnetCIDR3 %q when VPCCIDR is empty", cfg.Parameters.PublicSubnetCIDR3)
		case cfg.Parameters.PrivateSubnetCIDR1 != "":
			return fmt.Errorf("non-empty Parameters.PrivateSubnetCIDR1 %q when VPCCIDR is empty", cfg.Parameters.PrivateSubnetCIDR1)
		case cfg.Parameters.PrivateSubnetCIDR2 != "":
			return fmt.Errorf("non-empty Parameters.PrivateSubnetCIDR2 %q when VPCCIDR is empty", cfg.Parameters.PrivateSubnetCIDR2)
		}
	}

	switch cfg.RemoteAccessKeyCreate {
	case true: // need create one, or already created
		if cfg.RemoteAccessKeyName == "" {
			cfg.RemoteAccessKeyName = cfg.Name + "-key-nodes"
		}
		if cfg.RemoteAccessPrivateKeyPath == "" {
			cfg.RemoteAccessPrivateKeyPath = filepath.Join(os.TempDir(), randString(10)+".insecure.key")
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

	return nil
}

func (cfg *Config) validateAddOnNodeGroups() error {
	if !cfg.IsEnabledAddOnNodeGroups() {
		return nil
	}

	n := len(cfg.AddOnNodeGroups.ASGs)
	if n == 0 {
		return errors.New("empty ASGs")
	}
	if n > NGsMaxLimit {
		return fmt.Errorf("NGs %d exceeds maximum number of NGs which is %d", n, NGsMaxLimit)
	}

	if cfg.Parameters.VersionValue < 1.14 {
		return fmt.Errorf("Version %q not supported for AddOnNodeGroups", cfg.Parameters.Version)
	}

	if cfg.AddOnNodeGroups.LogsDir == "" {
		cfg.AddOnNodeGroups.LogsDir = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-logs-ngs")
	}

	switch cfg.AddOnNodeGroups.RoleCreate {
	case true: // need create one, or already created
		if cfg.AddOnNodeGroups.RoleName == "" {
			cfg.AddOnNodeGroups.RoleName = cfg.Name + "-role-ng"
		}
		if cfg.AddOnNodeGroups.RoleARN != "" {
			// just ignore...
			// could be populated from previous run
			// do not error, so long as RoleCreate false, role won't be deleted
		}
		if len(cfg.AddOnNodeGroups.RoleServicePrincipals) > 0 {
			/*
				create node group request failed (InvalidParameterException: Following required service principals [ec2.amazonaws.com] were not found in the trust relationships of nodeRole arn:aws:iam::...:role/test-ng-role
				{
				  ClusterName: "test",
				  Message_: "Following required service principals [ec2.amazonaws.com] were not found in the trust relationships of nodeRole arn:aws:iam::...:role/test-ng-role",
				  NodegroupName: "test-ng-cpu"
				})
			*/
			found := false
			for _, pv := range cfg.AddOnNodeGroups.RoleServicePrincipals {
				if pv == "ec2.amazonaws.com" { // TODO: support China regions ec2.amazonaws.com.cn or eks.amazonaws.com.cn
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("AddOnNodeGroups.RoleServicePrincipals %q must include 'ec2.amazonaws.com'", cfg.AddOnNodeGroups.RoleServicePrincipals)
			}
		}

	case false: // use existing one
		if cfg.AddOnNodeGroups.RoleARN == "" {
			return fmt.Errorf("AddOnNodeGroups.RoleCreate false; expect non-empty RoleARN but got %q", cfg.AddOnNodeGroups.RoleARN)
		}
		if cfg.AddOnNodeGroups.RoleName == "" {
			cfg.AddOnNodeGroups.RoleName = getNameFromARN(cfg.AddOnNodeGroups.RoleARN)
		}
		if len(cfg.AddOnNodeGroups.RoleManagedPolicyARNs) > 0 {
			return fmt.Errorf("AddOnNodeGroups.RoleCreate false; expect empty RoleManagedPolicyARNs but got %q", cfg.AddOnNodeGroups.RoleManagedPolicyARNs)
		}
		if len(cfg.AddOnNodeGroups.RoleServicePrincipals) > 0 {
			return fmt.Errorf("AddOnNodeGroups.RoleCreate false; expect empty RoleServicePrincipals but got %q", cfg.AddOnNodeGroups.RoleServicePrincipals)
		}
	}

	names, processed := make(map[string]struct{}), make(map[string]ASG)
	for k, v := range cfg.AddOnNodeGroups.ASGs {
		k = strings.ReplaceAll(k, "GetRef.Name", cfg.Name)
		v.Name = strings.ReplaceAll(v.Name, "GetRef.Name", cfg.Name)

		if v.Name == "" {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].Name is empty", k)
		}
		if k != v.Name {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].Name has different Name field %q", k, v.Name)
		}
		_, ok := names[v.Name]
		if !ok {
			names[v.Name] = struct{}{}
		} else {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].Name %q is redundant", k, v.Name)
		}

		if len(v.InstanceTypes) > 4 {
			return fmt.Errorf("too many InstaceTypes[%q]", v.InstanceTypes)
		}
		if v.VolumeSize == 0 {
			v.VolumeSize = DefaultNodeVolumeSize
		}
		if v.RemoteAccessUserName == "" {
			v.RemoteAccessUserName = "ec2-user"
		}

		if v.ImageID == "" && v.ImageIDSSMParameter == "" {
			return fmt.Errorf("%q both ImageID and ImageIDSSMParameter are empty", v.Name)
		}

		switch v.AMIType {
		case ec2config.AMITypeBottleRocketCPU:
			if v.RemoteAccessUserName != "ec2-user" {
				return fmt.Errorf("AMIType %q but unexpected RemoteAccessUserName %q", v.AMIType, v.RemoteAccessUserName)
			}
			if v.SSMDocumentName != "" && cfg.S3BucketName == "" {
				return fmt.Errorf("AMIType %q requires SSMDocumentName %q but no S3BucketName", v.AMIType, v.SSMDocumentName)
			}
			if v.KubeletExtraArgs != "" {
				return fmt.Errorf("AMIType %q but unexpected KubeletExtraArgs %q", v.AMIType, v.KubeletExtraArgs)
			}
		case eks.AMITypesAl2X8664:
			if v.RemoteAccessUserName != "ec2-user" {
				return fmt.Errorf("AMIType %q but unexpected RemoteAccessUserName %q", v.AMIType, v.RemoteAccessUserName)
			}
		case eks.AMITypesAl2X8664Gpu:
			if v.RemoteAccessUserName != "ec2-user" {
				return fmt.Errorf("AMIType %q but unexpected RemoteAccessUserName %q", v.AMIType, v.RemoteAccessUserName)
			}
		default:
			return fmt.Errorf("unknown ASGs[%q].AMIType %q", k, v.AMIType)
		}

		switch v.AMIType {
		case ec2config.AMITypeBottleRocketCPU:
			if len(v.InstanceTypes) == 0 {
				v.InstanceTypes = []string{DefaultNodeInstanceTypeCPU}
			}
		case eks.AMITypesAl2X8664:
			if len(v.InstanceTypes) == 0 {
				v.InstanceTypes = []string{DefaultNodeInstanceTypeCPU}
			}
		case eks.AMITypesAl2X8664Gpu:
			if len(v.InstanceTypes) == 0 {
				v.InstanceTypes = []string{DefaultNodeInstanceTypeGPU}
			}
		default:
			return fmt.Errorf("unknown AddOnNodeGroups.ASGs[%q].AMIType %q", k, v.AMIType)
		}

		if cfg.IsEnabledAddOnNLBHelloWorld() || cfg.IsEnabledAddOnALB2048() {
			// "m3.xlarge" or "c4.xlarge" will fail with "InvalidTarget: Targets {...} are not supported"
			// ref. https://github.com/aws/amazon-vpc-cni-k8s/pull/821
			// ref. https://github.com/kubernetes/kubernetes/issues/66044#issuecomment-408188524
			for _, ivt := range v.InstanceTypes {

				switch {
				case strings.HasPrefix(ivt, "m3."),
					strings.HasPrefix(ivt, "c4."):
					return fmt.Errorf("AddOnNLBHelloWorld.Enable[%v] || AddOnALB2048.Enable[%v], but older instance type InstanceType %q for %q",
						cfg.IsEnabledAddOnNLBHelloWorld(),
						cfg.IsEnabledAddOnALB2048(),
						ivt, k)
				}
			}
		}

		if v.ASGMinSize > v.ASGMaxSize {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].ASGMinSize %d > ASGMaxSize %d", k, v.ASGMinSize, v.ASGMaxSize)
		}
		if v.ASGDesiredCapacity > v.ASGMaxSize {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].ASGDesiredCapacity %d > ASGMaxSize %d", k, v.ASGDesiredCapacity, v.ASGMaxSize)
		}
		if v.ASGMaxSize > NGMaxLimit {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].ASGMaxSize %d > NGMaxLimit %d", k, v.ASGMaxSize, NGMaxLimit)
		}
		if v.ASGDesiredCapacity > NGMaxLimit {
			return fmt.Errorf("AddOnNodeGroups.ASGs[%q].ASGDesiredCapacity %d > NGMaxLimit %d", k, v.ASGDesiredCapacity, NGMaxLimit)
		}

		switch v.SSMDocumentCreate {
		case true: // need create one, or already created
			if v.SSMDocumentCFNStackName == "" {
				v.SSMDocumentCFNStackName = v.Name + "-ssm-document"
			}
			if v.SSMDocumentName == "" {
				v.SSMDocumentName = v.Name + "SSMDocument"
			}
			if v.SSMDocumentExecutionTimeoutSeconds == 0 {
				v.SSMDocumentExecutionTimeoutSeconds = 3600
			}

		case false: // use existing one, or don't run any SSM
		}

		v.SSMDocumentCFNStackName = strings.ReplaceAll(v.SSMDocumentCFNStackName, "GetRef.Name", cfg.Name)
		v.SSMDocumentName = strings.ReplaceAll(v.SSMDocumentName, "GetRef.Name", cfg.Name)
		v.SSMDocumentName = regex.ReplaceAllString(v.SSMDocumentName, "")

		if cfg.IsEnabledAddOnNLBHelloWorld() && cfg.AddOnNLBHelloWorld.DeploymentReplicas < int32(v.ASGDesiredCapacity) {
			cfg.AddOnNLBHelloWorld.DeploymentReplicas = int32(v.ASGDesiredCapacity)
		}
		if cfg.IsEnabledAddOnALB2048() && cfg.AddOnALB2048.DeploymentReplicasALB < int32(v.ASGDesiredCapacity) {
			cfg.AddOnALB2048.DeploymentReplicasALB = int32(v.ASGDesiredCapacity)
		}
		if cfg.IsEnabledAddOnALB2048() && cfg.AddOnALB2048.DeploymentReplicas2048 < int32(v.ASGDesiredCapacity) {
			cfg.AddOnALB2048.DeploymentReplicas2048 = int32(v.ASGDesiredCapacity)
		}

		processed[k] = v
	}

	cfg.AddOnNodeGroups.ASGs = processed
	return nil
}

// only letters and numbers
var regex = regexp.MustCompile("[^a-zA-Z0-9]+")

func (cfg *Config) validateAddOnManagedNodeGroups() error {
	if !cfg.IsEnabledAddOnManagedNodeGroups() {
		return nil
	}

	n := len(cfg.AddOnManagedNodeGroups.MNGs)
	if n == 0 {
		return errors.New("empty MNGs")
	}
	if n > MNGsMaxLimit {
		return fmt.Errorf("MNGs %d exceeds maximum number of MNGs which is %d", n, MNGsMaxLimit)
	}

	if cfg.Parameters.VersionValue < 1.14 {
		return fmt.Errorf("Version %q not supported for AddOnManagedNodeGroups", cfg.Parameters.Version)
	}

	if cfg.AddOnManagedNodeGroups.LogsDir == "" {
		cfg.AddOnManagedNodeGroups.LogsDir = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-logs-mngs")
	}

	switch cfg.AddOnManagedNodeGroups.RoleCreate {
	case true: // need create one, or already created
		if cfg.AddOnManagedNodeGroups.RoleName == "" {
			cfg.AddOnManagedNodeGroups.RoleName = cfg.Name + "-role-mng"
		}
		if cfg.AddOnManagedNodeGroups.RoleARN != "" {
			// just ignore...
			// could be populated from previous run
			// do not error, so long as RoleCreate false, role won't be deleted
		}
		if len(cfg.AddOnManagedNodeGroups.RoleServicePrincipals) > 0 {
			/*
				create node group request failed (InvalidParameterException: Following required service principals [ec2.amazonaws.com] were not found in the trust relationships of nodeRole arn:aws:iam::...:role/test-mng-role
				{
				  ClusterName: "test",
				  Message_: "Following required service principals [ec2.amazonaws.com] were not found in the trust relationships of nodeRole arn:aws:iam::...:role/test-mng-role",
				  NodegroupName: "test-mng-cpu"
				})
			*/
			found := false
			for _, pv := range cfg.AddOnManagedNodeGroups.RoleServicePrincipals {
				if pv == "ec2.amazonaws.com" { // TODO: support China regions ec2.amazonaws.com.cn or eks.amazonaws.com.cn
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("AddOnManagedNodeGroups.RoleServicePrincipals %q must include 'ec2.amazonaws.com'", cfg.AddOnManagedNodeGroups.RoleServicePrincipals)
			}
		}

	case false: // use existing one
		if cfg.AddOnManagedNodeGroups.RoleARN == "" {
			return fmt.Errorf("AddOnManagedNodeGroups.RoleCreate false; expect non-empty RoleARN but got %q", cfg.AddOnManagedNodeGroups.RoleARN)
		}
		if cfg.AddOnManagedNodeGroups.RoleName == "" {
			cfg.AddOnManagedNodeGroups.RoleName = getNameFromARN(cfg.AddOnManagedNodeGroups.RoleARN)
		}
		if len(cfg.AddOnManagedNodeGroups.RoleManagedPolicyARNs) > 0 {
			return fmt.Errorf("AddOnManagedNodeGroups.RoleCreate false; expect empty RoleManagedPolicyARNs but got %q", cfg.AddOnManagedNodeGroups.RoleManagedPolicyARNs)
		}
		if len(cfg.AddOnManagedNodeGroups.RoleServicePrincipals) > 0 {
			return fmt.Errorf("AddOnManagedNodeGroups.RoleCreate false; expect empty RoleServicePrincipals but got %q", cfg.AddOnManagedNodeGroups.RoleServicePrincipals)
		}
	}

	names, processed := make(map[string]struct{}), make(map[string]MNG)
	for k, v := range cfg.AddOnManagedNodeGroups.MNGs {
		k = strings.ReplaceAll(k, "GetRef.Name", cfg.Name)
		v.Name = strings.ReplaceAll(v.Name, "GetRef.Name", cfg.Name)

		if v.Name == "" {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].Name is empty", k)
		}
		if k != v.Name {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].Name has different Name field %q", k, v.Name)
		}
		_, ok := names[v.Name]
		if !ok {
			names[v.Name] = struct{}{}
		} else {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].Name %q is redundant", k, v.Name)
		}
		if cfg.IsEnabledAddOnNodeGroups() {
			_, ok = cfg.AddOnNodeGroups.ASGs[v.Name]
			if ok {
				return fmt.Errorf("MNGs[%q] name is conflicting with NG ASG", v.Name)
			}
		}

		if len(v.InstanceTypes) > 4 {
			return fmt.Errorf("too many InstaceTypes[%q]", v.InstanceTypes)
		}
		if v.VolumeSize == 0 {
			v.VolumeSize = DefaultNodeVolumeSize
		}
		if v.RemoteAccessUserName == "" {
			v.RemoteAccessUserName = "ec2-user"
		}

		if v.RemoteAccessUserName == "" {
			v.RemoteAccessUserName = "ec2-user"
		}

		switch v.AMIType {
		case eks.AMITypesAl2X8664:
			if v.RemoteAccessUserName != "ec2-user" {
				return fmt.Errorf("AMIType %q but unexpected RemoteAccessUserName %q", v.AMIType, v.RemoteAccessUserName)
			}
		case eks.AMITypesAl2X8664Gpu:
			if v.RemoteAccessUserName != "ec2-user" {
				return fmt.Errorf("AMIType %q but unexpected RemoteAccessUserName %q", v.AMIType, v.RemoteAccessUserName)
			}
		default:
			return fmt.Errorf("unknown ASGs[%q].AMIType %q", k, v.AMIType)
		}

		switch v.AMIType {
		case eks.AMITypesAl2X8664:
			if len(v.InstanceTypes) == 0 {
				v.InstanceTypes = []string{DefaultNodeInstanceTypeCPU}
			}
		case eks.AMITypesAl2X8664Gpu:
			if len(v.InstanceTypes) == 0 {
				v.InstanceTypes = []string{DefaultNodeInstanceTypeGPU}
			}
		default:
			return fmt.Errorf("unknown AddOnManagedNodeGroups.MNGs[%q].AMIType %q", k, v.AMIType)
		}

		if cfg.IsEnabledAddOnNLBHelloWorld() || cfg.IsEnabledAddOnALB2048() {
			for _, itp := range v.InstanceTypes {
				// "m3.xlarge" or "c4.xlarge" will fail with "InvalidTarget: Targets {...} are not supported"
				// ref. https://github.com/aws/amazon-vpc-cni-k8s/pull/821
				// ref. https://github.com/kubernetes/kubernetes/issues/66044#issuecomment-408188524
				switch {
				case strings.HasPrefix(itp, "m3."),
					strings.HasPrefix(itp, "c4."):
					return fmt.Errorf("AddOnNLBHelloWorld.Enable[%v] || AddOnALB2048.Enable[%v], but older instance type InstanceTypes %q for %q",
						cfg.IsEnabledAddOnNLBHelloWorld(),
						cfg.IsEnabledAddOnALB2048(),
						itp, k)
				default:
				}
			}
		}

		if v.ASGMinSize > v.ASGMaxSize {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].ASGMinSize %d > ASGMaxSize %d", k, v.ASGMinSize, v.ASGMaxSize)
		}
		if v.ASGDesiredCapacity > v.ASGMaxSize {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].ASGDesiredCapacity %d > ASGMaxSize %d", k, v.ASGDesiredCapacity, v.ASGMaxSize)
		}
		if v.ASGMaxSize > MNGMaxLimit {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].ASGMaxSize %d > MNGMaxLimit %d", k, v.ASGMaxSize, MNGMaxLimit)
		}
		if v.ASGDesiredCapacity > MNGMaxLimit {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].ASGDesiredCapacity %d > MNGMaxLimit %d", k, v.ASGDesiredCapacity, MNGMaxLimit)
		}

		if cfg.IsEnabledAddOnNLBHelloWorld() && cfg.AddOnNLBHelloWorld.DeploymentReplicas < int32(v.ASGDesiredCapacity) {
			cfg.AddOnNLBHelloWorld.DeploymentReplicas = int32(v.ASGDesiredCapacity)
		}
		if cfg.IsEnabledAddOnALB2048() && cfg.AddOnALB2048.DeploymentReplicasALB < int32(v.ASGDesiredCapacity) {
			cfg.AddOnALB2048.DeploymentReplicasALB = int32(v.ASGDesiredCapacity)
		}
		if cfg.IsEnabledAddOnALB2048() && cfg.AddOnALB2048.DeploymentReplicas2048 < int32(v.ASGDesiredCapacity) {
			cfg.AddOnALB2048.DeploymentReplicas2048 = int32(v.ASGDesiredCapacity)
		}

		processed[k] = v
	}

	cfg.AddOnManagedNodeGroups.MNGs = processed
	return nil
}

func (cfg *Config) validateAddOnCSIEBS() error {
	if !cfg.IsEnabledAddOnCSIEBS() {
		return nil
	}
	if cfg.AddOnCSIEBS.ChartRepoURL == "" {
		return errors.New("unexpected empty AddOnCSIEBS.ChartRepoURL")
	}
	return nil
}

func (cfg *Config) validateAddOnNLBHelloWorld() error {
	if !cfg.IsEnabledAddOnNLBHelloWorld() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnNLBHelloWorld.Enable true but no node group is enabled")
	}
	if cfg.AddOnNLBHelloWorld.Namespace == "" {
		cfg.AddOnNLBHelloWorld.Namespace = cfg.Name + "-nlb-hello-world"
	}
	return nil
}

func (cfg *Config) validateAddOnALB2048() error {
	if !cfg.IsEnabledAddOnALB2048() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnALB2048.Enable true but no node group is enabled")
	}
	if cfg.AddOnALB2048.Namespace == "" {
		cfg.AddOnALB2048.Namespace = cfg.Name + "-alb-2048"
	}
	return nil
}

func (cfg *Config) validateAddOnJobsPi() error {
	if !cfg.IsEnabledAddOnJobsPi() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnJobsPi.Enable true but no node group is enabled")
	}
	if cfg.AddOnJobsPi.Namespace == "" {
		cfg.AddOnJobsPi.Namespace = cfg.Name + "-job-perl"
	}
	return nil
}

func (cfg *Config) validateAddOnJobsEcho() error {
	if !cfg.IsEnabledAddOnJobsEcho() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnJobsEcho.Enable true but no node group is enabled")
	}
	if cfg.AddOnJobsEcho.Namespace == "" {
		cfg.AddOnJobsEcho.Namespace = cfg.Name + "-job-echo"
	}
	if cfg.AddOnJobsEcho.EchoSize > 250000 {
		return fmt.Errorf("echo size limit is 0.25 MB, got %d", cfg.AddOnJobsEcho.EchoSize)
	}
	return nil
}

func (cfg *Config) validateAddOnCronJobs() error {
	if !cfg.IsEnabledAddOnCronJobs() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnCronJobs.Enable true but no node group is enabled")
	}
	if cfg.AddOnCronJobs.Namespace == "" {
		cfg.AddOnCronJobs.Namespace = cfg.Name + "-cronjob"
	}
	if cfg.AddOnCronJobs.EchoSize > 250000 {
		return fmt.Errorf("echo size limit is 0.25 MB, got %d", cfg.AddOnCronJobs.EchoSize)
	}
	return nil
}

func (cfg *Config) validateAddOnCSRs() error {
	if !cfg.IsEnabledAddOnCSRs() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnCSRs.Enable true but no node group is enabled")
	}
	if cfg.AddOnCSRs.Namespace == "" {
		cfg.AddOnCSRs.Namespace = cfg.Name + "-csrs"
	}
	switch cfg.AddOnCSRs.InitialRequestConditionType {
	case "Approved":
	case "Denied":
	case "Pending", "":
	case "Random":
	default:
		return fmt.Errorf("unknown AddOnCSRs.InitialRequestConditionType %q", cfg.AddOnCSRs.InitialRequestConditionType)
	}
	return nil
}

func (cfg *Config) validateAddOnConfigMaps() error {
	if !cfg.IsEnabledAddOnConfigMaps() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnConfigMaps.Enable true but no node group is enabled")
	}
	if cfg.AddOnConfigMaps.Namespace == "" {
		cfg.AddOnConfigMaps.Namespace = cfg.Name + "-configmaps"
	}
	if cfg.AddOnConfigMaps.Size > 900000 {
		return fmt.Errorf("AddOnConfigMaps.Size limit is 0.9 MB, got %d", cfg.AddOnConfigMaps.Size)
	}
	return nil
}

func (cfg *Config) validateAddOnSecrets() error {
	if !cfg.IsEnabledAddOnSecrets() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnSecrets.Enable true but no node group is enabled")
	}
	if cfg.AddOnSecrets.Namespace == "" {
		cfg.AddOnSecrets.Namespace = cfg.Name + "-secrets"
	}
	if cfg.AddOnSecrets.WritesResultPath == "" {
		cfg.AddOnSecrets.WritesResultPath = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-secret-writes.csv")
	}
	if filepath.Ext(cfg.AddOnSecrets.WritesResultPath) != ".csv" {
		return fmt.Errorf("expected .csv extension for WritesResultPath, got %q", cfg.AddOnSecrets.WritesResultPath)
	}
	if cfg.AddOnSecrets.ReadsResultPath == "" {
		cfg.AddOnSecrets.ReadsResultPath = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-secret-reads.csv")
	}
	if filepath.Ext(cfg.AddOnSecrets.ReadsResultPath) != ".csv" {
		return fmt.Errorf("expected .csv extension for ReadsResultPath, got %q", cfg.AddOnSecrets.ReadsResultPath)
	}
	return nil
}

func (cfg *Config) validateAddOnIRSA() error {
	if !cfg.IsEnabledAddOnIRSA() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnIRSA.Enable true but no node group is enabled")
	}
	if cfg.Parameters.VersionValue < 1.14 {
		return fmt.Errorf("Version %q not supported for AddOnIRSA", cfg.Parameters.Version)
	}
	if cfg.S3BucketName == "" {
		return errors.New("AddOnIRSA requires S3 bucket but S3BucketName empty")
	}
	if cfg.AddOnIRSA.Namespace == "" {
		cfg.AddOnIRSA.Namespace = cfg.Name + "-irsa"
	}
	if cfg.AddOnIRSA.RoleName == "" {
		cfg.AddOnIRSA.RoleName = cfg.Name + "-role-irsa"
	}
	if cfg.AddOnIRSA.ServiceAccountName == "" {
		cfg.AddOnIRSA.ServiceAccountName = cfg.Name + "-service-account-irsa"
	}
	if cfg.AddOnIRSA.ConfigMapName == "" {
		cfg.AddOnIRSA.ConfigMapName = cfg.Name + "-configmap-irsa"
	}
	if cfg.AddOnIRSA.ConfigMapScriptFileName == "" {
		cfg.AddOnIRSA.ConfigMapScriptFileName = cfg.Name + "-configmap-irsa.sh"
	}
	if cfg.AddOnIRSA.S3Key == "" {
		cfg.AddOnIRSA.S3Key = path.Join(cfg.Name, "s3-key-irsa")
	}
	if cfg.AddOnIRSA.DeploymentName == "" {
		cfg.AddOnIRSA.DeploymentName = cfg.Name + "-deployment-irsa"
	}
	if cfg.AddOnIRSA.DeploymentResultPath == "" {
		cfg.AddOnIRSA.DeploymentResultPath = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-deployment-irsa-result.log")
	}
	return nil
}

// only letters and numbers for Secret key names
var secretRegex = regexp.MustCompile("[^a-zA-Z0-9]+")

func (cfg *Config) validateAddOnFargate() error {
	if !cfg.IsEnabledAddOnFargate() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnFargate.Enable true but no node group is enabled")
	}
	if cfg.Parameters.VersionValue < 1.14 {
		return fmt.Errorf("Version %q not supported for AddOnFargate", cfg.Parameters.Version)
	}
	if cfg.AddOnFargate.Namespace == "" {
		cfg.AddOnFargate.Namespace = cfg.Name + "-fargate"
	}
	// do not prefix with "eks-"
	// e.g. "The fargate profile name starts with the reserved prefix: 'eks-'."
	if cfg.AddOnFargate.ProfileName == "" {
		cfg.AddOnFargate.ProfileName = cfg.Name + "-fargate-profile"
	}
	if strings.HasPrefix(cfg.AddOnFargate.ProfileName, "eks-") {
		cfg.AddOnFargate.ProfileName = strings.Replace(cfg.AddOnFargate.ProfileName, "eks-", "", 1)
	}
	if cfg.AddOnFargate.SecretName == "" {
		cfg.AddOnFargate.SecretName = cfg.Name + "addonfargatesecret"
	}
	if cfg.AddOnFargate.PodName == "" {
		cfg.AddOnFargate.PodName = cfg.Name + "-pod-fargate"
	}
	if cfg.AddOnFargate.ContainerName == "" {
		cfg.AddOnFargate.ContainerName = cfg.Name + "-" + randString(10)
	}
	cfg.AddOnFargate.SecretName = strings.ToLower(secretRegex.ReplaceAllString(cfg.AddOnFargate.SecretName, ""))

	switch cfg.AddOnFargate.RoleCreate {
	case true: // need create one, or already created
		if cfg.AddOnFargate.RoleName == "" {
			cfg.AddOnFargate.RoleName = cfg.Name + "-role-fargate"
		}
		if cfg.AddOnFargate.RoleARN != "" {
			// just ignore...
			// could be populated from previous run
			// do not error, so long as RoleCreate false, role won't be deleted
		}
	case false: // use existing one
		if cfg.AddOnFargate.RoleARN == "" {
			return fmt.Errorf("AddOnFargate.RoleCreate false; expect non-empty RoleARN but got %q", cfg.AddOnFargate.RoleARN)
		}
		if cfg.AddOnFargate.RoleName == "" {
			cfg.AddOnFargate.RoleName = getNameFromARN(cfg.AddOnFargate.RoleARN)
		}
		if len(cfg.AddOnFargate.RoleManagedPolicyARNs) > 0 {
			return fmt.Errorf("AddOnFargate.RoleCreate false; expect empty RoleManagedPolicyARNs but got %q", cfg.AddOnFargate.RoleManagedPolicyARNs)
		}
		if len(cfg.AddOnFargate.RoleServicePrincipals) > 0 {
			return fmt.Errorf("AddOnFargate.RoleCreate false; expect empty RoleServicePrincipals but got %q", cfg.AddOnFargate.RoleServicePrincipals)
		}
	}

	return nil
}

func (cfg *Config) validateAddOnIRSAFargate() error {
	if !cfg.IsEnabledAddOnIRSAFargate() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnIRSAFargate.Enable true but no node group is enabled")
	}
	if cfg.Parameters.VersionValue < 1.14 {
		return fmt.Errorf("Version %q not supported for AddOnIRSAFargate", cfg.Parameters.Version)
	}
	if cfg.S3BucketName == "" {
		return errors.New("AddOnIRSAFargate requires S3 bucket but S3BucketName empty")
	}
	if cfg.AddOnIRSAFargate.Namespace == "" {
		cfg.AddOnIRSAFargate.Namespace = cfg.Name + "-irsa-fargate"
	}
	if cfg.AddOnIRSAFargate.RoleName == "" {
		cfg.AddOnIRSAFargate.RoleName = cfg.Name + "-role-irsa-fargate"
	}
	if cfg.AddOnIRSAFargate.ServiceAccountName == "" {
		cfg.AddOnIRSAFargate.ServiceAccountName = cfg.Name + "-service-account-irsa-fargate"
	}
	if cfg.AddOnIRSAFargate.ConfigMapName == "" {
		cfg.AddOnIRSAFargate.ConfigMapName = cfg.Name + "-configmap-irsa-fargate"
	}
	if cfg.AddOnIRSAFargate.ConfigMapScriptFileName == "" {
		cfg.AddOnIRSAFargate.ConfigMapScriptFileName = cfg.Name + "-configmap-irsa-fargate.sh"
	}
	if cfg.AddOnIRSAFargate.S3Key == "" {
		cfg.AddOnIRSAFargate.S3Key = path.Join(cfg.Name, "s3-key-irsa-fargate")
	}
	// do not prefix with "eks-"
	// e.g. "The fargate profile name starts with the reserved prefix: 'eks-'."
	if cfg.AddOnIRSAFargate.ProfileName == "" {
		cfg.AddOnIRSAFargate.ProfileName = cfg.Name + "-irsa-fargate-profile"
	}
	if strings.HasPrefix(cfg.AddOnIRSAFargate.ProfileName, "eks-") {
		cfg.AddOnIRSAFargate.ProfileName = strings.Replace(cfg.AddOnIRSAFargate.ProfileName, "eks-", "", 1)
	}
	if cfg.AddOnIRSAFargate.PodName == "" {
		cfg.AddOnIRSAFargate.PodName = cfg.Name + "-pod-irsa-fargate"
	}
	if cfg.AddOnIRSAFargate.ContainerName == "" {
		cfg.AddOnIRSAFargate.ContainerName = cfg.Name + "-" + randString(10)
	}
	return nil
}

func (cfg *Config) validateAddOnAppMesh() error {
	if !cfg.IsEnabledAddOnAppMesh() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnAppMesh.Enable true but no node group is enabled")
	}
	if cfg.AddOnAppMesh.Namespace == "" {
		cfg.AddOnAppMesh.Namespace = "appmesh-system"
	}
	return nil
}

// ref. https://docs.aws.amazon.com/eks/latest/userguide/dashboard-tutorial.html
const defaultKubernetesDashboardURL = "http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/#/login"

func (cfg *Config) validateAddOnKubernetesDashboard() error {
	if !cfg.IsEnabledAddOnKubernetesDashboard() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnKubernetesDashboard.Enable true but no node group is enabled")
	}
	if cfg.AddOnKubernetesDashboard.URL == "" {
		cfg.AddOnKubernetesDashboard.URL = defaultKubernetesDashboardURL
	}
	return nil
}

func (cfg *Config) validateAddOnPrometheusGrafana() error {
	if !cfg.IsEnabledAddOnPrometheusGrafana() {
		return nil
	}
	if !cfg.IsEnabledAddOnCSIEBS() {
		return errors.New("AddOnPrometheusGrafana.Enable true but IsEnabledAddOnCSIEBS.Enable false")
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnPrometheusGrafana.Enable true but no node group is enabled")
	}

	// TODO: PVC not working on BottleRocket
	// do not assign mariadb to Bottlerocket
	// e.g. MountVolume.MountDevice failed for volume "pvc-8e035a13-4d33-472f-a4c0-f36c7d39d170" : executable file not found in $PATH
	// e.g. Unable to mount volumes for pod "wordpress-84c567b89b-2jgh5_eks-2020042114-exclusivea3i-wordpress(d02336a3-1799-4b08-9f15-b90871f6a2f0)": timeout expired waiting for volumes to attach or mount for pod "eks-2020042114-exclusivea3i-wordpress"/"wordpress-84c567b89b-2jgh5". list of unmounted volumes=[wordpress-data]. list of unattached volumes=[wordpress-data default-token-7bdc2]
	// TODO: fix CSI EBS https://github.com/bottlerocket-os/bottlerocket/issues/877
	if cfg.IsEnabledAddOnNodeGroups() {
		x86Found, rocketFound := false, false
		for _, asg := range cfg.AddOnNodeGroups.ASGs {
			switch asg.AMIType {
			case ec2config.AMITypeAL2X8664,
				ec2config.AMITypeAL2X8664GPU:
				x86Found = true
			case ec2config.AMITypeBottleRocketCPU:
				rocketFound = true
			}
		}
		if !x86Found && rocketFound {
			return fmt.Errorf("AddOnPrometheusGrafana.Enabled true but AddOnNodeGroups [x86Found %v, rocketFound %v]", x86Found, rocketFound)
		}
	}
	if cfg.IsEnabledAddOnManagedNodeGroups() {
		x86Found, rocketFound := false, false
		for _, asg := range cfg.AddOnManagedNodeGroups.MNGs {
			switch asg.AMIType {
			case eks.AMITypesAl2X8664,
				eks.AMITypesAl2X8664Gpu:
				x86Found = true
			case ec2config.AMITypeBottleRocketCPU:
				rocketFound = true
			}
		}
		if !x86Found && rocketFound {
			return fmt.Errorf("AddOnPrometheusGrafana.Enabled true but AddOnManagedNodeGroups [x86Found %v, rocketFound %v]", x86Found, rocketFound)
		}
	}

	if cfg.AddOnPrometheusGrafana.GrafanaAdminUserName == "" {
		cfg.AddOnPrometheusGrafana.GrafanaAdminUserName = randString(10)
	}
	if cfg.AddOnPrometheusGrafana.GrafanaAdminPassword == "" {
		cfg.AddOnPrometheusGrafana.GrafanaAdminPassword = randString(10)
	}

	return nil
}

func (cfg *Config) validateAddOnWordpress() error {
	if !cfg.IsEnabledAddOnWordpress() {
		return nil
	}
	if !cfg.IsEnabledAddOnCSIEBS() {
		return errors.New("AddOnWordpress.Enable true but IsEnabledAddOnCSIEBS.Enable false")
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnWordpress.Enable true but no node group is enabled")
	}

	// TODO: PVC not working on BottleRocket
	// do not assign mariadb to Bottlerocket
	// e.g. MountVolume.MountDevice failed for volume "pvc-8e035a13-4d33-472f-a4c0-f36c7d39d170" : executable file not found in $PATH
	// e.g. Unable to mount volumes for pod "wordpress-84c567b89b-2jgh5_eks-2020042114-exclusivea3i-wordpress(d02336a3-1799-4b08-9f15-b90871f6a2f0)": timeout expired waiting for volumes to attach or mount for pod "eks-2020042114-exclusivea3i-wordpress"/"wordpress-84c567b89b-2jgh5". list of unmounted volumes=[wordpress-data]. list of unattached volumes=[wordpress-data default-token-7bdc2]
	// TODO: fix CSI EBS https://github.com/bottlerocket-os/bottlerocket/issues/877
	if cfg.IsEnabledAddOnNodeGroups() {
		x86Found, rocketFound := false, false
		for _, asg := range cfg.AddOnNodeGroups.ASGs {
			switch asg.AMIType {
			case ec2config.AMITypeAL2X8664,
				ec2config.AMITypeAL2X8664GPU:
				x86Found = true
			case ec2config.AMITypeBottleRocketCPU:
				rocketFound = true
			}
		}
		if !x86Found && rocketFound {
			return fmt.Errorf("AddOnWordpress.Enabled true but AddOnNodeGroups [x86Found %v, rocketFound %v]", x86Found, rocketFound)
		}
	}
	if cfg.IsEnabledAddOnManagedNodeGroups() {
		x86Found, rocketFound := false, false
		for _, asg := range cfg.AddOnManagedNodeGroups.MNGs {
			switch asg.AMIType {
			case eks.AMITypesAl2X8664,
				eks.AMITypesAl2X8664Gpu:
				x86Found = true
			case ec2config.AMITypeBottleRocketCPU:
				rocketFound = true
			}
		}
		if !x86Found && rocketFound {
			return fmt.Errorf("AddOnWordpress.Enabled true but AddOnManagedNodeGroups [x86Found %v, rocketFound %v]", x86Found, rocketFound)
		}
	}

	if cfg.AddOnWordpress.Namespace == "" {
		cfg.AddOnWordpress.Namespace = cfg.Name + "-wordpress"
	}
	if cfg.AddOnWordpress.UserName == "" {
		cfg.AddOnWordpress.UserName = "user"
	}
	if cfg.AddOnWordpress.Password == "" {
		cfg.AddOnWordpress.Password = randString(10)
	}

	return nil
}

func (cfg *Config) validateAddOnJupyterHub() error {
	if !cfg.IsEnabledAddOnJupyterHub() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnJupyterHub.Enable true but no node group is enabled")
	}

	gpuFound := false
	if cfg.IsEnabledAddOnNodeGroups() {
		for _, cur := range cfg.AddOnNodeGroups.ASGs {
			if cur.AMIType == ec2config.AMITypeAL2X8664GPU {
				gpuFound = true
				break
			}
		}
	}
	if !gpuFound && cfg.IsEnabledAddOnManagedNodeGroups() {
		for _, cur := range cfg.AddOnManagedNodeGroups.MNGs {
			if cur.AMIType == eks.AMITypesAl2X8664Gpu {
				gpuFound = true
				break
			}
		}
	}
	if !gpuFound {
		return errors.New("AddOnJupyterHub requires GPU AMI")
	}

	if cfg.AddOnJupyterHub.Namespace == "" {
		cfg.AddOnJupyterHub.Namespace = cfg.Name + "-jupyter-hub"
	}

	if cfg.AddOnJupyterHub.ProxySecretToken == "" {
		cfg.AddOnJupyterHub.ProxySecretToken = randHex(32)
	}
	_, err := hex.DecodeString(cfg.AddOnJupyterHub.ProxySecretToken)
	if err != nil {
		return fmt.Errorf("cannot hex decode AddOnJupyterHub.ProxySecretToken %q", err)
	}

	return nil
}

func (cfg *Config) validateAddOnKubeflow() error {
	if !cfg.IsEnabledAddOnKubeflow() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnKubeflow.Enable true but no node group is enabled")
	}
	if cfg.AddOnKubeflow.BaseDir == "" {
		cfg.AddOnKubeflow.BaseDir = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-kubeflow")
	}
	cfg.AddOnKubeflow.KfDir = filepath.Join(cfg.AddOnKubeflow.BaseDir, cfg.Name)
	cfg.AddOnKubeflow.KfctlConfigPath = filepath.Join(cfg.AddOnKubeflow.KfDir, "kfctl_aws.yaml")
	return nil
}

func (cfg *Config) validateAddOnClusterLoader() error {
	if !cfg.IsEnabledAddOnClusterLoader() {
		return nil
	}
	if !cfg.IsEnabledAddOnNodeGroups() && !cfg.IsEnabledAddOnManagedNodeGroups() {
		return errors.New("AddOnClusterLoader.Enable true but no node group is enabled")
	}

	if cfg.AddOnClusterLoader.Duration == time.Duration(0) {
		cfg.AddOnClusterLoader.Duration = time.Minute
	}
	cfg.AddOnClusterLoader.DurationString = cfg.AddOnClusterLoader.Duration.String()

	return nil
}

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
