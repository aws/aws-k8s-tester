package eksctl

import (
	"fmt"

	"github.com/aws/aws-k8s-tester/internal/util"
	"k8s.io/klog"
)

func (d *deployer) Down() error {
	d.initClusterName()

	var err error

	if d.DeployTarget == "nodegroup" {
		klog.Infof("deleting nodegroup %s from cluster %s", d.NodegroupName, d.clusterName)
		args := []string{"delete", "nodegroup", "--cluster", d.clusterName, "--name", d.NodegroupName}
		if d.SkipDrain {
			args = append(args, "--drain=false")
		}
		args = append(args, "--wait")
		err = util.ExecuteCommand("eksctl", args...)
		if err != nil {
			return fmt.Errorf("failed to delete nodegroup: %v", err)
		}
		klog.Infof("Successfully deleted nodegroup: %s from cluster: %s", d.NodegroupName, d.clusterName)
	} else if d.DeployTarget == "cluster" {
		klog.Infof("deleting cluster %s", d.clusterName)
		err = util.ExecuteCommand("eksctl", "delete", "cluster", "--name", d.clusterName, "--wait")
		if err != nil {
			return fmt.Errorf("failed to delete cluster: %v", err)
		}
		klog.Infof("Successfully deleted cluster: %s", d.clusterName)
	} else {
		return fmt.Errorf("Unsupported deploy target: %s, supported options: `cluster`, `nodegroup`.", d.DeployTarget)
	}
	return nil
}
