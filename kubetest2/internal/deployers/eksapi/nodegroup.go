package eksapi

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"k8s.io/klog/v2"

	"github.com/aws/aws-k8s-tester/kubetest2/internal/deployers/eksapi/templates"
)

const (
	nodegroupCreationTimeout = time.Minute * 20
	nodegroupDeletionTimeout = time.Minute * 20
)

type NodegroupManager struct {
	clients    *awsClients
	resourceID string
}

func NewNodegroupManager(clients *awsClients, resourceID string) *NodegroupManager {
	return &NodegroupManager{
		clients:    clients,
		resourceID: resourceID,
	}
}

func (m *NodegroupManager) createNodegroup(infra *Infrastructure, cluster *Cluster, opts *deployerOptions) error {
	if opts.UnmanagedNodes {
		if opts.EFA {
			return m.createUnmanagedNodegroupWithEFA(infra, cluster, opts)
		}
		return m.createUnmanagedNodegroup(infra, cluster, opts)
	} else {
		return m.createManagedNodegroup(infra, cluster, opts)
	}
}

func (m *NodegroupManager) createManagedNodegroup(infra *Infrastructure, cluster *Cluster, opts *deployerOptions) error {
	klog.Infof("creating nodegroup...")
	input := eks.CreateNodegroupInput{
		ClusterName:   aws.String(m.resourceID),
		NodegroupName: aws.String(m.resourceID),
		NodeRole:      aws.String(infra.nodeRole),
		Subnets:       infra.subnets(),
		DiskSize:      aws.Int32(100),
		CapacityType:  ekstypes.CapacityTypesOnDemand,
		ScalingConfig: &ekstypes.NodegroupScalingConfig{
			MinSize:     aws.Int32(int32(opts.Nodes)),
			MaxSize:     aws.Int32(int32(opts.Nodes)),
			DesiredSize: aws.Int32(int32(opts.Nodes)),
		},
	}
	if len(opts.InstanceTypes) > 0 {
		input.InstanceTypes = opts.InstanceTypes
	}
	out, err := m.clients.EKS().CreateNodegroup(context.TODO(), &input)
	if err != nil {
		return err
	}
	klog.Infof("waiting for nodegroup to be active: %s", *out.Nodegroup.NodegroupArn)
	err = eks.NewNodegroupActiveWaiter(m.clients.EKS()).
		Wait(context.TODO(), &eks.DescribeNodegroupInput{
			ClusterName:   input.ClusterName,
			NodegroupName: input.NodegroupName,
		}, nodegroupCreationTimeout)
	if err != nil {
		return err
	}
	klog.Infof("nodegroup is active: %s", *out.Nodegroup.NodegroupArn)
	if opts.ExpectedAMI != "" {
		out, err := m.clients.EKS().DescribeNodegroup(context.TODO(), &eks.DescribeNodegroupInput{
			ClusterName:   input.ClusterName,
			NodegroupName: input.NodegroupName,
		})
		if err != nil {
			return err
		}
		asgName := out.Nodegroup.Resources.AutoScalingGroups[0].Name
		if ok, err := m.verifyASGAMI(*asgName, opts.ExpectedAMI); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("ASG %s is not using expected AMI: %s", *asgName, opts.ExpectedAMI)
		}
	}
	return nil
}

func (m *NodegroupManager) createUnmanagedNodegroup(infra *Infrastructure, cluster *Cluster, opts *deployerOptions) error {
	stackName := m.getUnmanagedNodegroupStackName()
	klog.Infof("creating unmanaged nodegroup stack...")
	userData, err := generateUserData(opts.UserDataFormat, cluster)
	if err != nil {
		return err
	}
	templateBuf := bytes.Buffer{}
	err = templates.UnmanagedNodegroup.Execute(&templateBuf, struct {
		InstanceTypes     []string
		KubernetesVersion string
	}{
		InstanceTypes:     opts.InstanceTypes,
		KubernetesVersion: opts.KubernetesVersion,
	})
	if err != nil {
		return err
	}
	// pull the role name out of the ARN
	nodeRoleArnParts := strings.Split(infra.nodeRole, "/")
	nodeRoleName := nodeRoleArnParts[len(nodeRoleArnParts)-1]
	input := cloudformation.CreateStackInput{
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(templateBuf.String()),
		Capabilities: []cloudformationtypes.Capability{cloudformationtypes.CapabilityCapabilityIam},
		Parameters: []cloudformationtypes.Parameter{
			{
				ParameterKey:   aws.String("ResourceId"),
				ParameterValue: aws.String(m.resourceID),
			},
			{
				ParameterKey:   aws.String("VpcId"),
				ParameterValue: aws.String(infra.vpc),
			},
			{
				ParameterKey:   aws.String("SubnetIds"),
				ParameterValue: aws.String(strings.Join(infra.subnets(), ",")),
			},
			{
				ParameterKey:   aws.String("UserData"),
				ParameterValue: aws.String(userData),
			},
			{
				ParameterKey:   aws.String("ClusterName"),
				ParameterValue: aws.String(cluster.name),
			},
			{
				ParameterKey:   aws.String("NodeRoleName"),
				ParameterValue: aws.String(nodeRoleName),
			},
			{
				ParameterKey:   aws.String("NodeCount"),
				ParameterValue: aws.String(strconv.Itoa(opts.Nodes)),
			},
			{
				ParameterKey:   aws.String("SecurityGroup"),
				ParameterValue: aws.String(cluster.securityGroupId),
			},
			{
				ParameterKey:   aws.String("SSHSecurityGroup"),
				ParameterValue: aws.String(infra.sshSecurityGroup),
			},
			{
				ParameterKey:   aws.String("SSHKeyPair"),
				ParameterValue: aws.String(infra.sshKeyPair),
			},
			{
				ParameterKey:   aws.String("AMIId"),
				ParameterValue: aws.String(opts.AMI),
			},
		},
	}
	out, err := m.clients.CFN().CreateStack(context.TODO(), &input)
	if err != nil {
		return err
	}
	klog.Infof("waiting for unmanaged nodegroup to be created: %s", *out.StackId)
	err = cloudformation.NewStackCreateCompleteWaiter(m.clients.CFN()).
		Wait(context.TODO(),
			&cloudformation.DescribeStacksInput{
				StackName: out.StackId,
			},
			infraStackCreationTimeout)
	if err != nil {
		return fmt.Errorf("failed to wait for unmanaged nodegroup stack creation: %w", err)
	}
	klog.Infof("created unmanaged nodegroup stack: %s", *out.StackId)
	if opts.ExpectedAMI != "" {
		if ok, err := m.verifyASGAMI(m.resourceID, opts.ExpectedAMI); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("ASG %s is not using expected AMI: %s", m.resourceID, opts.ExpectedAMI)
		}
	}
	return nil
}

func (m *NodegroupManager) createUnmanagedNodegroupWithEFA(infra *Infrastructure, cluster *Cluster, opts *deployerOptions) error {
	stackName := m.getUnmanagedNodegroupStackName()
	klog.Infof("creating unmanaged nodegroup with EFA stack...")
	userData, err := generateUserData(opts.UserDataFormat, cluster)
	if err != nil {
		return err
	}
	// pull the role name out of the ARN
	nodeRoleArnParts := strings.Split(infra.nodeRole, "/")
	nodeRoleName := nodeRoleArnParts[len(nodeRoleArnParts)-1]
	input := cloudformation.CreateStackInput{
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(templates.UnmanagedNodegroupEFA),
		Capabilities: []cloudformationtypes.Capability{cloudformationtypes.CapabilityCapabilityIam},
		Parameters: []cloudformationtypes.Parameter{
			{
				ParameterKey:   aws.String("ResourceId"),
				ParameterValue: aws.String(m.resourceID),
			},
			{
				ParameterKey:   aws.String("VpcId"),
				ParameterValue: aws.String(infra.vpc),
			},
			{
				ParameterKey:   aws.String("SubnetIds"),
				ParameterValue: aws.String(infra.subnetsPrivate[0]), // this is load bearing! EFA requires a private subnet
			},
			{
				ParameterKey:   aws.String("UserData"),
				ParameterValue: aws.String(userData),
			},
			{
				ParameterKey:   aws.String("ClusterName"),
				ParameterValue: aws.String(cluster.name),
			},
			{
				ParameterKey:   aws.String("NodeRoleName"),
				ParameterValue: aws.String(nodeRoleName),
			},
			{
				ParameterKey:   aws.String("NodeCount"),
				ParameterValue: aws.String(strconv.Itoa(opts.Nodes)),
			},
			{
				ParameterKey:   aws.String("SecurityGroup"),
				ParameterValue: aws.String(cluster.securityGroupId),
			},
			{
				ParameterKey:   aws.String("SSHKeyPair"),
				ParameterValue: aws.String(infra.sshKeyPair),
			},
			{
				ParameterKey:   aws.String("AMIId"),
				ParameterValue: aws.String(opts.AMI),
			},
			{
				ParameterKey:   aws.String("InstanceType"),
				ParameterValue: aws.String(opts.InstanceTypes[0]),
			},
		},
	}
	out, err := m.clients.CFN().CreateStack(context.TODO(), &input)
	if err != nil {
		return err
	}
	klog.Infof("waiting for unmanaged nodegroup with EFA to be created: %s", *out.StackId)
	err = cloudformation.NewStackCreateCompleteWaiter(m.clients.CFN()).
		Wait(context.TODO(),
			&cloudformation.DescribeStacksInput{
				StackName: out.StackId,
			},
			infraStackCreationTimeout)
	if err != nil {
		return fmt.Errorf("failed to wait for unmanaged nodegroup stack creation: %w", err)
	}
	klog.Infof("created unmanaged nodegroup with EFA stack: %s", *out.StackId)
	if opts.ExpectedAMI != "" {
		if ok, err := m.verifyASGAMI(m.resourceID, opts.ExpectedAMI); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("ASG %s is not using expected AMI: %s", m.resourceID, opts.ExpectedAMI)
		}
	}
	return nil
}

func (m *NodegroupManager) deleteNodegroup() error {
	if err := m.deleteUnmanagedNodegroup(); err != nil {
		return err
	}
	return m.deleteManagedNodegroup()
}

func (m *NodegroupManager) deleteManagedNodegroup() error {
	input := eks.DeleteNodegroupInput{
		ClusterName:   aws.String(m.resourceID),
		NodegroupName: aws.String(m.resourceID),
	}
	klog.Infof("deleting nodegroup...")
	out, err := m.clients.EKS().DeleteNodegroup(context.TODO(), &input)
	if err != nil {
		var notFound *ekstypes.ResourceNotFoundException
		if errors.As(err, &notFound) {
			klog.Infof("nodegroup does not exist: %s", m.resourceID)
			return nil
		}
		return fmt.Errorf("failed to delete nodegroup: %v", err)
	}
	klog.Infof("waiting for nodegroup deletion: %s", *out.Nodegroup.NodegroupArn)
	err = eks.NewNodegroupDeletedWaiter(m.clients.EKS()).
		Wait(context.TODO(), &eks.DescribeNodegroupInput{
			ClusterName:   input.ClusterName,
			NodegroupName: input.NodegroupName,
		}, nodegroupDeletionTimeout)
	if err != nil {
		return fmt.Errorf("failed to wait for nodegroup deletion: %v", err)
	}
	klog.Infof("nodegroup deleted: %s", *out.Nodegroup.NodegroupArn)
	return nil
}

func (m *NodegroupManager) deleteUnmanagedNodegroup() error {
	stackName := m.getUnmanagedNodegroupStackName()
	input := cloudformation.DeleteStackInput{
		StackName: aws.String(stackName),
	}
	klog.Infof("deleting unmanaged nodegroup stack: %s", stackName)
	_, err := m.clients.CFN().DeleteStack(context.TODO(), &input)
	if err != nil {
		var notFound *cloudformationtypes.StackNotFoundException
		if errors.As(err, &notFound) {
			klog.Infof("unmanaged nodegroup stack does not exist: %s", stackName)
			return nil
		}
		return fmt.Errorf("failed to delete unmanaged nodegroup stack: %w", err)
	}
	klog.Infof("waiting for unmanaged nodegroup stack to be deleted: %s", stackName)
	err = cloudformation.NewStackDeleteCompleteWaiter(m.clients.CFN()).
		Wait(context.TODO(),
			&cloudformation.DescribeStacksInput{
				StackName: aws.String(stackName),
			},
			infraStackDeletionTimeout)
	if err != nil {
		return fmt.Errorf("failed to wait for unmanaged nodegroup stack deletion: %w", err)
	}
	klog.Infof("deleted unmanaged nodegroup stack: %s", stackName)
	return nil
}

func (m *NodegroupManager) getUnmanagedNodegroupStackName() string {
	return fmt.Sprintf("%s-unmanaged-nodegroup", m.resourceID)
}

func (m *NodegroupManager) verifyASGAMI(asgName string, amiId string) (bool, error) {
	klog.Infof("verifying AMI is %s for ASG: %s", amiId, asgName)
	asgOut, err := m.clients.ASG().DescribeAutoScalingGroups(context.TODO(), &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []string{asgName},
	})
	if err != nil {
		return false, nil
	}
	if len(asgOut.AutoScalingGroups) != 1 {
		return false, fmt.Errorf("autoscaling group not found: %s", asgName)
	}
	var instanceIds []string
	for _, instance := range asgOut.AutoScalingGroups[0].Instances {
		instanceIds = append(instanceIds, *instance.InstanceId)
	}
	klog.Infof("verifying AMI for instances: %v", instanceIds)
	ec2Out, err := m.clients.EC2().DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
		InstanceIds: instanceIds,
	})
	if err != nil {
		return false, err
	}
	var errs []error
	for _, reservation := range ec2Out.Reservations {
		for _, instance := range reservation.Instances {
			if *instance.ImageId != amiId {
				errs = append(errs, fmt.Errorf("instance %s using wrong AMI: %s", *instance.InstanceId, *instance.ImageId))
			}
		}
	}
	if len(errs) > 0 {
		return false, errors.Join(errs...)
	}
	klog.Infof("ASG instances are using expected AMI: %s", amiId)
	return true, nil
}
