package alb

import (
	"context"
	"fmt"
	"os"
	"time"

	awscfn "github.com/aws/aws-k8s-tester/pkg/aws/cloudformation"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"go.uber.org/zap"
)

// TemplateALBIngressControllerPolicy is the CloudFormation template for ALB Ingress Controller Policy.
// https://github.com/kubernetes-sigs/aws-alb-ingress-controller
// https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/iam-policy.json
// https://docs.aws.amazon.com/eks/latest/userguide/alb-ingress.html
const TemplateALBIngressControllerPolicy = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS ALB Ingress Controller Policy'

Parameters:

  ALBIngressControllerPolicyName:
    Description: The policy name for ALB Ingress Controller
    Type: String

  ManagedNodeGroupRoleName:
    Description: The name of the node instance role
    Type: String

Resources:

  ALBIngressControllerPolicy:
    Metadata:
      Comment: Minimal policy to allow worker node instance profile that allows the ALB Ingress Controller to make calls to AWS APIs on your behalf
    Type: AWS::IAM::Policy
    Properties:
      PolicyName: !Ref ALBIngressControllerPolicyName
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
      Roles:
      - !Ref ManagedNodeGroupRoleName

`

// https://docs.aws.amazon.com/eks/latest/userguide/alb-ingress.html
// https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/iam-policy.json
func (ts *tester) createALBPolicy() error {
	policyName := ts.cfg.EKSConfig.Name + "-alb-policy"

	ts.cfg.Logger.Info("creating ALB Ingress Controller Policy")
	stackInput := &cloudformation.CreateStackInput{
		StackName:    aws.String(policyName),
		Capabilities: aws.StringSlice([]string{"CAPABILITY_NAMED_IAM"}),
		OnFailure:    aws.String(cloudformation.OnFailureDelete),
		TemplateBody: aws.String(TemplateALBIngressControllerPolicy),
		Tags: awscfn.NewTags(map[string]string{
			"Kind": "aws-k8s-tester",
			"Name": ts.cfg.EKSConfig.Name,
		}),
		Parameters: []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String("ALBIngressControllerPolicyName"),
				ParameterValue: aws.String(policyName),
			},
			{
				ParameterKey:   aws.String("ManagedNodeGroupRoleName"),
				ParameterValue: aws.String(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleName),
			},
		},
	}
	stackOutput, err := ts.cfg.CFNAPI.CreateStack(stackInput)
	if err != nil {
		return err
	}
	ts.policyCFNStackID = aws.StringValue(stackOutput.StackId)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	ch := awscfn.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Sig,
		ts.cfg.Logger,
		ts.cfg.CFNAPI,
		ts.policyCFNStackID,
		cloudformation.ResourceStatusCreateComplete,
		25*time.Second,
		10*time.Second,
	)
	var st awscfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to wait for ALB Ingress Controller Policy creation (%v)", st.Error))
			ts.cfg.Logger.Error("polling errror", zap.Error(st.Error))
		}
	}
	cancel()
	if st.Error != nil {
		return st.Error
	}

	ts.cfg.Logger.Info("created ALB Ingress Controller Policy",
		zap.String("policy-cfn-stack-id", ts.policyCFNStackID),
		zap.String("policy-name", policyName),
	)

	return ts.cfg.EKSConfig.Sync()
}

// https://docs.aws.amazon.com/eks/latest/userguide/alb-ingress.html
// https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/iam-policy.json
func (ts *tester) deleteALBPolicy() error {
	if ts.policyCFNStackID == "" {
		ts.cfg.Logger.Info("empty policy CFN stack ID; no need to delete policy")
		return nil
	}

	ts.cfg.Logger.Info("deleting ALB Ingress Controller Policy",
		zap.String("policy-cfn-stack-id", ts.policyCFNStackID),
	)
	_, err := ts.cfg.CFNAPI.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(ts.policyCFNStackID),
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	ch := awscfn.Poll(
		ctx,
		make(chan struct{}),  // do not exit on stop
		make(chan os.Signal), // do not exit on stop
		ts.cfg.Logger,
		ts.cfg.CFNAPI,
		ts.policyCFNStackID,
		cloudformation.ResourceStatusDeleteComplete,
		25*time.Second,
		10*time.Second,
	)
	var st awscfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to wait for ALB Ingress Controller Policy deletion (%v)", st.Error))
			ts.cfg.Logger.Error("polling errror", zap.Error(st.Error))
		}
	}
	cancel()
	if st.Error != nil {
		return st.Error
	}
	ts.cfg.Logger.Info("deleted ALB Ingress Controller Policy",
		zap.String("policy-cfn-stack-id", ts.policyCFNStackID),
	)

	return ts.cfg.EKSConfig.Sync()
}
