module github.com/aws/aws-k8s-tester/k8s-tester

go 1.17

require (
	github.com/aws/aws-k8s-tester/client v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/aqua v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/armory v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/cloudwatch-agent v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/clusterloader v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/cni v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/configmaps v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/conformance v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/csi-ebs v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/csi-efs v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/csrs v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/epsagon v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/falco v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/falcon v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/fluent-bit v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/jobs-echo v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/jobs-pi v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/kubecost v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/kubernetes-dashboard v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/metrics-server v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/nlb-guestbook v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/nlb-hello-world v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/php-apache v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/secrets v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/secureCN v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/splunk v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/stress v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/sysdig v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/tester v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/vault v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/k8s-tester/wordpress v0.0.0-00010101000000-000000000000
	github.com/aws/aws-k8s-tester/utils v0.0.0-00010101000000-000000000000
	github.com/dustin/go-humanize v1.0.0
	github.com/manifoldco/promptui v0.8.0
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db
	go.uber.org/zap v1.19.1
	sigs.k8s.io/yaml v1.2.0
)

require github.com/ulikunitz/xz v0.5.10 // indirect

replace (
	github.com/aws/aws-k8s-tester/client => ../client
	github.com/aws/aws-k8s-tester/k8s-tester/aqua => ./aqua
	github.com/aws/aws-k8s-tester/k8s-tester/armory => ./armory
	github.com/aws/aws-k8s-tester/k8s-tester/cloudwatch-agent => ./cloudwatch-agent
	github.com/aws/aws-k8s-tester/k8s-tester/clusterloader => ./clusterloader
	github.com/aws/aws-k8s-tester/k8s-tester/cni => ./cni
	github.com/aws/aws-k8s-tester/k8s-tester/configmaps => ./configmaps
	github.com/aws/aws-k8s-tester/k8s-tester/conformance => ./conformance
	github.com/aws/aws-k8s-tester/k8s-tester/csi-ebs => ./csi-ebs
	github.com/aws/aws-k8s-tester/k8s-tester/csi-efs => ./csi-efs
	github.com/aws/aws-k8s-tester/k8s-tester/csrs => ./csrs
	github.com/aws/aws-k8s-tester/k8s-tester/epsagon => ./epsagon
	github.com/aws/aws-k8s-tester/k8s-tester/falco => ./falco
	github.com/aws/aws-k8s-tester/k8s-tester/falcon => ./falcon
	github.com/aws/aws-k8s-tester/k8s-tester/fluent-bit => ./fluent-bit
	github.com/aws/aws-k8s-tester/k8s-tester/helm => ./helm
	github.com/aws/aws-k8s-tester/k8s-tester/jobs-echo => ./jobs-echo
	github.com/aws/aws-k8s-tester/k8s-tester/jobs-pi => ./jobs-pi
	github.com/aws/aws-k8s-tester/k8s-tester/kubecost => ./kubecost
	github.com/aws/aws-k8s-tester/k8s-tester/kubernetes-dashboard => ./kubernetes-dashboard
	github.com/aws/aws-k8s-tester/k8s-tester/metrics-server => ./metrics-server
	github.com/aws/aws-k8s-tester/k8s-tester/nlb-guestbook => ./nlb-guestbook
	github.com/aws/aws-k8s-tester/k8s-tester/nlb-hello-world => ./nlb-hello-world
	github.com/aws/aws-k8s-tester/k8s-tester/php-apache => ./php-apache
	github.com/aws/aws-k8s-tester/k8s-tester/secrets => ./secrets
	github.com/aws/aws-k8s-tester/k8s-tester/secureCN => ./secureCN
	github.com/aws/aws-k8s-tester/k8s-tester/splunk => ./splunk
	github.com/aws/aws-k8s-tester/k8s-tester/stress => ./stress
	github.com/aws/aws-k8s-tester/k8s-tester/sysdig => ./sysdig
	github.com/aws/aws-k8s-tester/k8s-tester/tester => ./tester
	github.com/aws/aws-k8s-tester/k8s-tester/vault => ./vault
	github.com/aws/aws-k8s-tester/k8s-tester/version => ./version
	github.com/aws/aws-k8s-tester/k8s-tester/wordpress => ./wordpress
	github.com/aws/aws-k8s-tester/utils => ../utils
)
