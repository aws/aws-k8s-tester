package mng

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/aws/cfn"
	aws_iam "github.com/aws/aws-k8s-tester/pkg/aws/iam"
	aws_s3 "github.com/aws/aws-k8s-tester/pkg/aws/s3"
	"github.com/aws/aws-k8s-tester/pkg/user"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"go.uber.org/zap"
)

// TemplateRole is the CloudFormation template for EKS managed node group role.
// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
// https://github.com/aws/eks-charts/tree/master/stable/appmesh-controller
// ref. https://github.com/kubernetes-sigs/aws-alb-ingress-controller/pull/1126
//
// TODO: create policy instead based off cluster role?
const TemplateRole = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Managed Node Group Role'

Parameters:

  RoleName:
    Type: String
    Description: The name of the node instance role

  RoleServicePrincipals:
    Type: CommaDelimitedList
    Default: 'ec2.amazonaws.com,eks.amazonaws.com'
    Description: EKS Managed Node Group Service Principals, use "ec2.amazonaws.com.cn" for China partition.

  RoleManagedPolicyARNs:
    Type: CommaDelimitedList
    Default: 'arn:aws:iam::aws:policy/AmazonEKSServicePolicy,arn:aws:iam::aws:policy/AmazonEKSClusterPolicy,arn:aws:iam::aws:policy/AmazonSSMFullAccess,arn:aws:iam::aws:policy/AmazonS3FullAccess,arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy,arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy,arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly,arn:aws:iam::aws:policy/CloudWatchAgentServerPolicy,arn:aws:iam::aws:policy/ElasticLoadBalancingFullAccess,arn:aws:iam::aws:policy/AmazonEKSFargatePodExecutionRolePolicy'
    Description: EKS Managed Node Group managed policy ARNs

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
      - PolicyName: !Join ['-', [!Ref RoleName, 'mng-policy-more']]
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
            - iam:SetSecurityTokenServicePreferences
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
{{ if ne .S3BucketName "" }}          - Effect: Allow
            Action:
            - s3:ListBucket
            - s3:GetObject
            - s3:PutObject
            Resource:
            - !Join ['', [!Sub 'arn:${AWS::Partition}:s3:::', '{{.S3BucketName}}']]
            - !Join ['', [!Sub 'arn:${AWS::Partition}:s3:::', '{{.S3BucketName}}', '/', '{{.ClusterName}}', '/*']]{{ end }}
{{ if ne .LogsPolicyData "" }}{{.LogsPolicyData}}{{ end }}

Outputs:

  RoleARN:
    Value: !GetAtt Role.Arn
    Description: The node instance role ARN

`

type templateRole struct {
	S3BucketName   string
	ClusterName    string
	LogsPolicyData string
}

const logsPolicyData = `          - Effect: Allow
            Action:
            - logs:CreateLogGroup
            - logs:CreateLogStream
            - logs:DescribeLogGroups
            - logs:DescribeLogStreams
            - logs:PutLogEvents
            Resource: "*"`

func (ts *tester) createRole() error {
	if !ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleCreate {
		policyARNs := []string{
			"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
			"arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
			"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
			"arn:aws:iam::aws:policy/AmazonSSMFullAccess",
			"arn:aws:iam::aws:policy/AmazonS3FullAccess",
		}
		// ref. https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-prerequisites.html
		if ts.cfg.EKSConfig.IsEnabledAddOnCWAgent() ||
			ts.cfg.EKSConfig.IsEnabledAddOnFluentd() {
			policyARNs = append(policyARNs, "arn:aws:iam::aws:policy/CloudWatchAgentServerPolicy")
		}
		if ts.cfg.EKSConfig.IsEnabledAddOnNLBHelloWorld() ||
			ts.cfg.EKSConfig.IsEnabledAddOnALB2048() {
			policyARNs = append(policyARNs, "arn:aws:iam::aws:policy/ElasticLoadBalancingFullAccess")
		}
		ts.cfg.Logger.Info("EKSConfig.AddOnManagedNodeGroups.RoleCreate false; skipping creation")
		return aws_iam.Validate(
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

	tr := templateRole{
		S3BucketName: ts.cfg.EKSConfig.S3BucketName,
		ClusterName:  ts.cfg.EKSConfig.Name,
	}
	if ts.cfg.EKSConfig.IsEnabledAddOnFluentd() {
		ts.cfg.Logger.Info("adding cloudwatch policy for container insights logs")
		tr.LogsPolicyData = logsPolicyData
	}
	tpl := template.Must(template.New("TemplateRole").Parse(TemplateRole))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, tr); err != nil {
		return err
	}
	// grant write permission in case of overwrites
	if err := ioutil.WriteFile(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleCFNStackYAMLPath, buf.Bytes(), 0600); err != nil {
		return err
	}
	if err := aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleCFNStackYAMLS3Key,
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleCFNStackYAMLPath,
	); err != nil {
		return err
	}
	ts.cfg.Logger.Info("creating a new node group role using CFN",
		zap.String("role-name", ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleName),
		zap.String("role-cfn-file-path", ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleCFNStackYAMLPath),
	)
	stackInput := &cloudformation.CreateStackInput{
		StackName:    aws.String(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleName),
		Capabilities: aws.StringSlice([]string{"CAPABILITY_NAMED_IAM"}),
		OnFailure:    aws.String(cloudformation.OnFailureDelete),
		TemplateBody: aws.String(buf.String()),
		Tags: cfn.NewTags(map[string]string{
			"Kind":                   "aws-k8s-tester",
			"Name":                   ts.cfg.EKSConfig.Name,
			"aws-k8s-tester-version": version.ReleaseVersion,
			"User":                   user.Get(),
		}),
		Parameters: []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String("RoleName"),
				ParameterValue: aws.String(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleName),
			},
		},
	}

	if len(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleServicePrincipals) > 0 {
		ts.cfg.Logger.Info("creating a new node group role with custom service principals",
			zap.Strings("service-principals", ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleServicePrincipals),
		)
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("RoleServicePrincipals"),
			ParameterValue: aws.String(strings.Join(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleServicePrincipals, ",")),
		})
	}

	if len(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleManagedPolicyARNs) > 0 {
		ts.cfg.Logger.Info("creating a new node group role with custom managed role policies",
			zap.Strings("policy-arns", ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleManagedPolicyARNs),
		)
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("RoleManagedPolicyARNs"),
			ParameterValue: aws.String(strings.Join(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleManagedPolicyARNs, ",")),
		})
	}

	stackOutput, err := ts.cfg.CFNAPI.CreateStack(stackInput)
	if err != nil {
		return err
	}
	ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleCFNStackID = aws.StringValue(stackOutput.StackId)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	ch := cfn.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.CFNAPI,
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleCFNStackID,
		cloudformation.ResourceStatusCreateComplete,
		time.Minute,
		10*time.Second,
	)
	var st cfn.StackStatus
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
		case "RoleARN":
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
	ch := cfn.Poll(
		ctx,
		make(chan struct{}), // do not exit on stop
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.CFNAPI,
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleCFNStackID,
		cloudformation.ResourceStatusDeleteComplete,
		time.Minute,
		10*time.Second,
	)
	var st cfn.StackStatus
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
