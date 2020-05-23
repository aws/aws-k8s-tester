package ng

import (
	"bytes"
	"context"
	"fmt"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/aws/cfn"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"go.uber.org/zap"
)

// TemplateSG is the CloudFormation template for EKS node group security group.
// ref. https://docs.aws.amazon.com/eks/latest/userguide/launch-workers.html
// ref. https://github.com/awslabs/amazon-eks-ami/blob/master/amazon-eks-nodegroup.yaml
// ref. https://github.com/aws/aws-k8s-tester/pull/33
// ref. https://github.com/kubernetes/kubernetes/blob/release-1.16/test/e2e/network/service.go#L544
const TemplateSG = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Node Group Security Group'

Parameters:

  ClusterName:
    Type: String
    Description: The cluster name provided when the cluster was created. If it is incorrect, nodes will not be able to join the cluster.

  ClusterControlPlaneSecurityGroupID:
    Type: AWS::EC2::SecurityGroup::Id
    Description: The security group of the cluster control plane.

  VPCID:
    Type: AWS::EC2::VPC::Id
    Description: The VPC of the worker instances

Resources:

  NodeGroupSecurityGroup:
    Type: AWS::EC2::SecurityGroup
    Properties:
      GroupDescription: Security group for all nodes in the cluster
      Tags:
      - Key: !Sub kubernetes.io/cluster/${ClusterName}
        Value: owned
      VpcId: !Ref VPCID

  IngressWithinNodeGroupSecurityGroup:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeGroupSecurityGroup
    Properties:
      Description: Allow node to communicate with each other
      GroupId: !Ref NodeGroupSecurityGroup
      SourceSecurityGroupId: !Ref NodeGroupSecurityGroup
      IpProtocol: "-1"
      FromPort: 0
      ToPort: 65535

  Ingress443FromNGtoCP:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeGroupSecurityGroup
    Properties:
      Description: Allow pods to communicate with the cluster API Server
      SourceSecurityGroupId: !Ref NodeGroupSecurityGroup
      GroupId: !Ref ClusterControlPlaneSecurityGroupID
      IpProtocol: tcp
      FromPort: 443
      ToPort: 443

  Ingress443FromCPtoNG:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeGroupSecurityGroup
    Properties:
      Description: Allow pods running extension API servers on port 443 to receive communication from cluster control plane
      SourceSecurityGroupId: !Ref ClusterControlPlaneSecurityGroupID
      GroupId: !Ref NodeGroupSecurityGroup
      IpProtocol: tcp
      FromPort: 443
      ToPort: 443

  Egress443FromCPtoNG:
    Type: AWS::EC2::SecurityGroupEgress
    DependsOn: NodeGroupSecurityGroup
    Properties:
      Description: Allow the cluster control plane to communicate with pods running extension API servers on port 443
      GroupId: !Ref ClusterControlPlaneSecurityGroupID
      DestinationSecurityGroupId: !Ref NodeGroupSecurityGroup
      IpProtocol: tcp
      FromPort: 443
      ToPort: 443

  IngressAllFromCPtoNG:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeGroupSecurityGroup
    Properties:
      Description: Allow worker Kubelets and pods to receive communication from the cluster control plane
      SourceSecurityGroupId: !Ref ClusterControlPlaneSecurityGroupID
      GroupId: !Ref NodeGroupSecurityGroup
      IpProtocol: tcp
      FromPort: 0
      ToPort: 65535

  EgressAllFromCPtoNG:
    Type: AWS::EC2::SecurityGroupEgress
    DependsOn: NodeGroupSecurityGroup
    Properties:
      Description: Allow the cluster control plane to communicate with worker Kubelet and pods
      GroupId: !Ref ClusterControlPlaneSecurityGroupID
      DestinationSecurityGroupId: !Ref NodeGroupSecurityGroup
      IpProtocol: tcp
      FromPort: 0
      ToPort: 65535

  Ingress22ForSSH:
    Type: AWS::EC2::SecurityGroupIngress
    Properties:
      GroupId: !Ref NodeGroupSecurityGroup
      IpProtocol: 'tcp'
      CidrIp: '0.0.0.0/0'
      FromPort: 22
      ToPort: 22

  IngressForGuestBook:
    Type: AWS::EC2::SecurityGroupIngress
    Properties:
      GroupId: !Ref NodeGroupSecurityGroup
      IpProtocol: 'tcp'
      CidrIp: '0.0.0.0/0'
      FromPort: 1
      ToPort: 10000

  EgressForGuestBook:
    Type: AWS::EC2::SecurityGroupIngress
    Properties:
      GroupId: !Ref ClusterControlPlaneSecurityGroupID
      IpProtocol: 'tcp'
      CidrIp: '0.0.0.0/0'
      FromPort: 1
      ToPort: 10000

  IngressForNodePortConformance:
    Type: AWS::EC2::SecurityGroupIngress
    Properties:
      Description: NodePort requires {{.InternetIngressFromPort}}-{{.InternetIngressToPort}} open from nodes to internet, request to node over public IP in those range https://github.com/kubernetes/kubernetes/blob/release-1.16/test/e2e/network/service.go#L544
      GroupId: !Ref NodeGroupSecurityGroup
      IpProtocol: 'tcp'
      CidrIp: '0.0.0.0/0'
      FromPort: {{.InternetIngressFromPort}}
      ToPort: {{.InternetIngressToPort}}

Outputs:

  NodeGroupSecurityGroupID:
    Value: !Ref NodeGroupSecurityGroup
    Description: The security group ID for the node group

`

// "[sig-network] Networking Granular Checks" in "test/e2e/network/dns.go"
// requires "e2enetwork.EndpointUDPPort/EndpointHTTPPort", 8081 and 8080
// just open all for now...
// TODO: restrict ports

type templateSG struct {
	InternetIngressFromPort int
	InternetIngressToPort   int
}

func (ts *tester) createSG() error {
	if ts.cfg.EKSConfig.AddOnNodeGroups.NodeGroupSecurityGroupCFNStackID != "" &&
		ts.cfg.EKSConfig.Status.ClusterControlPlaneSecurityGroupID != "" {
		ts.cfg.Logger.Info("security group already created; no need to create a new one")
		return nil
	}

	fromPort := 30000
	if ts.cfg.EKSConfig.IsEnabledAddOnConformance() {
		fromPort = 0
	}
	tpl := template.Must(template.New("TemplateSG").Parse(TemplateSG))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, templateSG{
		InternetIngressFromPort: fromPort,
		InternetIngressToPort:   32767,
	}); err != nil {
		return err
	}
	tmpl := buf.String()

	ts.cfg.Logger.Info("creating a new node group security group using CFN",
		zap.String("name", ts.cfg.EKSConfig.AddOnNodeGroups.RoleName),
		zap.Int("internet-ingress-from-port", fromPort),
		zap.Int("internet-ingress-to-port", 32767),
	)
	stackInput := &cloudformation.CreateStackInput{
		StackName:    aws.String(ts.cfg.EKSConfig.Name + "-ng-sg"),
		OnFailure:    aws.String(cloudformation.OnFailureDelete),
		TemplateBody: aws.String(tmpl),
		Tags: cfn.NewTags(map[string]string{
			"Kind":                   "aws-k8s-tester",
			"Name":                   ts.cfg.EKSConfig.Name,
			"aws-k8s-tester-version": version.ReleaseVersion,
		}),
		Parameters: []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String("ClusterName"),
				ParameterValue: aws.String(ts.cfg.EKSConfig.Name),
			},
			{
				ParameterKey:   aws.String("ClusterControlPlaneSecurityGroupID"),
				ParameterValue: aws.String(ts.cfg.EKSConfig.Status.ClusterControlPlaneSecurityGroupID),
			},
			{
				ParameterKey:   aws.String("VPCID"),
				ParameterValue: aws.String(ts.cfg.EKSConfig.Parameters.VPCID),
			},
		},
	}

	stackOutput, err := ts.cfg.CFNAPI.CreateStack(stackInput)
	if err != nil {
		return err
	}
	ts.cfg.EKSConfig.AddOnNodeGroups.NodeGroupSecurityGroupCFNStackID = aws.StringValue(stackOutput.StackId)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	ch := cfn.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.CFNAPI,
		ts.cfg.EKSConfig.AddOnNodeGroups.NodeGroupSecurityGroupCFNStackID,
		cloudformation.ResourceStatusCreateComplete,
		time.Minute,
		10*time.Second,
	)
	var st cfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to create node group security group (%v)", st.Error))
			return st.Error
		}
	}
	cancel()

	for _, o := range st.Stack.Outputs {
		switch k := aws.StringValue(o.OutputKey); k {
		case "NodeGroupSecurityGroupID":
			ts.cfg.EKSConfig.AddOnNodeGroups.NodeGroupSecurityGroupID = aws.StringValue(o.OutputValue)
		default:
			return fmt.Errorf("unexpected OutputKey %q from %q", k, ts.cfg.EKSConfig.AddOnNodeGroups.NodeGroupSecurityGroupCFNStackID)
		}
	}

	ts.cfg.Logger.Info("created a node group security group",
		zap.String("cfn-stack-id", ts.cfg.EKSConfig.AddOnNodeGroups.NodeGroupSecurityGroupCFNStackID),
		zap.String("role-arn", ts.cfg.EKSConfig.AddOnNodeGroups.RoleARN),
	)
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteSG() error {
	if ts.cfg.EKSConfig.AddOnNodeGroups.NodeGroupSecurityGroupCFNStackID == "" {
		ts.cfg.Logger.Info("empty node group security group CFN stack ID; no need to delete node group")
		return nil
	}

	ts.cfg.Logger.Info("deleting node group security group CFN stack",
		zap.String("role-cfn-stack-id", ts.cfg.EKSConfig.AddOnNodeGroups.NodeGroupSecurityGroupCFNStackID),
	)
	_, err := ts.cfg.CFNAPI.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(ts.cfg.EKSConfig.AddOnNodeGroups.NodeGroupSecurityGroupCFNStackID),
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	ch := cfn.Poll(
		ctx,
		make(chan struct{}), // do not exit on stop

		ts.cfg.Logger,
		ts.cfg.CFNAPI,
		ts.cfg.EKSConfig.AddOnNodeGroups.NodeGroupSecurityGroupCFNStackID,
		cloudformation.ResourceStatusDeleteComplete,
		time.Minute,
		10*time.Second,
	)
	var st cfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to delete node group security group (%v)", st.Error))
			return st.Error
		}
	}
	cancel()
	ts.cfg.Logger.Info("deleted a node group security group",
		zap.String("role-cfn-stack-id", ts.cfg.EKSConfig.AddOnNodeGroups.NodeGroupSecurityGroupCFNStackID),
	)
	return ts.cfg.EKSConfig.Sync()
}
