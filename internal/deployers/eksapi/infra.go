package eksapi

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"math"
	"path"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go/aws/arn"
	"k8s.io/klog"

	"github.com/aws/aws-k8s-tester/internal/deployers/eksapi/templates"
	"github.com/aws/aws-k8s-tester/internal/metrics"
)

const (
	infraStackCreationTimeout         = time.Minute * 15
	infraStackDeletionTimeout         = time.Minute * 30
	networkInterfaceDetachmentTimeout = time.Minute * 5
)

const (
	// the VPC CNI will always add this tag to ENI's that it creates
	vpcCNIENITagKey = "node.k8s.amazonaws.com/createdAt"

	// the IPAM controller will add this tag to the ENI's that it creates
	ipamControllerENITagKey = "eks:kubernetes-cni-node-name"
)

// this value is not currently configurable, the infra stack is hardcoded to create 2
const numInfraAZs = 2

// eksEndpointURLTag is the key for an optional tag on the infrastructure CloudFormation stack,
// which indicates which EKS environment is associated with the stack's resources.
// The tag is only added when --endpoint-url is passed to the deployer.
const eksEndpointURLTag = "eks-endpoint-url"

var (
	infraMetricNamespace     = path.Join(DeployerMetricNamespace, "infrastructure")
	infraStackDeletionFailed = &metrics.MetricSpec{
		Namespace: infraMetricNamespace,
		Metric:    "StackDeletionFailed",
		Unit:      cloudwatchtypes.StandardUnitCount,
	}
	infraLeakedENIs = &metrics.MetricSpec{
		Namespace: infraMetricNamespace,
		Metric:    "LeakedENIs",
		Unit:      cloudwatchtypes.StandardUnitCount,
	}
)

type InfrastructureManager struct {
	clients    *awsClients
	resourceID string
	metrics    metrics.MetricRegistry
}

func NewInfrastructureManager(clients *awsClients, resourceID string, metrics metrics.MetricRegistry) *InfrastructureManager {
	return &InfrastructureManager{
		clients:    clients,
		resourceID: resourceID,
		metrics:    metrics,
	}
}

type Infrastructure struct {
	availabilityZones []string
	vpc               string
	subnetsPublic     []string
	subnetsPrivate    []string
	clusterRoleARN    string
	nodeRoleARN       string
	nodeRoleName      string
}

func (i *Infrastructure) subnets() []string {
	return append(i.subnetsPublic, i.subnetsPrivate...)
}

func (m *InfrastructureManager) createInfrastructureStack(opts *deployerOptions) (*Infrastructure, error) {
	// TODO: create a subnet in every AZ
	// get two AZs for the subnets
	azs, err := m.clients.EC2().DescribeAvailabilityZones(context.TODO(), &ec2.DescribeAvailabilityZonesInput{})
	if err != nil {
		return nil, err
	}
	var subnetAzs []string
	if opts.CapacityReservation {
		subnetAzs, err = m.getAZsWithCapacity(opts)
		if err != nil {
			return nil, err
		}
	} else if len(opts.InstanceTypes) > 0 {
		azs, err := m.getRankedAZsForInstanceTypes(opts)
		if err != nil {
			return nil, err
		}
		if len(azs) == 0 {
			return nil, fmt.Errorf("no AZs support any of the provided instance types (%v)", opts.InstanceTypes)
		}
		subnetAzs = azs[0:int(math.Min(float64(len(azs)), numInfraAZs))]
	} else if len(opts.AvailabilityZones) > 0 {
		subnetAzs = opts.AvailabilityZones[0:int(math.Min(float64(len(opts.AvailabilityZones)), numInfraAZs))]
	}
	// make sure we always have the number of AZs used in the infra stack. can end up here if using
	// a single capacity reservation or the provided instance types are offered in fewer AZs
	for _, az := range azs.AvailabilityZones {
		if len(subnetAzs) == numInfraAZs {
			break
		}
		if !slices.Contains(subnetAzs, *az.ZoneName) {
			az := *az.ZoneName
			klog.Infof("padding infra stack with AZ: %v", az)
			subnetAzs = append(subnetAzs, az)
		}
	}
	klog.Infof("creating infrastructure stack with AZs: %v", subnetAzs)
	input := cloudformation.CreateStackInput{
		StackName:    aws.String(m.resourceID),
		TemplateBody: aws.String(templates.Infrastructure),
		Capabilities: []cloudformationtypes.Capability{cloudformationtypes.CapabilityCapabilityIam},
		Parameters: []cloudformationtypes.Parameter{
			{
				ParameterKey:   aws.String("ResourceId"),
				ParameterValue: aws.String(m.resourceID),
			},
			{
				ParameterKey:   aws.String("Subnet01AZ"),
				ParameterValue: aws.String(subnetAzs[0]),
			},
			{
				ParameterKey:   aws.String("Subnet02AZ"),
				ParameterValue: aws.String(subnetAzs[1]),
			},
			{
				ParameterKey:   aws.String("AutoMode"),
				ParameterValue: aws.String(fmt.Sprintf("%t", opts.AutoMode)),
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
	infra.availabilityZones = subnetAzs
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
		return nil, err
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
			arn, err := arn.Parse(value)
			if err != nil {
				return nil, fmt.Errorf("infrastructure stack ClusterRole output is not a valid ARN: '%s': %v", value, err)
			}
			infra.clusterRoleARN = arn.String()
		case "NodeRole":
			arn, err := arn.Parse(value)
			if err != nil {
				return nil, fmt.Errorf("infrastructure stack NodeRole output is not a valid ARN: '%s': %v", value, err)
			}
			infra.nodeRoleARN = arn.String()
			// Resource looks like 'role:/MyRole'
			resourceParts := strings.Split(arn.Resource, "/")
			infra.nodeRoleName = resourceParts[len(resourceParts)-1]
		}
	}
	return &infra, nil
}

func (m *InfrastructureManager) deleteInfrastructureStack() error {
	infra, err := m.getInfrastructureStackResources()
	if err != nil {
		var notFound *cloudformationtypes.StackNotFoundException
		if errors.As(err, &notFound) {
			klog.Infof("infrastructure stack does not exist: %s", m.resourceID)
			return nil
		}
		return err
	}
	if err := m.deleteLeakedInstanceProfiles(infra); err != nil {
		return err
	}
	input := cloudformation.DeleteStackInput{
		StackName: aws.String(m.resourceID),
	}
	klog.Infof("deleting infrastructure stack: %s", m.resourceID)
	_, err = m.clients.CFN().DeleteStack(context.TODO(), &input)
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
		// don't fail the overall test, the janitor can clean this up
		klog.Warningf("failed to wait for infrastructure stack deletion: %v", err)
		m.metrics.Record(infraStackDeletionFailed, 1, nil)
		return nil
	}
	klog.Infof("deleted infrastructure stack: %s", m.resourceID)
	return nil
}

// deleteLeakedIntanceProfiles deletes any instance profiles to which the node role is attached,
// because this will block node role deletion (and deletion of the infrastructure stack).
// For example, when --auto-mode is used, an instance profile will be created for us and won't be deleted automatically with the cluster.
func (m *InfrastructureManager) deleteLeakedInstanceProfiles(infra *Infrastructure) error {
	if infra.nodeRoleName == "" {
		// if the infra stack failed to create, it could end up in a weird state with no node role
		// we know there aren't any instance profiles in that case, so all good
		return nil
	}
	out, err := m.clients.IAM().ListInstanceProfilesForRole(context.TODO(), &iam.ListInstanceProfilesForRoleInput{
		RoleName: aws.String(infra.nodeRoleName),
	})
	if err != nil {
		var notFound *iamtypes.NoSuchEntityException
		if errors.As(err, &notFound) {
			return nil
		}
		return fmt.Errorf("failed to list instance profiles for role name: '%s': %v", infra.nodeRoleName, err)
	} else if len(out.InstanceProfiles) > 0 {
		var deletedInstanceProfiles []string
		for _, instanceProfile := range out.InstanceProfiles {
			_, err := m.clients.IAM().RemoveRoleFromInstanceProfile(context.TODO(), &iam.RemoveRoleFromInstanceProfileInput{
				RoleName:            aws.String(infra.nodeRoleName),
				InstanceProfileName: instanceProfile.InstanceProfileName,
			})
			if err != nil {
				var notFound *iamtypes.NoSuchEntityException
				if errors.As(err, &notFound) {
					klog.Infof("instance profile does not exist: %s", aws.ToString(instanceProfile.InstanceProfileName))
					continue
				}
				return fmt.Errorf("failed to remove node role %s from instance profile: %s: %v", infra.nodeRoleName, aws.ToString(instanceProfile.InstanceProfileName), err)
			}
			_, err = m.clients.IAM().DeleteInstanceProfile(context.TODO(), &iam.DeleteInstanceProfileInput{
				InstanceProfileName: instanceProfile.InstanceProfileName,
			})
			if err != nil {
				var notFound *iamtypes.NoSuchEntityException
				if errors.As(err, &notFound) {
					klog.Infof("instance profile does not exist: %s", aws.ToString(instanceProfile.InstanceProfileName))
					continue
				}
				return fmt.Errorf("failed to delete instance profile: %s: %v", aws.ToString(instanceProfile.InstanceProfileName), err)
			}
			deletedInstanceProfiles = append(deletedInstanceProfiles, aws.ToString(instanceProfile.InstanceProfileName))
		}
		klog.Infof("deleted %d leaked instance profile(s): %v", len(deletedInstanceProfiles), deletedInstanceProfiles)
	}
	return nil
}

// deleteLeakedENIs deletes Elastic Network Interfaces that may have been allocated (and left behind) by the VPC CNI.
// These leaked ENIs will prevent deletion of their associated subnets and security groups.
func (m *InfrastructureManager) deleteLeakedENIs() error {
	infra, err := m.getInfrastructureStackResources()
	if err != nil {
		var notFound *cloudformationtypes.StackNotFoundException
		if errors.As(err, &notFound) {
			return nil
		}
		return fmt.Errorf("failed to get infrastructure stack resources: %w", err)
	}
	enis, err := m.getVPCCNINetworkInterfaceIds(infra.vpc)
	if err != nil {
		return err
	}
	if len(enis) == 0 {
		return nil
	}
	klog.Infof("waiting for %d leaked ENI(s) to become available: %v", len(enis), enis)
	if err := ec2.NewNetworkInterfaceAvailableWaiter(m.clients.EC2()).Wait(context.TODO(), &ec2.DescribeNetworkInterfacesInput{
		NetworkInterfaceIds: enis,
	}, networkInterfaceDetachmentTimeout); err != nil {
		return fmt.Errorf("failed to wait for ENI(s) to become available: %v", err)
	}
	for _, eni := range enis {
		klog.Infof("deleting leaked ENI: %s", eni)
		_, err := m.clients.EC2().DeleteNetworkInterface(context.TODO(), &ec2.DeleteNetworkInterfaceInput{
			NetworkInterfaceId: aws.String(eni),
		})
		if err != nil {
			return fmt.Errorf("failed to delete leaked ENI: %w", err)
		}
	}
	klog.Infof("deleted %d leaked ENI(s)!", len(enis))
	m.metrics.Record(infraLeakedENIs, float64(len(enis)), nil)
	return nil
}

// getVPCCNINetworkInterfaceIds returns the IDs of ENIs in the specified VPC that were created by the VPC CNI
func (m *InfrastructureManager) getVPCCNINetworkInterfaceIds(vpcId string) ([]string, error) {
	paginator := ec2.NewDescribeNetworkInterfacesPaginator(m.clients.EC2(), &ec2.DescribeNetworkInterfacesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcId},
			},
			{
				Name:   aws.String("interface-type"),
				Values: []string{"interface"},
			},
			{
				Name:   aws.String("tag-key"),
				Values: []string{vpcCNIENITagKey, ipamControllerENITagKey},
			},
		},
	})
	var enis []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("failed to describe ENIs: %w", err)
		}
		for _, eni := range page.NetworkInterfaces {
			enis = append(enis, *eni.NetworkInterfaceId)
		}
	}
	return enis, nil
}

// getAZsWithInstanceTypes returns the availability zones ordered decreasingly by the number of
// requested instance types they support
func (m *InfrastructureManager) getRankedAZsForInstanceTypes(opts *deployerOptions) ([]string, error) {
	instanceTypeOfferingsFilter := []ec2types.Filter{
		{
			Name:   aws.String("instance-type"),
			Values: opts.InstanceTypes,
		},
	}
	if len(opts.AvailabilityZones) > 0 {
		instanceTypeOfferingsFilter = append(instanceTypeOfferingsFilter, ec2types.Filter{
			Name:   aws.String("location"),
			Values: opts.AvailabilityZones,
		})
	}
	offerings, err := m.clients.EC2().DescribeInstanceTypeOfferings(context.TODO(), &ec2.DescribeInstanceTypeOfferingsInput{
		LocationType: ec2types.LocationTypeAvailabilityZone,
		Filters:      instanceTypeOfferingsFilter,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance type offerings: %v", err)
	}
	counts := make(map[string]int)
	for _, offering := range offerings.InstanceTypeOfferings {
		az := aws.ToString(offering.Location)
		count := counts[az]
		counts[az] = count + 1
	}
	var azs []string
	for az := range counts {
		azs = append(azs, az)
	}
	sort.Slice(azs, func(i, j int) bool {
		return counts[azs[i]] > counts[azs[j]]
	})
	return azs, nil
}

func (m *InfrastructureManager) getAZsWithCapacity(opts *deployerOptions) ([]string, error) {
	var subnetAzs []string
	capacityReservationFilter := []ec2types.Filter{
		{
			Name:   aws.String("instance-type"),
			Values: opts.InstanceTypes,
		},
		{
			Name:   aws.String("state"),
			Values: []string{"active"},
		},
	}
	if len(opts.AvailabilityZones) > 0 {
		capacityReservationFilter = append(capacityReservationFilter, ec2types.Filter{
			Name:   aws.String("availability-zone"),
			Values: opts.AvailabilityZones,
		})
	}
	capacityReservations, err := m.clients.EC2().DescribeCapacityReservations(context.TODO(), &ec2.DescribeCapacityReservationsInput{
		Filters: capacityReservationFilter,
	})
	if err != nil {
		return nil, err
	}
	for _, cr := range capacityReservations.CapacityReservations {
		if *cr.AvailableInstanceCount >= int32(opts.Nodes) {
			subnetAzs = append(subnetAzs, *cr.AvailabilityZone)
			break
		}
	}
	return subnetAzs, nil
}
