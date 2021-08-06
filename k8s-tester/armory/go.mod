module github.com/aws/aws-k8s-tester/k8s-tester/armory

go 1.16

replace (
	github.com/aws/aws-k8s-tester/client => ../../client
	github.com/aws/aws-k8s-tester/k8s-tester/helm => ../helm
	github.com/aws/aws-k8s-tester/k8s-tester/tester => ../tester
	github.com/aws/aws-k8s-tester/utils => ../../utils
)

require (
	github.com/aws/aws-k8s-tester/client v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/helm v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/tester v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/utils v0.0.0-20210722211631-7411a257e5a3
	github.com/manifoldco/promptui v0.8.0
	github.com/spf13/cobra v1.2.1
	go.uber.org/zap v1.18.1
	k8s.io/utils v0.0.0-20210802155522-efc7438f0176
)
