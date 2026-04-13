package eksapi

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
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
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go/aws/arn"

	"github.com/aws/aws-k8s-tester/internal/deployers/eksapi/templates"
	"github.com/aws/aws-k8s-tester/internal/metrics"
	"github.com/aws/aws-k8s-tester/internal/util"
)

const (
	infraStackCreationTimeout         = time.Minute * 15
	infraStackDeletionTimeout         = time.Minute * 30
	networkInterfaceDetachmentTimeout = time.Minute * 10
)

const (
	// the VPC CNI will always add this tag to ENI's that it creates
	vpcCNIENITagKey         = "node.k8s.amazonaws.com/createdAt"
	// the IPAM controller will add this tag to the ENI's that it creates
	ipamControllerENITagKey = "eks:kubernetes-cni-node-name"
)

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
	cloudwatchRoleArn string
}

func (i *Infrastructure) subnets() []string {
	return append(i.subnetsPublic, i.subnetsPrivate...)
}

func (m *InfrastructureManager) createInfrastructureStack(opts *deployerOptions) (*Infrastructure, error) {
	var subnetAzs []string
	if opts.CapacityReservation {
		azs, err := m.getAZsWithCapacity(opts)
		if err != nil {
			return nil, err
		}
		subnetAzs = azs
	} else if len(opts.InstanceTypes) > 0 {
		azs, err := m.getRankedAZsForInstanceTypes(opts)
		if err != nil {
			return nil, err
		}
		if len(azs) == 0 {
			return nil, fmt.Errorf("no AZs support any of the provided instance types (%v)", opts.InstanceTypes)
		}
		subnetAzs = azs
	}

	// this value is not currently configurable, the infra stack is hardcoded to create 2
	// TODO: create a subnet in every AZ. today we need exactly 2 AZs for the subnets.
	const numInfraAZs = 2

	subnetAzs, err := m.normalizeAZs(opts, subnetAzs, numInfraAZs)
	if err != nil {
		return nil, err
	}

	slog.Info("creating infrastructure stack", "availabilityZones", subnetAzs)
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
	slog.Info("creating infrastructure stack...")
	out, err := m.clients.CFN().CreateStack(context.TODO(), &input)
	if err != nil {
		return nil, err
	}
	slog.Info("waiting for infrastructure stack to be created", "stackId", *out.StackId)
	err = cloudformation.NewStackCreateCompleteWaiter(m.clients.CFN()).
		Wait(context.TODO(),
			&cloudformation.DescribeStacksInput{
				StackName: out.StackId,
			},
			infraStackCreationTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for infrastructure stack creation: %w", err)
	}
	slog.Info("getting infrastructure stack resources", "stackId", *out.StackId)
	infra, err := m.getInfrastructureStackResources()
	infra.availabilityZones = subnetAzs
	if err != nil {
		return nil, fmt.Errorf("failed to get infrastructure stack resources: %w", err)
	}
	slog.Info("created infrastructure", "infra", infra)

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
			slog.Info("infrastructure stack does not exist", "resourceID", m.resourceID)
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
	slog.Info("deleting infrastructure stack", "resourceID", m.resourceID)
	_, err = m.clients.CFN().DeleteStack(context.TODO(), &input)
	if err != nil {
		var notFound *cloudformationtypes.StackNotFoundException
		if errors.As(err, &notFound) {
			slog.Info("infrastructure stack does not exist", "resourceID", m.resourceID)
			return nil
		}
		return fmt.Errorf("failed to delete infrastructure stack: %w", err)
	}
	slog.Info("waiting for infrastructure stack to be deleted", "resourceID", m.resourceID)
	err = cloudformation.NewStackDeleteCompleteWaiter(m.clients.CFN()).
		Wait(context.TODO(),
			&cloudformation.DescribeStacksInput{
				StackName: aws.String(m.resourceID),
			},
			infraStackDeletionTimeout)
	if err != nil {
		// don't fail the overall test, the janitor can clean this up
		slog.Warn("failed to wait for infrastructure stack deletion", "error", err)
		m.metrics.Record(infraStackDeletionFailed, 1, nil)
		return nil
	}
	slog.Info("deleted infrastructure stack", "resourceID", m.resourceID)
	return nil
}

// deleteLeakedInstanceProfiles deletes any instance profiles to which the node role is attached,
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
					slog.Info("instance profile does not exist", "name", aws.ToString(instanceProfile.InstanceProfileName))
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
					slog.Info("instance profile does not exist", "name", aws.ToString(instanceProfile.InstanceProfileName))
					continue
				}
				return fmt.Errorf("failed to delete instance profile: %s: %v", aws.ToString(instanceProfile.InstanceProfileName), err)
			}
			deletedInstanceProfiles = append(deletedInstanceProfiles, aws.ToString(instanceProfile.InstanceProfileName))
		}
		slog.Info("deleted leaked instance profiles", "count", len(deletedInstanceProfiles), "profiles", deletedInstanceProfiles)
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
	slog.Info("waiting for leaked ENIs to become available", "count", len(enis), "enis", enis)
	if err := ec2.NewNetworkInterfaceAvailableWaiter(m.clients.EC2()).Wait(context.TODO(), &ec2.DescribeNetworkInterfacesInput{
		NetworkInterfaceIds: enis,
	}, networkInterfaceDetachmentTimeout); err != nil {
		refreshedENIs, err2 := m.getVPCCNINetworkInterfaceIds(infra.vpc)
		if err2 != nil {
			return fmt.Errorf("waiter failed, and re-checking ENIs also failed: %w", err2)
		}
		if len(refreshedENIs) == 0 {
			slog.Info("ENIs were deleted during waiter timeout, skipping delete")
			return nil
		}
		return fmt.Errorf("failed to wait for ENI(s) to become available: %v", err)
	}
	for _, eni := range enis {
		slog.Info("deleting leaked ENI", "eni", eni)
		_, err := m.clients.EC2().DeleteNetworkInterface(context.TODO(), &ec2.DeleteNetworkInterfaceInput{
			NetworkInterfaceId: aws.String(eni),
		})
		if err != nil {
			return fmt.Errorf("failed to delete leaked ENI: %w", err)
		}
	}
	slog.Info("deleted leaked ENIs", "count", len(enis))
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

// normalizeAZs removes availability zones that don't meet launch requirements
// for instances and ensures that the resulting list containers enough AZs to
// satisfy the deployment.
func (m *InfrastructureManager) normalizeAZs(opts *deployerOptions, subnetAZs []string, expectedCount int) ([]string, error) {
	azs, err := m.clients.EC2().DescribeAvailabilityZones(context.TODO(), &ec2.DescribeAvailabilityZonesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("zone-type"),
				Values: []string{opts.ZoneType},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	var supporttedAZs []string
	for _, az := range azs.AvailabilityZones {
		supporttedAZs = append(supporttedAZs, aws.ToString(az.ZoneName))
	}

	var filteredAZs []string
	for _, az := range subnetAZs {
		if slices.Contains(supporttedAZs, az) {
			filteredAZs = append(filteredAZs, az)
		}
	}

	// enforce users' preferred ordering over AZs
	filteredAZs = availabilityZoneHintedOrder(filteredAZs)
	// truncate the list if we went over the max
	filteredAZs = filteredAZs[:min(len(filteredAZs), expectedCount)]

	// pad the availability zones with supported entries if we end up not having
	// enough after filtering.
	if len(filteredAZs) < expectedCount {
		for _, az := range supporttedAZs {
			if len(filteredAZs) == expectedCount {
				break
			}
			if !slices.Contains(filteredAZs, az) {
				slog.Info("padding infra stack with AZ", "az", az)
				filteredAZs = append(filteredAZs, az)
			}
		}
	}

	if len(filteredAZs) != expectedCount {
		return nil, fmt.Errorf("failed to provide AZs with expected count %d: %v", expectedCount, filteredAZs)
	}

	return filteredAZs, nil
}

// getAZsWithInstanceTypes returns the availability zones ordered decreasingly by the number of
// requested instance types they support
func (m *InfrastructureManager) getRankedAZsForInstanceTypes(opts *deployerOptions) ([]string, error) {
	offerings, err := m.clients.EC2().DescribeInstanceTypeOfferings(context.TODO(), &ec2.DescribeInstanceTypeOfferingsInput{
		LocationType: ec2types.LocationTypeAvailabilityZone,
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("instance-type"),
				Values: opts.InstanceTypes,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance type offerings: %v", err)
	}
	counts := make(map[string]int)
	for _, offering := range offerings.InstanceTypeOfferings {
		counts[aws.ToString(offering.Location)]++
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
	// TODO: consolidate this with the CapacityReservation logic in node.go
	var subnetAzs []string
	describeReservationsInput := ec2.DescribeCapacityReservationsInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("instance-type"),
				Values: opts.InstanceTypes,
			},
			{
				Name:   aws.String("state"),
				Values: []string{"active"},
			},
		},
	}
	if opts.TargetCapacityReservationId != "" {
		describeReservationsInput.CapacityReservationIds = []string{opts.TargetCapacityReservationId}
	}
	capacityReservations, err := m.clients.EC2().DescribeCapacityReservations(context.TODO(), &describeReservationsInput)
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

func getCloudWatchStackName(resourceID string) (string, string) {
	clusterUUID := strings.TrimPrefix(resourceID, ResourcePrefix+"-")
	return fmt.Sprintf("%s-cw", resourceID), clusterUUID
}

func (m *InfrastructureManager) createCloudWatchInfrastructureStack(clusterName string) (string, error) {
	stackName, clusterUUID := getCloudWatchStackName(clusterName)
	slog.Info("creating CloudWatch infrastructure stack", "stackName", stackName)
	out, err := m.clients.CFN().CreateStack(context.TODO(), &cloudformation.CreateStackInput{
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(templates.CloudWatchInfra),
		Capabilities: []cloudformationtypes.Capability{cloudformationtypes.CapabilityCapabilityNamedIam},
		Parameters: []cloudformationtypes.Parameter{
			{
				ParameterKey:   aws.String("ClusterUUID"),
				ParameterValue: aws.String(clusterUUID),
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create CloudWatch infrastructure stack: %w", err)
	}

	slog.Info("waiting for CloudWatch infrastructure stack to be created", "stackId", *out.StackId)
	if err := cloudformation.NewStackCreateCompleteWaiter(m.clients.CFN()).
		Wait(context.TODO(),
			&cloudformation.DescribeStacksInput{
				StackName: out.StackId,
			},
			infraStackCreationTimeout); err != nil {
		return "", util.WrapCFNStackFailure(context.TODO(), m.clients.CFN(), fmt.Errorf("failed to wait for CloudWatch infrastructure stack creation: %w", err), stackName)
	}

	stack, err := m.clients.CFN().DescribeStacks(context.TODO(), &cloudformation.DescribeStacksInput{
		StackName: out.StackId,
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe CloudWatch infrastructure stack: %w", err)
	}

	// Get the CloudWatch role ARN from stack outputs
	var roleArn string
	for _, output := range stack.Stacks[0].Outputs {
		if aws.ToString(output.OutputKey) == "CloudWatchRoleArn" {
			roleArn = aws.ToString(output.OutputValue)
			break
		}
	}
	if roleArn == "" {
		return "", fmt.Errorf("CloudWatch role ARN not found in stack outputs")
	}

	slog.Info("CloudWatch infrastructure stack created successfully", "roleArn", roleArn)
	return roleArn, nil
}

// createCloudWatchPodIdentityAssociation creates a PodIdentityAssociation
// via the EKS API directly, rather than through CloudFormation, to ensure
// the correct EKS endpoint is used when a custom endpoint URL is configured.
// The association is automatically cleaned up when the cluster is deleted.
func (m *InfrastructureManager) createCloudWatchPodIdentityAssociation(clusterName string, roleArn string) error {
	slog.Info("creating PodIdentityAssociation", "clusterName", clusterName)
	_, err := m.clients.EKS().CreatePodIdentityAssociation(context.TODO(), &eks.CreatePodIdentityAssociationInput{
		ClusterName:    aws.String(clusterName),
		Namespace:      aws.String("amazon-cloudwatch"),
		ServiceAccount: aws.String("cwagent"),
		RoleArn:        aws.String(roleArn),
	})
	if err != nil {
		return fmt.Errorf("failed to create PodIdentityAssociation: %w", err)
	}
	slog.Info("PodIdentityAssociation created successfully", "clusterName", clusterName)
	return nil
}

func (m *InfrastructureManager) deleteCloudWatchInfrastructureStack() error {
	stackName, _ := getCloudWatchStackName(m.resourceID)

	// The PodIdentityAssociation created via the EKS API is automatically
	// cleaned up when the cluster is deleted, so no explicit deletion is needed.

	slog.Info("deleting CloudWatch infrastructure stack", "stackName", stackName)
	if _, err := m.clients.CFN().DeleteStack(context.TODO(), &cloudformation.DeleteStackInput{
		StackName: aws.String(stackName),
	}); err != nil {
		var notFound *cloudformationtypes.StackNotFoundException
		if errors.As(err, &notFound) {
			slog.Info("CloudWatch infrastructure stack does not exist", "stackName", stackName)
			return nil
		}
		return fmt.Errorf("failed to delete CloudWatch infrastructure stack: %w", err)
	}

	slog.Info("initiated deletion of CloudWatch infrastructure stack", "stackName", stackName)
	return nil
}
