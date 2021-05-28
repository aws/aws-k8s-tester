module github.com/aws/aws-k8s-tester/k8s-tester

go 1.16

require (
	github.com/aws/aws-k8s-tester/utils v0.0.0-00010101000000-000000000000
	github.com/spf13/cobra v1.1.3
	go.uber.org/zap v1.17.0
)

replace (
	github.com/aws/aws-k8s-tester/client => ../client

	github.com/aws/aws-k8s-tester/k8s-tester/cloudwatch-agent => ./cloudwatch-agent
	github.com/aws/aws-k8s-tester/k8s-tester/fluent-bit => ./fluent-bit
	github.com/aws/aws-k8s-tester/k8s-tester/jobs-echo => ./jobs-echo
	github.com/aws/aws-k8s-tester/k8s-tester/jobs-pi => ./jobs-pi
	github.com/aws/aws-k8s-tester/k8s-tester/kubernetes-dashboard => ./kubernetes-dashboard
	github.com/aws/aws-k8s-tester/k8s-tester/metrics-server => ./metrics-server
	github.com/aws/aws-k8s-tester/k8s-tester/nlb-hello-world => ./nlb-hello-world
	github.com/aws/aws-k8s-tester/k8s-tester/tester => ./tester
	github.com/aws/aws-k8s-tester/utils => ../utils
)
