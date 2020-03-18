package ec2config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/aws"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"k8s.io/client-go/util/homedir"
)

// DefaultConfig is the default configuration.
//  - empty string creates a non-nil object for pointer-type field
//  - omitting an entire field returns nil value
//  - make sure to check both
//
var DefaultConfig = Config{
	// to be auto-generated
	ConfigPath:                     "",
	RemoteAccessCommandsOutputPath: "",
	Name:                           "",

	Region: "us-west-2",

	LogLevel: logutil.DefaultLogLevel,
	// default, stderr, stdout, or file name
	// log file named with cluster name will be added automatically
	LogOutputs: []string{"stderr"},

	OnFailureDelete:            true,
	OnFailureDeleteWaitSeconds: 120,

	S3BucketName:                    "",
	S3BucketCreate:                  false,
	S3BucketLifecycleExpirationDays: 0,

	RoleCreate:                 true,
	VPCCreate:                  true,
	RemoteAccessKeyCreate:      true,
	RemoteAccessPrivateKeyPath: filepath.Join(homedir.HomeDir(), ".ssh", "ec2_aws_rsa"),

	ASGsFetchLogs: true,
}

func init() {
	if runtime.GOOS == "darwin" {
		DefaultConfig.RemoteAccessPrivateKeyPath = filepath.Join(os.TempDir(), randString(10)+".insecure.key")
	}
}

// NewDefault returns a copy of the default configuration.
func NewDefault() *Config {
	vv := DefaultConfig
	vv.mu = new(sync.RWMutex)

	if name := os.Getenv(EnvironmentVariablePrefix + "NAME"); name != "" {
		vv.Name = name
	} else {
		vv.Name = fmt.Sprintf("ec2-%s-%s", getTS()[:10], randString(12))
	}

	ssmDocName := vv.Name + "SSMDocument"
	ssmDocName = strings.ReplaceAll(ssmDocName, "-", "")
	vv.ASGs = map[string]ASG{
		vv.Name + "-asg": {
			Name:                               vv.Name + "-asg",
			RemoteAccessUserName:               "ec2-user", // for AL2
			SSMDocumentName:                    vv.Name + "SSMDocument",
			SSMDocumentCreate:                  false,
			SSMDocumentCommands:                "",
			SSMDocumentExecutionTimeoutSeconds: 3600,
			AMIType:                            AMITypeAL2X8664,
			InstanceTypes:                      []string{DefaultNodeInstanceTypeCPU},
			VolumeSize:                         DefaultNodeVolumeSize,
			ASGMinSize:                         1,
			ASGMaxSize:                         1,
			ASGDesiredCapacity:                 1,
		},
	}

	return &vv
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
	if err := cfg.validateASGs(); err != nil {
		return fmt.Errorf("validateASGs failed [%v]", err)
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
	if cfg.ASGsLogsDir == "" {
		cfg.ASGsLogsDir = filepath.Join(filepath.Dir(cfg.ConfigPath), cfg.Name+"-logs-remote")
	}

	if cfg.RemoteAccessCommandsOutputPath == "" {
		cfg.RemoteAccessCommandsOutputPath = strings.ReplaceAll(cfg.ConfigPath, ".yaml", "") + ".ssh.sh"
	}
	if filepath.Ext(cfg.RemoteAccessCommandsOutputPath) != ".sh" {
		cfg.RemoteAccessCommandsOutputPath = cfg.RemoteAccessCommandsOutputPath + ".sh"
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

	switch cfg.RoleCreate {
	case true: // need create one, or already created
		if cfg.RoleName == "" {
			cfg.RoleName = cfg.Name + "-role-ec2"
		}
		if cfg.RoleARN != "" {
			// just ignore...
			// could be populated from previous run
			// do not error, so long as RoleCreate false, role won't be deleted
		}
		if len(cfg.RoleServicePrincipals) > 0 {
			/*
				create node group request failed (InvalidParameterException: Following required service principals [ec2.amazonaws.com] were not found in the trust relationships of nodeRole arn:aws:iam::...:role/test-mng-role
				{
				  ClusterName: "test",
				  Message_: "Following required service principals [ec2.amazonaws.com] were not found in the trust relationships of nodeRole arn:aws:iam::...:role/test-mng-role",
				  NodegroupName: "test-mng-cpu"
				})
			*/
			found := false
			for _, pv := range cfg.RoleServicePrincipals {
				if pv == "ec2.amazonaws.com" { // TODO: support China regions ec2.amazonaws.com.cn or eks.amazonaws.com.cn
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("RoleServicePrincipals %q must include 'ec2.amazonaws.com'", cfg.RoleServicePrincipals)
			}
		}

	case false: // use existing one
		if cfg.RoleARN == "" {
			return fmt.Errorf("Parameters.RoleCreate false; expect non-empty RoleARN but got %q", cfg.RoleARN)
		}
		if cfg.RoleName == "" {
			cfg.RoleName = getNameFromARN(cfg.RoleARN)
		}
		if len(cfg.RoleManagedPolicyARNs) > 0 {
			return fmt.Errorf("Parameters.RoleCreate false; expect empty RoleManagedPolicyARNs but got %q", cfg.RoleManagedPolicyARNs)
		}
		if len(cfg.RoleServicePrincipals) > 0 {
			return fmt.Errorf("Parameters.RoleCreate false; expect empty RoleServicePrincipals but got %q", cfg.RoleServicePrincipals)
		}
	}

	switch cfg.VPCCreate {
	case true: // need create one, or already created
		if cfg.VPCID != "" {
			// just ignore...
			// could be populated from previous run
			// do not error, so long as VPCCreate false, VPC won't be deleted
		}
	case false: // use existing one
		if cfg.VPCID == "" {
			return fmt.Errorf("Parameters.RoleCreate false; expect non-empty VPCID but got %q", cfg.VPCID)
		}
	}

	switch {
	case cfg.VPCCIDR != "":
		switch {
		case cfg.PublicSubnetCIDR1 == "":
			return fmt.Errorf("empty Parameters.PublicSubnetCIDR1 when VPCCIDR is %q", cfg.VPCCIDR)
		case cfg.PublicSubnetCIDR2 == "":
			return fmt.Errorf("empty Parameters.PublicSubnetCIDR2 when VPCCIDR is %q", cfg.VPCCIDR)
		case cfg.PublicSubnetCIDR3 == "":
			return fmt.Errorf("empty Parameters.PublicSubnetCIDR3 when VPCCIDR is %q", cfg.VPCCIDR)
		case cfg.PrivateSubnetCIDR1 == "":
			return fmt.Errorf("empty Parameters.PrivateSubnetCIDR1 when VPCCIDR is %q", cfg.VPCCIDR)
		case cfg.PrivateSubnetCIDR2 == "":
			return fmt.Errorf("empty Parameters.PrivateSubnetCIDR2 when VPCCIDR is %q", cfg.VPCCIDR)
		}

	case cfg.VPCCIDR == "":
		switch {
		case cfg.PublicSubnetCIDR1 != "":
			return fmt.Errorf("non-empty Parameters.PublicSubnetCIDR1 %q when VPCCIDR is empty", cfg.PublicSubnetCIDR1)
		case cfg.PublicSubnetCIDR2 != "":
			return fmt.Errorf("non-empty Parameters.PublicSubnetCIDR2 %q when VPCCIDR is empty", cfg.PublicSubnetCIDR2)
		case cfg.PublicSubnetCIDR3 != "":
			return fmt.Errorf("non-empty Parameters.PublicSubnetCIDR3 %q when VPCCIDR is empty", cfg.PublicSubnetCIDR3)
		case cfg.PrivateSubnetCIDR1 != "":
			return fmt.Errorf("non-empty Parameters.PrivateSubnetCIDR1 %q when VPCCIDR is empty", cfg.PrivateSubnetCIDR1)
		case cfg.PrivateSubnetCIDR2 != "":
			return fmt.Errorf("non-empty Parameters.PrivateSubnetCIDR2 %q when VPCCIDR is empty", cfg.PrivateSubnetCIDR2)
		}
	}

	switch cfg.RemoteAccessKeyCreate {
	case true: // need create one, or already created
		if cfg.RemoteAccessKeyName == "" {
			cfg.RemoteAccessKeyName = cfg.Name + "-key-ec2"
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

func (cfg *Config) validateASGs() error {
	n := len(cfg.ASGs)
	if n == 0 {
		return errors.New("empty ASGs")
	}
	if n > ASGsMaxLimit {
		return fmt.Errorf("ASGs %d exceeds maximum number of ASGs which is %d", n, ASGsMaxLimit)
	}
	names := make(map[string]struct{})
	for k, v := range cfg.ASGs {
		if v.Name == "" {
			return fmt.Errorf("ASGs[%q].Name is empty", k)
		}
		if k != v.Name {
			return fmt.Errorf("ASGs[%q].Name has different Name field %q", k, v.Name)
		}
		_, ok := names[v.Name]
		if !ok {
			names[v.Name] = struct{}{}
		} else {
			return fmt.Errorf("ASGs[%q].Name %q is redundant", k, v.Name)
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

		switch v.AMIType {
		case AMITypeBottleRocketCPU:
			if v.RemoteAccessUserName != "ec2-user" {
				return fmt.Errorf("AMIType %q but unexpected RemoteAccessUserName %q", v.AMIType, v.RemoteAccessUserName)
			}
			if cfg.ASGsFetchLogs { // TODO: fetch logs via SSM + S3
				return fmt.Errorf("AMIType %q does not suppport log fetch yet", v.AMIType)
			}
		case AMITypeAL2X8664:
			if v.RemoteAccessUserName != "ec2-user" {
				return fmt.Errorf("AMIType %q but unexpected RemoteAccessUserName %q", v.AMIType, v.RemoteAccessUserName)
			}
		case AMITypeAL2X8664GPU:
			if v.RemoteAccessUserName != "ec2-user" {
				return fmt.Errorf("AMIType %q but unexpected RemoteAccessUserName %q", v.AMIType, v.RemoteAccessUserName)
			}
		default:
			return fmt.Errorf("unknown ASGs[%q].AMIType %q", k, v.AMIType)
		}

		switch v.AMIType {
		case AMITypeBottleRocketCPU:
			if len(v.InstanceTypes) == 0 {
				v.InstanceTypes = []string{DefaultNodeInstanceTypeCPU}
			}
		case AMITypeAL2X8664:
			if len(v.InstanceTypes) == 0 {
				v.InstanceTypes = []string{DefaultNodeInstanceTypeCPU}
			}
		case AMITypeAL2X8664GPU:
			if len(v.InstanceTypes) == 0 {
				v.InstanceTypes = []string{DefaultNodeInstanceTypeGPU}
			}
		default:
			return fmt.Errorf("unknown ASGs[%q].AMIType %q", k, v.AMIType)
		}

		if v.ASGMinSize > v.ASGMaxSize {
			return fmt.Errorf("ASGs[%q].ASGMinSize %d > ASGMaxSize %d", k, v.ASGMinSize, v.ASGMaxSize)
		}
		if v.ASGDesiredCapacity > v.ASGMaxSize {
			return fmt.Errorf("ASGs[%q].ASGDesiredCapacity %d > ASGMaxSize %d", k, v.ASGDesiredCapacity, v.ASGMaxSize)
		}
		if v.ASGMaxSize > ASGMaxLimit {
			return fmt.Errorf("ASGs[%q].ASGMaxSize %d > ASGMaxLimit %d", k, v.ASGMaxSize, ASGMaxLimit)
		}
		if v.ASGDesiredCapacity > ASGMaxLimit {
			return fmt.Errorf("ASGs[%q].ASGDesiredCapacity %d > ASGMaxLimit %d", k, v.ASGDesiredCapacity, ASGMaxLimit)
		}

		switch v.SSMDocumentCreate {
		case true: // need create one, or already created
			if v.SSMDocumentName == "" {
				v.SSMDocumentName = v.Name + "SSMDocument"
			}
			if v.SSMDocumentExecutionTimeoutSeconds == 0 {
				v.SSMDocumentExecutionTimeoutSeconds = 3600
			}

		case false: // use existing one, or don't run any SSM
		}

		cfg.ASGs[k] = v
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
