package eksconfig

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/logutil"
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
	SSHCommandsOutputPath:     "",
	KubeConfigPath:            "",
	Name:                      "",
	AWSCLIPath:                "",

	Region: "us-west-2",

	LogLevel: logutil.DefaultLogLevel,
	// default, stderr, stdout, or file name
	// log file named with cluster name will be added automatically
	LogOutputs: []string{"stderr"},

	// https://kubernetes.io/docs/tasks/tools/install-kubectl/
	// https://docs.aws.amazon.com/eks/latest/userguide/install-kubectl.html
	KubectlDownloadURL: "https://storage.googleapis.com/kubernetes-release/release/v1.14.10/bin/linux/amd64/kubectl",
	KubectlPath:        "/tmp/aws-k8s-tester/kubectl",

	OnFailureDelete:            true,
	OnFailureDeleteWaitSeconds: 60,

	Parameters: &Parameters{
		ClusterSigningName: "eks",
		Version:            "1.14",
	},

	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
	AddOnManagedNodeGroups: &AddOnManagedNodeGroups{
		Enable:      true,
		SigningName: "eks",

		RoleServicePrincipals: []string{
			"ec2.amazonaws.com",
			"eks.amazonaws.com",
		},
		RoleManagedPolicyARNs: []string{
			"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
			"arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
			"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
		},

		// keep in-sync with the default value in https://godoc.org/k8s.io/kubernetes/test/e2e/framework#GetSigner
		RemoteAccessPrivateKeyPath: filepath.Join(homedir.HomeDir(), ".ssh", "kube_aws_rsa"),
		RemoteAccessUserName:       "ec2-user", // assume Amazon Linux 2

		// to be auto-generated
		LogsDir: "",
	},

	AddOnNLBHelloWorld: &AddOnNLBHelloWorld{
		Enable:             true,
		DeploymentReplicas: 3,
	},
	AddOnALB2048: &AddOnALB2048{
		Enable:                 false,
		DeploymentReplicasALB:  3,
		DeploymentReplicas2048: 3,
	},
	AddOnJobPerl: &AddOnJobPerl{
		Enable:    false,
		Completes: 30,
		Parallels: 10,
	},
	AddOnJobEcho: &AddOnJobEcho{ // writes total 100 MB data to etcd
		Enable:    false,
		Completes: 1000,
		Parallels: 100,
		Size:      100 * 1024, // 100 KB
	},
	AddOnSecrets: &AddOnSecrets{
		Enable: false,

		Objects:     1000,
		Size:        10 * 1024, // 10 KB
		SecretQPS:   1,
		SecretBurst: 1,
		PodQPS:      100,
		PodBurst:    5,

		// writes total 153.6 MB for "Secret" objects,
		// plus "Pod" objects, writes total 330 MB to etcd
		//
		// with 3 nodes, takes about 1.5 hour for all
		// these "Pod"s to complete
		//
		// Objects:     15000,
		// Size:        10 * 1024, // 10 KB
		// SecretQPS:   150,
		// SecretBurst: 5,
		// PodQPS:      150,
		// PodBurst:    5,
	},
	AddOnIRSA: &AddOnIRSA{
		Enable:                false,
		RoleManagedPolicyARNs: []string{"arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"},
		DeploymentReplicas:    10,
	},

	// read-only
	Status: &Status{Up: false},
	StatusManagedNodeGroups: &StatusManagedNodeGroups{
		RoleCFNStackID:        "",
		NvidiaDriverInstalled: false,
		Nodes:                 make(map[string]StatusManagedNodeGroup),
	},
}

// NewDefault returns a copy of the default configuration.
func NewDefault() *Config {
	vv := DefaultConfig

	now := time.Now()
	vv.Name = fmt.Sprintf(
		"eks-%d%02d%02d%02d-%s",
		now.Year(),
		int(now.Month()),
		now.Day(),
		now.Hour(),
		randString(12),
	)

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
		DefaultConfig.AddOnManagedNodeGroups.RemoteAccessPrivateKeyPath = filepath.Join(os.TempDir(), randString(10)+".insecure.key")
	}
}
