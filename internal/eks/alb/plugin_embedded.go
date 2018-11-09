package alb

import (
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/internal/eks/s3"

	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

type embedded struct {
	stopc chan struct{}

	lg  *zap.Logger
	cfg *eksconfig.Config

	// TODO: move this "kubectl" to AWS CLI deployer
	// and instead use "k8s.io/client-go" with STS token
	kubectl     exec.Interface
	kubectlPath string

	im       iamiface.IAMAPI
	ec2      ec2iface.EC2API
	elbv2    elbv2iface.ELBV2API
	s3Plugin s3.Plugin
}

// NewEmbedded creates a new Plugin using AWS CLI.
func NewEmbedded(
	stopc chan struct{},
	lg *zap.Logger,
	cfg *eksconfig.Config,
	kubectlPath string,
	im iamiface.IAMAPI,
	ec2 ec2iface.EC2API,
	elbv2 elbv2iface.ELBV2API,
	s3Plugin s3.Plugin,
) Plugin {
	return &embedded{
		stopc:       stopc,
		lg:          lg,
		cfg:         cfg,
		kubectlPath: kubectlPath,
		kubectl:     exec.New(),
		im:          im,
		ec2:         ec2,
		elbv2:       elbv2,
		s3Plugin:    s3Plugin,
	}
}
