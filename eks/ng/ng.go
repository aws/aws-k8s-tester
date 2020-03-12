// Package ng implements EKS worker nodes with a custom AMI.
package ng

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

// TemplateNG is the CloudFormation template for EKS node group.
// ref. https://github.com/awslabs/amazon-eks-ami/blob/master/amazon-eks-nodegroup.yaml
const TemplateNG = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Node Group'

Parameters:

  BootstrapArguments:
    Type: String
    Default: ""
    Description: Arguments to pass to the bootstrap script. See files/bootstrap.sh in https://github.com/awslabs/amazon-eks-ami

  ControlPlaneSecurityGroupID:
    Type: AWS::EC2::SecurityGroup::Id
    Description: The security group of the cluster control plane.

  ClusterName:
    Type: String
    Description: The cluster name provided when the cluster was created. If it is incorrect, nodes will not be able to join the cluster.

  NodeGroupRemoteAccessKeyName:
    Type: AWS::EC2::KeyPair::KeyName
    Description: The EC2 Key Pair to allow SSH access to the instances

  NodeGroupRoleName:
    Type: String
    Description: The name of the node instance role

  NodeGroupName:
    Type: String
    Description: Unique identifier for the Node Group.

  NodeGroupImageID:
    Type: String
    Default: ""
    Description: (Optional) Specify your own custom image ID. This value overrides any AWS Systems Manager Parameter Store value specified above.

  NodeGroupImageIDSSMParameter:
    Type: AWS::SSM::Parameter::Value<AWS::EC2::Image::Id>
    Default: /aws/service/eks/optimized-ami/1.15/amazon-linux-2/recommended/image_id
    Description: AWS Systems Manager Parameter Store parameter of the AMI ID for the worker node instances.

  NodeGroupInstanceType:
    Type: String
    Default: c5.xlarge
    ConstraintDescription: Must be a valid EC2 instance type
    Description: EC2 instance type for the node instances

  NodeGroupVolumeSize:
    Type: Number
    Default: 20
    Description: Node volume size

  NodeGroupASGMinSize:
    Type: Number
    Default: 2
    Description: Minimum size of Node Group ASG.

  NodeGroupASGMaxSize:
    Type: Number
    Default: 2
    Description: Maximum size of Node Group ASG. Set to at least 1 greater than NodeGroupASGDesiredCapacity.

  NodeGroupASGDesiredCapacity:
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

  HasNodeGroupImageID:
    Fn::Not:
      - Fn::Equals:
          - Ref: NodeGroupImageID
          - ""

Resources:

  NodeGroupInstanceProfile:
    Type: AWS::IAM::InstanceProfile
    Properties:
      Path: /
      Roles:
      - Ref: NodeGroupRoleName

  NodeGroupSecurityGroup:
    Type: AWS::EC2::SecurityGroup
    Properties:
      GroupDescription: Security group for all nodes in the cluster
      Tags:
      - Key: !Sub kubernetes.io/cluster/${ClusterName}
        Value: owned
      VpcId: !Ref VPCID

  NodeGroupSecurityGroupIngress:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeGroupSecurityGroup
    Properties:
      Description: Allow node to communicate with each other
      FromPort: 0
      GroupId: !Ref NodeGroupSecurityGroup
      IpProtocol: "-1"
      SourceSecurityGroupId: !Ref NodeGroupSecurityGroup
      ToPort: 65535

  ControlPlaneSecurityGroupIngress:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeGroupSecurityGroup
    Properties:
      Description: Allow pods to communicate with the cluster API Server
      FromPort: 443
      GroupId: !Ref ControlPlaneSecurityGroupID
      IpProtocol: tcp
      SourceSecurityGroupId: !Ref NodeGroupSecurityGroup
      ToPort: 443

  ControlPlaneEgressToNodeGroupSecurityGroup:
    Type: AWS::EC2::SecurityGroupEgress
    DependsOn: NodeGroupSecurityGroup
    Properties:
      Description: Allow the cluster control plane to communicate with worker Kubelet and pods
      DestinationSecurityGroupId: !Ref NodeGroupSecurityGroup
      FromPort: 1025
      GroupId: !Ref ControlPlaneSecurityGroupID
      IpProtocol: tcp
      ToPort: 65535

  ControlPlaneEgressToNodeGroupSecurityGroupOn443:
    Type: AWS::EC2::SecurityGroupEgress
    DependsOn: NodeGroupSecurityGroup
    Properties:
      Description: Allow the cluster control plane to communicate with pods running extension API servers on port 443
      DestinationSecurityGroupId: !Ref NodeGroupSecurityGroup
      FromPort: 443
      GroupId: !Ref ControlPlaneSecurityGroupID
      IpProtocol: tcp
      ToPort: 443

  NodeGroupSecurityGroupFromControlPlaneIngress:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeGroupSecurityGroup
    Properties:
      Description: Allow worker Kubelets and pods to receive communication from the cluster control plane
      FromPort: 1025
      GroupId: !Ref NodeGroupSecurityGroup
      IpProtocol: tcp
      SourceSecurityGroupId: !Ref ControlPlaneSecurityGroupID
      ToPort: 65535

  NodeGroupSecurityGroupFromControlPlaneOn443Ingress:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeGroupSecurityGroup
    Properties:
      Description: Allow pods running extension API servers on port 443 to receive communication from cluster control plane
      FromPort: 443
      GroupId: !Ref NodeGroupSecurityGroup
      IpProtocol: tcp
      SourceSecurityGroupId: !Ref ControlPlaneSecurityGroupID
      ToPort: 443

  NodeGroupLaunchConfig:
    Type: AWS::AutoScaling::LaunchConfiguration
    Properties:
      AssociatePublicIpAddress: "true"
      BlockDeviceMappings:
        - DeviceName: /dev/xvda
          Ebs:
            DeleteOnTermination: true
            VolumeSize: !Ref NodeGroupVolumeSize
            VolumeType: gp2
      IamInstanceProfile: !Ref NodeGroupInstanceProfile
      ImageId:
        Fn::If:
        - HasNodeGroupImageID
        - !Ref NodeGroupImageID
        - !Ref NodeGroupImageIDSSMParameter
      InstanceType: !Ref NodeGroupInstanceType
      KeyName: !Ref NodeGroupRemoteAccessKeyName
      SecurityGroups:
      - Ref: NodeGroupSecurityGroup
      UserData: !Base64
        "Fn::Sub": |
          #!/bin/bash
          set -o xtrace
          /etc/eks/bootstrap.sh ${ClusterName} ${BootstrapArguments}
          /opt/aws/bin/cfn-signal --exit-code $? \
                   --stack  ${AWS::StackName} \
                   --resource NodeGroup  \
                   --region ${AWS::Region}

  NodeGroup:
    Type: AWS::AutoScaling::AutoScalingGroup
    UpdatePolicy:
      AutoScalingRollingUpdate:
        MinInstancesInService: !Ref NodeGroupASGDesiredCapacity
        MaxBatchSize: 1
        PauseTime: PT5M
    Properties:
      MinSize: !Ref NodeGroupASGMinSize
      MaxSize: !Ref NodeGroupASGMaxSize
      DesiredCapacity: !Ref NodeGroupASGDesiredCapacity
      LaunchConfigurationName: !Ref NodeGroupLaunchConfig
      Tags:
      - Key: Name
        PropagateAtLaunch: true
        Value: !Sub ${ClusterName}-${NodeGroupName}
      - Key: !Sub kubernetes.io/cluster/${ClusterName}
        PropagateAtLaunch: true
        Value: owned
      VPCZoneIdentifier: !Ref PublicSubnetIDs

Outputs:

  NodeGroupSecurityGroupID:
    Value: !Ref NodeGroupSecurityGroup
    Description: The security group ID for the node group

  NodeGroupASG:
    Value: !Ref NodeGroup
    Description: The autoscaling group

  NodeGroupInstanceProfileARN:
    Value: !GetAtt NodeGroupInstanceProfile.Arn

`
