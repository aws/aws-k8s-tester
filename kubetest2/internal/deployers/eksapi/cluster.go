package eksapi

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-k8s-tester/kubetest2/internal/util"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"k8s.io/klog/v2"
)

const (
	clusterCreationTimeout = time.Minute * 15
	clusterDeletionTimeout = time.Minute * 15
)

type cluster struct {
	endpoint                 string
	certificateAuthorityData string
	securityGroupId          string
	arn                      string
	name                     string
}

func createCluster(clients *awsClients, infra *infra, opts *deployerOptions, resourceID string) (*cluster, error) {
	input := eks.CreateClusterInput{
		Name: aws.String(resourceID),
		ResourcesVpcConfig: &ekstypes.VpcConfigRequest{
			EndpointPrivateAccess: aws.Bool(true),
			EndpointPublicAccess:  aws.Bool(true),
			SubnetIds:             append(infra.subnetsPublic, infra.subnetsPrivate...),
		},
		RoleArn: aws.String(infra.clusterRole),
		KubernetesNetworkConfig: &ekstypes.KubernetesNetworkConfigRequest{
			IpFamily: ekstypes.IpFamily(opts.IPFamily),
		},
		Version: aws.String(opts.KubernetesVersion),
	}
	apiOpts, err := util.NewHTTPHeaderAPIOptions(opts.UpClusterHeaders)
	if err != nil {
		return nil, fmt.Errorf("failed to create API options: %v", err)
	}
	klog.Infof("creating cluster...")
	createOutput, err := clients.EKS().CreateCluster(context.TODO(), &input,
		func(o *eks.Options) {
			o.APIOptions = apiOpts
		})
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster: %v", err)
	}
	klog.Infof("waiting for cluster to be active: %s", *createOutput.Cluster.Arn)
	err = eks.NewClusterActiveWaiter(clients.EKS()).
		Wait(context.TODO(), &eks.DescribeClusterInput{
			Name: createOutput.Cluster.Name,
		},
			clusterCreationTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for cluster to become active: %v", err)
	}
	klog.Infof("cluster is active: %s", *createOutput.Cluster.Arn)
	describeOutput, err := clients.EKS().DescribeCluster(context.TODO(), &eks.DescribeClusterInput{
		Name: createOutput.Cluster.Name,
	})
	if err != nil {
		return nil, err
	}
	return &cluster{
		arn:                      *describeOutput.Cluster.Arn,
		certificateAuthorityData: *describeOutput.Cluster.CertificateAuthority.Data,
		endpoint:                 *describeOutput.Cluster.Endpoint,
		name:                     *describeOutput.Cluster.Name,
		securityGroupId:          *describeOutput.Cluster.ResourcesVpcConfig.ClusterSecurityGroupId,
	}, nil
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
