module github.com/aws/aws-k8s-tester/k8s-tester/helm

go 1.17

require (
	github.com/aws/aws-k8s-tester/utils v0.0.0-00010101000000-000000000000
	github.com/gofrs/flock v0.8.0
	go.uber.org/zap v1.19.1
	gopkg.in/yaml.v2 v2.4.0
	helm.sh/helm/v3 v3.6.3
	k8s.io/cli-runtime v0.21.1
	rsc.io/letsencrypt v0.0.3 // indirect
)

replace (
	github.com/aws/aws-k8s-tester/utils => ../../utils
)
