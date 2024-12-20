package eksapi

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type awsClients struct {
	_eks *eks.Client
	_cfn *cloudformation.Client
	_ec2 *ec2.Client
	_asg *autoscaling.Client
	_ssm *ssm.Client
	_iam *iam.Client
}

func newAWSClients(config aws.Config, eksEndpointURL string) *awsClients {
	clients := awsClients{
		_cfn: cloudformation.NewFromConfig(config),
		_ec2: ec2.NewFromConfig(config),
		_asg: autoscaling.NewFromConfig(config),
		_ssm: ssm.NewFromConfig(config),
		_iam: iam.NewFromConfig(config),
	}
	if eksEndpointURL != "" {
		clients._eks = eks.NewFromConfig(config, func(o *eks.Options) {
			o.BaseEndpoint = aws.String(eksEndpointURL)
		})
	} else {
		clients._eks = eks.NewFromConfig(config)
	}
	return &clients
}

func (c *awsClients) EKS() *eks.Client {
	return c._eks
}

func (c *awsClients) CFN() *cloudformation.Client {
	return c._cfn
}

func (c *awsClients) EC2() *ec2.Client {
	return c._ec2
}

func (c *awsClients) ASG() *autoscaling.Client {
	return c._asg
}

func (c *awsClients) SSM() *ssm.Client {
	return c._ssm
}

func (c *awsClients) IAM() *iam.Client {
	return c._iam
}
