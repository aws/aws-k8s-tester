package eksapi

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"k8s.io/klog/v2"
)

const (
	nodegroupCreationTimeout = time.Minute * 20
	nodegroupDeletionTimeout = time.Minute * 20
)

func createNodegroup(clients *awsClients, infra *infra, opts *deployerOptions, resourceID string) error {
	klog.Infof("creating nodegroup...")
	input := eks.CreateNodegroupInput{
		ClusterName:   aws.String(resourceID),
		NodegroupName: aws.String(resourceID),
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
	out, err := clients.EKS().CreateNodegroup(context.TODO(), &input)
	if err != nil {
		return err
	}
	klog.Infof("waiting for nodegroup to be active: %s", *out.Nodegroup.NodegroupArn)
	err = eks.NewNodegroupActiveWaiter(clients.EKS()).
		Wait(context.TODO(), &eks.DescribeNodegroupInput{
			ClusterName:   input.ClusterName,
			NodegroupName: input.NodegroupName,
		}, nodegroupCreationTimeout)
	if err != nil {
		return err
	}
	klog.Infof("nodegroup is active: %s", *out.Nodegroup.NodegroupArn)
	return nil
}

func deleteNodegroup(clients *awsClients, resourceID string) error {
	input := eks.DeleteNodegroupInput{
		ClusterName:   aws.String(resourceID),
		NodegroupName: aws.String(resourceID),
	}
	klog.Infof("deleting nodegroup...")
	out, err := clients.EKS().DeleteNodegroup(context.TODO(), &input)
	if err != nil {
		var notFound *ekstypes.ResourceNotFoundException
		if errors.As(err, &notFound) {
			klog.Infof("nodegroup does not exist: %s", resourceID)
			return nil
		}
		return fmt.Errorf("failed to delete nodegroup: %v", err)
	}
	klog.Infof("waiting for nodegroup deletion: %s", *out.Nodegroup.NodegroupArn)
	err = eks.NewNodegroupDeletedWaiter(clients.EKS()).
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
