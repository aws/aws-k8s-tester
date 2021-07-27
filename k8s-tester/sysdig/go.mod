module github.com/aws/aws-k8s-tester/k8s-tester/sysdig

go 1.16

require (
	github.com/aws/aws-k8s-tester/client v0.0.0-20210610170531-2e5d31e5196c
	github.com/aws/aws-k8s-tester/k8s-tester/helm v0.0.0-20210610170531-2e5d31e5196c
	github.com/aws/aws-k8s-tester/k8s-tester/tester v0.0.0-20210610170531-2e5d31e5196c
	github.com/aws/aws-k8s-tester/utils v0.0.0-20210610170531-2e5d31e5196c
	github.com/manifoldco/promptui v0.8.0
	github.com/spf13/cobra v1.1.3
	go.uber.org/zap v1.17.0
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
)

replace (
	github.com/aws/aws-k8s-tester/client => ../../client
	github.com/aws/aws-k8s-tester/k8s-tester/helm => ../helm
	github.com/aws/aws-k8s-tester/k8s-tester/tester => ../tester
	github.com/aws/aws-k8s-tester/utils => ../../utils
)
