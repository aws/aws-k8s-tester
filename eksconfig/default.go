package eksconfig

import (
	"path/filepath"
	"time"

	"k8s.io/client-go/util/homedir"
)

// NewDefault returns a copy of the default configuration.
func NewDefault() *Config {
	vv := defaultConfig
	return &vv
}

// defaultConfig is the default configuration.
//  - empty string creates a non-nil object for pointer-type field
//  - omitting an entire field returns nil value
//  - make sure to check both
var defaultConfig = Config{
	KubetestEmbeddedBinary: true,

	// enough time for ALB access log
	WaitBeforeDown: 10 * time.Minute,
	Down:           true,

	ConfigPath: "test.yaml",

	EnableWorkerNodeHA:  true,
	EnableWorkerNodeSSH: true,

	AWSAccountID: "",
	// to be overwritten by AWS_SHARED_CREDENTIALS_FILE
	AWSCredentialToMountPath: filepath.Join(homedir.HomeDir(), ".aws", "credentials"),
	AWSRegion:                "us-west-2",
	AWSCustomEndpoint:        "",

	// https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html
	WorkerNodeAMI:          "ami-0a54c984b9f908c81",
	WorkerNodeInstanceType: "m5.large",
	WorkderNodeASGMin:      1,
	WorkderNodeASGMax:      1,
	WorkerNodeVolumeSizeGB: 20,

	KubernetesVersion: "1.10",

	LogDebug: false,

	// default, stderr, stdout, or file name
	// log file named with cluster name will be added automatically
	LogOutputs:           []string{"stderr"},
	LogAccess:            true,
	UploadTesterLogs:     true,
	UploadWorkerNodeLogs: true,

	ClusterState: &ClusterState{},
	ALBIngressController: &ALBIngressController{
		Enable:           true,
		UploadTesterLogs: true,

		IngressControllerImage: "quay.io/coreos/alb-ingress-controller:1.0-beta.7",

		// 'instance' to use node port
		// 'ip' to use pod IP
		TargetType: "instance",
		TestMode:   "nginx",

		TestScalability:          true,
		TestMetrics:              true,
		TestServerReplicas:       1,
		TestServerRoutes:         1,
		TestClients:              200,
		TestClientRequests:       20000,
		TestResponseSize:         40 * 1024, // 40 KB
		TestClientErrorThreshold: 10,
		TestExpectQPS:            20000,
	},
}
