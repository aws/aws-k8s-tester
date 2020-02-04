package mng

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	awsapicfn "github.com/aws/aws-k8s-tester/pkg/awsapi/cloudformation"
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
    Default: 'arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy,arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy,arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly'

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

Outputs:

  ManagedNodeGroupRoleARN:
    Description: The node instance role ARN
    Value: !GetAtt ManagedNodeGroupRole.Arn

`

func (ts *tester) createRole() error {
	if ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleName == "" {
		return errors.New("empty Parameters.ManagedNodeGroupRoleName")
	}
	if ts.cfg.EKSConfig.StatusManagedNodeGroups.RoleCFNStackID != "" ||
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleARN != "" {
		ts.cfg.Logger.Info("non-empty roleARN given; no need to create a new one")
		return nil
	}

	// node group attributes are empty, create a new VPC
	// otherwise, use the existing one
	ts.cfg.Logger.Info("creating a new node group role using CFN", zap.String("name", ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleName))
	stackInput := &cloudformation.CreateStackInput{
		StackName:    aws.String(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleName),
		Capabilities: aws.StringSlice([]string{"CAPABILITY_NAMED_IAM"}),
		OnFailure:    aws.String("DELETE"),
		TemplateBody: aws.String(TemplateRole),
		Tags: awsapicfn.NewTags(map[string]string{
			"Kind": "aws-k8s-tester",
			"Name": ts.cfg.EKSConfig.Name,
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
	ts.cfg.EKSConfig.StatusManagedNodeGroups.RoleCFNStackID = aws.StringValue(stackOutput.StackId)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	ch := awsapicfn.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Sig,
		ts.cfg.Logger,
		ts.cfg.CFNAPI,
		ts.cfg.EKSConfig.StatusManagedNodeGroups.RoleCFNStackID,
		cloudformation.ResourceStatusCreateComplete,
		time.Minute,
		10*time.Second,
	)
	var st awsapicfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.Status.ClusterStatus = fmt.Sprintf("failed to create managed node group role (%v)", st.Error)
			ts.cfg.EKSConfig.Sync()
			return st.Error
		}
	}
	cancel()

	for _, o := range st.Stack.Outputs {
		switch k := aws.StringValue(o.OutputKey); k {
		case "ManagedNodeGroupRoleARN":
			ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleARN = aws.StringValue(o.OutputValue)
		default:
			return fmt.Errorf("unexpected OutputKey %q from %q", k, ts.cfg.EKSConfig.StatusManagedNodeGroups.RoleCFNStackID)
		}
	}

	ts.cfg.Logger.Info("created a managed node group role",
		zap.String("cfn-stack-id", ts.cfg.EKSConfig.StatusManagedNodeGroups.RoleCFNStackID),
		zap.String("role-name", ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleName),
		zap.String("role-arn", ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleARN),
	)
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteRole() error {
	if ts.cfg.EKSConfig.StatusManagedNodeGroups.RoleCFNStackID == "" {
		ts.cfg.Logger.Info("empty managed node group role CFN stack ID; no need to delete managed node group")
		return nil
	}

	ts.cfg.Logger.Info("deleting managed node group role CFN stack",
		zap.String("role-cfn-stack-id", ts.cfg.EKSConfig.StatusManagedNodeGroups.RoleCFNStackID),
	)
	_, err := ts.cfg.CFNAPI.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(ts.cfg.EKSConfig.StatusManagedNodeGroups.RoleCFNStackID),
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	ch := awsapicfn.Poll(
		ctx,
		make(chan struct{}),  // do not exit on stop
		make(chan os.Signal), // do not exit on stop
		ts.cfg.Logger,
		ts.cfg.CFNAPI,
		ts.cfg.EKSConfig.StatusManagedNodeGroups.RoleCFNStackID,
		cloudformation.ResourceStatusDeleteComplete,
		time.Minute,
		10*time.Second,
	)
	var st awsapicfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.Status.ClusterStatus = fmt.Sprintf("failed to delete managed node group role (%v)", st.Error)
			ts.cfg.EKSConfig.Sync()
			return st.Error
		}
	}
	cancel()
	ts.cfg.Logger.Info("deleted a managed node group role",
		zap.String("role-cfn-stack-id", ts.cfg.EKSConfig.StatusManagedNodeGroups.RoleCFNStackID),
	)
	return ts.cfg.EKSConfig.Sync()
}
