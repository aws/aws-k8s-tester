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
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/smithy-go"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	"github.com/aws/aws-k8s-tester/kubetest2/internal/deployers/eksapi/templates"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	nodeDeletionTimeout = time.Minute * 20
)

var (
	defaultInstanceTypes_x86_64 = []string{
		"m6i.xlarge",
		"m6i.large",
		"m5.large",
		"m4.large",
	}

	defaultInstanceTypes_arm64 = []string{
		"m7g.xlarge",
		"m7g.large",
		"m6g.xlarge",
		"m6g.large",
		"t4g.xlarge",
		"t4g.large",
	}

	defaultInstanceTypesByEC2ArchitectureValues = map[ec2types.ArchitectureValues][]string{
		ec2types.ArchitectureValuesX8664: defaultInstanceTypes_x86_64,
		ec2types.ArchitectureValuesArm64: defaultInstanceTypes_arm64,
	}

	defaultInstanceTypesByEKSAMITypes = map[ekstypes.AMITypes][]string{
		ekstypes.AMITypesAl2X8664:            defaultInstanceTypes_x86_64,
		ekstypes.AMITypesAl2Arm64:            defaultInstanceTypes_arm64,
		ekstypes.AMITypesAl2023X8664Standard: defaultInstanceTypes_x86_64,
		ekstypes.AMITypesAl2023Arm64Standard: defaultInstanceTypes_arm64,
	}
)

type nodeManager struct {
	clients    *awsClients
	resourceID string
}

func NewNodeManager(clients *awsClients, resourceID string) *nodeManager {
	return &nodeManager{
		clients:    clients,
		resourceID: resourceID,
	}
}

func (m *nodeManager) createNodes(infra *Infrastructure, cluster *Cluster, opts *deployerOptions, k8sClient *k8sClient) error {
	if err := m.resolveInstanceTypes(opts); err != nil {
		return fmt.Errorf("failed to resolve instance types: %v", err)
	}
	if opts.AutoMode {
		if err := m.createNodePool(opts, k8sClient); err != nil {
			return err
		}
		_, err := m.createPlaceholderDeployment(opts, k8sClient)
		return err
	} else if opts.UnmanagedNodes {
		if opts.EFA {
			return m.createUnmanagedNodegroupWithEFA(infra, cluster, opts)
		}
		return m.createUnmanagedNodegroup(infra, cluster, opts)
	} else {
		return m.createManagedNodegroup(infra, cluster, opts)
	}
}

func (m *nodeManager) resolveInstanceTypes(opts *deployerOptions) (err error) {
	instanceTypes := opts.InstanceTypes
	if len(instanceTypes) == 0 {
		if len(opts.InstanceTypeArchs) > 0 {
			klog.Infof("choosing instance types based on architecture(s): %v", opts.InstanceTypeArchs)
			for _, arch := range opts.InstanceTypeArchs {
				var ec2Arch ec2types.ArchitectureValues
				switch arch {
				case "x86_64", "amd64":
					ec2Arch = ec2types.ArchitectureValuesX8664
				case "aarch64", "arm64":
					ec2Arch = ec2types.ArchitectureValuesArm64
				default:
					return fmt.Errorf("unknown architecture: '%s'", arch)
				}
				instanceTypesForArch, ok := defaultInstanceTypesByEC2ArchitectureValues[ec2Arch]
				if !ok {
					return fmt.Errorf("no default instance types known for architecture: '%s'", arch)
				}
				instanceTypes = append(instanceTypes, instanceTypesForArch...)
			}
		} else if opts.UnmanagedNodes {
			klog.Infof("choosing instance types based on AMI architecture...")
			if out, err := m.clients.EC2().DescribeImages(context.TODO(), &ec2.DescribeImagesInput{
				ImageIds: []string{opts.AMI},
			}); err != nil {
				return fmt.Errorf("failed to describe AMI: %s: %v", opts.AMI, err)
			} else {
				amiArch := out.Images[0].Architecture
				instanceTypesForAMIArchitecture, ok := defaultInstanceTypesByEC2ArchitectureValues[amiArch]
				if !ok {
					return fmt.Errorf("no default instance types known for AMI architecture: %v", amiArch)
				}
				instanceTypes = instanceTypesForAMIArchitecture
			}
		} else {
			// we don't rely on the service's default instance types, because they're a bit too small for the k8s e2e suite
			klog.Infof("choosing instance types based on managed nodegroup's AMI type...")
			instanceTypesForAMIType, ok := defaultInstanceTypesByEKSAMITypes[ekstypes.AMITypes(opts.AMIType)]
			if !ok {
				return fmt.Errorf("no default instance types known for AMI type: %v", opts.AMIType)
			}
			instanceTypes = instanceTypesForAMIType
		}
	}
	validInstanceTypes, err := m.getValidInstanceTypes(instanceTypes)
	if err != nil {
		return err
	}
	opts.InstanceTypes = validInstanceTypes
	klog.Infof("using instance types: %v", opts.InstanceTypes)
	return nil
}

func (m *nodeManager) createNodePool(opts *deployerOptions, k8sClient *k8sClient) error {
	nodePool := karpv1.NodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name: m.resourceID,
		},
		Spec: karpv1.NodePoolSpec{
			Weight: pointer.Int32(100), // max
			Disruption: karpv1.Disruption{
				Budgets: []karpv1.Budget{
					{
						Nodes: "10%",
					},
				},
				ConsolidationPolicy: karpv1.ConsolidationPolicyWhenEmpty,
				ConsolidateAfter:    karpv1.MustParseNillableDuration("600s"),
			},
			Template: karpv1.NodeClaimTemplate{
				Spec: karpv1.NodeClaimTemplateSpec{
					ExpireAfter: karpv1.MustParseNillableDuration("24h"),
					NodeClassRef: &karpv1.NodeClassReference{
						Group: "eks.amazonaws.com",
						Kind:  "NodeClass",
						Name:  "default",
					},
					Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
						{
							NodeSelectorRequirement: corev1.NodeSelectorRequirement{
								Key:      "kubernetes.io/os",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"linux"},
							},
						},
						{
							NodeSelectorRequirement: corev1.NodeSelectorRequirement{
								Key:      "karpenter.sh/capacity-type",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"on-demand"},
							},
						},
						{
							NodeSelectorRequirement: corev1.NodeSelectorRequirement{
								Key:      "node.kubernetes.io/instance-type",
								Operator: corev1.NodeSelectorOpIn,
								Values:   opts.InstanceTypes,
							},
						},
					},
				},
			},
		},
	}
	klog.Infof("creating node pool...")
	if err := k8sClient.client.Create(context.TODO(), &nodePool); err != nil {
		return fmt.Errorf("failed to create node pool: %v", err)
	}
	klog.Infof("created node pool: %+v", nodePool)
	return nil
}

func (m *nodeManager) deleteNodePool(k8sClient *k8sClient) error {
	nodePool := karpv1.NodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name: m.resourceID,
		},
	}
	klog.Infof("deleting node pool...")
	if err := k8sClient.client.Delete(context.TODO(), &nodePool); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("node pool does not exist: %s", m.resourceID)
			return nil
		}
		return fmt.Errorf("failed to delete node pool: %v", err)
	}
	klog.Infof("deleted node pool!")
	return nil
}

// createPlaceholderDeployment creates a Deployment with the specified number of replicas that requires
// each replica to be scheduled on different nodes.
// This ensures that (at least) the specified number of nodes exist in an EKS Auto cluster
func (m *nodeManager) createPlaceholderDeployment(opts *deployerOptions, k8sClient *k8sClient) (*appsv1.Deployment, error) {
	if opts.Nodes == 0 {
		klog.Info("not creating placeholder deployment!")
		return nil, nil
	}
	labels := map[string]string{
		"app": m.resourceID,
	}
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: m.resourceID, Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(int32(opts.Nodes)),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: labels,
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:    "main",
							Image:   "public.ecr.aws/amazonlinux/amazonlinux:2023",
							Command: []string{"sleep", "infinity"},
						},
					},
				},
			},
		},
	}
	klog.Infof("creating placeholder deployment...")
	d, err := k8sClient.clientset.AppsV1().Deployments("default").Create(context.TODO(), d, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create placeholder deployment: %v", err)
	}
	klog.Infof("created placeholder deployment: %+v", d)
	return d, nil
}

func (m *nodeManager) deletePlaceholderDeployment(k8sClient *k8sClient) error {
	klog.Infof("deleting placeholder deployment...")
	if err := k8sClient.clientset.AppsV1().Deployments("default").Delete(context.TODO(), m.resourceID, *metav1.NewDeleteOptions( /* no grace period */ 0)); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("placeholder deployment does not exist: %s", m.resourceID)
			return nil
		}
		return fmt.Errorf("failed to delete placeholder deployment: %v", err)
	}
	klog.Infof("deleted placeholder deployment!")
	return nil
}

func (m *nodeManager) createManagedNodegroup(infra *Infrastructure, cluster *Cluster, opts *deployerOptions) error {
	klog.Infof("creating nodegroup...")
	input := eks.CreateNodegroupInput{
		ClusterName:   aws.String(m.resourceID),
		NodegroupName: aws.String(m.resourceID),
		NodeRole:      aws.String(infra.nodeRoleARN),
		Subnets:       infra.subnets(),
		DiskSize:      aws.Int32(100),
		CapacityType:  ekstypes.CapacityTypesOnDemand,
		ScalingConfig: &ekstypes.NodegroupScalingConfig{
			MinSize:     aws.Int32(int32(opts.Nodes)),
			MaxSize:     aws.Int32(int32(opts.Nodes)),
			DesiredSize: aws.Int32(int32(opts.Nodes)),
		},
		AmiType:       ekstypes.AMITypes(opts.AMIType),
		InstanceTypes: opts.InstanceTypes,
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
		}, opts.NodeCreationTimeout)
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

func (m *nodeManager) createUnmanagedNodegroup(infra *Infrastructure, cluster *Cluster, opts *deployerOptions) error {
	stackName := m.getUnmanagedNodegroupStackName()
	klog.Infof("creating unmanaged nodegroup stack...")
	userData, userDataIsMimePart, err := generateUserData(opts.UserDataFormat, cluster)
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
	volumeMountPath := "/dev/xvda"
	if opts.UserDataFormat == "bottlerocket" {
		volumeMountPath = "/dev/xvdb"
	}
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
				ParameterKey:   aws.String("UserDataIsMIMEPart"),
				ParameterValue: aws.String(strconv.FormatBool(userDataIsMimePart)),
			},
			{
				ParameterKey:   aws.String("ClusterName"),
				ParameterValue: aws.String(cluster.name),
			},
			{
				ParameterKey:   aws.String("NodeRoleName"),
				ParameterValue: aws.String(infra.nodeRoleName),
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
				ParameterKey:   aws.String("AMIId"),
				ParameterValue: aws.String(opts.AMI),
			},
			{
				ParameterKey:   aws.String("VolumeMountPath"),
				ParameterValue: aws.String(volumeMountPath),
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
			opts.NodeCreationTimeout)
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

func (m *nodeManager) createUnmanagedNodegroupWithEFA(infra *Infrastructure, cluster *Cluster, opts *deployerOptions) error {
	stackName := m.getUnmanagedNodegroupStackName()
	klog.Infof("creating unmanaged nodegroup with EFA stack...")
	userData, userDataIsMimePart, err := generateUserData(opts.UserDataFormat, cluster)
	if err != nil {
		return err
	}
	var subnetId, capacityReservationId string
	if opts.CapacityReservation {
		subnetId, capacityReservationId, err = m.getSubnetWithCapacity(infra, opts)
		if err != nil {
			return err
		}
	} else {
		subnetId = infra.subnetsPrivate[0]
	}

	volumeMountPath := "/dev/xvda"
	if opts.UserDataFormat == "bottlerocket" {
		volumeMountPath = "/dev/xvdb"
	}
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
				ParameterValue: aws.String(subnetId), // this is load bearing! EFA requires a private subnet
			},
			{
				ParameterKey:   aws.String("UserData"),
				ParameterValue: aws.String(userData),
			},
			{
				ParameterKey:   aws.String("UserDataIsMIMEPart"),
				ParameterValue: aws.String(strconv.FormatBool(userDataIsMimePart)),
			},
			{
				ParameterKey:   aws.String("VolumeMountPath"),
				ParameterValue: aws.String(volumeMountPath),
			},
			{
				ParameterKey:   aws.String("ClusterName"),
				ParameterValue: aws.String(cluster.name),
			},
			{
				ParameterKey:   aws.String("NodeRoleName"),
				ParameterValue: aws.String(infra.nodeRoleName),
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
				ParameterKey:   aws.String("AMIId"),
				ParameterValue: aws.String(opts.AMI),
			},
			{
				ParameterKey:   aws.String("InstanceType"),
				ParameterValue: aws.String(opts.InstanceTypes[0]),
			},
			{
				ParameterKey:   aws.String("CapacityReservationId"),
				ParameterValue: aws.String(capacityReservationId),
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

// deleteNodes cleans up any nodes in the cluster
// it will be called outside the context of a deployer run (by the janitor, for example)
// so will try to delete nodes of any type
func (m *nodeManager) deleteNodes(k8sClient *k8sClient, opts *deployerOptions) error {
	if err := m.deleteUnmanagedNodegroup(); err != nil {
		return err
	}
	if err := m.deleteManagedNodegroup(); err != nil {
		return err
	}
	// we only have a k8sClient when this is called by the deployer, not by the janitor
	// TODO implement cleanup of Auto nodes in the janitor
	if k8sClient != nil && opts != nil && opts.AutoMode {
		if err := m.deletePlaceholderDeployment(k8sClient); err != nil {
			return err
		}
		if err := m.deleteNodePool(k8sClient); err != nil {
			return err
		}
		if err := k8sClient.waitForNodeDeletion(nodeDeletionTimeout); err != nil {
			return err
		}
	}
	return nil
}

func (m *nodeManager) deleteManagedNodegroup() error {
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
		}, nodeDeletionTimeout)
	if err != nil {
		return fmt.Errorf("failed to wait for nodegroup deletion: %v", err)
	}
	klog.Infof("nodegroup deleted: %s", *out.Nodegroup.NodegroupArn)
	return nil
}

func (m *nodeManager) deleteUnmanagedNodegroup() error {
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

	efaSecurityGroupID, err := m.getEFASecurityGroupIDFromStack(stackName)
	if err != nil {
		return fmt.Errorf("failed to get EFASecurityGroup ID from stack: %w", err)
	}

	if efaSecurityGroupID != "" {
		klog.Infof("clean up leakage ENIs in EFA Security Group")
		err = m.cleanupLeakageENIs(efaSecurityGroupID)
		if err != nil {
			return fmt.Errorf("failed to wait for ASG deletion: %w", err)
		}
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

func (m *nodeManager) cleanupLeakageENIs(efaSecurityGroupID string) error {
	klog.Infof("waiting for ASG in stack to be deleted: %s", m.resourceID)
	err := m.waitForASGDeletion(m.resourceID)
	if err != nil {
		return fmt.Errorf("failed to wait for ASG deletion: %w", err)
	}

	klog.Infof("cleaning up ENIs attached to EFASecurityGroup: %s", efaSecurityGroupID)
	err = m.cleanupEFASecurityGroupENIs(efaSecurityGroupID)
	if err != nil {
		return fmt.Errorf("failed to clean up EFASecurityGroup ENIs: %w", err)
	}
	return nil
}

func (m *nodeManager) waitForASGDeletion(asgName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20 * time.Minute)
	defer cancel()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for ASG %s deletion", asgName)
		case <-ticker.C:
			deleted, err := m.isASGDeleted(asgName)
			if err != nil {
				return fmt.Errorf("failed to check ASG deletion: %w", err)
			} else if deleted {
				return nil
			}
		}
	}
}

func (m *nodeManager) isASGDeleted(asgName string) (bool, error) {
	asgOutput, err := m.clients.ASG().DescribeAutoScalingGroups(context.TODO(), &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []string{asgName},
	})
	if err != nil  {
		return false, fmt.Errorf("failed to describe ASG: %w", err)
	} else if len(asgOutput.AutoScalingGroups) == 0 {
		return true, nil
	}
	return false, nil
}

func (m *nodeManager) cleanupEFASecurityGroupENIs(efaSecurityGroupID string) error {
	enis, err := m.getSecurityGroupNetworkInterfaceIds(efaSecurityGroupID)
	if err != nil {
		return fmt.Errorf("failed to describe ENIs: %w", err)
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
	klog.Infof("deleted %d leaked ENI(s) attached to EFA security group!", len(enis))
	return nil
}

func (m *nodeManager) getSecurityGroupNetworkInterfaceIds(efaSecurityGroupID string) ([]string, error) {
	output, err := m.clients.EC2().DescribeNetworkInterfaces(context.TODO(), &ec2.DescribeNetworkInterfacesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("group-id"),
				Values: []string{efaSecurityGroupID},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe ENIs: %w", err)
	}

	var enis []string
	for _, eni := range output.NetworkInterfaces {
		enis = append(enis, *eni.NetworkInterfaceId)
	}
	return enis, nil
}

func (m *nodeManager) getEFASecurityGroupIDFromStack(stackName string) (string, error) {
	describeInput := cloudformation.DescribeStackResourcesInput{
		StackName: aws.String(stackName),
	}
	output, err := m.clients.CFN().DescribeStackResources(context.TODO(), &describeInput)
	if err != nil {
		return "", fmt.Errorf("failed to describe stack resources: %w", err)
	}
	for _, resource := range output.StackResources {
		if *resource.LogicalResourceId == "EFASecurityGroup" {
			return *resource.PhysicalResourceId, nil
		}
	}
	return "", nil
}

func (m *nodeManager) getUnmanagedNodegroupStackName() string {
	return fmt.Sprintf("%s-unmanaged-nodegroup", m.resourceID)
}

func (m *nodeManager) verifyASGAMI(asgName string, amiId string) (bool, error) {
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

func (m *nodeManager) getSubnetWithCapacity(infra *Infrastructure, opts *deployerOptions) (string, string, error) {
	var capacityReservationId string
	capacityReservations, err := m.clients.EC2().DescribeCapacityReservations(context.TODO(), &ec2.DescribeCapacityReservationsInput{
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
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to describe capacity reservation")
	}
	var az string
	for _, cr := range capacityReservations.CapacityReservations {
		if *cr.AvailableInstanceCount >= int32(opts.Nodes) {
			capacityReservationId = *cr.CapacityReservationId
			az = *cr.AvailabilityZone
			break
		}
	}
	if capacityReservationId == "" {
		return "", "", fmt.Errorf("no capacity reservation found for instance type %s with %d nodes count", opts.InstanceTypes[0], opts.Nodes)
	}
	klog.Infof("Using capacity reservation: %s", capacityReservationId)
	subnet, err := m.clients.EC2().DescribeSubnets(context.TODO(), &ec2.DescribeSubnetsInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("availability-zone"),
				Values: []string{az},
			},
			{
				Name:   aws.String("subnet-id"),
				Values: infra.subnetsPrivate,
			},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to describe subnet")
	}
	if subnet == nil || len(subnet.Subnets) == 0 {
		return "", "", fmt.Errorf("no subnet found for availability zone %s", az)
	}
	subnetId := *subnet.Subnets[0].SubnetId
	klog.Infof("Using subnet: %s", subnetId)
	return subnetId, capacityReservationId, nil
}

func (m *nodeManager) getValidInstanceTypes(desiredInstanceTypes []string) ([]string, error) {
	var validInstanceTypes []string
	for _, instanceType := range desiredInstanceTypes {
		ec2InstanceType := ec2types.InstanceType(instanceType)
		_, err := m.clients.EC2().DescribeInstanceTypes(context.TODO(), &ec2.DescribeInstanceTypesInput{
			InstanceTypes: []ec2types.InstanceType{ec2InstanceType},
		})
		if err != nil {
			var apierr smithy.APIError
			if errors.As(err, &apierr) && apierr.ErrorCode() == "InvalidInstanceType" {
				klog.Infof("Eliminating instance type %s as an option", instanceType)
			} else {
				return nil, fmt.Errorf("failed to describe instance type: %s: %v", instanceType, err)
			}
		} else {
			validInstanceTypes = append(validInstanceTypes, instanceType)
		}
	}
	return validInstanceTypes, nil
}
