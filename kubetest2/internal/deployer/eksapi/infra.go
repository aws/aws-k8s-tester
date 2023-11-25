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
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
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

func createInfrastructureStack(clients *awsClients, opts *deployerOptions, resourceID string) (*infra, error) {
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
	out, err := clients.CFN().CreateStack(context.TODO(), &input)
	if err != nil {
		return nil, err
	}
	klog.Infof("waiting for infrastructure stack to be created: %s", *out.StackId)
	err = cloudformation.NewStackCreateCompleteWaiter(clients.CFN()).
		Wait(context.TODO(),
			&cloudformation.DescribeStacksInput{
				StackName: out.StackId,
			},
			infraStackCreationTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for infrastructure stack creation: %w", err)
	}
	klog.Infof("getting infrastructure stack resources: %s", *out.StackId)
	infra, err := getInfrastructureStackResources(clients, *out.StackId)
	if err != nil {
		return nil, fmt.Errorf("failed to get infrastructure stack resources: %w", err)
	}
	klog.Infof("created infrastructure: %+v", infra)
	return infra, nil
}

func getInfrastructureStackResources(clients *awsClients, resourceID string) (*infra, error) {
	stack, err := clients.CFN().DescribeStacks(context.TODO(), &cloudformation.DescribeStacksInput{
		StackName: aws.String(resourceID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe infrastructure stack: %w", err)
	}
	infra := infra{}
	for _, output := range stack.Stacks[0].Outputs {
		value := *output.OutputValue
		switch *output.OutputKey {
		case "VPC":
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
	return &infra, nil
}

func deleteInfrastructureStack(clients *awsClients, resourceID string) error {
	input := cloudformation.DeleteStackInput{
		StackName: aws.String(resourceID),
	}
	klog.Infof("deleting infrastructure stack: %s", resourceID)
	_, err := clients.CFN().DeleteStack(context.TODO(), &input)
	if err != nil {
		var notFound *cloudformationtypes.StackNotFoundException
		if errors.As(err, &notFound) {
			klog.Infof("infrastructure stack does not exist: %s", resourceID)
			return nil
		}
		return fmt.Errorf("failed to delete infrastructure stack: %w", err)
	}
	klog.Infof("waiting for infrastructure stack to be deleted: %s", resourceID)
	err = cloudformation.NewStackDeleteCompleteWaiter(clients.CFN()).
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

// deleteLeakedENIs deletes Elastic Network Interfaces that may have been allocated (and left behind) by the VPC CNI.
// These leaked ENIs will prevent deletion of their associated subnets and security groups.
func deleteLeakedENIs(clients *awsClients, resourceID string) error {
	infra, err := getInfrastructureStackResources(clients, resourceID)
	if err != nil {
		var notFound *cloudformationtypes.StackNotFoundException
		if errors.As(err, &notFound) {
			klog.Infof("infrastructure stack does not exist: %s", resourceID)
			return nil
		}
		return fmt.Errorf("failed to get infrastructure stack resources: %w", err)
	}
	klog.Infof("deleting leaked ENIs...")
	enis := ec2.NewDescribeNetworkInterfacesPaginator(clients.EC2(), &ec2.DescribeNetworkInterfacesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{infra.vpc},
			},
		},
	})
	deleted := 0
	for enis.HasMorePages() {
		page, err := enis.NextPage(context.TODO())
		if err != nil {
			return fmt.Errorf("failed to describe ENIs: %w", err)
		}
		for _, eni := range page.NetworkInterfaces {
			klog.Infof("deleting leaked ENI: %s", *eni.NetworkInterfaceId)
			_, err := clients.EC2().DeleteNetworkInterface(context.TODO(), &ec2.DeleteNetworkInterfaceInput{
				NetworkInterfaceId: eni.NetworkInterfaceId,
			})
			if err != nil {
				return fmt.Errorf("failed to delete leaked ENI: %w", err)
			}
			deleted++
		}
	}
	klog.Infof("deleted %d leaked ENIs", deleted)
	return nil
}
