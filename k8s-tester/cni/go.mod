module github.com/aws/aws-k8s-tester/k8s-tester/cni

go 1.16

require (
	github.com/aws/aws-k8s-tester/client v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/helm v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/tester v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/utils v0.0.0-00010101000000-000000000000
	github.com/manifoldco/promptui v0.8.0
	github.com/spf13/cobra v1.1.3
	go.uber.org/zap v1.17.0
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
)

replace (
	github.com/aws/aws-k8s-tester/client => ../../client
	github.com/aws/aws-k8s-tester/k8s-tester/helm => ../helm
	github.com/aws/aws-k8s-tester/k8s-tester/tester => ../tester
	github.com/aws/aws-k8s-tester/utils => ../../utils
)