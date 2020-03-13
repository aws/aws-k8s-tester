package eksconfig

import (
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
	"time"

	"github.com/aws/aws-k8s-tester/pkg/aws"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-sdk-go/service/eks"
	"k8s.io/client-go/util/homedir"
)

// DefaultConfig is the default configuration.
//  - empty string creates a non-nil object for pointer-type field
//  - omitting an entire field returns nil value
//  - make sure to check both
//
// MAKE SURE TO SYNC THE DEFAULT VALUES in "eks" templates
//
var DefaultConfig = Config{
	// to be auto-generated
	ConfigPath:                "",
	KubectlCommandsOutputPath: "",
	KubeConfigPath:            "",
	Name:                      "",
	AWSCLIPath:                "",

	Region: "us-west-2",

	LogLevel: logutil.DefaultLogLevel,
	// default, stderr, stdout, or file name
	// log file named with cluster name will be added automatically
	LogOutputs: []string{"stderr"},

	// https://github.com/kubernetes/kubernetes/tags
	// https://kubernetes.io/docs/tasks/tools/install-kubectl/
	// https://docs.aws.amazon.com/eks/latest/userguide/install-kubectl.html
	KubectlDownloadURL: "https://storage.googleapis.com/kubernetes-release/release/v1.14.10/bin/linux/amd64/kubectl",
	KubectlPath:        "/tmp/kubectl-test-1.14.10",

	OnFailureDelete:            true,
	OnFailureDeleteWaitSeconds: 120,

	S3BucketName:                    "aws-k8s-tester-eks-s3-bucket",
	S3BucketCreate:                  true,
	S3BucketDelete:                  false,
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

	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	AddOnManagedNodeGroups: &AddOnManagedNodeGroups{
		Enable:      false,
		FetchLogs:   true,
		SigningName: "eks",

		RoleCreate: true,

		RemoteAccessUserName: "ec2-user", // assume Amazon Linux 2

		// to be auto-generated
		LogsDir: "",
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

	AddOnJobPi: &AddOnJobPi{
		Enable:    false,
		Completes: 30,
		Parallels: 10,
	},

	AddOnJobEcho: &AddOnJobEcho{
		Enable:    false,
		Completes: 10,
		Parallels: 10,
		EchoSize:  100 * 1024, // 100 KB

		// writes total 100 MB data to etcd
		// Completes: 1000,
		// Parallels: 100,
		// EchoSize:      100 * 1024, // 100 KB
	},

	AddOnCronJob: &AddOnCronJob{
		Enable:                     false,
		Schedule:                   "*/10 * * * *", // every 10-min
		Completes:                  10,
		Parallels:                  10,
		SuccessfulJobsHistoryLimit: 3,
		FailedJobsHistoryLimit:     1,
		EchoSize:                   100 * 1024, // 100 KB
	},

	AddOnSecrets: &AddOnSecrets{
		Enable:      false,
		Objects:     10,
		Size:        10 * 1024, // 10 KB
		SecretQPS:   1,
		SecretBurst: 1,
		PodQPS:      100,
		PodBurst:    5,

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

	AddOnAppMesh: &AddOnAppMesh{
		Enable: false,
	},
	// read-only
	Status: &Status{Up: false},
	StatusManagedNodeGroups: &StatusManagedNodeGroups{
		NvidiaDriverInstalled: false,
		Nodes:                 make(map[string]StatusManagedNodeGroup),
	},
}

// NewDefault returns a copy of the default configuration.
func NewDefault() *Config {
	vv := DefaultConfig

	if name := os.Getenv(EnvironmentVariablePrefix + "NAME"); name != "" {
		vv.Name = name
	} else {
		vv.Name = fmt.Sprintf("eks-%s-%s", getTS()[:10], randString(12))
	}

	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	vv.AddOnManagedNodeGroups.MNGs = map[string]MNG{
		vv.Name + "-mng-cpu": MNG{
			Name:               vv.Name + "-mng-cpu",
			ReleaseVersion:     "", // to be auto-filled by EKS API
			AMIType:            "AL2_x86_64",
			ASGMinSize:         2,
			ASGMaxSize:         2,
			ASGDesiredCapacity: 2,
			InstanceTypes:      []string{DefaultNodeInstanceTypeCPU},
			VolumeSize:         DefaultNodeVolumeSize,
		},
	}

	return &vv
}

const (
	// DefaultNodeInstanceTypeCPU is the default EC2 instance type for CPU worker node.
	DefaultNodeInstanceTypeCPU = "c5.xlarge"
	// DefaultNodeInstanceTypeGPU is the default EC2 instance type for GPU worker node.
	DefaultNodeInstanceTypeGPU = "p3.8xlarge"

	// DefaultNodeVolumeSize is the default EC2 instance volume size for a worker node.
	DefaultNodeVolumeSize = 40

	// MNGMaxLimit is the maximum number of "Managed Node Group"s per a EKS cluster.
	MNGMaxLimit = 10
	// MNGNodesMaxLimit is the maximum number of nodes per a "Managed Node Group".
	MNGNodesMaxLimit = 100
)

func init() {
	// https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-welcome.html
	// pip3 install awscli --no-cache-dir --upgrade
	var err error
	DefaultConfig.AWSCLIPath, err = exec.LookPath("aws")
	if err != nil {
		panic(fmt.Errorf("aws CLI is not installed (%v)", err))
	}

	if runtime.GOOS == "darwin" {
		DefaultConfig.KubectlDownloadURL = strings.Replace(DefaultConfig.KubectlDownloadURL, "linux", "darwin", -1)
		DefaultConfig.RemoteAccessPrivateKeyPath = filepath.Join(os.TempDir(), randString(10)+".insecure.key")
	}
}

// ValidateAndSetDefaults returns an error for invalid configurations.
// And updates empty fields with default values.
// At the end, it writes populated YAML to aws-k8s-tester config path.
func (cfg *Config) ValidateAndSetDefaults() error {
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
	if err := cfg.validateAddOnManagedNodeGroups(); err != nil {
		return fmt.Errorf("validateAddOnManagedNodeGroups failed [%v]", err)
	}
	if err := cfg.validateAddOnNLBHelloWorld(); err != nil {
		return fmt.Errorf("validateAddOnNLBHelloWorld failed [%v]", err)
	}
	if err := cfg.validateAddOnALB2048(); err != nil {
		return fmt.Errorf("validateAddOnALB2048 failed [%v]", err)
	}
	if err := cfg.validateAddOnJobPi(); err != nil {
		return fmt.Errorf("validateAddOnJobPi failed [%v]", err)
	}
	if err := cfg.validateAddOnJobEcho(); err != nil {
		return fmt.Errorf("validateAddOnJobEcho failed [%v]", err)
	}
	if err := cfg.validateAddOnCronJob(); err != nil {
		return fmt.Errorf("validateAddOnCronJob failed [%v]", err)
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
	if err := cfg.validateAddOnAppMesh(); err != nil {
		return fmt.Errorf("validateAddOnAppMesh failed [%v]", err)
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
	if cfg.CommandAfterCreateClusterOutputPath == "" {
		cfg.CommandAfterCreateClusterOutputPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".after-create-cluster.out.log"
	}
	if filepath.Ext(cfg.CommandAfterCreateClusterOutputPath) != ".log" {
		cfg.CommandAfterCreateClusterOutputPath = cfg.CommandAfterCreateClusterOutputPath + ".log"
	}
	if cfg.CommandAfterCreateAddOnsOutputPath == "" {
		cfg.CommandAfterCreateAddOnsOutputPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".after-create-add-ons.out.log"
	}
	if filepath.Ext(cfg.CommandAfterCreateAddOnsOutputPath) != ".log" {
		cfg.CommandAfterCreateAddOnsOutputPath = cfg.CommandAfterCreateAddOnsOutputPath + ".log"
	}
	if cfg.KubeConfigPath == "" {
		cfg.KubeConfigPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".kubeconfig.yaml"
	}

	if !strings.Contains(cfg.KubectlDownloadURL, runtime.GOOS) {
		return fmt.Errorf("kubectl-download-url %q build OS mismatch, expected %q", cfg.KubectlDownloadURL, runtime.GOOS)
	}

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
			return errors.New("S3BucketCreate false to use existing one but empty S3BucketName")
		}
		if cfg.S3BucketDelete {
			return errors.New("S3BucketCreate false to use existing one but S3BucketDelete true; error to prevent accidental S3 bucket delete")
		}
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

	if cfg.RemoteAccessPrivateKeyPath == "" {
		return errors.New("empty RemoteAccessPrivateKeyPath")
	}

	switch cfg.RemoteAccessKeyCreate {
	case true: // need create one, or already created
		if cfg.RemoteAccessKeyName == "" {
			cfg.RemoteAccessKeyName = cfg.Name + "-key-mng"
		}
		if cfg.RemoteAccessPrivateKeyPath != "" {
			// just ignore...
			// could be populated from previous run
			// do not error, so long as RoleCreate false, role won't be deleted
		}

	case false: // use existing one
		if cfg.RemoteAccessKeyName == "" {
			return fmt.Errorf("RemoteAccessKeyCreate false; expect non-empty RemoteAccessKeyName but got %q", cfg.RemoteAccessKeyName)
		}
		if cfg.RemoteAccessPrivateKeyPath == "" {
			return fmt.Errorf("RemoteAccessKeyCreate false; expect non-empty RemoteAccessPrivateKeyPath but got %q", cfg.RemoteAccessPrivateKeyPath)
		}
	}

	return nil
}

func (cfg *Config) validateAddOnManagedNodeGroups() error {
	if cfg.AddOnManagedNodeGroups == nil {
		return nil
	}
	if !cfg.AddOnManagedNodeGroups.Enable {
		cfg.AddOnManagedNodeGroups = nil
		return nil
	}

	if cfg.Parameters.VersionValue < 1.14 {
		return fmt.Errorf("Version %q not supported for AddOnManagedNodeGroups", cfg.Parameters.Version)
	}
	if cfg.AddOnManagedNodeGroups.RemoteAccessUserName == "" {
		return errors.New("empty AddOnManagedNodeGroups.RemoteAccessUserName")
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

	n := len(cfg.AddOnManagedNodeGroups.MNGs)
	if n == 0 {
		return errors.New("AddOnManagedNodeGroups.Enable but empty AddOnManagedNodeGroups.MNGs")
	}
	if n > MNGNodesMaxLimit {
		return fmt.Errorf("AddOnManagedNodeGroups.MNGs %d exceeds maximum number of node groups per EKS which is %d", n, MNGNodesMaxLimit)
	}
	names := make(map[string]struct{})
	for k, v := range cfg.AddOnManagedNodeGroups.MNGs {
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

		if v.VolumeSize == 0 {
			v.VolumeSize = DefaultNodeVolumeSize
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

		if cfg.IsAddOnNLBHelloWorldEnabled() || cfg.IsAddOnALB2048Enabled() {
			for _, itp := range v.InstanceTypes {
				// "m3.xlarge" or "c4.xlarge" will fail with "InvalidTarget: Targets {...} are not supported"
				// ref. https://github.com/aws/amazon-vpc-cni-k8s/pull/821
				// ref. https://github.com/kubernetes/kubernetes/issues/66044#issuecomment-408188524
				switch {
				case strings.HasPrefix(itp, "m3."),
					strings.HasPrefix(itp, "c4."):
					return fmt.Errorf("AddOnNLBHelloWorld.Enable[%v] || AddOnALB2048.Enable[%v], but older instance type InstanceTypes %q for %q",
						cfg.IsAddOnNLBHelloWorldEnabled(),
						cfg.IsAddOnALB2048Enabled(),
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
		if v.ASGMaxSize > MNGNodesMaxLimit {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].ASGMaxSize %d > MNGNodesMaxLimit %d", k, v.ASGMaxSize, MNGNodesMaxLimit)
		}
		if v.ASGDesiredCapacity > MNGNodesMaxLimit {
			return fmt.Errorf("AddOnManagedNodeGroups.MNGs[%q].ASGDesiredCapacity %d > MNGNodesMaxLimit %d", k, v.ASGDesiredCapacity, MNGNodesMaxLimit)
		}

		if cfg.IsAddOnNLBHelloWorldEnabled() && cfg.AddOnNLBHelloWorld.DeploymentReplicas < int32(v.ASGDesiredCapacity) {
			cfg.AddOnNLBHelloWorld.DeploymentReplicas = int32(v.ASGDesiredCapacity)
		}
		if cfg.IsAddOnALB2048Enabled() && cfg.AddOnALB2048.DeploymentReplicasALB < int32(v.ASGDesiredCapacity) {
			cfg.AddOnALB2048.DeploymentReplicasALB = int32(v.ASGDesiredCapacity)
		}
		if cfg.IsAddOnALB2048Enabled() && cfg.AddOnALB2048.DeploymentReplicas2048 < int32(v.ASGDesiredCapacity) {
			cfg.AddOnALB2048.DeploymentReplicas2048 = int32(v.ASGDesiredCapacity)
		}

		cfg.AddOnManagedNodeGroups.MNGs[k] = v
	}

	return nil
}

func (cfg *Config) validateAddOnNLBHelloWorld() error {
	if cfg.AddOnNLBHelloWorld == nil {
		return nil
	}
	switch cfg.AddOnNLBHelloWorld.Enable {
	case true:
		if cfg.AddOnManagedNodeGroups == nil {
			return errors.New("AddOnManagedNodeGroups disabled but AddOnNLBHelloWorld.Enable true")
		}
		if !cfg.AddOnManagedNodeGroups.Enable {
			return errors.New("AddOnManagedNodeGroups disabled but AddOnNLBHelloWorld.Enable true")
		}
	case false:
		cfg.AddOnNLBHelloWorld = nil
		return nil
	}

	if cfg.AddOnNLBHelloWorld.Namespace == "" {
		cfg.AddOnNLBHelloWorld.Namespace = cfg.Name + "-nlb-hello-world"
	}
	return nil
}

func (cfg *Config) validateAddOnALB2048() error {
	if cfg.AddOnALB2048 == nil {
		return nil
	}
	switch cfg.AddOnALB2048.Enable {
	case true:
		if cfg.AddOnManagedNodeGroups == nil {
			return errors.New("AddOnManagedNodeGroups disabled but AddOnALB2048.Enable true")
		}
		if !cfg.AddOnManagedNodeGroups.Enable {
			return errors.New("AddOnManagedNodeGroups disabled but AddOnALB2048.Enable true")
		}
	case false:
		cfg.AddOnALB2048 = nil
		return nil
	}

	if cfg.AddOnALB2048.Namespace == "" {
		cfg.AddOnALB2048.Namespace = cfg.Name + "-alb-2048"
	}
	return nil
}

func (cfg *Config) validateAddOnJobPi() error {
	if cfg.AddOnJobPi == nil {
		return nil
	}
	switch cfg.AddOnJobPi.Enable {
	case true:
		if cfg.AddOnManagedNodeGroups == nil {
			return errors.New("AddOnManagedNodeGroups disabled but AddOnJobPi.Enable true")
		}
		if !cfg.AddOnManagedNodeGroups.Enable {
			return errors.New("AddOnManagedNodeGroups disabled but AddOnJobPi.Enable true")
		}
	case false:
		cfg.AddOnJobPi = nil
		return nil
	}

	if cfg.AddOnJobPi.Namespace == "" {
		cfg.AddOnJobPi.Namespace = cfg.Name + "-job-perl"
	}
	return nil
}

func (cfg *Config) validateAddOnJobEcho() error {
	if cfg.AddOnJobEcho == nil {
		return nil
	}
	switch cfg.AddOnJobEcho.Enable {
	case true:
		if cfg.AddOnManagedNodeGroups == nil {
			return errors.New("AddOnManagedNodeGroups disabled but AddOnJobEcho.Enable true")
		}
		if !cfg.AddOnManagedNodeGroups.Enable {
			return errors.New("AddOnManagedNodeGroups disabled but AddOnJobEcho.Enable true")
		}
	case false:
		cfg.AddOnJobEcho = nil
		return nil
	}

	if cfg.AddOnJobEcho.Namespace == "" {
		cfg.AddOnJobEcho.Namespace = cfg.Name + "-job-echo"
	}
	if cfg.AddOnJobEcho.EchoSize > 250000 {
		return fmt.Errorf("echo size limit is 0.25 MB, got %d", cfg.AddOnJobEcho.EchoSize)
	}
	return nil
}

func (cfg *Config) validateAddOnCronJob() error {
	if cfg.AddOnCronJob == nil {
		return nil
	}
	switch cfg.AddOnCronJob.Enable {
	case true:
		if cfg.AddOnManagedNodeGroups == nil {
			return errors.New("AddOnManagedNodeGroups disabled but AddOnCronJob.Enable true")
		}
		if !cfg.AddOnManagedNodeGroups.Enable {
			return errors.New("AddOnManagedNodeGroups disabled but AddOnCronJob.Enable true")
		}
	case false:
		cfg.AddOnCronJob = nil
		return nil
	}

	if cfg.AddOnCronJob.Namespace == "" {
		cfg.AddOnCronJob.Namespace = cfg.Name + "-cronjob"
	}
	if cfg.AddOnCronJob.EchoSize > 250000 {
		return fmt.Errorf("echo size limit is 0.25 MB, got %d", cfg.AddOnCronJob.EchoSize)
	}
	return nil
}

// only letters and numbers for Secret key names
var secretRegex = regexp.MustCompile("[^a-zA-Z0-9]+")

func (cfg *Config) validateAddOnSecrets() error {
	if cfg.AddOnSecrets == nil {
		return nil
	}
	switch cfg.AddOnSecrets.Enable {
	case true:
		if cfg.AddOnManagedNodeGroups == nil {
			return errors.New("AddOnManagedNodeGroups disabled but AddOnSecrets.Enable true")
		}
		if !cfg.AddOnManagedNodeGroups.Enable {
			return errors.New("AddOnManagedNodeGroups disabled but AddOnSecrets.Enable true")
		}
	case false:
		cfg.AddOnSecrets = nil
		return nil
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
	if cfg.AddOnIRSA == nil {
		return nil
	}
	switch cfg.AddOnIRSA.Enable {
	case true:
		if cfg.AddOnManagedNodeGroups == nil {
			return errors.New("AddOnManagedNodeGroups disabled but AddOnIRSA.Enable true")
		}
		if !cfg.AddOnManagedNodeGroups.Enable {
			return errors.New("AddOnManagedNodeGroups disabled but AddOnIRSA.Enable true")
		}
	case false:
		cfg.AddOnIRSA = nil
		return nil
	}

	if cfg.Parameters.VersionValue < 1.14 {
		return fmt.Errorf("Version %q not supported for AddOnIRSA", cfg.Parameters.Version)
	}
	if cfg.AddOnIRSA.Namespace == "" {
		cfg.AddOnIRSA.Namespace = cfg.Name + "-irsa"
	}
	if cfg.AddOnIRSA.RoleName == "" {
		cfg.AddOnIRSA.RoleName = cfg.Name + "-role-irsa"
	}
	if cfg.AddOnIRSA.ServiceAccountName == "" {
		cfg.AddOnIRSA.ServiceAccountName = cfg.Name + "-irsa-service-account"
	}
	if cfg.AddOnIRSA.ConfigMapName == "" {
		cfg.AddOnIRSA.ConfigMapName = cfg.Name + "-irsa-configmap"
	}
	if cfg.AddOnIRSA.ConfigMapScriptFileName == "" {
		cfg.AddOnIRSA.ConfigMapScriptFileName = cfg.Name + "-irsa-configmap.sh"
	}
	if cfg.AddOnIRSA.S3Key == "" {
		cfg.AddOnIRSA.S3Key = path.Join(cfg.Name, "irsa-s3-key")
	}
	if cfg.AddOnIRSA.DeploymentName == "" {
		cfg.AddOnIRSA.DeploymentName = cfg.Name + "-irsa-deployment"
	}
	if cfg.AddOnIRSA.DeploymentResultPath == "" {
		cfg.AddOnIRSA.DeploymentResultPath = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-irsa-deployment-result.log")
	}
	return nil
}

func (cfg *Config) validateAddOnFargate() error {
	if cfg.AddOnFargate == nil {
		return nil
	}
	switch cfg.AddOnFargate.Enable {
	case true:
		if cfg.AddOnManagedNodeGroups == nil {
			return errors.New("AddOnManagedNodeGroups disabled but AddOnFargate.Enable true")
		}
		if !cfg.AddOnManagedNodeGroups.Enable {
			return errors.New("AddOnManagedNodeGroups disabled but AddOnFargate.Enable true")
		}
	case false:
		cfg.AddOnFargate = nil
		return nil
	}

	if cfg.Parameters.VersionValue < 1.14 {
		return fmt.Errorf("Version %q not supported for AddOnFargate", cfg.Parameters.Version)
	}
	if cfg.AddOnFargate.Namespace == "" {
		cfg.AddOnFargate.Namespace = cfg.Name + "-fargate"
	}
	if cfg.AddOnFargate.ProfileName == "" {
		cfg.AddOnFargate.ProfileName = cfg.Name + "-fargate-profile"
	}
	if cfg.AddOnFargate.SecretName == "" {
		cfg.AddOnFargate.SecretName = cfg.Name + "addonfargatesecret"
	}
	if cfg.AddOnFargate.PodName == "" {
		cfg.AddOnFargate.PodName = cfg.Name + "-fargate-pod"
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

func (cfg *Config) validateAddOnAppMesh() error {
	if cfg.AddOnAppMesh == nil {
		return nil
	}
	switch cfg.AddOnAppMesh.Enable {
	case true:
		if cfg.AddOnManagedNodeGroups == nil {
			return errors.New("AddOnManagedNodeGroups disabled but AddOnAppMesh.Enable true")
		}
		if !cfg.AddOnManagedNodeGroups.Enable {
			return errors.New("AddOnManagedNodeGroups disabled but AddOnAppMesh.Enable true")
		}
	case false:
		cfg.AddOnAppMesh = nil
		return nil
	}

	if cfg.AddOnAppMesh.Namespace == "" {
		cfg.AddOnAppMesh.Namespace = "appmesh-system"
	}
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
