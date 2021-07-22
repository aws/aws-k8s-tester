module github.com/aws/aws-k8s-tester/k8s-tester/cmd/readme-gen

go 1.16

require (
	github.com/aws/aws-k8s-tester/k8s-tester v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/cloudwatch-agent v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/clusterloader v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/cni v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/configmaps v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/conformance v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/csi-ebs v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/csrs v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/falco v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/fluent-bit v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/jobs-echo v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/jobs-pi v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/kubernetes-dashboard v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/metrics-server v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/nlb-guestbook v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/nlb-hello-world v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/php-apache v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/secrets v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/stress v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/wordpress v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/utils v0.0.0-20210610170531-2e5d31e5196c
	github.com/olekukonko/tablewriter v0.0.5
)

replace (
	github.com/aws/aws-k8s-tester/client => ../../../client
	github.com/aws/aws-k8s-tester/k8s-tester => ../..
	github.com/aws/aws-k8s-tester/k8s-tester/cloudwatch-agent => ../../cloudwatch-agent
	github.com/aws/aws-k8s-tester/k8s-tester/clusterloader => ../../clusterloader
	github.com/aws/aws-k8s-tester/k8s-tester/cni => ../../cni
	github.com/aws/aws-k8s-tester/k8s-tester/configmaps => ../../configmaps
	github.com/aws/aws-k8s-tester/k8s-tester/conformance => ../../conformance
	github.com/aws/aws-k8s-tester/k8s-tester/csi-ebs => ../../csi-ebs
	github.com/aws/aws-k8s-tester/k8s-tester/csrs => ../../csrs
	github.com/aws/aws-k8s-tester/k8s-tester/falco => ../../falco
	github.com/aws/aws-k8s-tester/k8s-tester/fluent-bit => ../../fluent-bit
	github.com/aws/aws-k8s-tester/k8s-tester/helm => ../../helm
	github.com/aws/aws-k8s-tester/k8s-tester/jobs-echo => ../../jobs-echo
	github.com/aws/aws-k8s-tester/k8s-tester/jobs-pi => ../../jobs-pi
	github.com/aws/aws-k8s-tester/k8s-tester/kubernetes-dashboard => ../../kubernetes-dashboard
	github.com/aws/aws-k8s-tester/k8s-tester/metrics-server => ../../metrics-server
	github.com/aws/aws-k8s-tester/k8s-tester/nlb-guestbook => ../../nlb-guestbook
	github.com/aws/aws-k8s-tester/k8s-tester/nlb-hello-world => ../../nlb-hello-world
	github.com/aws/aws-k8s-tester/k8s-tester/php-apache => ../../php-apache
	github.com/aws/aws-k8s-tester/k8s-tester/secrets => ../../secrets
	github.com/aws/aws-k8s-tester/k8s-tester/stress => ../../stress
	github.com/aws/aws-k8s-tester/k8s-tester/tester => ../../tester
	github.com/aws/aws-k8s-tester/k8s-tester/wordpress => ../../wordpress
	github.com/aws/aws-k8s-tester/utils => ../../../utils
)
