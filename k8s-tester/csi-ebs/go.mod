module github.com/aws/aws-k8s-tester/k8s-tester/csi-ebs

go 1.16

require (
	github.com/aws/aws-k8s-tester/client v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/helm v0.0.0-20210602225304-3e5119396cf5 // indirect
	github.com/aws/aws-k8s-tester/k8s-tester/tester v0.0.0-20210512161402-cf8f6d8f50e2
	github.com/aws/aws-k8s-tester/utils v0.0.0-20210512161402-cf8f6d8f50e2
	github.com/docker/docker v20.10.2+incompatible // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/manifoldco/promptui v0.8.0
	github.com/spf13/cobra v1.1.3
	go.uber.org/zap v1.17.0
	helm.sh/helm/v3 v3.6.0 // indirect
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v1.5.2 // indirect
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b // indirect
	rsc.io/letsencrypt v0.0.3 // indirect
)

replace (
	github.com/aws/aws-k8s-tester/client => /Users/jonahjo/go/src/github.com/aws-k8s-tester/client
	github.com/aws/aws-k8s-tester/helm => ../helm
	github.com/aws/aws-k8s-tester/utils => ../../utils
	k8s.io/api => k8s.io/api v0.21.1
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.21.1
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.1
	k8s.io/apiserver => k8s.io/apiserver v0.21.1
	//EBS stuff
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.21.1
	k8s.io/client-go => k8s.io/client-go v0.21.1
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.21.1
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.21.1
	k8s.io/code-generator => k8s.io/code-generator v0.21.1
	k8s.io/component-base => k8s.io/component-base v0.21.1
	k8s.io/component-helpers => k8s.io/component-helpers v0.21.1
	k8s.io/controller-manager => k8s.io/controller-manager v0.21.1
	k8s.io/cri-api => k8s.io/cri-api v0.21.1
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.21.1
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.21.1
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.21.1
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.21.1
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.21.1
	k8s.io/kubectl => k8s.io/kubectl v0.21.1
	k8s.io/kubelet => k8s.io/kubelet v0.21.1
	//K8s
	k8s.io/kubernetes => k8s.io/kubernetes v1.21.1
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.21.1
	k8s.io/metrics => k8s.io/metrics v0.21.1
	k8s.io/mount-utils => k8s.io/mount-utils v0.21.1
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.21.1
)
