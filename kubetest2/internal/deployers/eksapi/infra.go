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

	"github.com/aws/aws-k8s-tester/kubetest2/internal/deployers/eksapi/templates"
)

const (
	infraStackCreationTimeout = time.Minute * 15
	infraStackDeletionTimeout = time.Minute * 15
)

// eksEndpointURLTag is the key for an optional tag on the infrastructure CloudFormation stack,
// which indicates which EKS environment is associated with the stack's resources.
// The tag is only added when --endpoint-url is passed to the deployer.
const eksEndpointURLTag = "eks-endpoint-url"

type InfrastructureManager struct {
	clients    *awsClients
	resourceID string
}

func NewInfrastructureManager(clients *awsClients, resourceID string) *InfrastructureManager {
	return &InfrastructureManager{
		clients:    clients,
		resourceID: resourceID,
	}
}

type Infrastructure struct {
	vpc              string
	subnetsPublic    []string
	subnetsPrivate   []string
	clusterRole      string
	nodeRole         string
	sshSecurityGroup string
	sshKeyPair       string
}

func (i *Infrastructure) subnets() []string {
	return append(i.subnetsPublic, i.subnetsPrivate...)
}

func (m *InfrastructureManager) createInfrastructureStack(opts *deployerOptions) (*Infrastructure, error) {
	publicKeyMaterial, err := loadSSHPublicKey()
	if err != nil {
		return nil, err
	}
	input := cloudformation.CreateStackInput{
		StackName:    aws.String(m.resourceID),
		TemplateBody: aws.String(templates.Infrastructure),
		Capabilities: []cloudformationtypes.Capability{cloudformationtypes.CapabilityCapabilityIam},
		Parameters: []cloudformationtypes.Parameter{
			{
				ParameterKey:   aws.String("SSHPublicKeyMaterial"),
				ParameterValue: aws.String(publicKeyMaterial),
			},
			{
				ParameterKey:   aws.String("ResourceId"),
				ParameterValue: aws.String(m.resourceID),
			},
		},
	}
	if opts.ClusterRoleServicePrincipal != "" {
		input.Parameters = append(input.Parameters, cloudformationtypes.Parameter{
			ParameterKey:   aws.String("AdditionalClusterRoleServicePrincipal"),
			ParameterValue: aws.String(opts.ClusterRoleServicePrincipal),
		})
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
	out, err := m.clients.CFN().CreateStack(context.TODO(), &input)
	if err != nil {
		return nil, err
	}
	klog.Infof("waiting for infrastructure stack to be created: %s", *out.StackId)
	err = cloudformation.NewStackCreateCompleteWaiter(m.clients.CFN()).
		Wait(context.TODO(),
			&cloudformation.DescribeStacksInput{
				StackName: out.StackId,
			},
			infraStackCreationTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for infrastructure stack creation: %w", err)
	}
	klog.Infof("getting infrastructure stack resources: %s", *out.StackId)
	infra, err := m.getInfrastructureStackResources()
	if err != nil {
		return nil, fmt.Errorf("failed to get infrastructure stack resources: %w", err)
	}
	klog.Infof("created infrastructure: %+v", infra)
	return infra, nil
}

func (m *InfrastructureManager) getInfrastructureStackResources() (*Infrastructure, error) {
	stack, err := m.clients.CFN().DescribeStacks(context.TODO(), &cloudformation.DescribeStacksInput{
		StackName: aws.String(m.resourceID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe infrastructure stack: %w", err)
	}
	infra := Infrastructure{}
	for _, output := range stack.Stacks[0].Outputs {
		value := *output.OutputValue
		switch *output.OutputKey {
		case "VPC":
			infra.vpc = value
		case "SubnetsPublic":
			infra.subnetsPublic = strings.Split(value, ",")
		case "SubnetsPrivate":
			infra.subnetsPrivate = strings.Split(value, ",")
		case "ClusterRole":
			infra.clusterRole = value
		case "NodeRole":
			infra.nodeRole = value
		case "SSHSecurityGroup":
			infra.sshSecurityGroup = value
		case "SSHKeyPair":
			infra.sshKeyPair = value
		}
	}
	return &infra, nil
}

func (m *InfrastructureManager) deleteInfrastructureStack() error {
	input := cloudformation.DeleteStackInput{
		StackName: aws.String(m.resourceID),
	}
	klog.Infof("deleting infrastructure stack: %s", m.resourceID)
	_, err := m.clients.CFN().DeleteStack(context.TODO(), &input)
	if err != nil {
		var notFound *cloudformationtypes.StackNotFoundException
		if errors.As(err, &notFound) {
			klog.Infof("infrastructure stack does not exist: %s", m.resourceID)
			return nil
		}
		return fmt.Errorf("failed to delete infrastructure stack: %w", err)
	}
	klog.Infof("waiting for infrastructure stack to be deleted: %s", m.resourceID)
	err = cloudformation.NewStackDeleteCompleteWaiter(m.clients.CFN()).
		Wait(context.TODO(),
			&cloudformation.DescribeStacksInput{
				StackName: aws.String(m.resourceID),
			},
			infraStackDeletionTimeout)
	if err != nil {
		return fmt.Errorf("failed to wait for infrastructure stack deletion: %w", err)
	}
	klog.Infof("deleted infrastructure stack: %s", m.resourceID)
	return nil
}

// deleteLeakedENIs deletes Elastic Network Interfaces that may have been allocated (and left behind) by the VPC CNI.
// These leaked ENIs will prevent deletion of their associated subnets and security groups.
func (m *InfrastructureManager) deleteLeakedENIs() error {
	infra, err := m.getInfrastructureStackResources()
	if err != nil {
		var notFound *cloudformationtypes.StackNotFoundException
		if errors.As(err, &notFound) {
			klog.Infof("infrastructure stack does not exist: %s", m.resourceID)
			return nil
		}
		return fmt.Errorf("failed to get infrastructure stack resources: %w", err)
	}
	klog.Infof("deleting leaked ENIs...")
	enis := ec2.NewDescribeNetworkInterfacesPaginator(m.clients.EC2(), &ec2.DescribeNetworkInterfacesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{infra.vpc},
			},
			{
				Name:   aws.String("attachment.status"),
				Values: []string{"detached"},
			},
			{
				Name:   aws.String("interface-type"),
				Values: []string{"interface"},
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
			_, err := m.clients.EC2().DeleteNetworkInterface(context.TODO(), &ec2.DeleteNetworkInterfaceInput{
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
