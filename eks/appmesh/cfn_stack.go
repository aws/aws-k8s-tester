package appmesh

import (
	"context"
	"fmt"
	"os"
	"time"

	awscfn "github.com/aws/aws-k8s-tester/pkg/aws/cloudformation"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"go.uber.org/zap"
)

const addOnCFNStackTemplate = `
---
AWSTemplateFormatVersion: "2010-09-09"
Description: "Amazon EKS AppMesh Controller AddOn stack"

Parameters:
  AppMeshControllerPolicyName:
    Description: The policy name for AppMesh Controller
    Type: String
  ManagedNodeGroupRoleName:
    Description: The name of the node instance role
    Type: String
Resources:
  AppMeshControllerPolicy:
    Metadata:
      Comment: Minimal policy to allow worker node instance profile that allows the AppMesh Controller to make calls to AWS APIs on your behalf
    Type: AWS::IAM::Policy
    Properties:
      PolicyName: !Ref AppMeshControllerPolicyName
      PolicyDocument:
        Version: "2012-10-17"
        Statement:
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
      Roles:
        - !Ref ManagedNodeGroupRoleName
`

// createAppMeshAddOnCFNStack creates the cfn stack needed for AppMesh addOn.
func (ts *tester) createAppMeshAddOnCFNStack() error {
	if ts.cfg.EKSConfig.AddOnAppMesh.AddOnCFNStackARN != "" {
		ts.cfg.Logger.Info("already created AppMesh Controller AddOn CFN stack, ignoring")
		return nil
	}

	ts.cfg.Logger.Info("creating AppMesh Controller AddOn CFN stack")

	stackName := ts.cfg.EKSConfig.Name + "-app-mesh-addOn"
	policyName := ts.cfg.EKSConfig.Name + "-app-mesh-policy"
	stackInput := &cloudformation.CreateStackInput{
		StackName:    aws.String(stackName),
		Capabilities: aws.StringSlice([]string{"CAPABILITY_NAMED_IAM"}),
		OnFailure:    aws.String(cloudformation.OnFailureDelete),
		TemplateBody: aws.String(addOnCFNStackTemplate),
		Tags: awscfn.NewTags(map[string]string{
			"Kind":                   "aws-k8s-tester",
			"Name":                   ts.cfg.EKSConfig.Name,
			"aws-k8s-tester-version": version.ReleaseVersion,
		}),
		Parameters: []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String("AppMeshControllerPolicyName"),
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
	ts.cfg.EKSConfig.AddOnAppMesh.AddOnCFNStackARN = aws.StringValue(stackOutput.StackId)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	ch := awscfn.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Sig,
		ts.cfg.Logger,
		ts.cfg.CFNAPI,
		ts.cfg.EKSConfig.AddOnAppMesh.AddOnCFNStackARN,
		cloudformation.ResourceStatusCreateComplete,
		25*time.Second,
		10*time.Second,
	)
	var st awscfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to wait for AppMesh Controller AddOn CFN stack creation (%v)", st.Error))
			ts.cfg.Logger.Error("polling error", zap.Error(st.Error))
		}
	}
	cancel()
	if st.Error != nil {
		return st.Error
	}

	ts.cfg.Logger.Info("created AppMesh Controller AddOn CFN stack",
		zap.String("add-on-stack-arn", ts.cfg.EKSConfig.AddOnAppMesh.AddOnCFNStackARN),
		zap.String("policy-name", policyName),
	)

	return ts.cfg.EKSConfig.Sync()
}

// deleteAppMeshAddOnCFNStack deletes the cfn stack needed for AppMesh addOn.
func (ts *tester) deleteAppMeshAddOnCFNStack() error {
	if ts.cfg.EKSConfig.AddOnAppMesh.AddOnCFNStackARN == "" {
		ts.cfg.Logger.Info("empty AppMesh Controller AddOn CFN stack, no need to delete")
		return nil
	}

	ts.cfg.Logger.Info("deleting AppMesh Controller AddOn CFN stack",
		zap.String("add-on-stack-arn", ts.cfg.EKSConfig.AddOnAppMesh.AddOnCFNStackARN),
	)

	_, err := ts.cfg.CFNAPI.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(ts.cfg.EKSConfig.AddOnAppMesh.AddOnCFNStackARN),
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
		ts.cfg.EKSConfig.AddOnAppMesh.AddOnCFNStackARN,
		cloudformation.ResourceStatusDeleteComplete,
		25*time.Second,
		10*time.Second,
	)
	var st awscfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to wait for AppMesh Controller AddOn CFN stack  deletion (%v)", st.Error))
			ts.cfg.Logger.Error("polling error", zap.Error(st.Error))
		}
	}
	cancel()
	if st.Error != nil {
		return st.Error
	}
	ts.cfg.Logger.Info("AppMesh Controller AddOn CFN stack",
		zap.String("add-on-stack-arn", ts.cfg.EKSConfig.AddOnAppMesh.AddOnCFNStackARN),
	)
	ts.cfg.EKSConfig.AddOnAppMesh.AddOnCFNStackARN = ""

	return ts.cfg.EKSConfig.Sync()
}
