package eksctl

import (
	"github.com/aws/aws-k8s-tester/internal/util"
	"k8s.io/klog"
)

func (d *deployer) Down() error {
	klog.Infof("deleting cluster %s", d.commonOptions.RunID())
	err := util.ExecuteCommand("eksctl", "delete", "cluster", "--name", d.commonOptions.RunID(), "--wait")
	if err != nil {
		return err
	}
	klog.Infof("deleted cluster: %s", d.commonOptions.RunID())
	return nil
}
