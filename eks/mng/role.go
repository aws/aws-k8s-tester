package mng

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	awscfn "github.com/aws/aws-k8s-tester/pkg/aws/cloudformation"
	awsiam "github.com/aws/aws-k8s-tester/pkg/aws/iam"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"go.uber.org/zap"
)

// MAKE SURE TO SYNC THE DEFAULT VALUES in "eksconfig"

// TemplateRole is the CloudFormation template for EKS managed node group role.
// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
const TemplateRole = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Cluster Node Group Managed Role'

Parameters:

  ManagedNodeGroupRoleName:
    Description: The name of the node instance role
    Type: String

  ManagedNodeGroupRoleServicePrincipals:
    Description: EKS Node Group Service Principals
    Type: CommaDelimitedList
    Default: 'ec2.amazonaws.com,eks.amazonaws.com'

  ManagedNodeGroupRoleManagedPolicyARNs:
    Description: EKS Node Group managed policy ARNs
    Type: CommaDelimitedList
    Default: 'arn:aws:iam::aws:policy/AmazonEKSServicePolicy,arn:aws:iam::aws:policy/AmazonEKSClusterPolicy,arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy,arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy,arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly,arn:aws:iam::aws:policy/ElasticLoadBalancingFullAccess,arn:aws:iam::aws:policy/AmazonEKSFargatePodExecutionRolePolicy'

Resources:

  ManagedNodeGroupRole:
    Type: AWS::IAM::Role
    Properties:
      RoleName: !Ref ManagedNodeGroupRoleName
      AssumeRolePolicyDocument:
        Version: '2012-10-17'
        Statement:
          - Effect: Allow
            Principal:
              Service: !Ref ManagedNodeGroupRoleServicePrincipals
            Action:
            - sts:AssumeRole
      ManagedPolicyArns: !Ref ManagedNodeGroupRoleManagedPolicyARNs
      Path: /
      Policies:
      # https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/iam-policy.json
      - PolicyName: !Join ['-', [!Ref ManagedNodeGroupRoleName, 'alb-policy']]
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
            - shield:DescribeProtection
            - shield:GetSubscriptionState
            - shield:DeleteProtection
            - shield:CreateProtection
            - shield:DescribeSubscription
            - shield:ListProtections
            Resource: "*"

Outputs:

  ManagedNodeGroupRoleARN:
    Description: The node instance role ARN
    Value: !GetAtt ManagedNodeGroupRole.Arn

`

func (ts *tester) createRole() error {
	if !ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleCreate {
		policyARNs := []string{
			"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
			"arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
			"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
		}
		if ts.cfg.EKSConfig.IsAddOnNLBHelloWorldEnabled() ||
			ts.cfg.EKSConfig.IsAddOnALB2048Enabled() {
			policyARNs = append(policyARNs, "arn:aws:iam::aws:policy/ElasticLoadBalancingFullAccess")
		}
		ts.cfg.Logger.Info("EKSConfig.AddOnManagedNodeGroups.RoleCreate false; skipping creation")
		return awsiam.Validate(
			ts.cfg.Logger,
			ts.cfg.IAMAPI,
			ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleName,
			[]string{
				"ec2.amazonaws.com",
				"eks.amazonaws.com",
			},
			policyARNs,
		)
	}
	if ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleCFNStackID != "" &&
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleARN != "" {
		ts.cfg.Logger.Info("role already created; no need to create a new one")
		return nil
	}
	if ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleName == "" {
		return errors.New("cannot create a cluster role with an empty AddOnManagedNodeGroups.RoleName")
	}

	ts.cfg.Logger.Info("creating a new node group role using CFN", zap.String("name", ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleName))
	stackInput := &cloudformation.CreateStackInput{
		StackName:    aws.String(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleName),
		Capabilities: aws.StringSlice([]string{"CAPABILITY_NAMED_IAM"}),
		OnFailure:    aws.String(cloudformation.OnFailureDelete),
		TemplateBody: aws.String(TemplateRole),
		Tags: awscfn.NewTags(map[string]string{
			"Kind":                   "aws-k8s-tester",
			"Name":                   ts.cfg.EKSConfig.Name,
			"aws-k8s-tester-version": version.ReleaseVersion,
		}),
		Parameters: []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String("ManagedNodeGroupRoleName"),
				ParameterValue: aws.String(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleName),
			},
		},
	}

	if len(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleServicePrincipals) > 0 {
		ts.cfg.Logger.Info("creating a new node group role with custom service principals",
			zap.Strings("service-principals", ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleServicePrincipals),
		)
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("ManagedNodeGroupRoleServicePrincipals"),
			ParameterValue: aws.String(strings.Join(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleServicePrincipals, ",")),
		})
	}

	if len(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleManagedPolicyARNs) > 0 {
		ts.cfg.Logger.Info("creating a new node group role with custom managed role policies",
			zap.Strings("policy-arns", ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleManagedPolicyARNs),
		)
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("ManagedNodeGroupRoleManagedPolicyARNs"),
			ParameterValue: aws.String(strings.Join(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleManagedPolicyARNs, ",")),
		})
	}

	stackOutput, err := ts.cfg.CFNAPI.CreateStack(stackInput)
	if err != nil {
		return err
	}
	ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleCFNStackID = aws.StringValue(stackOutput.StackId)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	ch := awscfn.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Sig,
		ts.cfg.Logger,
		ts.cfg.CFNAPI,
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleCFNStackID,
		cloudformation.ResourceStatusCreateComplete,
		time.Minute,
		10*time.Second,
	)
	var st awscfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to create managed node group role (%v)", st.Error))
			return st.Error
		}
	}
	cancel()

	for _, o := range st.Stack.Outputs {
		switch k := aws.StringValue(o.OutputKey); k {
		case "ManagedNodeGroupRoleARN":
			ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleARN = aws.StringValue(o.OutputValue)
		default:
			return fmt.Errorf("unexpected OutputKey %q from %q", k, ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleCFNStackID)
		}
	}

	ts.cfg.Logger.Info("created a managed node group role",
		zap.String("cfn-stack-id", ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleCFNStackID),
		zap.String("role-arn", ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleARN),
	)
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteRole() error {
	if !ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleCreate {
		ts.cfg.Logger.Info("AddOnManagedNodeGroups.RoleCreate false; skipping deletion")
		return nil
	}
	if ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleCFNStackID == "" {
		ts.cfg.Logger.Info("empty managed node group role CFN stack ID; no need to delete managed node group")
		return nil
	}

	ts.cfg.Logger.Info("deleting managed node group role CFN stack",
		zap.String("role-cfn-stack-id", ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleCFNStackID),
	)
	_, err := ts.cfg.CFNAPI.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleCFNStackID),
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	ch := awscfn.Poll(
		ctx,
		make(chan struct{}),  // do not exit on stop
		make(chan os.Signal), // do not exit on stop
		ts.cfg.Logger,
		ts.cfg.CFNAPI,
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleCFNStackID,
		cloudformation.ResourceStatusDeleteComplete,
		time.Minute,
		10*time.Second,
	)
	var st awscfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to delete managed node group role (%v)", st.Error))
			return st.Error
		}
	}
	cancel()
	ts.cfg.Logger.Info("deleted a managed node group role",
		zap.String("role-cfn-stack-id", ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleCFNStackID),
	)
	return ts.cfg.EKSConfig.Sync()
}
