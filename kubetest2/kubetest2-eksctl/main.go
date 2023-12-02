package main

import (
	"github.com/aws/aws-k8s-tester/kubetest2/internal/deployers/eksctl"
	"sigs.k8s.io/kubetest2/pkg/app"
)

func main() {
	app.Main(eksctl.DeployerName, eksctl.NewDeployer)
}
