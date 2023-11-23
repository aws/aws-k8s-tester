module github.com/aws/aws-k8s-tester/kubetest2

go 1.21

require (
	github.com/aws/aws-sdk-go-v2 v1.23.1
	github.com/aws/aws-sdk-go-v2/config v1.18.14
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.40.1
	github.com/aws/aws-sdk-go-v2/service/eks v1.33.2
	github.com/octago/sflags v0.2.0
	github.com/spf13/pflag v1.0.5
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.100.1
	sigs.k8s.io/kubetest2 v0.0.0-20231014151303-89f09b65e8dd
)

require (
	github.com/aws/aws-sdk-go-v2/credentials v1.13.14 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.12.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.2.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.5.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.30 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.23 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.12.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.14.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.18.4 // indirect
	github.com/aws/smithy-go v1.17.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect
	github.com/spf13/cobra v1.7.0 // indirect
	github.com/stretchr/testify v1.8.4 // indirect
)
