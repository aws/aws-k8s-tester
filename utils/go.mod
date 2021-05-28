module github.com/aws/aws-k8s-tester/utils

go 1.16

require (
	github.com/aws/aws-sdk-go v1.38.50
	github.com/briandowns/spinner v1.13.0
	github.com/dustin/go-humanize v1.0.0
	github.com/mitchellh/ioprogress v0.0.0-20180201004757-6a23b12fa88e
	go.uber.org/zap v1.17.0
	k8s.io/client-go v0.21.1
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
)

replace (
	github.com/aws/aws-k8s-tester/utils/aws => ./aws
	github.com/aws/aws-k8s-tester/utils/file => ./file
	github.com/aws/aws-k8s-tester/utils/http => ./http
	github.com/aws/aws-k8s-tester/utils/log => ./log
	github.com/aws/aws-k8s-tester/utils/rand => ./rand
	github.com/aws/aws-k8s-tester/utils/spinner => ./spinner
	github.com/aws/aws-k8s-tester/utils/terminal => ./terminal
)
