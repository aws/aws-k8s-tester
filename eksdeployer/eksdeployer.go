// Package eksdeployer defines interface EKS cluster deployer.
package eksdeployer

import (
	"time"

	"github.com/aws/awstester/eksconfig"
)

// Interface defines EKS deployer.
// Satisfies "k8s.io/test-infra/kubetest/main.go" 'deployer' interfaces.
// Reference https://github.com/kubernetes/test-infra/blob/master/kubetest/main.go.
type Interface interface {
	Up() error
	IsUp() error
	TestSetup() error
	Down() error
	GetClusterCreated(gcpProject string) (time.Time, error)
	DumpClusterLogs(artifactDir, _ string) error

	AWSTester
	ALB
}

// AWSTester defines awstester eks specific operations.
type AWSTester interface {
	// GetWorkerNodeLogs downloads logs from worker node machines.
	GetWorkerNodeLogs() error

	// UploadToBucketForTests uploads a local file to awstester S3 bucket.
	UploadToBucketForTests(localPath, remotePath string) error

	// Stop stops ongoing operation.
	Stop()

	// LoadConfig returns the current config and its state.
	// It's either returned from embedded EKS deployer
	// or reloaded from disk.
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
