module github.com/aws/aws-k8s-tester

go 1.13

// Pin all k8s.io staging repositories to kubernetes-1.15.3.
// When bumping Kubernetes dependencies, you should update each of these lines
// to point to the same kubernetes-1.x.y release branch before running update-deps.sh.
replace (
	k8s.io/api => k8s.io/api v0.0.0-20190918195907-bd6ac527cfd2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190918201827-3de75813f604
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190817020851-f2f3a405f61d
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190918200256-06eb1244587a
)

// https://github.com/aws/aws-sdk-go/releases
// https://github.com/uber-go/zap/releases
// https://github.com/kubernetes-sigs/yaml/releases
// https://github.com/blang/semver/releases
require (
	github.com/aws/aws-sdk-go v1.25.33
	github.com/awslabs/goformation/v3 v3.1.0
	github.com/blang/semver v3.6.1+incompatible
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20190719114852-fd7a80b32e1f // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/dustin/go-humanize v1.0.0
	github.com/go-ini/ini v1.46.0
	github.com/gorilla/websocket v1.4.1 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.1.0 // indirect
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/mitchellh/ioprogress v0.0.0-20180201004757-6a23b12fa88e
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/prometheus/client_golang v1.0.0
	github.com/prometheus/common v0.4.1
	github.com/smartystreets/goconvey v0.0.0-20190731233626-505e41936337 // indirect
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.4.0
	github.com/tmc/grpc-websocket-proxy v0.0.0-20190109142713-0ad062ec5ee5 // indirect
	go.etcd.io/etcd v0.0.0-20191004122535-555eb1951f2e
	go.uber.org/zap v1.12.0
	golang.org/x/crypto v0.0.0-20190611184440-5c40567a22f8
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	gopkg.in/ini.v1 v1.46.0 // indirect
	k8s.io/api v0.0.0-20191016225839-816a9b7df678
	k8s.io/apimachinery v0.0.0-20191020214737-6c8691705fc5
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/utils v0.0.0-20191010214722-8d271d903fe4
	sigs.k8s.io/yaml v1.1.0
)
