package cluster

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/aws/cfn"
	aws_iam "github.com/aws/aws-k8s-tester/pkg/aws/iam"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/mitchellh/colorstring"
	"go.uber.org/zap"
)

// TemplateClusterRole is the CloudFormation template for EKS cluster role.
//
// e.g.
//   Error creating load balancer (will retry): failed to ensure load balancer for service eks-*/hello-world-service: Error creating load balancer: "AccessDenied: User: arn:aws:sts::404174646922:assumed-role/eks-*-cluster-role/* is not authorized to perform: ec2:DescribeAccountAttributes\n\tstatus code: 403"
//
// TODO: scope down (e.g. ec2:DescribeAccountAttributes, ec2:DescribeInternetGateways)
// mng, fargate, etc. may use other roles
const TemplateClusterRole = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Cluster Role Basic'

Parameters:

  RoleName:
    Type: String
    Default: aws-k8s-tester-eks-role
    Description: EKS Role name

  RoleServicePrincipals:
    Type: CommaDelimitedList
    Default: 'ec2.amazonaws.com,eks.amazonaws.com,eks-fargate-pods.amazonaws.com'
    Description: EKS Role Service Principals

  RoleManagedPolicyARNs:
    Type: CommaDelimitedList
    Default: 'arn:aws:iam::aws:policy/AmazonEKSServicePolicy,arn:aws:iam::aws:policy/AmazonEKSClusterPolicy,arn:aws:iam::aws:policy/AmazonSSMFullAccess,arn:aws:iam::aws:policy/AmazonS3FullAccess,arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy,arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy,arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly,arn:aws:iam::aws:policy/ElasticLoadBalancingFullAccess'
    Description: EKS Role managed policy ARNs

Resources:

  Role:
    Type: AWS::IAM::Role
    Properties:
      RoleName: !Ref RoleName
      AssumeRolePolicyDocument:
        Version: '2012-10-17'
        Statement:
        - Effect: Allow
          Principal:
            Service: !Ref RoleServicePrincipals
          Action:
          - sts:AssumeRole
      ManagedPolicyArns: !Ref RoleManagedPolicyARNs
      Path: /
      Policies:
      # https://docs.aws.amazon.com/eks/latest/userguide/ebs-csi.html
      # https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/iam-policy.json
      # https://github.com/aws/eks-charts/tree/master/stable/appmesh-controller
      - PolicyName: !Join ['-', [!Ref RoleName, 'alb-appmesh-policy']]
        PolicyDocument:
          Version: '2012-10-17'
          Statement:
          - Effect: Allow
            Action:
            - acm:DescribeCertificate
            - acm:ListCertificates
            - acm:GetCertificate
            Resource: "*"
          - Effect: Allow
            Action:
            - ec2:AuthorizeSecurityGroupIngress
            - ec2:CreateSecurityGroup
            - ec2:CreateTags
            - ec2:DeleteTags
            - ec2:DeleteSecurityGroup
            - ec2:DescribeAccountAttributes
            - ec2:DescribeAddresses
            - ec2:DescribeInstances
            - ec2:DescribeInstanceStatus
            - ec2:DescribeInternetGateways
            - ec2:DescribeNetworkInterfaces
            - ec2:DescribeSecurityGroups
            - ec2:DescribeSubnets
            - ec2:DescribeTags
            - ec2:DescribeVpcs
            - ec2:ModifyInstanceAttribute
            - ec2:ModifyNetworkInterfaceAttribute
            - ec2:RevokeSecurityGroupIngress
            - ec2:AttachVolume
            - ec2:CreateSnapshot
            - ec2:CreateVolume
            - ec2:DeleteSnapshot
            - ec2:DeleteVolume
            - ec2:DescribeSnapshots
            - ec2:DescribeVolumes
            - ec2:DetachVolume
            Resource: "*"
          - Effect: Allow
            Action:
            - elasticloadbalancing:AddListenerCertificates
            - elasticloadbalancing:AddTags
            - elasticloadbalancing:CreateListener
            - elasticloadbalancing:CreateLoadBalancer
            - elasticloadbalancing:CreateRule
            - elasticloadbalancing:CreateTargetGroup
            - elasticloadbalancing:DeleteListener
            - elasticloadbalancing:DeleteLoadBalancer
            - elasticloadbalancing:DeleteRule
            - elasticloadbalancing:DeleteTargetGroup
            - elasticloadbalancing:DeregisterTargets
            - elasticloadbalancing:DescribeListenerCertificates
            - elasticloadbalancing:DescribeListeners
            - elasticloadbalancing:DescribeLoadBalancers
            - elasticloadbalancing:DescribeLoadBalancerAttributes
            - elasticloadbalancing:DescribeRules
            - elasticloadbalancing:DescribeSSLPolicies
            - elasticloadbalancing:DescribeTags
            - elasticloadbalancing:DescribeTargetGroups
            - elasticloadbalancing:DescribeTargetGroupAttributes
            - elasticloadbalancing:DescribeTargetHealth
            - elasticloadbalancing:ModifyListener
            - elasticloadbalancing:ModifyLoadBalancerAttributes
            - elasticloadbalancing:ModifyRule
            - elasticloadbalancing:ModifyTargetGroup
            - elasticloadbalancing:ModifyTargetGroupAttributes
            - elasticloadbalancing:RegisterTargets
            - elasticloadbalancing:RemoveListenerCertificates
            - elasticloadbalancing:RemoveTags
            - elasticloadbalancing:SetIpAddressType
            - elasticloadbalancing:SetSecurityGroups
            - elasticloadbalancing:SetSubnets
            - elasticloadbalancing:SetWebACL
            Resource: "*"
          - Effect: Allow
            Action:
            - iam:CreateServiceLinkedRole
            - iam:GetServerCertificate
            - iam:ListServerCertificates
            Resource: "*"
          - Effect: Allow
            Action:
            - cognito-idp:DescribeUserPoolClient
            Resource: "*"
          - Effect: Allow
            Action:
            - waf-regional:GetWebACLForResource
            - waf-regional:GetWebACL
            - waf-regional:AssociateWebACL
            - waf-regional:DisassociateWebACL
            Resource: "*"
          - Effect: Allow
            Action:
            - tag:GetResources
            - tag:TagResources
            Resource: "*"
          - Effect: Allow
            Action:
            - waf:GetWebACL
            Resource: "*"
          - Effect: Allow
            Action:
            - wafv2:GetWebACL
            - wafv2:GetWebACLForResource
            - wafv2:AssociateWebACL
            - wafv2:DisassociateWebACL
            Resource: "*"
          - Effect: Allow
            Action:
            - shield:DescribeProtection
            - shield:GetSubscriptionState
            - shield:DeleteProtection
            - shield:CreateProtection
            - shield:DescribeSubscription
            - shield:ListProtections
            Resource: "*"
          - Effect: Allow
            Action:
            - appmesh:*
            - servicediscovery:CreateService
            - servicediscovery:GetService
            - servicediscovery:RegisterInstance
            - servicediscovery:DeregisterInstance
            - servicediscovery:ListInstances
            - servicediscovery:ListNamespaces
            - servicediscovery:ListServices
            - route53:GetHealthCheck
            - route53:CreateHealthCheck
            - route53:UpdateHealthCheck
            - route53:ChangeResourceRecordSets
            - route53:DeleteHealthCheck
            Resource: "*"

Outputs:

  RoleARN:
    Value: !GetAtt Role.Arn
    Description: Role ARN that EKS uses to create AWS resources for Kubernetes

`

func (ts *tester) createClusterRole() error {
	if ts.cfg.EKSConfig.LogColor {
		colorstring.Printf("\n\n[yellow]*********************************\n")
		colorstring.Printf("[light_green]createClusterRole [default](%q)\n", ts.cfg.EKSConfig.ConfigPath)
	} else {
		fmt.Printf("\n\n*********************************\n")
		fmt.Printf("createClusterRole (%q)\n", ts.cfg.EKSConfig.ConfigPath)
	}

	if !ts.cfg.EKSConfig.Parameters.RoleCreate {
		ts.cfg.Logger.Info("Parameters.RoleCreate false; skipping creation")
		return aws_iam.Validate(
			ts.cfg.Logger,
			ts.cfg.IAMAPI,
			ts.cfg.EKSConfig.Parameters.RoleName,
			[]string{"eks.amazonaws.com"},
			[]string{
				"arn:aws:iam::aws:policy/AmazonEKSServicePolicy",
				"arn:aws:iam::aws:policy/AmazonEKSClusterPolicy",
			},
		)
	}
	if ts.cfg.EKSConfig.Parameters.RoleCFNStackID != "" &&
		ts.cfg.EKSConfig.Parameters.RoleARN != "" {
		ts.cfg.Logger.Info("role already created; no need to create a new one")
		return nil
	}
	if ts.cfg.EKSConfig.Parameters.RoleName == "" {
		return errors.New("cannot create a cluster role with an empty Parameters.RoleName")
	}

	tmpl := TemplateClusterRole

	// role ARN is empty, create a default role
	// otherwise, use the existing one
	ts.cfg.Logger.Info("creating a new role", zap.String("cluster-role-name", ts.cfg.EKSConfig.Parameters.RoleName))
	stackInput := &cloudformation.CreateStackInput{
		StackName:    aws.String(ts.cfg.EKSConfig.Parameters.RoleName),
		Capabilities: aws.StringSlice([]string{"CAPABILITY_NAMED_IAM"}),
		OnFailure:    aws.String(cloudformation.OnFailureDelete),
		TemplateBody: aws.String(tmpl),
		Tags: cfn.NewTags(map[string]string{
			"Kind":                   "aws-k8s-tester",
			"Name":                   ts.cfg.EKSConfig.Name,
			"aws-k8s-tester-version": version.ReleaseVersion,
		}),
		Parameters: []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String("RoleName"),
				ParameterValue: aws.String(ts.cfg.EKSConfig.Parameters.RoleName),
			},
		},
	}
	if len(ts.cfg.EKSConfig.Parameters.RoleServicePrincipals) > 0 {
		ts.cfg.Logger.Info("creating a new cluster role with custom service principals",
			zap.Strings("service-principals", ts.cfg.EKSConfig.Parameters.RoleServicePrincipals),
		)
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("RoleServicePrincipals"),
			ParameterValue: aws.String(strings.Join(ts.cfg.EKSConfig.Parameters.RoleServicePrincipals, ",")),
		})
	}
	if len(ts.cfg.EKSConfig.Parameters.RoleManagedPolicyARNs) > 0 {
		ts.cfg.Logger.Info("creating a new cluster role with custom managed role policies",
			zap.Strings("policy-arns", ts.cfg.EKSConfig.Parameters.RoleManagedPolicyARNs),
		)
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("RoleManagedPolicyARNs"),
			ParameterValue: aws.String(strings.Join(ts.cfg.EKSConfig.Parameters.RoleManagedPolicyARNs, ",")),
		})
	}
	stackOutput, err := ts.cfg.CFNAPI.CreateStack(stackInput)
	if err != nil {
		return err
	}
	ts.cfg.EKSConfig.Parameters.RoleCFNStackID = aws.StringValue(stackOutput.StackId)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	ch := cfn.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.CFNAPI,
		ts.cfg.EKSConfig.Parameters.RoleCFNStackID,
		cloudformation.ResourceStatusCreateComplete,
		25*time.Second,
		10*time.Second,
	)
	var st cfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to create role (%v)", st.Error))
			ts.cfg.Logger.Warn("polling errror", zap.Error(st.Error))
		}
	}
	cancel()
	if st.Error != nil {
		return st.Error
	}
	// update status after creating a new IAM role
	for _, o := range st.Stack.Outputs {
		switch k := aws.StringValue(o.OutputKey); k {
		case "RoleARN":
			ts.cfg.EKSConfig.Parameters.RoleARN = aws.StringValue(o.OutputValue)
		default:
			return fmt.Errorf("unexpected OutputKey %q from %q", k, ts.cfg.EKSConfig.Parameters.RoleCFNStackID)
		}
	}

	ts.cfg.Logger.Info("created a new role",
		zap.String("cluster-role-cfn-stack-id", ts.cfg.EKSConfig.Parameters.RoleCFNStackID),
		zap.String("cluster-role-arn", ts.cfg.EKSConfig.Parameters.RoleARN),
	)
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteClusterRole() error {
	if ts.cfg.EKSConfig.LogColor {
		colorstring.Printf("\n\n[yellow]*********************************\n")
		colorstring.Printf("[light_blue]deleteClusterRole [default](%q)\n", ts.cfg.EKSConfig.ConfigPath)
	} else {
		fmt.Printf("\n\n*********************************\n")
		fmt.Printf("deleteClusterRole (%q)\n", ts.cfg.EKSConfig.ConfigPath)
	}

	if !ts.cfg.EKSConfig.Parameters.RoleCreate {
		ts.cfg.Logger.Info("Parameters.RoleCreate false; skipping deletion")
		return nil
	}
	if ts.cfg.EKSConfig.Parameters.RoleCFNStackID == "" {
		ts.cfg.Logger.Info("empty role CFN stack ID; no need to delete role")
		return nil
	}

	ts.cfg.Logger.Info("deleting role CFN stack", zap.String("cluster-role-cfn-stack-id", ts.cfg.EKSConfig.Parameters.RoleCFNStackID))
	_, err := ts.cfg.CFNAPI.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(ts.cfg.EKSConfig.Parameters.RoleCFNStackID),
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	ch := cfn.Poll(
		ctx,
		make(chan struct{}), // do not exit on stop
		ts.cfg.Logger,
		ts.cfg.CFNAPI,
		ts.cfg.EKSConfig.Parameters.RoleCFNStackID,
		cloudformation.ResourceStatusDeleteComplete,
		25*time.Second,
		10*time.Second,
	)
	var st cfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to delete role (%v)", st.Error))
			ts.cfg.Logger.Warn("polling errror", zap.Error(st.Error))
		}
	}
	cancel()
	if st.Error != nil {
		return st.Error
	}
	ts.cfg.Logger.Info("deleted a role",
		zap.String("cluster-role-cfn-stack-id", ts.cfg.EKSConfig.Parameters.RoleCFNStackID),
		zap.String("cluster-role-arn", ts.cfg.EKSConfig.Parameters.RoleARN),
		zap.String("cluster-role-name", ts.cfg.EKSConfig.Parameters.RoleName),
	)
	return ts.cfg.EKSConfig.Sync()
}
