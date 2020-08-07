package clusterautoscaler

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-k8s-tester/eksconfig"
	k8sclient "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	gotemplate "github.com/aws/aws-k8s-tester/pkg/util"
)

// Constants
const (
	NodeGroupFormatter         string = "%s-nodegroup-%d"
	NodeGroupArgumentFormatter string = "--nodes=%d:%d:%s"
)

// ClusterAutoscaler is an addon that installs Cluster Autoscaler
type ClusterAutoscaler struct {
	K8sClient k8sclient.EKS
	Config    *eksconfig.Config
}

// IsEnabled returns true if enabled
func (c *ClusterAutoscaler) IsEnabled() bool {
	return c.Config.Spec.ClusterAutoscaler != nil
}

// Apply installs the addon
func (c *ClusterAutoscaler) Apply() (err error) {
	template, err := gotemplate.FromLocalDirectory(struct {
		*eksconfig.ClusterAutoscalerSpec
		Command string
	}{
		ClusterAutoscalerSpec: c.Config.Spec.ClusterAutoscaler,
		Command:               c.buildCommand(),
	})
	if err != nil {
		return fmt.Errorf("while building templates, %v", err)
	}
	if err := c.K8sClient.Apply(template.String()); err != nil {
		return fmt.Errorf("while applying resources, %v", err)
	}
	c.Config.Status.ClusterAutoscaler = &eksconfig.ClusterAutoscalerStatus{
		AddonStatus: eksconfig.AddonStatus{
			Installed: true,
			Ready:     true,
		},
	}
	return nil
}

// Delete removes the addon
func (c *ClusterAutoscaler) Delete() (err error) {
	template, err := gotemplate.FromLocalDirectory(struct {
		*eksconfig.ClusterAutoscalerSpec
		Command string
	}{
		ClusterAutoscalerSpec: c.Config.Spec.ClusterAutoscaler,
		Command:               c.buildCommand(),
	})
	if err != nil {
		return fmt.Errorf("while building templates, %v", err)
	}
	if err := c.K8sClient.Delete(template.String()); err != nil {
		return fmt.Errorf("while deleting resources, %v", err)
	}
	c.Config.Status.ClusterAutoscaler = &eksconfig.ClusterAutoscalerStatus{
		AddonStatus: eksconfig.AddonStatus{
			Installed: false,
			Ready:     false,
		},
	}
	return nil
}

func (c *ClusterAutoscaler) buildCommand() string {
	args := append([]string{
		"./cluster-autoscaler",
		"--v=4",
		"--stderrthreshold=info",
		"--skip-nodes-with-local-storage=false",
		"--expander=least-waste",
		fmt.Sprintf("--scale-down-unneeded-time=%s", c.Config.Spec.ClusterAutoscaler.ScaleDownDelay),
		fmt.Sprintf("--scale-down-delay-after-add=%s", c.Config.Spec.ClusterAutoscaler.ScaleDownDelay),
		fmt.Sprintf("--cloud-provider=%s", c.Config.Spec.ClusterAutoscaler.CloudProvider),
	}, c.buildNodeGroupArguments()...)
	json, _ := json.Marshal(args)
	return string(json)
}

func (c *ClusterAutoscaler) buildNodeGroupArguments() (args []string) {
	spec := c.Config.Spec.ClusterAutoscaler
	if spec.CloudProvider == eksconfig.CloudProviderKubemark {
		for i := 0; i < c.Config.AddOnHollowNodesRemote.NodeGroups; i++ {
			name := fmt.Sprintf(NodeGroupFormatter, c.Config.AddOnHollowNodesRemote.NodeLabelPrefix, i)
			args = append(args, fmt.Sprintf(
				NodeGroupArgumentFormatter,
				spec.MinNodes,
				spec.MaxNodes,
				name,
			))
		}
	}
	if spec.CloudProvider == eksconfig.CloudProviderAWS {
		for _, asg := range c.Config.AddOnNodeGroups.ASGs {
			args = append(args, fmt.Sprintf(NodeGroupArgumentFormatter, spec.MinNodes, spec.MinNodes, asg.Name))
		}
		for _, mng := range c.Config.AddOnManagedNodeGroups.MNGs {
			args = append(args, fmt.Sprintf(NodeGroupArgumentFormatter, spec.MinNodes, spec.MinNodes, mng.Name))
		}
	}
	return args
}
