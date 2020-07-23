package overprovisioning

import (
	"fmt"

	"github.com/aws/aws-k8s-tester/eksconfig"
	k8sclient "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	gotemplate "github.com/aws/aws-k8s-tester/pkg/util"
)

// Overprovisioning is an addon that installs extra pods to the cluster
type Overprovisioning struct {
	K8sClient k8sclient.EKS
	Config    *eksconfig.Config
}

// IsEnabled returns true if enabled
func (c *Overprovisioning) IsEnabled() bool {
	return c.Config.Spec.Overprovisioning != nil
}

// Apply installs the addon
func (c *Overprovisioning) Apply() (err error) {
	template, err := gotemplate.FromLocalDirectory(c.Config.Spec.Overprovisioning)
	if err != nil {
		return fmt.Errorf("while building templates, %v", err)
	}
	if err := c.K8sClient.Apply(template.String()); err != nil {
		return fmt.Errorf("while applying resources, %v", err)
	}
	c.Config.Status.Overprovisioning = &eksconfig.OverprovisioningStatus{
		AddonStatus: eksconfig.AddonStatus{
			Installed: true,
			Ready:     true,
		},
	}
	return nil
}

// Delete removes the addon
func (c *Overprovisioning) Delete() (err error) {
	template, err := gotemplate.FromLocalDirectory(c.Config.Spec.Overprovisioning)
	if err != nil {
		return fmt.Errorf("while building templates, %v", err)
	}
	if err := c.K8sClient.Delete(template.String()); err != nil {
		return fmt.Errorf("while deleting resources, %v", err)
	}
	c.Config.Status.Overprovisioning = &eksconfig.OverprovisioningStatus{
		AddonStatus: eksconfig.AddonStatus{
			Installed: false,
			Ready:     false,
		},
	}
	return nil
}
