package eksapi

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"k8s.io/klog"
)

//go:embed cloudformation/infra.yaml
var infraTemplate string

const (
	infraStackCreationTimeout = time.Minute * 10
	infraStackDeletionTimeout = time.Minute * 10
)

// eksEndpointURLTag is the key for an optional tag on the infrastructure CloudFormation stack,
// which indicates which EKS environment is associated with the stack's resources.
// The tag is only added when --endpoint-url is passed to the deployer.
const eksEndpointURLTag = "eks-endpoint-url"

type infra struct {
	vpc            string
	securityGroups []string
	subnetsPublic  []string
	subnetsPrivate []string
	clusterRole    string
	nodeRole       string
}

func (i *infra) subnets() []string {
	return append(i.subnetsPublic, i.subnetsPrivate...)
}

func createInfrastructureStack(cfnClient *cloudformation.Client, infra *infra, opts *deployerOptions, resourceID string) error {
	input := cloudformation.CreateStackInput{
		StackName:    aws.String(resourceID),
		TemplateBody: aws.String(infraTemplate),
		Capabilities: []cloudformationtypes.Capability{cloudformationtypes.CapabilityCapabilityIam},
	}
	if opts.ClusterRoleServicePrincipal != "" {
		input.Parameters = []cloudformationtypes.Parameter{
			{
				ParameterKey:   aws.String("AdditionalClusterRoleServicePrincipal"),
				ParameterValue: aws.String(opts.ClusterRoleServicePrincipal),
			},
		}
	}
	if opts.EKSEndpointURL != "" {
		input.Tags = []cloudformationtypes.Tag{
			{
				Key:   aws.String(eksEndpointURLTag),
				Value: aws.String(opts.EKSEndpointURL),
			},
		}
	}
	klog.Infof("creating infrastructure stack...")
	out, err := cfnClient.CreateStack(context.TODO(), &input)
	if err != nil {
		return err
	}
	klog.Infof("waiting for infrastructure stack to be created: %s", *out.StackId)
	err = cloudformation.NewStackCreateCompleteWaiter(cfnClient).
		Wait(context.TODO(),
			&cloudformation.DescribeStacksInput{
				StackName: out.StackId,
			},
			infraStackCreationTimeout)
	if err != nil {
		return fmt.Errorf("failed to wait for infrastructure stack creation: %w", err)
	}
	klog.Infof("getting infrastructure stack outputs: %s", *out.StackId)
	stack, err := cfnClient.DescribeStacks(context.TODO(), &cloudformation.DescribeStacksInput{
		StackName: out.StackId,
	})
	if err != nil {
		return fmt.Errorf("failed to describe infrastructure stack: %w", err)
	}
	for _, output := range stack.Stacks[0].Outputs {
		value := *output.OutputValue
		switch *output.OutputKey {
		case "Vpc":
			infra.vpc = value
		case "SecurityGroups":
			infra.securityGroups = strings.Split(value, ",")
		case "SubnetsPublic":
			infra.subnetsPublic = strings.Split(value, ",")
		case "SubnetsPrivate":
			infra.subnetsPrivate = strings.Split(value, ",")
		case "ClusterRole":
			infra.clusterRole = value
		case "NodeRole":
			infra.nodeRole = value
		}
	}
	klog.Infof("created infrastructure: %+v", infra)
	return nil
}

func deleteInfrastructureStack(cfnClient *cloudformation.Client, resourceID string) error {
	input := cloudformation.DeleteStackInput{
		StackName: aws.String(resourceID),
	}
	klog.Infof("deleting infrastructure stack: %s", resourceID)
	_, err := cfnClient.DeleteStack(context.TODO(), &input)
	if err != nil {
		var notFound *cloudformationtypes.StackNotFoundException
		if errors.As(err, &notFound) {
			klog.Infof("infrastructure stack does not exist: %s", resourceID)
			return nil
		}
		return fmt.Errorf("failed to delete infrastructure stack: %w", err)
	}
	klog.Infof("waiting for infrastructure stack to be deleted: %s", resourceID)
	err = cloudformation.NewStackDeleteCompleteWaiter(cfnClient).
		Wait(context.TODO(),
			&cloudformation.DescribeStacksInput{
				StackName: aws.String(resourceID),
			},
			infraStackDeletionTimeout)
	if err != nil {
		return fmt.Errorf("failed to wait for infrastructure stack deletion: %w", err)
	}
	klog.Infof("deleted infrastructure stack: %s", resourceID)
	return nil
}
