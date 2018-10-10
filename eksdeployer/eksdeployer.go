// Package eksdeployer defines interface EKS cluster deployer.
package eksdeployer

import (
	"time"

	"github.com/aws/awstester/eksconfig"
)

// Interface defines 'kubetest' deployer.
// Satisfies "k8s.io/test-infra/kubetest/main.go" 'deployer' and 'publisher" interfaces.
// Reference https://github.com/kubernetes/test-infra/blob/master/kubetest/main.go.
type Interface interface {
	Up() error
	IsUp() error
	TestSetup() error
	Down() error
	GetClusterCreated(gcpProject string) (time.Time, error)
	DumpClusterLogs(localPath, remotePath string) error
	Publish() error

	///////////////////////////////////////////////
	// Extra methods for EKS specific operations //
	///////////////////////////////////////////////

	// Stop stops ongoing operation.
	Stop()

	// LoadConfig returns the current config and its state.
	// It's either returned from embedded EKS deployer
	// or reloaded from disk.
	LoadConfig() (eksconfig.Config, error)

	ALB
}

type ALB interface {
	// TestCorrectness checks if a test ingress service
	// returns the expected output.
	TestCorrectness() error
	// TestQPS runs ingress load testing.
	// And returns an error if QPS is less than expected QPS.
	TestQPS() error
	// TestMetrics checks if ALB Ingress Controller
	// is serving /metrics endpoint.
	TestMetrics() error
}
