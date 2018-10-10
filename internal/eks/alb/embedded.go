package alb

import (
	"fmt"

	"github.com/aws/awstester/eksconfig"
	"github.com/aws/awstester/internal/eks/s3"

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
	im iamiface.IAMAPI,
	ec2 ec2iface.EC2API,
	elbv2 elbv2iface.ELBV2API,
	s3Plugin s3.Plugin,
) (Plugin, error) {
	md := &embedded{
		stopc:    stopc,
		lg:       lg,
		cfg:      cfg,
		kubectl:  exec.New(),
		im:       im,
		ec2:      ec2,
		elbv2:    elbv2,
		s3Plugin: s3Plugin,
	}

	var err error
	md.kubectlPath, err = md.kubectl.LookPath("kubectl")
	if err != nil {
		return nil, fmt.Errorf("cannot find 'kubectl' executable (%v)", err)
	}
	if _, err = exec.New().LookPath("aws-iam-authenticator"); err != nil {
		return nil, fmt.Errorf("cannot find 'aws-iam-authenticator' executable (%v)", err)
	}

	return md, nil
}
