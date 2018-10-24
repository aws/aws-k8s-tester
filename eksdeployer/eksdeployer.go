// Package eksdeployer defines interface EKS cluster deployer.
package eksdeployer

import (
	"time"

	"github.com/aws/awstester/eksconfig"
)

// Tester defines awstester eks specific operations.
type Tester interface {
	Deployer

	ALB

	// UploadToBucketForTests uploads a local file to awstester S3 bucket.
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

	// Stop stops ongoing operation.
	Stop()

	// LoadConfig reloads or returns the current config and its state.
	// It's either reloaded from disk or returned from embedded EKS deployer.
	LoadConfig() (eksconfig.Config, error)
}

// ALB defines AWS application load balancer tester.
type ALB interface {
	// TestALBCorrectness checks if a test ingress service
	// returns the expected output.
	TestALBCorrectness() error
	// TestALBQPS runs ingress load testing.
	// And returns an error if QPS is less than expected QPS.
	TestALBQPS() error
	// TestALBMetrics checks if ALB Ingress Controller
	// is serving /metrics endpoint.
	TestALBMetrics() error
}
