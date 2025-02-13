package eksapi

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-k8s-tester/internal/util"
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

func (m *ClusterManager) getOrCreateCluster(infra *Infrastructure, opts *deployerOptions) (*Cluster, error) {
	targetClusterName := opts.StaticClusterName
	if targetClusterName == "" {
		klog.Infof("creating cluster...")
		input := eks.CreateClusterInput{
			Name: aws.String(m.resourceID),
			ResourcesVpcConfig: &ekstypes.VpcConfigRequest{
				EndpointPrivateAccess: aws.Bool(true),
				EndpointPublicAccess:  aws.Bool(true),
				SubnetIds:             append(infra.subnetsPublic, infra.subnetsPrivate...),
			},
			RoleArn: aws.String(infra.clusterRoleARN),
			KubernetesNetworkConfig: &ekstypes.KubernetesNetworkConfigRequest{
				IpFamily: ekstypes.IpFamily(opts.IPFamily),
			},
			Version: aws.String(opts.KubernetesVersion),
		}
		if opts.AutoMode {
			input.ComputeConfig = &ekstypes.ComputeConfigRequest{
				// we don't enable any of the default node pools, we'll create our own
				Enabled:     aws.Bool(true),
				NodeRoleArn: aws.String(infra.nodeRoleARN),
				// TODO: we can't currently enable managed compute without a default NodePool
				// the system NodePool is tainted for critical addons only, so will be ignored for our test workloads
				NodePools: []string{"system"},
			}
			input.StorageConfig = &ekstypes.StorageConfigRequest{
				BlockStorage: &ekstypes.BlockStorage{
					Enabled: aws.Bool(true),
				},
			}
			input.KubernetesNetworkConfig.ElasticLoadBalancing = &ekstypes.ElasticLoadBalancing{
				Enabled: aws.Bool(true),
			}
			input.AccessConfig = &ekstypes.CreateAccessConfigRequest{
				AuthenticationMode: ekstypes.AuthenticationModeApi,
			}
			input.BootstrapSelfManagedAddons = aws.Bool(false)
		}
		apiOpts, err := util.NewHTTPHeaderAPIOptions(opts.UpClusterHeaders)
		if err != nil {
			return nil, fmt.Errorf("failed to create API options: %v", err)
		}
		createOutput, err := m.clients.EKS().CreateCluster(context.TODO(), &input,
			func(o *eks.Options) {
				o.APIOptions = apiOpts
			})
		if err != nil {
			return nil, fmt.Errorf("failed to create cluster: %v", err)
		}
		targetClusterName = aws.ToString(createOutput.Cluster.Name)
	} else {
		klog.Infof("reusing existing static cluster %s", opts.StaticClusterName)
	}
	cluster, waitErr := m.waitForClusterActive(targetClusterName)
	if waitErr != nil {
		return nil, fmt.Errorf("failed to wait for cluster to become active: %v", waitErr)
	}
	return cluster, nil
}

func (m *ClusterManager) waitForClusterActive(clusterName string) (*Cluster, error) {
	klog.Infof("waiting for cluster to be active: %s", clusterName)
	out, err := eks.NewClusterActiveWaiter(m.clients.EKS()).WaitForOutput(context.TODO(), &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	}, clusterCreationTimeout)
	// log when possible, whether there was an error or not
	if out != nil {
		klog.Infof("cluster details: %+v", out.Cluster)
	}
	if err != nil {
		return nil, fmt.Errorf("failed waiting for cluster be active: %v", err)
	}
	klog.Infof("cluster is active: %s", *out.Cluster.Arn)
	var cidr string
	switch out.Cluster.KubernetesNetworkConfig.IpFamily {
	case ekstypes.IpFamilyIpv4:
		cidr = *out.Cluster.KubernetesNetworkConfig.ServiceIpv4Cidr
	case ekstypes.IpFamilyIpv6:
		cidr = *out.Cluster.KubernetesNetworkConfig.ServiceIpv6Cidr
	default:
		return nil, fmt.Errorf("unknown cluster IP family: '%v'", out.Cluster.KubernetesNetworkConfig.IpFamily)
	}
	return &Cluster{
		arn:                      *out.Cluster.Arn,
		certificateAuthorityData: *out.Cluster.CertificateAuthority.Data,
		cidr:                     cidr,
		endpoint:                 *out.Cluster.Endpoint,
		name:                     *out.Cluster.Name,
		securityGroupId:          *out.Cluster.ResourcesVpcConfig.ClusterSecurityGroupId,
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
