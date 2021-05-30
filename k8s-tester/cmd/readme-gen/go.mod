module github.com/aws/aws-k8s-tester/k8s-tester/cmd/readme-gen

go 1.16

require (
	github.com/aws/aws-k8s-tester/k8s-tester v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/cloudwatch-agent v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/fluent-bit v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/jobs-echo v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/jobs-pi v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/kubernetes-dashboard v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/metrics-server v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/nlb-hello-world v0.0.0-00010101000000-000000000000
	github.com/olekukonko/tablewriter v0.0.5
)

replace (
	github.com/aws/aws-k8s-tester/client => ../../../client

	github.com/aws/aws-k8s-tester/k8s-tester => ../..
	github.com/aws/aws-k8s-tester/k8s-tester/cloudwatch-agent => ../../cloudwatch-agent
	github.com/aws/aws-k8s-tester/k8s-tester/fluent-bit => ../../fluent-bit
	github.com/aws/aws-k8s-tester/k8s-tester/helm => ../../helm
	github.com/aws/aws-k8s-tester/k8s-tester/jobs-echo => ../../jobs-echo
	github.com/aws/aws-k8s-tester/k8s-tester/jobs-pi => ../../jobs-pi
	github.com/aws/aws-k8s-tester/k8s-tester/kubernetes-dashboard => ../../kubernetes-dashboard
	github.com/aws/aws-k8s-tester/k8s-tester/metrics-server => ../../metrics-server
	github.com/aws/aws-k8s-tester/k8s-tester/nlb-hello-world => ../../nlb-hello-world
	github.com/aws/aws-k8s-tester/k8s-tester/tester => ../../tester
	github.com/aws/aws-k8s-tester/utils => ../../../utils
)
