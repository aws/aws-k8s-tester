module github.com/aws/aws-k8s-tester/k8s-tester/fluent-bit

go 1.16

replace github.com/aws/aws-k8s-tester/client => /Users/jonahjo/go/src/code.amazon.com/aws-k8s-tester/client

require (
	github.com/aws/aws-k8s-tester/client v0.0.0-20210426175539-e17fe4b411c6
	github.com/aws/aws-k8s-tester/k8s-tester/tester v0.0.0-20210426175539-e17fe4b411c6
	github.com/aws/aws-k8s-tester/utils v0.0.0-20210426175539-e17fe4b411c6
	github.com/aws/aws-sdk-go v1.38.21
	github.com/manifoldco/promptui v0.8.0
	github.com/onsi/ginkgo v1.12.1 // indirect
	github.com/onsi/gomega v1.11.0 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/stretchr/testify v1.6.1 // indirect
	go.uber.org/zap v1.16.0
	k8s.io/api v0.21.0
	k8s.io/apimachinery v0.21.0
	k8s.io/client-go v0.21.0
	k8s.io/utils v0.0.0-20210305010621-2afb4311ab10
)
