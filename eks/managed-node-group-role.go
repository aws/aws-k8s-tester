package eks

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

// TemplateManagedNodeGroupRole is the CloudFormation template for EKS managed node group role.
// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
const TemplateManagedNodeGroupRole = `
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

func (ts *Tester) createManagedNodeGroupRole() error {
	if ts.cfg.Parameters.ManagedNodeGroupRoleName == "" {
		return errors.New("empty Parameters.ManagedNodeGroupRoleName")
	}
	if ts.cfg.Status.ManagedNodeGroupRoleCFNStackID != "" ||
		ts.cfg.Parameters.ManagedNodeGroupRoleARN != "" {
		ts.lg.Info("non-empty  roleARN given; no need to create a new one")
		return nil
	}

	// node group attributes are empty, create a new VPC
	// otherwise, use the existing one
	ts.lg.Info("creating a new node group role using CFN", zap.String("name", ts.cfg.Parameters.ManagedNodeGroupRoleName))
	stackInput := &cloudformation.CreateStackInput{
		StackName:    aws.String(ts.cfg.Parameters.ManagedNodeGroupRoleName),
		Capabilities: aws.StringSlice([]string{"CAPABILITY_NAMED_IAM"}),
		OnFailure:    aws.String("DELETE"),
		TemplateBody: aws.String(TemplateManagedNodeGroupRole),
		Tags: awsapicfn.NewTags(map[string]string{
			"Kind": "aws-k8s-tester",
			"Name": ts.cfg.Name,
		}),
		Parameters: []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String("ManagedNodeGroupRoleName"),
				ParameterValue: aws.String(ts.cfg.Parameters.ManagedNodeGroupRoleName),
			},
		},
	}
	if len(ts.cfg.Parameters.ManagedNodeGroupRoleServicePrincipals) > 0 {
		ts.lg.Info("creating a new node group role with custom service principals",
			zap.Strings("service-principals", ts.cfg.Parameters.ManagedNodeGroupRoleServicePrincipals),
		)
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("ManagedNodeGroupRoleServicePrincipals"),
			ParameterValue: aws.String(strings.Join(ts.cfg.Parameters.ManagedNodeGroupRoleServicePrincipals, ",")),
		})
	}
	if len(ts.cfg.Parameters.ManagedNodeGroupRoleManagedPolicyARNs) > 0 {
		ts.lg.Info("creating a new node group role with custom managed role policies",
			zap.Strings("policy-arns", ts.cfg.Parameters.ManagedNodeGroupRoleManagedPolicyARNs),
		)
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("ManagedNodeGroupRoleManagedPolicyARNs"),
			ParameterValue: aws.String(strings.Join(ts.cfg.Parameters.ManagedNodeGroupRoleManagedPolicyARNs, ",")),
		})
	}
	stackOutput, err := ts.cfnAPI.CreateStack(stackInput)
	if err != nil {
		return err
	}
	ts.cfg.Status.ManagedNodeGroupRoleCFNStackID = aws.StringValue(stackOutput.StackId)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	ch := awsapicfn.Poll(
		ctx,
		ts.stopCreationCh,
		ts.interruptSig,
		ts.lg,
		ts.cfnAPI,
		ts.cfg.Status.ManagedNodeGroupRoleCFNStackID,
		cloudformation.ResourceStatusCreateComplete,
		time.Minute,
		10*time.Second,
	)
	var st awsapicfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.Status.ClusterStatus = fmt.Sprintf("failed to create managed node group role (%v)", st.Error)
			ts.cfg.Sync()
			return st.Error
		}
	}
	cancel()
	for _, o := range st.Stack.Outputs {
		switch k := aws.StringValue(o.OutputKey); k {
		case "ManagedNodeGroupRoleARN":
			ts.cfg.Parameters.ManagedNodeGroupRoleARN = aws.StringValue(o.OutputValue)
		default:
			return fmt.Errorf("unexpected OutputKey %q from %q", k, ts.cfg.Status.ManagedNodeGroupRoleCFNStackID)
		}
	}
	ts.lg.Info("created a managed node group role",
		zap.String("managed-node-group-cfn-stack-id", ts.cfg.Status.ManagedNodeGroupRoleCFNStackID),
		zap.String("managed-node-group-role-name", ts.cfg.Parameters.ManagedNodeGroupRoleName),
		zap.String("managed-node-group-role-arn", ts.cfg.Parameters.ManagedNodeGroupRoleARN),
	)
	return ts.cfg.Sync()
}

func (ts *Tester) deleteManagedNodeGroupRole() error {
	if ts.cfg.Status.ManagedNodeGroupRoleCFNStackID == "" {
		ts.lg.Info("empty managed node group role CFN stack ID; no need to delete managed node group")
		return nil
	}

	ts.lg.Info("deleting managed node group role CFN stack",
		zap.String("managed-node-group-role-cfn-stack-id", ts.cfg.Status.ManagedNodeGroupRoleCFNStackID),
	)
	_, err := ts.cfnAPI.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(ts.cfg.Status.ManagedNodeGroupRoleCFNStackID),
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	ch := awsapicfn.Poll(
		ctx,
		make(chan struct{}),  // do not exit on stop
		make(chan os.Signal), // do not exit on stop
		ts.lg,
		ts.cfnAPI,
		ts.cfg.Status.ManagedNodeGroupRoleCFNStackID,
		cloudformation.ResourceStatusDeleteComplete,
		time.Minute,
		10*time.Second,
	)
	var st awsapicfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.Status.ClusterStatus = fmt.Sprintf("failed to delete managed node group role (%v)", st.Error)
			ts.cfg.Sync()
			return st.Error
		}
	}
	cancel()
	ts.lg.Info("deleted a managed node group role",
		zap.String("managed-node-group-role-cfn-stack-id", ts.cfg.Status.ManagedNodeGroupRoleCFNStackID),
	)
	return ts.cfg.Sync()
}
