// Package ng implements EKS worker nodes with a custom AMI.
package ng

import (
	"errors"
	"os"
	"sync"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"go.uber.org/zap"
	clientset "k8s.io/client-go/kubernetes"
)

/*
https://docs.aws.amazon.com/eks/latest/userguide/launch-workers.html
https://github.com/awslabs/amazon-eks-ami/blob/master/amazon-eks-nodegroup.yaml

https://github.com/awslabs/amazon-eks-ami/blob/master/amazon-eks-nodegroup.yaml
https://raw.githubusercontent.com/awslabs/amazon-eks-ami/master/amazon-eks-nodegroup.yaml

https://aws.amazon.com/about-aws/whats-new/2019/09/amazon-eks-provides-eks-optimized-ami-metadata-via-ssm-parameters/



e.g.
aws ssm get-parameters --names /aws/service/ami-amazon-linux-latest/amzn2-ami-hvm-x86_64-gp2

e.g.
aws ssm get-parameters --names /aws/service/eks/optimized-ami/1.15/amazon-linux-2/recommended
aws ssm get-parameters --names /aws/service/bottlerocket/aws-k8s-1.15/x86_64/latest/image_id
*/

// TemplateNGAL2 is the CloudFormation template for EKS node group.
// ref. https://docs.aws.amazon.com/eks/latest/userguide/launch-workers.html
// ref. https://github.com/awslabs/amazon-eks-ami/blob/master/amazon-eks-nodegroup.yaml
const TemplateNGAL2 = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Node Group'

Parameters:

  Name:
    Type: String
    Description: Unique identifier for the Node Group.

  ClusterName:
    Type: String
    Description: The cluster name provided when the cluster was created. If it is incorrect, nodes will not be able to join the cluster.

  BootstrapArguments:
    Type: String
    Default: ""
    Description: Arguments to pass to the bootstrap script. See files/bootstrap.sh in https://github.com/awslabs/amazon-eks-ami

  ControlPlaneSecurityGroupID:
    Type: AWS::EC2::SecurityGroup::Id
    Description: The security group of the cluster control plane.

  RemoteAccessKeyName:
    Type: AWS::EC2::KeyPair::KeyName
    Description: The EC2 Key Pair to allow SSH access to the instances

  RoleName:
    Type: String
    Description: The name of the node instance role

  ImageID:
    Type: String
    Default: ""
    Description: (Optional) Specify your own custom image ID. This value overrides any AWS Systems Manager Parameter Store value specified above.

  ImageIDSSMParameter:
    Type: AWS::SSM::Parameter::Value<AWS::EC2::Image::Id>
    Default: /aws/service/eks/optimized-ami/1.15/amazon-linux-2/recommended/image_id
    Description: AWS Systems Manager Parameter Store parameter of the AMI ID for the worker node instances.

  InstanceType:
    Type: String
    Default: c5.xlarge
    ConstraintDescription: Must be a valid EC2 instance type
    Description: EC2 instance type for the node instances

  VolumeSize:
    Type: Number
    Default: 20
    Description: Node volume size

  ASGMinSize:
    Type: Number
    Default: 2
    Description: Minimum size of Node Group ASG.

  ASGMaxSize:
    Type: Number
    Default: 2
    Description: Maximum size of Node Group ASG. Set to at least 1 greater than ASGDesiredCapacity.

  ASGDesiredCapacity:
    Type: Number
    Default: 2
    Description: Desired capacity of Node Group ASG.

  VPCID:
    Type: AWS::EC2::VPC::Id
    Description: The VPC of the worker instances

  PublicSubnetIDs:
    Type: List<AWS::EC2::Subnet::Id>
    Description: The public subnet IDs where workers can be created.

Conditions:

  HasImageID:
    Fn::Not:
      - Fn::Equals:
          - Ref: ImageID
          - ""

Resources:

  InstanceProfile:
    Type: AWS::IAM::InstanceProfile
    Properties:
      Path: /
      Roles:
      - Ref: RoleName

  NodeSecurityGroup:
    Type: AWS::EC2::SecurityGroup
    Properties:
      GroupDescription: Security group for all nodes in the cluster
      Tags:
      - Key: !Sub kubernetes.io/cluster/${ClusterName}
        Value: owned
      VpcId: !Ref VPCID

  IngressWithinNodeSecurityGroup:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow node to communicate with each other
      GroupId: !Ref NodeSecurityGroup
      SourceSecurityGroupId: !Ref NodeSecurityGroup
      IpProtocol: "-1"
      FromPort: 0
      ToPort: 65535

  Ingress443FromNGtoCP:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow pods to communicate with the cluster API Server
      SourceSecurityGroupId: !Ref NodeSecurityGroup
      GroupId: !Ref ControlPlaneSecurityGroupID
      IpProtocol: tcp
      FromPort: 443
      ToPort: 443

  Ingress443FromCPtoNG:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow pods running extension API servers on port 443 to receive communication from cluster control plane
      SourceSecurityGroupId: !Ref ControlPlaneSecurityGroupID
      GroupId: !Ref NodeSecurityGroup
      IpProtocol: tcp
      FromPort: 443
      ToPort: 443

  Egress443FromCPtoNG:
    Type: AWS::EC2::SecurityGroupEgress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow the cluster control plane to communicate with pods running extension API servers on port 443
      GroupId: !Ref ControlPlaneSecurityGroupID
      DestinationSecurityGroupId: !Ref NodeSecurityGroup
      IpProtocol: tcp
      FromPort: 443
      ToPort: 443

  Ingress1025FromCPtoNG:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow worker Kubelets and pods to receive communication from the cluster control plane
      SourceSecurityGroupId: !Ref ControlPlaneSecurityGroupID
      GroupId: !Ref NodeSecurityGroup
      IpProtocol: tcp
      FromPort: 1025
      ToPort: 65535

  Egress1025FromCPtoNG:
    Type: AWS::EC2::SecurityGroupEgress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow the cluster control plane to communicate with worker Kubelet and pods
      GroupId: !Ref ControlPlaneSecurityGroupID
      DestinationSecurityGroupId: !Ref NodeSecurityGroup
      IpProtocol: tcp
      FromPort: 1025
      ToPort: 65535

  Ingress22ForSSH:
    Type: AWS::EC2::SecurityGroupIngress
    Properties:
      GroupId: !Ref NodeSecurityGroup
      IpProtocol: 'tcp'
      FromPort: '22'
      ToPort: '22'
      CidrIp: '0.0.0.0/0'

  Ingress1024ForGuestBook:
    Type: AWS::EC2::SecurityGroupIngress
    Properties:
      GroupId: !Ref NodeSecurityGroup
      IpProtocol: 'tcp'
      FromPort: '1'
      ToPort: '1024'
      CidrIp: '0.0.0.0/0'

  Egress1024ForGuestBook:
    Type: AWS::EC2::SecurityGroupIngress
    Properties:
      GroupId: !Ref ControlPlaneSecurityGroupID
      IpProtocol: 'tcp'
      FromPort: '1'
      ToPort: '1024'
      CidrIp: '0.0.0.0/0'

  LaunchConfiguration:
    Type: AWS::AutoScaling::LaunchConfiguration
    Properties:
      AssociatePublicIpAddress: "true"
      BlockDeviceMappings:
        - DeviceName: /dev/xvda
          Ebs:
            DeleteOnTermination: true
            VolumeSize: !Ref VolumeSize
            VolumeType: gp2
      IamInstanceProfile: !Ref InstanceProfile
      ImageId:
        Fn::If:
        - HasImageID
        - !Ref ImageID
        - !Ref ImageIDSSMParameter
      InstanceType: !Ref InstanceType
      KeyName: !Ref RemoteAccessKeyName
      SecurityGroups:
      - Ref: NodeSecurityGroup
      UserData: !Base64
        "Fn::Sub": |
          #!/bin/bash
          set -o xtrace
          /etc/eks/bootstrap.sh ${ClusterName} ${BootstrapArguments}
          /opt/aws/bin/cfn-signal --exit-code $? \
                   --stack  ${AWS::StackName} \
                   --resource NodeGroup  \
                   --region ${AWS::Region}

  AutoScalingGroup:
    Type: AWS::AutoScaling::AutoScalingGroup
    UpdatePolicy:
      AutoScalingRollingUpdate:
        MinInstancesInService: !Ref ASGDesiredCapacity
        MaxBatchSize: 1
        PauseTime: PT5M
    Properties:
      MinSize: !Ref ASGMinSize
      MaxSize: !Ref ASGMaxSize
      DesiredCapacity: !Ref ASGDesiredCapacity
      LaunchConfigurationName: !Ref LaunchConfiguration
      Tags:
      - Key: Name
        PropagateAtLaunch: true
        Value: !Sub ${ClusterName}-${Name}
      - Key: !Sub kubernetes.io/cluster/${ClusterName}
        PropagateAtLaunch: true
        Value: owned
      VPCZoneIdentifier: !Ref PublicSubnetIDs

Outputs:

  SecurityGroupID:
    Value: !Ref NodeSecurityGroup
    Description: The security group ID for the node group

  AutoScalingGroupName:
    Value: !Ref AutoScalingGroup
    Description: The autoscaling group

  InstanceProfileARN:
    Value: !GetAtt InstanceProfile.Arn

`

// Config defines Node Group configuration.
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	Sig       chan os.Signal
	EKSConfig *eksconfig.Config
	K8SClient k8sClientSetGetter
	IAMAPI    iamiface.IAMAPI
	CFNAPI    cloudformationiface.CloudFormationAPI
	EC2API    ec2iface.EC2API
	ASGAPI    autoscalingiface.AutoScalingAPI
	EKSAPI    eksiface.EKSAPI
}

type k8sClientSetGetter interface {
	KubernetesClientSet() *clientset.Clientset
}

// Tester implements EKS "Node Group" for "kubetest2" Deployer.
// ref. https://github.com/kubernetes/test-infra/blob/master/kubetest2/pkg/types/types.go
// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
type Tester interface {
	// Create creates EKS "Node Group", and waits for completion.
	Create() error
	// Delete deletes all EKS "Node Group" resources.
	Delete() error

	// FetchLogs fetches logs from all worker nodes.
	// FetchLogs() error
	// DownloadClusterLogs dumps all logs to artifact directory.
	// Let default kubetest log dumper handle all artifact uploads.
	// See https://github.com/kubernetes/test-infra/pull/9811/files#r225776067.
	// DownloadClusterLogs(artifactDir string) error
}

// New creates a new Job tester.
func New(cfg Config) (Tester, error) {
	return &tester{
		cfg:    cfg,
		logsMu: new(sync.RWMutex),
	}, nil
}

type tester struct {
	cfg        Config
	logsMu     *sync.RWMutex
	failedOnce bool
}

func (ts *tester) Create() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() {
		return nil
	}
	if ts.cfg.EKSConfig.AddOnNodeGroups.Created {
		ts.cfg.Logger.Info("ManagedNodeGroup is already created; skipping creation")
		return nil
	}
	if len(ts.cfg.EKSConfig.Parameters.PublicSubnetIDs) == 0 {
		return errors.New("empty EKSConfig.Parameters.PublicSubnetIDs")
	}

	defer func() {
		ts.cfg.EKSConfig.AddOnNodeGroups.Created = true
		ts.cfg.EKSConfig.Sync()
	}()

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() {
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnNodeGroups.Created {
		ts.cfg.Logger.Info("ManagedNodeGroup is not created; skipping deletion")
		return nil
	}
	return nil
}
