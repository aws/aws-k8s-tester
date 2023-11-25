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
	clusterCreationTimeout = time.Minute * 15
	clusterDeletionTimeout = time.Minute * 15
)

func createCluster(clients *awsClients, infra *infra, opts *deployerOptions, resourceID string) error {
	input := eks.CreateClusterInput{
		Name: aws.String(resourceID),
		ResourcesVpcConfig: &ekstypes.VpcConfigRequest{
			EndpointPrivateAccess: aws.Bool(true),
			EndpointPublicAccess:  aws.Bool(true),
			SecurityGroupIds:      infra.securityGroups,
			SubnetIds:             append(infra.subnetsPublic, infra.subnetsPrivate...),
		},
		RoleArn: aws.String(infra.clusterRole),
		KubernetesNetworkConfig: &ekstypes.KubernetesNetworkConfigRequest{
			IpFamily: ekstypes.IpFamilyIpv4,
		},
		Version: aws.String(opts.KubernetesVersion),
	}
	klog.Infof("creating cluster...")
	out, err := clients.EKS().CreateCluster(context.TODO(), &input)
	if err != nil {
		return fmt.Errorf("failed to create cluster: %v", err)
	}
	klog.Infof("waiting for cluster to be active: %s", *out.Cluster.Arn)
	err = eks.NewClusterActiveWaiter(clients.EKS()).
		Wait(context.TODO(), &eks.DescribeClusterInput{
			Name: out.Cluster.Name,
		},
			clusterCreationTimeout)
	if err != nil {
		return fmt.Errorf("failed to wait for cluster to become active: %v", err)
	}
	klog.Infof("cluster is active: %s", *out.Cluster.Arn)
	return nil
}

func deleteCluster(clients *awsClients, resourceID string) error {
	input := eks.DeleteClusterInput{
		Name: aws.String(resourceID),
	}
	klog.Infof("deleting cluster...")
	out, err := clients.EKS().DeleteCluster(context.TODO(), &input)
	if err != nil {
		var notFound *ekstypes.ResourceNotFoundException
		if errors.As(err, &notFound) {
			klog.Infof("cluster does not exist: %s", resourceID)
			return nil
		}
		return fmt.Errorf("failed to delete cluster: %v", err)
	}
	klog.Infof("waiting for cluster to be deleted: %s", *out.Cluster.Arn)
	err = eks.NewClusterDeletedWaiter(clients.EKS()).
		Wait(context.TODO(), &eks.DescribeClusterInput{
			Name: aws.String(resourceID),
		},
			clusterDeletionTimeout)
	if err != nil {
		return fmt.Errorf("failed to wait for cluster to be deleted: %v", err)
	}
	return nil
}
