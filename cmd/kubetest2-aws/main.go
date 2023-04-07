package main

import (
	"sigs.k8s.io/kubetest2/pkg/app"

	"github.com/aws/aws-k8s-tester/cmd/kubetest2-aws/deployer"
)

func main() {
	app.Main(deployer.Name, deployer.New)
}
