// Package metricsserver implements Kubernetes metrics server.
// ref. https://github.com/kubernetes-sigs/metrics-server/releases
package metricsserver

import (
	"fmt"

	"github.com/aws/aws-k8s-tester/eksconfig"
	k8sclient "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	gotemplate "github.com/aws/aws-k8s-tester/pkg/util"
)

// MetricsServer is an addon that installs Metrics Server
type MetricsServer struct {
	K8sClient k8sclient.EKS
	Config    *eksconfig.Config
}

// IsEnabled returns true if enabled
func (c *MetricsServer) IsEnabled() bool {
	return c.Config.Spec.MetricsServer != nil
}

// Apply installs the addon
func (c *MetricsServer) Apply() (err error) {
	template, err := gotemplate.FromLocalDirectory(c.Config.Spec.MetricsServer)
	if err != nil {
		return fmt.Errorf("while building templates, %v", err)
	}
	if err := c.K8sClient.Apply(template.String()); err != nil {
		return fmt.Errorf("while applying resources, %v", err)
	}
	c.Config.Status.MetricsServer = &eksconfig.MetricsServerStatus{
		AddonStatus: eksconfig.AddonStatus{
			Installed: true,
			Ready:     true,
		},
	}
	return nil
}

// Delete removes the addon
func (c *MetricsServer) Delete() (err error) {
	template, err := gotemplate.FromLocalDirectory(c.Config.Spec.MetricsServer)
	if err != nil {
		return fmt.Errorf("while building templates, %v", err)
	}
	if err := c.K8sClient.Delete(template.String()); err != nil {
		return fmt.Errorf("while deleting resources, %v", err)
	}
	c.Config.Status.MetricsServer = &eksconfig.MetricsServerStatus{
		AddonStatus: eksconfig.AddonStatus{
			Installed: false,
			Ready:     false,
		},
	}
	return nil
}
