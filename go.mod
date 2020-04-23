module github.com/aws/aws-k8s-tester

go 1.14

// https://github.com/aws/aws-sdk-go/releases
// https://github.com/uber-go/zap/releases
// https://github.com/helm/helm/releases
// https://github.com/kubernetes/client-go/releases
// https://github.com/kubernetes-sigs/yaml/releases
// https://github.com/blang/semver/releases
// https://github.com/gyuho/semver/releases
// https://github.com/manifoldco/promptui/releases
require (
	github.com/aws/aws-sdk-go v1.30.13
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575
	github.com/davecgh/go-spew v1.1.1
	github.com/dustin/go-humanize v1.0.0
	github.com/fatih/color v1.9.0 // indirect
	github.com/go-ini/ini v1.55.0
	github.com/gofrs/flock v0.7.1
	github.com/manifoldco/promptui v0.7.0
	github.com/mattn/go-runewidth v0.0.8 // indirect
	github.com/mholt/archiver/v3 v3.3.0
	github.com/mitchellh/ioprogress v0.0.0-20180201004757-6a23b12fa88e
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.7.0
	github.com/prometheus/client_golang v1.0.0
	github.com/prometheus/common v0.4.1
	github.com/smartystreets/goconvey v0.0.0-20190731233626-505e41936337 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.5.1
	go.etcd.io/etcd v0.0.0-20200410171415-59f5fb25a533
	go.uber.org/zap v1.14.1
	golang.org/x/crypto v0.0.0-20200414173820-0848c9571904
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4
	gopkg.in/ini.v1 v1.46.0 // indirect
	gopkg.in/yaml.v2 v2.2.8
	helm.sh/helm/v3 v3.2.0-rc.1
	k8s.io/api v0.18.2
	k8s.io/apimachinery v0.18.2
	k8s.io/cli-runtime v0.18.2
	k8s.io/client-go v0.18.2
	k8s.io/utils v0.0.0-20200324210504-a9aa75ae1b89
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/yaml v1.2.0
)
