module github.com/aws/aws-k8s-tester/k8s-tester/secureCN

go 1.16

require (
	github.com/Portshift/escher-client v0.0.0-20211130125722-d700a6a0cb1b
	github.com/aws/aws-k8s-tester/client v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/tester v0.0.0-00010101000000-00000000
	github.com/aws/aws-k8s-tester/utils v0.0.0-00010101000000-000000000000
	github.com/go-openapi/strfmt v0.19.5
	github.com/spf13/cobra v1.2.1
	go.uber.org/zap v1.19.1
	k8s.io/utils v0.0.0-20210707171843-4b05e18ac7d9
)

replace (
	github.com/aws/aws-k8s-tester/client => ../../client
	github.com/aws/aws-k8s-tester/k8s-tester/tester => ../tester
	github.com/aws/aws-k8s-tester/utils => ../../utils
)
