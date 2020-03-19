module github.com/aws/aws-k8s-tester

go 1.14

// Pin all k8s.io staging repositories to kubernetes-1.15.3.
// When bumping Kubernetes dependencies, you should update each of these lines
// to point to the same kubernetes-1.x.y release branch before running update-deps.sh.
replace k8s.io/client-go => k8s.io/client-go v0.0.0-20190918200256-06eb1244587a

// https://github.com/aws/aws-sdk-go/releases
// https://github.com/uber-go/zap/releases
// https://github.com/kubernetes-sigs/yaml/releases
// https://github.com/blang/semver/releases
// https://github.com/gyuho/semver/releases
require (
	github.com/aws/aws-sdk-go v1.29.28
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575
	github.com/davecgh/go-spew v1.1.1
	github.com/dustin/go-humanize v1.0.0
	github.com/fatih/color v1.9.0 // indirect
	github.com/go-ini/ini v1.46.0
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/mattn/go-runewidth v0.0.8 // indirect
	github.com/mholt/archiver/v3 v3.3.0
	github.com/mitchellh/ioprogress v0.0.0-20180201004757-6a23b12fa88e
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.0.0
	github.com/prometheus/common v0.4.1
	github.com/smartystreets/goconvey v0.0.0-20190731233626-505e41936337 // indirect
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.4.0
	go.uber.org/zap v1.14.1
	golang.org/x/crypto v0.0.0-20200128174031-69ecbb4d6d5d
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/time v0.0.0-20180412165947-fbb02b2291d2
	gopkg.in/ini.v1 v1.46.0 // indirect
	helm.sh/helm/v3 v3.1.1
	k8s.io/api v0.17.3
	k8s.io/apimachinery v0.17.3
	k8s.io/cli-runtime v0.17.3
	k8s.io/client-go v0.17.3
	k8s.io/utils v0.0.0-20191114184206-e782cd3c129f
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/yaml v1.1.0
)
