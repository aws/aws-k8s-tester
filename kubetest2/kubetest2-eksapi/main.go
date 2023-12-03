package main

import (
	"github.com/aws/aws-k8s-tester/kubetest2/internal/deployers/eksapi"
	"sigs.k8s.io/kubetest2/pkg/app"
)

func main() {
	app.Main(eksapi.DeployerName, eksapi.NewDeployer)
}
