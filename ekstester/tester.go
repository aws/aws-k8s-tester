/**************************/
/* TO BE DEPRECATED
/**************************/

// Package ekstester defines interface EKS cluster tester and deployer.
package ekstester

import (
	osexec "os/exec"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"k8s.io/client-go/kubernetes"
)

// Tester defines aws-k8s-tester eks specific operations.
type Tester interface {
	Deployer
	// UploadToBucketForTests uploads a local file to aws-k8s-tester S3 bucket.
	UploadToBucketForTests(localPath, remotePath string) error
}

// Deployer defines EKS deployer.
// Satisfies "k8s.io/test-infra/kubetest/main.go" 'deployer' interfaces.
// Reference https://github.com/kubernetes/test-infra/blob/master/kubetest/main.go.
type Deployer interface {
	Up() error
	IsUp() error
	TestSetup() error
	Down() error
	GetClusterCreated(gcpProject string) (time.Time, error)
	DumpClusterLogs(artifactDir, _ string) error

	// GetWorkerNodeLogs downloads logs from worker node machines.
	GetWorkerNodeLogs() error

	// Stop stops ongoing operations.
	// This is useful for local development.
	// For example, one may run "Up" but have to cancel onging "Up"
	// operation. Then, it can just send syscall.SIGINT to trigger "Stop".
	Stop()

	// LoadConfig reloads configuration from disk to read the latest
	// cluster configuration and its states.
	// It's either reloaded from disk or returned from embedded EKS deployer.
	LoadConfig() (eksconfig.Config, error)

	// KubectlCommand returns "kubectl" command object for API reachability tests.
	KubectlCommand() (*osexec.Cmd, error)
	// KubernetesClientSet returns Kubernetes Go client.
	KubernetesClientSet() *kubernetes.Clientset
}
