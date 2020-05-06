package eksconfig

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"k8s.io/client-go/util/homedir"
)

const (
	// DefaultClients is the default number of clients to create.
	DefaultClients = 3
	// DefaultClientQPS is the default client QPS.
	DefaultClientQPS float32 = 10
	// DefaultClientBurst is the default client burst.
	DefaultClientBurst = 20
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

		LogLevel: logutil.DefaultLogLevel,
		// default, stderr, stdout, or file name
		// log file named with cluster name will be added automatically
		LogOutputs: []string{"stderr"},

		// https://github.com/kubernetes/kubernetes/tags
		// https://kubernetes.io/docs/tasks/tools/install-kubectl/
		// https://docs.aws.amazon.com/eks/latest/userguide/install-kubectl.html
		KubectlPath:        "/tmp/kubectl-test-v1.16.9",
		KubectlDownloadURL: "https://storage.googleapis.com/kubernetes-release/release/v1.16.9/bin/linux/amd64/kubectl",

		OnFailureDelete:            true,
		OnFailureDeleteWaitSeconds: 120,

		S3BucketName:                    "",
		S3BucketCreate:                  false,
		S3BucketCreateKeep:              false,
		S3BucketLifecycleExpirationDays: 0,

		Parameters: getDefaultParameters(),

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

		AddOnNodeGroups:          getDefaultAddOnNodeGroups(name),
		AddOnManagedNodeGroups:   getDefaultAddOnManagedNodeGroups(name),
		AddOnCSIEBS:              getDefaultAddOnCSIEBS(),
		AddOnKubernetesDashboard: getDefaultAddOnKubernetesDashboard(),
		AddOnPrometheusGrafana:   getDefaultAddOnPrometheusGrafana(),
		AddOnNLBHelloWorld:       getDefaultAddOnNLBHelloWorld(),
		AddOnALB2048:             getDefaultAddOnALB2048(),
		AddOnJobsPi:              getDefaultAddOnJobsPi(),
		AddOnJobsEcho:            getDefaultAddOnJobsEcho(),
		AddOnCronJobs:            getDefaultAddOnCronJobs(),
		AddOnCSRs:                getDefaultAddOnCSRs(),
		AddOnConfigMaps:          getDefaultAddOnConfigMaps(),
		AddOnSecrets:             getDefaultAddOnSecrets(),
		AddOnIRSA:                getDefaultAddOnIRSA(),
		AddOnFargate:             getDefaultAddOnFargate(),
		AddOnIRSAFargate:         getDefaultAddOnIRSAFargate(),
		AddOnAppMesh:             getDefaultAddOnAppMesh(),
		AddOnWordpress:           getDefaultAddOnWordpress(),
		AddOnJupyterHub:          getDefaultAddOnJupyterHub(),
		AddOnKubeflow:            getDefaultAddOnKubeflow(),
		AddOnHollowNodes:         getDefaultAddOnHollowNodes(),
		AddOnClusterLoader:       getDefaultAddOnClusterLoader(),
		AddOnConformance:         getDefaultAddOnConformance(),

		// read-only
		Status: &Status{Up: false},
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
		cfg.RemoteAccessPrivateKeyPath = filepath.Join(os.TempDir(), randutil.String(10)+".insecure.key")
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
	if err := cfg.validateAddOnKubernetesDashboard(); err != nil {
		return fmt.Errorf("validateAddOnKubernetesDashboard failed [%v]", err)
	}
	if err := cfg.validateAddOnPrometheusGrafana(); err != nil {
		return fmt.Errorf("validateAddOnPrometheusGrafana failed [%v]", err)
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
	if err := cfg.validateAddOnWordpress(); err != nil {
		return fmt.Errorf("validateAddOnWordpress failed [%v]", err)
	}
	if err := cfg.validateAddOnJupyterHub(); err != nil {
		return fmt.Errorf("validateAddOnJupyterHub failed [%v]", err)
	}
	if err := cfg.validateAddOnKubeflow(); err != nil {
		return fmt.Errorf("validateAddOnKubeflow failed [%v]", err)
	}
	if err := cfg.validateAddOnHollowNodes(); err != nil {
		return fmt.Errorf("validateAddOnHollowNodes failed [%v]", err)
	}
	if err := cfg.validateAddOnClusterLoader(); err != nil {
		return fmt.Errorf("validateAddOnClusterLoader failed [%v]", err)
	}
	if err := cfg.validateAddOnConformance(); err != nil {
		return fmt.Errorf("validateAddOnConformance failed [%v]", err)
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
		return fmt.Errorf("region %q for partition %q not found in %+v", cfg.Partition, regions)
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
		cfg.LogOutputs = append(cfg.LogOutputs, strings.ReplaceAll(cfg.ConfigPath, ".yaml", "")+".log")
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
