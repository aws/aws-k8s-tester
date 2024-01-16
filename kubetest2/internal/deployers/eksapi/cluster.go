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

type ClusterManager struct {
	clients    *awsClients
	resourceID string
}

func NewClusterManager(clients *awsClients, resourceID string) *ClusterManager {
	return &ClusterManager{
		clients:    clients,
		resourceID: resourceID,
	}
}

type Cluster struct {
	endpoint                 string
	certificateAuthorityData string
	securityGroupId          string
	arn                      string
	name                     string
	cidr                     string
}

func (m *ClusterManager) createCluster(infra *Infrastructure, opts *deployerOptions) (*Cluster, error) {
	input := eks.CreateClusterInput{
		Name: aws.String(m.resourceID),
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
	createOutput, err := m.clients.EKS().CreateCluster(context.TODO(), &input,
		func(o *eks.Options) {
			o.APIOptions = apiOpts
		})
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster: %v", err)
	}
	describeInput := eks.DescribeClusterInput{
		Name: createOutput.Cluster.Name,
	}
	klog.Infof("waiting for cluster to be active: %s", *createOutput.Cluster.Arn)
	waitErr := eks.NewClusterActiveWaiter(m.clients.EKS()).Wait(context.TODO(), &describeInput, clusterCreationTimeout)
	describeOutput, describeErr := m.clients.EKS().DescribeCluster(context.TODO(), &describeInput)
	if describeErr != nil {
		return nil, fmt.Errorf("failed to describe cluster after creation: %v", describeErr)
	}
	klog.Infof("cluster details after creation: %+v", describeOutput.Cluster)
	if waitErr != nil {
		return nil, fmt.Errorf("failed to wait for cluster to become active: %v", waitErr)
	}
	klog.Infof("cluster is active: %s", *createOutput.Cluster.Arn)
	var cidr string
	switch describeOutput.Cluster.KubernetesNetworkConfig.IpFamily {
	case ekstypes.IpFamilyIpv4:
		cidr = *describeOutput.Cluster.KubernetesNetworkConfig.ServiceIpv4Cidr
	case ekstypes.IpFamilyIpv6:
		cidr = *describeOutput.Cluster.KubernetesNetworkConfig.ServiceIpv6Cidr
	default:
		return nil, fmt.Errorf("unknown cluster IP family: '%v'", describeOutput.Cluster.KubernetesNetworkConfig.IpFamily)
	}
	return &Cluster{
		arn:                      *describeOutput.Cluster.Arn,
		certificateAuthorityData: *describeOutput.Cluster.CertificateAuthority.Data,
		cidr:                     cidr,
		endpoint:                 *describeOutput.Cluster.Endpoint,
		name:                     *describeOutput.Cluster.Name,
		securityGroupId:          *describeOutput.Cluster.ResourcesVpcConfig.ClusterSecurityGroupId,
	}, nil
}

func (m *ClusterManager) isClusterActive() (bool, error) {
	result, err := m.clients.EKS().DescribeCluster(context.TODO(), &eks.DescribeClusterInput{
		Name: aws.String(m.resourceID),
	})
	if err != nil {
		return false, err
	}
	switch result.Cluster.Status {
	case ekstypes.ClusterStatusActive:
		return true, nil
	case ekstypes.ClusterStatusCreating:
		return false, nil
	default:
		return false, fmt.Errorf("cluster status is: %v", result.Cluster.Status)
	}
}

func (m *ClusterManager) deleteCluster() error {
	input := eks.DeleteClusterInput{
		Name: aws.String(m.resourceID),
	}
	klog.Infof("deleting cluster...")
	out, err := m.clients.EKS().DeleteCluster(context.TODO(), &input)
	if err != nil {
		var notFound *ekstypes.ResourceNotFoundException
		if errors.As(err, &notFound) {
			klog.Infof("cluster does not exist: %s", m.resourceID)
			return nil
		}
		return fmt.Errorf("failed to delete cluster: %v", err)
	}
	klog.Infof("waiting for cluster to be deleted: %s", *out.Cluster.Arn)
	err = eks.NewClusterDeletedWaiter(m.clients.EKS()).
		Wait(context.TODO(), &eks.DescribeClusterInput{
			Name: aws.String(m.resourceID),
		},
			clusterDeletionTimeout)
	if err != nil {
		return fmt.Errorf("failed to wait for cluster to be deleted: %v", err)
	}
	return nil
}
