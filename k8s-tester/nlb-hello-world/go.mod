module github.com/aws/aws-k8s-tester/k8s-tester/nlb-hello-world

go 1.16

require (
	github.com/aws/aws-k8s-tester/client v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/tester v0.0.0-00010101000000-000000000000
	github.com/aws/aws-sdk-go v1.38.21
	github.com/dustin/go-humanize v1.0.0
	github.com/manifoldco/promptui v0.8.0
	github.com/mitchellh/ioprogress v0.0.0-20180201004757-6a23b12fa88e
	github.com/spf13/cobra v1.1.3
	go.uber.org/zap v1.16.0
	k8s.io/api v0.21.0
	k8s.io/apimachinery v0.21.0
	k8s.io/client-go v0.21.0
	k8s.io/utils v0.0.0-20210305010621-2afb4311ab10
)

replace (
	github.com/aws/aws-k8s-tester/client => ../../client
	github.com/aws/aws-k8s-tester/k8s-tester/tester => ../tester
)
