package ec2

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

// TemplateRoleBasic is the CloudFormation template for EC2 role.
const TemplateRoleBasic = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EC2 Role'

Parameters:

  RoleName:
    Type: String
    Default: aws-k8s-tester-ec2-role
    Description: EC2 Role name

  RoleServicePrincipals:
    Type: CommaDelimitedList
    Default: 'ec2.amazonaws.com'
    Description: EC2 Role Service Principals

  RoleManagedPolicyARNs:
    Type: CommaDelimitedList
    Default: 'arn:aws:iam::aws:policy/AmazonEC2FullAccess,arn:aws:iam::aws:policy/AmazonSSMFullAccess,arn:aws:iam::aws:policy/AmazonS3FullAccess,arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly,arn:aws:iam::aws:policy/ElasticLoadBalancingFullAccess'
    Description: EC2 Role managed policy ARNs

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

Outputs:

  RoleARN:
    Value: !GetAtt Role.Arn
    Description: Role ARN that EC2 uses to create AWS resources

`

func (ts *Tester) createRole() error {
	if !ts.cfg.RoleCreate {
		ts.lg.Info("RoleCreate false; skipping creation")
		return awsiam.Validate(
			ts.lg,
			ts.iamAPI,
			ts.cfg.RoleName,
			[]string{"ec2.amazonaws.com"}, // TODO: support China regions ec2.amazonaws.com.cn or eks.amazonaws.com.cn
			[]string{
				"arn:aws:iam::aws:policy/AmazonEC2FullAccess",
				"arn:aws:iam::aws:policy/AmazonSSMFullAccess",
				"arn:aws:iam::aws:policy/AmazonS3FullAccess",
			},
		)
	}
	if ts.cfg.RoleCFNStackID != "" &&
		ts.cfg.RoleARN != "" {
		ts.lg.Info("role already created; no need to create a new one")
		return nil
	}
	if ts.cfg.RoleName == "" {
		return errors.New("cannot create a cluster role with an empty Parameters.RoleName")
	}

	tmpl := TemplateRoleBasic

	// role ARN is empty, create a default role
	// otherwise, use the existing one
	ts.lg.Info("creating a new role", zap.String("role-name", ts.cfg.RoleName))
	stackInput := &cloudformation.CreateStackInput{
		StackName:    aws.String(ts.cfg.RoleName),
		Capabilities: aws.StringSlice([]string{"CAPABILITY_NAMED_IAM"}),
		OnFailure:    aws.String(cloudformation.OnFailureDelete),
		TemplateBody: aws.String(tmpl),
		Tags: awscfn.NewTags(map[string]string{
			"Kind":                   "aws-k8s-tester",
			"Name":                   ts.cfg.Name,
			"aws-k8s-tester-version": version.ReleaseVersion,
		}),
		Parameters: []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String("RoleName"),
				ParameterValue: aws.String(ts.cfg.RoleName),
			},
		},
	}
	if len(ts.cfg.RoleServicePrincipals) > 0 {
		ts.lg.Info("creating a new role with custom service principals",
			zap.Strings("service-principals", ts.cfg.RoleServicePrincipals),
		)
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("RoleServicePrincipals"),
			ParameterValue: aws.String(strings.Join(ts.cfg.RoleServicePrincipals, ",")),
		})
	}
	if len(ts.cfg.RoleManagedPolicyARNs) > 0 {
		ts.lg.Info("creating a new role with custom managed role policies",
			zap.Strings("policy-arns", ts.cfg.RoleManagedPolicyARNs),
		)
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("RoleManagedPolicyARNs"),
			ParameterValue: aws.String(strings.Join(ts.cfg.RoleManagedPolicyARNs, ",")),
		})
	}
	stackOutput, err := ts.cfnAPI.CreateStack(stackInput)
	if err != nil {
		return err
	}
	ts.cfg.RoleCFNStackID = aws.StringValue(stackOutput.StackId)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	ch := awscfn.Poll(
		ctx,
		ts.stopCreationCh,
		ts.interruptSig,
		ts.lg,
		ts.cfnAPI,
		ts.cfg.RoleCFNStackID,
		cloudformation.ResourceStatusCreateComplete,
		25*time.Second,
		10*time.Second,
	)
	var st awscfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.RecordStatus(fmt.Sprintf("failed to create role (%v)", st.Error))
			ts.lg.Warn("polling errror", zap.Error(st.Error))
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
			ts.cfg.RoleARN = aws.StringValue(o.OutputValue)
		default:
			return fmt.Errorf("unexpected OutputKey %q from %q", k, ts.cfg.RoleCFNStackID)
		}
	}

	ts.lg.Info("created a new role",
		zap.String("role-cfn-stack-id", ts.cfg.RoleCFNStackID),
		zap.String("role-arn", ts.cfg.RoleARN),
	)
	return ts.cfg.Sync()
}

func (ts *Tester) deleteRole() error {
	if !ts.cfg.RoleCreate {
		ts.lg.Info("RoleCreate false; skipping deletion")
		return nil
	}
	if ts.cfg.RoleCFNStackID == "" {
		ts.lg.Info("empty role CFN stack ID; no need to delete role")
		return nil
	}

	ts.lg.Info("deleting role CFN stack", zap.String("role-cfn-stack-id", ts.cfg.RoleCFNStackID))
	_, err := ts.cfnAPI.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(ts.cfg.RoleCFNStackID),
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	ch := awscfn.Poll(
		ctx,
		make(chan struct{}),  // do not exit on stop
		make(chan os.Signal), // do not exit on stop
		ts.lg,
		ts.cfnAPI,
		ts.cfg.RoleCFNStackID,
		cloudformation.ResourceStatusDeleteComplete,
		25*time.Second,
		10*time.Second,
	)
	var st awscfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.RecordStatus(fmt.Sprintf("failed to delete role (%v)", st.Error))
			ts.lg.Warn("polling errror", zap.Error(st.Error))
		}
	}
	cancel()
	if st.Error != nil {
		return st.Error
	}
	ts.lg.Info("deleted a role",
		zap.String("role-cfn-stack-id", ts.cfg.RoleCFNStackID),
		zap.String("role-arn", ts.cfg.RoleARN),
		zap.String("role-name", ts.cfg.RoleName),
	)
	return ts.cfg.Sync()
}
