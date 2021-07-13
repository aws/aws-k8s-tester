package cluster

import (
	"errors"
	"fmt"

	aws_iam "github.com/aws/aws-k8s-tester/pkg/aws/iam"
	"go.uber.org/zap"
)

// TemplateClusterRole is the CloudFormation template for EKS cluster role.
//
// ref. https://amazon-eks.s3.us-west-2.amazonaws.com/cloudformation/2019-09-17/amazon-eks-service-role.yaml
//
// e.g.
//   Error creating load balancer (will retry): failed to ensure load balancer for service eks-*/hello-world-service: Error creating load balancer: "AccessDenied: User: arn:aws:sts::404174646922:assumed-role/eks-*-cluster-role/* is not authorized to perform: ec2:DescribeAccountAttributes\n\tstatus code: 403"
//
// TODO: scope down (e.g. ec2:DescribeAccountAttributes, ec2:DescribeInternetGateways)
// mng, fargate, etc. may use other roles
const TemplateClusterRole = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Cluster Role'

Parameters:

  RoleName:
    Type: String
    Default: aws-k8s-tester-eks-role
    Description: EKS Role name

  RoleServicePrincipals:
    Type: CommaDelimitedList
    Default: 'ec2.amazonaws.com,eks.amazonaws.com,eks-fargate-pods.amazonaws.com'
    Description: EKS Role Service Principals, use "ec2.amazonaws.com.cn" for China partition.

  RoleManagedPolicyARNs:
    Type: CommaDelimitedList
    Default: 'arn:aws:iam::aws:policy/AmazonEKSClusterPolicy,arn:aws:iam::aws:policy/AmazonSSMFullAccess,arn:aws:iam::aws:policy/AmazonS3FullAccess,arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy,arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy,arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly,arn:aws:iam::aws:policy/ElasticLoadBalancingFullAccess'
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

// Prior to April 16, 2020, AmazonEKSServicePolicy was also required and the suggested name was eksServiceRole. With the AWSServiceRoleForAmazonEKS service-linked role, that policy is no longer required for clusters created on or after April 16, 2020.
// ref. https://docs.aws.amazon.com/eks/latest/userguide/service_IAM_role.html

func (ts *tester) createClusterRole() error {
	fmt.Print(ts.cfg.EKSConfig.Colorize("\n\n[yellow]*********************************\n"))
	fmt.Printf(ts.cfg.EKSConfig.Colorize("[light_green]createClusterRole [default](%q)\n"), ts.cfg.EKSConfig.ConfigPath)

	if !ts.cfg.EKSConfig.Role.Create {
		ts.cfg.Logger.Info("Role.Create false; skipping creation")
		return aws_iam.ValidateV2(
			ts.cfg.Logger,
			ts.cfg.IAMAPIV2,
			ts.cfg.EKSConfig.Role.Name,
			[]string{"eks.amazonaws.com"},
			[]string{
				"arn:aws:iam::aws:policy/AmazonEKSClusterPolicy",
			},
		)
	}
	if ts.cfg.EKSConfig.Role.ARN != "" {
		ts.cfg.Logger.Info("role already created; no need to create a new one")
		return nil
	}
	if ts.cfg.EKSConfig.Role.Name == "" {
		return errors.New("cannot create a cluster role with an empty Role.Name")
	}

	ts.cfg.Logger.Info("created a new role",
		zap.String("cluster-role-arn", ts.cfg.EKSConfig.Role.ARN),
	)
	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) deleteClusterRole() error {
	fmt.Print(ts.cfg.EKSConfig.Colorize("\n\n[yellow]*********************************\n"))
	fmt.Printf(ts.cfg.EKSConfig.Colorize("[light_blue]deleteClusterRole [default](%q)\n"), ts.cfg.EKSConfig.ConfigPath)

	if !ts.cfg.EKSConfig.Role.Create {
		ts.cfg.Logger.Info("Role.Create false; skipping deletion")
		return nil
	}

	ts.cfg.Logger.Info("deleted a role",
		zap.String("cluster-role-arn", ts.cfg.EKSConfig.Role.ARN),
		zap.String("cluster-role-name", ts.cfg.EKSConfig.Role.Name),
	)
	ts.cfg.EKSConfig.Sync()
	return nil
}
