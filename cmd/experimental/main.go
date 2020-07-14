package main

import (
	"github.com/aws/aws-k8s-tester/pkg/deployer"
	"github.com/aws/aws-k8s-tester/pkg/deployer/types"
)

// TODO Delete this main in favor of something more real.
func main() {
	t := deployer.NewDeployer(&types.DeployerConfiguration{
		Spec: types.DeployerConfigurationSpec{
			ClusterAutoscaler: &types.ClusterAutoscalerSpec{},
		},
	})
	t.Up()
	t.Down()
}
