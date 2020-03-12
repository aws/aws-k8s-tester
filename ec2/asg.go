package ec2

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	awscfn "github.com/aws/aws-k8s-tester/pkg/aws/cloudformation"
	awsapiec2 "github.com/aws/aws-k8s-tester/pkg/aws/ec2"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// MAKE SURE TO SYNC THE DEFAULT VALUES in "ec2config"

// TemplateASG is the CloudFormation template for ASG.
const TemplateASG = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EC2 ASG'

# https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-as-group.html
Parameters:

  LaunchConfigurationName:
    Type: String
    Default: aws-k8s-tester-ec2-asg-launch-configuration
    Description: EC2 LaunchConfiguration name

  AutoScalingGroupName:
    Type: String
    Default: aws-k8s-tester-ec2-asg
    Description: EC2 AutoScalingGroup name

  PublicSubnetID1:
    Type: String
    Description: EC2 public subnet ID

  PublicSubnetID2:
    Type: String
    Description: EC2 public subnet ID

  PublicSubnetID3:
    Type: String
    Description: EC2 public subnet ID

  MinSize:
    Type: Number
    Description: Minimum size auto scaling group
    Default: 1
    MinValue: 1
    MaxValue: 1000

  MaxSize:
    Type: Number
    Description: Maximum size auto scaling group
    Default: 1
    MinValue: 1
    MaxValue: 1000

  DesiredCapacity:
    Type: Number
    Description: Desired size auto scaling group
    Default: 1
    MinValue: 1
    MaxValue: 1000

Conditions:

  IsPublicSubnetID3Configured:
    Fn::Not:
      - Fn::Equals:
          - Ref: PublicSubnetID3
          - ""

Resources:

  AutoScalingGroup:
    Type: AWS::AutoScaling::AutoScalingGroup
    UpdatePolicy:
      AutoScalingRollingUpdate:
        MinInstancesInService: !Ref DesiredCapacity
        MaxBatchSize: 1
        SuspendProcesses:
        - HealthCheck
        - ReplaceUnhealthy
        - AZRebalance
        - AlarmNotification
        - ScheduledActions
    Properties:
      AutoScalingGroupName: !Ref AutoScalingGroupName
      MinSize: !Ref MinSize
      MaxSize: !Ref MaxSize
      DesiredCapacity: !Ref DesiredCapacity
      VPCZoneIdentifier:
      - !Ref PublicSubnetID1
      - !Ref PublicSubnetID2
      - Fn::If:
        - IsPublicSubnetID3Configured
        - !Ref PublicSubnetID3
        - !Ref AWS::NoValue
      MetricsCollection:
      - Granularity: "1Minute"
      Tags:
      - Key: Name
        PropagateAtLaunch: true
        Value: !Ref AutoScalingGroupName
      LaunchConfigurationName: !Ref LaunchConfigurationName
      HealthCheckType: EC2
      HealthCheckGracePeriod: 300

Outputs:

  AutoScalingGroupName:
    Value: !Ref AutoScalingGroup

`

func (ts *Tester) createASGs() error {
	createStart := time.Now()
	defer func() {
		ts.cfg.CreateTook = time.Since(createStart)
		ts.cfg.CreateTookString = ts.cfg.CreateTook.String()
		ts.cfg.Sync()
	}()

	if ts.cfg.Up {
		ts.lg.Info("cluster is up; no need to create cluster")
		return nil
	}

	ts.lg.Info("creating ASGs using CFN", zap.String("name", ts.cfg.Name))
	for asgName, asg := range ts.cfg.ASGs {
		ts.lg.Info("creating ASG", zap.String("name", asgName))
		stackInput := &cloudformation.CreateStackInput{
			StackName:    aws.String(asgName),
			Capabilities: aws.StringSlice([]string{"CAPABILITY_IAM"}),
			OnFailure:    aws.String(cloudformation.OnFailureDelete),
			TemplateBody: aws.String(TemplateASG),
			Tags: awscfn.NewTags(map[string]string{
				"Kind":                   "aws-k8s-tester",
				"Name":                   ts.cfg.Name,
				"aws-k8s-tester-version": version.ReleaseVersion,
			}),
			Parameters: []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String("LaunchConfigurationName"),
					ParameterValue: aws.String(asg.LaunchConfigurationName),
				},
				{
					ParameterKey:   aws.String("AutoScalingGroupName"),
					ParameterValue: aws.String(asg.Name),
				},
				{
					ParameterKey:   aws.String("PublicSubnetID1"),
					ParameterValue: aws.String(ts.cfg.PublicSubnetIDs[0]),
				},
				{
					ParameterKey:   aws.String("PublicSubnetID2"),
					ParameterValue: aws.String(ts.cfg.PublicSubnetIDs[1]),
				},
				{
					ParameterKey:   aws.String("PublicSubnetID3"),
					ParameterValue: aws.String(ts.cfg.PublicSubnetIDs[2]),
				},
			},
		}
		if asg.MinSize > 0 {
			ts.lg.Info("added min size", zap.Int64("min-size", asg.MinSize))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("MinSize"),
				ParameterValue: aws.String(fmt.Sprintf("%d", asg.MinSize)),
			})
		}
		if asg.MaxSize > 0 {
			ts.lg.Info("added max size", zap.Int64("max-size", asg.MaxSize))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("MaxSize"),
				ParameterValue: aws.String(fmt.Sprintf("%d", asg.MaxSize)),
			})
		}
		if asg.DesiredCapacity > 0 {
			ts.lg.Info("added desired size", zap.Int64("desired-capacity", asg.DesiredCapacity))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("DesiredCapacity"),
				ParameterValue: aws.String(fmt.Sprintf("%d", asg.DesiredCapacity)),
			})
		}
		stackOutput, err := ts.cfnAPI.CreateStack(stackInput)
		if err != nil {
			return err
		}
		asg.ASGCFNStackID = aws.StringValue(stackOutput.StackId)
		ts.cfg.ASGs[asgName] = asg
		ts.cfg.Sync()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		ch := awscfn.Poll(
			ctx,
			ts.stopCreationCh,
			ts.interruptSig,
			ts.lg,
			ts.cfnAPI,
			asg.ASGCFNStackID,
			cloudformation.ResourceStatusCreateComplete,
			2*time.Minute,
			30*time.Second,
		)
		var st awscfn.StackStatus
		for st = range ch {
			if st.Error != nil {
				ts.cfg.RecordStatus(fmt.Sprintf("failed to create ASG (%v)", st.Error))
				ts.lg.Warn("polling errror", zap.Error(st.Error))
			}
		}
		cancel()
		if st.Error != nil {
			return st.Error
		}
		// update status after creating a new ASG
		for _, o := range st.Stack.Outputs {
			switch k := aws.StringValue(o.OutputKey); k {
			case "AutoScalingGroupName":
				ts.lg.Info("found AutoScalingGroupName value from CFN", zap.String("value", aws.StringValue(o.OutputValue)))
			default:
				return fmt.Errorf("unexpected OutputKey %q from %q", k, asg.ASGCFNStackID)
			}
		}

		ts.lg.Info("created ASG",
			zap.String("name", asg.Name),
			zap.String("cfn-stack-id", asg.ASGCFNStackID),
			zap.String("request-started", humanize.RelTime(createStart, time.Now(), "ago", "from now")),
		)
		ts.cfg.ASGs[asgName] = asg
		ts.cfg.Up = true
		ts.cfg.Sync()

		var aout *autoscaling.DescribeAutoScalingGroupsOutput
		aout, err = ts.asgAPI.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
			AutoScalingGroupNames: aws.StringSlice([]string{asgName}),
		})
		if err != nil {
			return fmt.Errorf("ASG %q not found (%v)", asgName, err)
		}
		if len(aout.AutoScalingGroups) != 1 {
			return fmt.Errorf("%q expected only 1 ASG, got %+v", asgName, aout.AutoScalingGroups)
		}
		av := aout.AutoScalingGroups[0]
		instanceIDs := make([]string, 0, len(av.Instances))
		for _, iv := range av.Instances {
			instanceIDs = append(instanceIDs, aws.StringValue(iv.InstanceId))
		}
		ts.lg.Info(
			"describing EC2 instances in ASG",
			zap.String("asg-name", asgName),
			zap.Strings("instance-ids", instanceIDs),
		)
		ec2Instances, err := awsapiec2.PollUntilRunning(
			10*time.Minute,
			ts.lg,
			ts.ec2API,
			instanceIDs...,
		)
		if err != nil {
			return err
		}
		asg.Instances = make(map[string]ec2config.Instance)
		for id, vv := range ec2Instances {
			asg.Instances[id] = ec2config.ConvertInstance(vv)
		}
		ts.cfg.ASGs[asgName] = asg
		ts.cfg.RecordStatus(fmt.Sprintf("%q/%s", asgName, cloudformation.ResourceStatusCreateComplete))
		ts.cfg.Sync()
	}

	return ts.cfg.Sync()
}

func (ts *Tester) deleteASGs() error {
	deleteStart := time.Now()
	defer func() {
		ts.cfg.DeleteTook = time.Since(deleteStart)
		ts.cfg.DeleteTookString = ts.cfg.DeleteTook.String()
		ts.cfg.Sync()
	}()

	ts.lg.Info("deleting ASGs using CFN", zap.String("name", ts.cfg.Name))
	for asgName, asg := range ts.cfg.ASGs {
		if asg.ASGCFNStackID == "" {
			return fmt.Errorf("%q ASG stack ID is empty", asg.Name)
		}
		ts.lg.Info("deleting ASG", zap.String("name", asgName), zap.String("cfn-stack-id", asg.ASGCFNStackID))
		_, err := ts.cfnAPI.DeleteStack(&cloudformation.DeleteStackInput{
			StackName: aws.String(asg.ASGCFNStackID),
		})
		if err != nil {
			ts.cfg.RecordStatus(fmt.Sprintf("failed to delete ASG (%v)", err))
			return err
		}
		ts.cfg.Up = false
		ts.cfg.Sync()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		ch := awscfn.Poll(
			ctx,
			make(chan struct{}),  // do not exit on stop
			make(chan os.Signal), // do not exit on stop
			ts.lg,
			ts.cfnAPI,
			asg.ASGCFNStackID,
			cloudformation.ResourceStatusDeleteComplete,
			2*time.Minute,
			20*time.Second,
		)
		var st awscfn.StackStatus
		for st = range ch {
			if st.Error != nil {
				cancel()
				ts.cfg.RecordStatus(fmt.Sprintf("failed to delete ASG (%v)", st.Error))
				ts.lg.Warn("polling errror", zap.Error(st.Error))
			}
		}
		cancel()
		if st.Error != nil {
			return st.Error
		}
		ts.cfg.RecordStatus(fmt.Sprintf("%q/%s", asgName, ec2config.StatusDELETEDORNOTEXIST))
	}

	return ts.cfg.Sync()
}
