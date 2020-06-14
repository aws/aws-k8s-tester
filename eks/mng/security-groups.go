package mng

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/aws/cfn"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"go.uber.org/zap"
)

/*
// 'SourceSecurityGroups' does not open to '0.0.0.0/0'
RemoteAccess:
  Ec2SshKey: !Ref RemoteAccessKeyName
  SourceSecurityGroups:
  - !Ref ManagedNodeGroupSecurityGroup
*/

/*
// Not supported yet
// Fn::GetAtt: Resource type AWS::EKS::Nodegroup does not support attribute {Resources.RemoteAccessSecurityGroup}

  SecurityGroupIngress22:
    Type: AWS::EC2::SecurityGroupIngress
    Properties:
      GroupId: !GetAtt ManagedNodeGroup.Resources.RemoteAccessSecurityGroup
      IpProtocol: 'tcp'
      FromPort: '22'
      ToPort: '22'
      CidrIp: '0.0.0.0/0'

  SecurityGroupIngress1024:
    Type: AWS::EC2::SecurityGroupIngress
    Properties:
      GroupId: !GetAtt ManagedNodeGroup.Resources.RemoteAccessSecurityGroup
      IpProtocol: 'tcp'
      FromPort: '1'
      ToPort: '1024'
      CidrIp: '0.0.0.0/0'

  SecurityGroupEgress1024:
    Type: AWS::EC2::SecurityGroupIngress
    Properties:
      GroupId: !Ref ClusterControlPlaneSecurityGroupID
      IpProtocol: 'tcp'
      FromPort: '1'
      ToPort: '1024'
      CidrIp: '0.0.0.0/0'

Outputs:

  ManagedNodeGroupRemoteAccessSecurityGroupID:
    Description: Security group ID for the node group SSH
    Value: !GetAtt ManagedNodeGroup.Resources.RemoteAccessSecurityGroup
*/

// TemplateSG is the CloudFormation template for EKS managed node group security group.
// ref. https://docs.aws.amazon.com/eks/latest/userguide/launch-workers.html
// ref. https://github.com/awslabs/amazon-eks-ami/blob/master/amazon-eks-nodegroup.yaml
// ref. https://github.com/aws/aws-k8s-tester/pull/33
// ref. https://github.com/kubernetes/kubernetes/blob/release-1.16/test/e2e/network/service.go#L544
const TemplateSG = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Managed Node Group Security Group'

Parameters:

  ClusterControlPlaneSecurityGroupID:
    Type: AWS::EC2::SecurityGroup::Id
    Description: The security group of the cluster control plane.

  ManagedNodeGroupSecurityGroupID:
    Type: AWS::EC2::SecurityGroup::Id
    Description: The security group of a managed node group

Resources:

  IngressWithinManagedNodeGroupSecurityGroup:
    Type: AWS::EC2::SecurityGroupIngress
    Properties:
      Description: Allow node to communicate with each other
      GroupId: !Ref ManagedNodeGroupSecurityGroupID
      SourceSecurityGroupId: !Ref ManagedNodeGroupSecurityGroupID
      IpProtocol: "-1"
      FromPort: 0
      ToPort: 65535

  Ingress443FromNGtoCP:
    Type: AWS::EC2::SecurityGroupIngress
    Properties:
      Description: Allow pods to communicate with the cluster API Server
      SourceSecurityGroupId: !Ref ManagedNodeGroupSecurityGroupID
      GroupId: !Ref ClusterControlPlaneSecurityGroupID
      IpProtocol: tcp
      FromPort: 443
      ToPort: 443

  Ingress443FromCPtoNG:
    Type: AWS::EC2::SecurityGroupIngress
    Properties:
      Description: Allow pods running extension API servers on port 443 to receive communication from cluster control plane
      SourceSecurityGroupId: !Ref ClusterControlPlaneSecurityGroupID
      GroupId: !Ref ManagedNodeGroupSecurityGroupID
      IpProtocol: tcp
      FromPort: 443
      ToPort: 443

  Egress443FromCPtoNG:
    Type: AWS::EC2::SecurityGroupEgress
    Properties:
      Description: Allow the cluster control plane to communicate with pods running extension API servers on port 443
      GroupId: !Ref ClusterControlPlaneSecurityGroupID
      DestinationSecurityGroupId: !Ref ManagedNodeGroupSecurityGroupID
      IpProtocol: tcp
      FromPort: 443
      ToPort: 443

  IngressAllFromCPtoNG:
    Type: AWS::EC2::SecurityGroupIngress
    Properties:
      Description: Allow worker Kubelets and pods to receive communication from the cluster control plane
      SourceSecurityGroupId: !Ref ClusterControlPlaneSecurityGroupID
      GroupId: !Ref ManagedNodeGroupSecurityGroupID
      IpProtocol: tcp
      FromPort: 0
      ToPort: 65535

  EgressAllFromCPtoNG:
    Type: AWS::EC2::SecurityGroupEgress
    Properties:
      Description: Allow the cluster control plane to communicate with worker Kubelet and pods
      GroupId: !Ref ClusterControlPlaneSecurityGroupID
      DestinationSecurityGroupId: !Ref ManagedNodeGroupSecurityGroupID
      IpProtocol: tcp
      FromPort: 0
      ToPort: 65535

  Ingress22ForSSH:
    Type: AWS::EC2::SecurityGroupIngress
    Properties:
      GroupId: !Ref ManagedNodeGroupSecurityGroupID
      IpProtocol: 'tcp'
      CidrIp: '0.0.0.0/0'
      FromPort: 22
      ToPort: 22

  IngressForGuestBook:
    Type: AWS::EC2::SecurityGroupIngress
    Properties:
      GroupId: !Ref ManagedNodeGroupSecurityGroupID
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
      GroupId: !Ref ManagedNodeGroupSecurityGroupID
      IpProtocol: 'tcp'
      CidrIp: '0.0.0.0/0'
      FromPort: {{.InternetIngressFromPort}}
      ToPort: {{.InternetIngressToPort}}

`

// "[sig-network] Networking Granular Checks" in "test/e2e/network/dns.go"
// requires "e2enetwork.EndpointUDPPort/EndpointHTTPPort", 8081 and 8080
// just open all for now...
// TODO: restrict ports

type templateSG struct {
	InternetIngressFromPort int
	InternetIngressToPort   int
}

func (ts *tester) createSG(name string) error {
	cur, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[name]
	if !ok {
		return fmt.Errorf("MNGs[%q] not found; cannot create ingress/egress security group", name)
	}
	if cur.RemoteAccessSecurityGroupID == "" {
		return fmt.Errorf("MNG[%q] security group ID not found; cannot create ingress/egress security group", name)
	}
	if cur.RemoteAccessSecurityGroupIngressEgressCFNStackID != "" {
		ts.cfg.Logger.Info("managed node group already has opened ports",
			zap.String("mng-name", name),
			zap.String("cfn-stack-id", cur.RemoteAccessSecurityGroupIngressEgressCFNStackID),
		)
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

	if err := ioutil.WriteFile(cur.RemoteAccessSecurityCFNStackYAMLFilePath, buf.Bytes(), 0400); err != nil {
		return err
	}
	sgID := cur.RemoteAccessSecurityGroupID
	ts.cfg.Logger.Info("creating ingress/egress security group for mng using CFN",
		zap.String("mng-name", name),
		zap.String("security-group-id", sgID),
		zap.String("security-cfn-stack-file", cur.RemoteAccessSecurityCFNStackYAMLFilePath),
		zap.Int("internet-ingress-from-port", fromPort),
		zap.Int("internet-ingress-to-port", 32767),
	)
	stackInput := &cloudformation.CreateStackInput{
		StackName:    aws.String(name + "-mng-sg"),
		OnFailure:    aws.String(cloudformation.OnFailureDelete),
		TemplateBody: aws.String(buf.String()),
		Tags: cfn.NewTags(map[string]string{
			"Kind":                   "aws-k8s-tester",
			"Name":                   ts.cfg.EKSConfig.Name,
			"aws-k8s-tester-version": version.ReleaseVersion,
		}),
		Parameters: []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String("ClusterControlPlaneSecurityGroupID"),
				ParameterValue: aws.String(ts.cfg.EKSConfig.Status.ClusterControlPlaneSecurityGroupID),
			},
			{
				ParameterKey:   aws.String("ManagedNodeGroupSecurityGroupID"),
				ParameterValue: aws.String(sgID),
			},
		},
	}

	stackOutput, err := ts.cfg.CFNAPI.CreateStack(stackInput)
	if err != nil {
		return err
	}
	cur.RemoteAccessSecurityGroupIngressEgressCFNStackID = aws.StringValue(stackOutput.StackId)
	ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[name] = cur
	ts.cfg.EKSConfig.Sync()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	ch := cfn.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.CFNAPI,
		cur.RemoteAccessSecurityGroupIngressEgressCFNStackID,
		cloudformation.ResourceStatusCreateComplete,
		time.Minute,
		10*time.Second,
	)
	for st := range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to create managed node group ingress/egress security group (%v)", st.Error))
			return st.Error
		}
	}
	cancel()

	ts.cfg.Logger.Info("created ingress/egress security group for mng",
		zap.String("mng-name", name),
		zap.String("cfn-stack-id", cur.RemoteAccessSecurityGroupIngressEgressCFNStackID),
	)
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteSG(name string) error {
	cur, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[name]
	if !ok {
		return fmt.Errorf("MNGs[%q] not found; cannot delete ingress/egress security group", name)
	}
	if cur.RemoteAccessSecurityGroupID == "" {
		return fmt.Errorf("MNG[%q] security group ID not found; cannot delete ingress/egress security group", name)
	}
	if cur.RemoteAccessSecurityGroupIngressEgressCFNStackID == "" {
		ts.cfg.Logger.Info("managed node group has no open ports",
			zap.String("mng-name", name),
			zap.String("cfn-stack-id", cur.RemoteAccessSecurityGroupIngressEgressCFNStackID),
		)
		return nil
	}

	ts.cfg.Logger.Info("deleting managed node group ingress/egress security group CFN stack",
		zap.String("mng-name", name),
		zap.String("cfn-stack-id", cur.RemoteAccessSecurityGroupIngressEgressCFNStackID),
	)
	_, err := ts.cfg.CFNAPI.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(cur.RemoteAccessSecurityGroupIngressEgressCFNStackID),
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
		cur.RemoteAccessSecurityGroupIngressEgressCFNStackID,
		cloudformation.ResourceStatusDeleteComplete,
		time.Minute,
		10*time.Second,
	)
	var st cfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to delete managed node group ingress/egress security group (%v)", st.Error))
			return st.Error
		}
	}
	cancel()
	ts.cfg.Logger.Info("deleted a managed node group ingress/egress security group",
		zap.String("mng-name", name),
		zap.String("cfn-stack-id", cur.RemoteAccessSecurityGroupIngressEgressCFNStackID),
	)
	return ts.cfg.EKSConfig.Sync()
}
