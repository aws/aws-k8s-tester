package eks

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	awsapicfn "github.com/aws/aws-k8s-tester/pkg/awsapi/cloudformation"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"go.uber.org/zap"
)

// TemplateClusterRole is the CloudFormation template for EKS cluster role.
const TemplateClusterRole = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Cluster Role'

Parameters:

  ClusterRoleName:
    Description: EKS Role name
    Type: String

  ClusterRoleServicePrincipals:
    Description: EKS Role Service Principals
    Type: CommaDelimitedList
    Default: eks.amazonaws.com

  ClusterRoleManagedPolicyARNs:
    Description: EKS Role managed policy ARNs
    Type: CommaDelimitedList
    Default: 'arn:aws:iam::aws:policy/AmazonEKSServicePolicy,arn:aws:iam::aws:policy/AmazonEKSClusterPolicy'

Resources:

  ClusterRole:
    Type: AWS::IAM::Role
    Properties:
      RoleName: !Ref ClusterRoleName
      AssumeRolePolicyDocument:
        Version: '2012-10-17'
        Statement:
        - Effect: Allow
          Principal:
            Service: !Ref ClusterRoleServicePrincipals
          Action:
          - sts:AssumeRole
      ManagedPolicyArns: !Ref ClusterRoleManagedPolicyARNs
      Path: /

Outputs:

  ClusterRoleARN:
    Description: Cluster role ARN that EKS uses to create AWS resources for Kubernetes
    Value: !GetAtt ClusterRole.Arn

`

func (ts *Tester) createClusterRole() error {
	if ts.cfg.Parameters.ClusterRoleARN != "" ||
		ts.cfg.Status.ClusterRoleCFNStackID != "" ||
		ts.cfg.Status.ClusterRoleARN != "" ||
		ts.cfg.Status.ClusterRoleName != "" {
		ts.lg.Info("non-empty role given; no need to create a new one")
		return nil
	}

	ts.cfg.Status.ClusterRoleName = ts.cfg.Name + "-cluster-role"

	// role ARN is empty, create a default role
	// otherwise, use the existing one
	ts.lg.Info("creating a new role", zap.String("cluster-role-name", ts.cfg.Status.ClusterRoleName))
	stackInput := &cloudformation.CreateStackInput{
		StackName:    aws.String(ts.cfg.Status.ClusterRoleName),
		Capabilities: aws.StringSlice([]string{"CAPABILITY_NAMED_IAM"}),
		OnFailure:    aws.String("DELETE"),
		TemplateBody: aws.String(TemplateClusterRole),
		Tags: awsapicfn.NewTags(map[string]string{
			"Kind": "aws-k8s-tester",
			"Name": ts.cfg.Name,
		}),
		Parameters: []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String("ClusterRoleName"),
				ParameterValue: aws.String(ts.cfg.Status.ClusterRoleName),
			},
		},
	}
	if len(ts.cfg.Parameters.ClusterRoleServicePrincipals) > 0 {
		ts.lg.Info("creating a new cluster role with custom service principals",
			zap.Strings("service-principals", ts.cfg.Parameters.ClusterRoleServicePrincipals),
		)
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("ClusterRoleServicePrincipals"),
			ParameterValue: aws.String(strings.Join(ts.cfg.Parameters.ClusterRoleServicePrincipals, ",")),
		})
	}
	if len(ts.cfg.Parameters.ClusterRoleManagedPolicyARNs) > 0 {
		ts.lg.Info("creating a new cluster role with custom managed role policies",
			zap.Strings("policy-arns", ts.cfg.Parameters.ClusterRoleManagedPolicyARNs),
		)
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("ClusterRoleManagedPolicyARNs"),
			ParameterValue: aws.String(strings.Join(ts.cfg.Parameters.ClusterRoleManagedPolicyARNs, ",")),
		})
	}
	stackOutput, err := ts.cfnAPI.CreateStack(stackInput)
	if err != nil {
		return err
	}
	ts.cfg.Status.ClusterRoleCFNStackID = aws.StringValue(stackOutput.StackId)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	ch := awsapicfn.Poll(
		ctx,
		ts.stopCreationCh,
		ts.interruptSig,
		ts.lg,
		ts.cfnAPI,
		ts.cfg.Status.ClusterRoleCFNStackID,
		cloudformation.ResourceStatusCreateComplete,
		25*time.Second,
		10*time.Second,
	)
	var st awsapicfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.Status.ClusterStatus = fmt.Sprintf("failed to create role (%v)", st.Error)
			ts.cfg.Sync()
			ts.lg.Error("polling errror", zap.Error(st.Error))
		}
	}
	cancel()
	if st.Error != nil {
		return st.Error
	}
	// update status after creating a new IAM role
	for _, o := range st.Stack.Outputs {
		switch k := aws.StringValue(o.OutputKey); k {
		case "ClusterRoleARN":
			ts.cfg.Status.ClusterRoleARN = aws.StringValue(o.OutputValue)
		default:
			return fmt.Errorf("unexpected OutputKey %q from %q", k, ts.cfg.Status.ClusterRoleCFNStackID)
		}
	}

	ts.lg.Info("created a new role",
		zap.String("cluster-role-cfn-stack-id", ts.cfg.Status.ClusterRoleCFNStackID),
		zap.String("cluster-role-arn", ts.cfg.Status.ClusterRoleARN),
		zap.String("cluster-role-name", ts.cfg.Status.ClusterRoleName),
	)
	return ts.cfg.Sync()
}

func (ts *Tester) deleteClusterRole() error {
	if ts.cfg.Status.ClusterRoleCFNStackID == "" {
		ts.lg.Info("empty role CFN stack ID; no need to delete role")
		return nil
	}

	ts.lg.Info("deleting role CFN stack", zap.String("cluster-role-cfn-stack-id", ts.cfg.Status.ClusterRoleCFNStackID))
	_, err := ts.cfnAPI.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(ts.cfg.Status.ClusterRoleCFNStackID),
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	ch := awsapicfn.Poll(
		ctx,
		make(chan struct{}),  // do not exit on stop
		make(chan os.Signal), // do not exit on stop
		ts.lg,
		ts.cfnAPI,
		ts.cfg.Status.ClusterRoleCFNStackID,
		cloudformation.ResourceStatusDeleteComplete,
		25*time.Second,
		10*time.Second,
	)
	var st awsapicfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.Status.ClusterStatus = fmt.Sprintf("failed to delete role (%v)", st.Error)
			ts.cfg.Sync()
			ts.lg.Error("polling errror", zap.Error(st.Error))
		}
	}
	cancel()
	if st.Error != nil {
		return st.Error
	}
	ts.lg.Info("deleted a role",
		zap.String("cluster-role-cfn-stack-id", ts.cfg.Status.ClusterRoleCFNStackID),
		zap.String("cluster-role-arn", ts.cfg.Status.ClusterRoleARN),
		zap.String("cluster-role-name", ts.cfg.Status.ClusterRoleName),
	)
	return ts.cfg.Sync()
}
