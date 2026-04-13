package eksctl

import (
	"fmt"
	"log/slog"

	"github.com/aws/aws-k8s-tester/internal/util"
)

func (d *deployer) Down() error {
	d.initClusterName()

	var err error

	if d.DeployTarget == "nodegroup" {
		slog.Info("deleting nodegroup", "nodegroupName", d.NodegroupName, "clusterName", d.clusterName)
		err = util.ExecuteCommand("eksctl", "delete", "nodegroup", "--cluster", d.clusterName, "--name", d.NodegroupName, "--drain=false", "--wait")
		if err != nil {
			return fmt.Errorf("failed to delete nodegroup: %v", err)
		}
		slog.Info("successfully deleted nodegroup", "nodegroupName", d.NodegroupName, "clusterName", d.clusterName)
	} else if d.DeployTarget == "cluster" {
		slog.Info("deleting cluster", "clusterName", d.clusterName)
		err = util.ExecuteCommand("eksctl", "delete", "cluster", "--name", d.clusterName, "--wait", "--disable-nodegroup-eviction")
		if err != nil {
			return fmt.Errorf("failed to delete cluster: %v", err)
		}
		slog.Info("successfully deleted cluster", "clusterName", d.clusterName)
	} else {
		return fmt.Errorf("Unsupported deploy target: %s, supported options: `cluster`, `nodegroup`.", d.DeployTarget)
	}
	return nil
}
