module github.com/aws/aws-k8s-tester/client

go 1.16

require (
	github.com/aws/aws-k8s-tester/utils v0.0.0-00010101000000-000000000000
	github.com/aws/aws-sdk-go v1.38.50
	github.com/briandowns/spinner v1.13.0
	go.uber.org/zap v1.17.0
	golang.org/x/oauth2 v0.0.0-20210514164344-f6687ab2804c
	k8s.io/api v0.21.1
	k8s.io/apiextensions-apiserver v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
)

replace github.com/aws/aws-k8s-tester/utils => ../utils
