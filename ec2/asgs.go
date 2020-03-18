package ec2

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"text/template"
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

/*
https://docs.aws.amazon.com/eks/latest/userguide/launch-workers.html
https://github.com/awslabs/amazon-eks-ami/blob/master/amazon-eks-nodegroup.yaml
https://raw.githubusercontent.com/awslabs/amazon-eks-ami/master/amazon-eks-nodegroup.yaml

https://aws.amazon.com/about-aws/whats-new/2019/09/amazon-eks-provides-eks-optimized-ami-metadata-via-ssm-parameters/

e.g.
/aws/service/ami-amazon-linux-latest/amzn2-ami-hvm-x86_64-gp2

e.g.
/aws/service/eks/optimized-ami/1.15/amazon-linux-2/recommended
/aws/service/bottlerocket/aws-k8s-1.15/x86_64/latest/image_id
*/

// TemplateASG is the CloudFormation template for ASG.
// "must specify one of the following properties: LaunchConfigurationName,
// LaunchTemplate, InstanceId, or MixedInstancesPolicy"
// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-as-group.html
// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-attribute-updatepolicy.html
// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-launchtemplate-launchtemplatedata.html
// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-launchtemplate-networkinterface.html
const TemplateASG = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EC2 ASG'

Parameters:

  ASGName:
    Type: String
    Default: aws-k8s-tester-ec2-asg
    Description: EC2 AutoScalingGroup name

  ASGLaunchTemplateName:
    Type: String
    Default: aws-k8s-tester-ec2-asg-launch-template
    Description: EC2 LaunchConfiguration name

  RoleName:
    Type: String
    Default: aws-k8s-tester-ec2-role
    Description: EC2 Role name

  PublicSubnetIDs:
    Type: List<AWS::EC2::Subnet::Id>
    Description: The public subnet IDs where workers can be created.

  SecurityGroupID:
    Type: AWS::EC2::SecurityGroup::Id
    Description: EC2 security group ID

  RemoteAccessKeyName:
    Type: AWS::EC2::KeyPair::KeyName
    Description: EC2 SSH key name
    Default: aws-k8s-tester-ec2-key

  ImageID:
    Type: String
    Default: ""
    Description: (Optional) Custom image ID. This value overrides any AWS Systems Manager Parameter Store value specified above.

  ImageIDSSMParameter:
    Type : AWS::SSM::Parameter::Value<AWS::EC2::Image::Id>
    Default: /aws/service/ami-amazon-linux-latest/amzn2-ami-hvm-x86_64-gp2
    Description: AWS Systems Manager Parameter Store parameter of the AMI ID.

  InstanceTypes:
    Type: CommaDelimitedList
    Default: c5.xlarge
    Description: EC2 instance types

  InstanceTypesCount:
    Type: Number
    Default: 1
    MinValue: 1
    MaxValue: 4
    Description: The number of instance types

  VolumeSize:
    Type: Number
    Default: 40
    MinValue: 8
    MaxValue: 1024
    Description: Size of the root disk for the EC2 instances, in GiB.

  ASGMinSize:
    Type: Number
    Description: Minimum size auto scaling group
    Default: 1
    MinValue: 1
    MaxValue: 1000

  ASGMaxSize:
    Type: Number
    Description: Maximum size auto scaling group
    Default: 1
    MinValue: 1
    MaxValue: 1000

  ASGDesiredCapacity:
    Type: Number
    Description: Desired size auto scaling group
    Default: 1
    MinValue: 1
    MaxValue: 1000

Conditions:

  HasImageID:
    Fn::Not:
      - Fn::Equals:
          - Ref: ImageID
          - ""

  Has2InstanceTypes:
    Fn::Or:
      - Fn::Equals:
          - Ref: InstanceTypesCount
          - 2
      - Fn::Equals:
          - Ref: InstanceTypesCount
          - 3
      - Fn::Equals:
          - Ref: InstanceTypesCount
          - 4

  Has3InstanceTypes:
    Fn::Or:
      - Fn::Equals:
          - Ref: InstanceTypesCount
          - 3
      - Fn::Equals:
          - Ref: InstanceTypesCount
          - 4

  Has4InstanceTypes:
    Fn::Equals:
      - Ref: InstanceTypesCount
      - 4

Resources:

  InstanceProfile:
    Type: AWS::IAM::InstanceProfile
    Properties:
      Path: "/"
      Roles:
      - !Ref RoleName

  # https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-launchtemplate-launchtemplatedata.html
  NodeLaunchTemplate:
    Type: AWS::EC2::LaunchTemplate
    DependsOn:
    - InstanceProfile
{{ if ne .Metadata "" }}{{.Metadata}}{{ end }}
    Properties:
      LaunchTemplateName: !Ref ASGLaunchTemplateName
      LaunchTemplateData:
        IamInstanceProfile:
          Arn: !GetAtt InstanceProfile.Arn
        ImageId:
          Fn::If:
            - HasImageID
            - !Ref ImageID
            - !Ref ImageIDSSMParameter
        KeyName: !Ref RemoteAccessKeyName
        BlockDeviceMappings:
        - DeviceName: /dev/xvda
          Ebs:
            VolumeType: gp2
            VolumeSize: !Ref VolumeSize
            DeleteOnTermination: true
        Monitoring:
          Enabled: true
        # need this for public DNS + SSH access
        NetworkInterfaces:
        - AssociatePublicIpAddress: true
          DeleteOnTermination: true
          DeviceIndex: 0
          Groups:
          - !Ref SecurityGroupID
        TagSpecifications:
        - ResourceType: instance
          Tags:
          - { Key: Name, Value: !Sub '${ASGName}-instance' }
        - ResourceType: volume
          Tags:
          - { Key: Name, Value: !Sub '${ASGName}-volume' }
{{ if ne .UserData "" }}{{.UserData}}{{ end }}

  # specify "MixedInstancesPolicy" or "LaunchConfiguration"
  ASG:
    Type: AWS::AutoScaling::AutoScalingGroup
    UpdatePolicy:
      AutoScalingRollingUpdate:
        MinInstancesInService: !Ref ASGDesiredCapacity
        MaxBatchSize: 1
        SuspendProcesses:
        - HealthCheck
        - ReplaceUnhealthy
        - AZRebalance
        - AlarmNotification
        - ScheduledActions
    Properties:
      AutoScalingGroupName: !Ref ASGName
      MinSize: !Ref ASGMinSize
      MaxSize: !Ref ASGMaxSize
      DesiredCapacity: !Ref ASGDesiredCapacity
      VPCZoneIdentifier: !Ref PublicSubnetIDs
      MetricsCollection:
      - Granularity: "1Minute"
      Tags:
      - Key: Name
        PropagateAtLaunch: true
        Value: !Ref ASGName
      MixedInstancesPolicy:
        InstancesDistribution:
          OnDemandAllocationStrategy: "prioritized"
        LaunchTemplate:
          LaunchTemplateSpecification:
            LaunchTemplateId: !Ref NodeLaunchTemplate
            Version: !GetAtt NodeLaunchTemplate.LatestVersionNumber
          Overrides:
          - InstanceType: !Select [ 0, !Ref InstanceTypes ]
          - Fn::If:
            - Has2InstanceTypes
            - !Select [ 1, !Ref InstanceTypes ]
            - !Ref AWS::NoValue
          - Fn::If:
            - Has3InstanceTypes
            - !Select [ 2, !Ref InstanceTypes ]
            - !Ref AWS::NoValue
          - Fn::If:
            - Has4InstanceTypes
            - !Select [ 3, !Ref InstanceTypes ]
            - !Ref AWS::NoValue
      HealthCheckType: EC2
      HealthCheckGracePeriod: 300

Outputs:

  ASGName:
    Value: !Ref ASG

  InstanceProfileARN:
    Value: !GetAtt InstanceProfile.Arn

`

const installSSMAL2Metadata = `    Metadata:
      AWS::CloudFormation::Init:
        configSets:
          default:
          - InstallAWSCLI
          - InstallSSM
        InstallAWSCLI:
          packages:
            # zsh: most Amazon users stations are set to zsh as a default
            # unzip: required to install aws cli
            # wget: under the hood SPIE requires wget
            yum:
              unzip: []
              zsh: []
              wget: []
          commands:
            01InstallAWSCLI:
              # AL2 doesn't have aws cli installed
              command: |
                curl "https://s3.amazonaws.com/aws-cli/awscli-bundle.zip" -o "awscli-bundle.zip"
                unzip awscli-bundle.zip
                sudo ./awscli-bundle/install -i /usr/local/aws -b /usr/bin/aws
                which aws
                rm -r awscli*
        InstallSSM:
          packages:
            rpm:
              ssm:
                - Fn::Sub: 'https://s3.${AWS::Region}.${AWS::URLSuffix}/amazon-ssm-${AWS::Region}/latest/linux_amd64/amazon-ssm-agent.rpm'`

const installSSMAL2UserData = `        UserData:
          Fn::Base64:
            Fn::Sub: |
              #!/bin/bash
              set -xeu

              sudo yum update -y \
                && sudo yum install -y \
                gcc \
                zlib-devel \
                openssl-devel \
                ncurses-devel \
                git \
                wget \
                jq \
                tar \
                curl \
                unzip \
                screen \
                mercurial \
                aws-cfn-bootstrap \
                awscli \
                chrony \
                conntrack \
                nfs-utils \
                socat

              # Make sure Amazon Time Sync Service starts on boot.
              sudo chkconfig chronyd on

              # Make sure that chronyd syncs RTC clock to the kernel.
              cat <<EOF | sudo tee -a /etc/chrony.conf
              # This directive enables kernel synchronisation (every 11 minutes) of the
              # real-time clock. Note that it can’t be used along with the 'rtcfile' directive.
              rtcsync
              EOF

              # https://docs.aws.amazon.com/inspector/latest/userguide/inspector_installing-uninstalling-agents.html
              curl -O https://inspector-agent.amazonaws.com/linux/latest/install
              chmod +x install
              sudo ./install -u false
              rm install

              sudo yum install -y yum-utils device-mapper-persistent-data lvm2
              sudo amazon-linux-extras install docker -y

              sudo systemctl daemon-reload
              sudo systemctl enable docker || true
              sudo systemctl start docker || true
              sudo systemctl restart docker || true

              sudo systemctl status docker --full --no-pager || true
              sudo usermod -aG docker ec2-user || true

              # su - ec2-user
              # or logout and login to use docker without 'sudo'
              id -nG
              sudo docker version
              sudo docker info`

type templateASG struct {
	Metadata string
	UserData string
}

func (ts *Tester) createASGs() error {
	createStart := time.Now()
	defer func() {
		ts.cfg.CreateTook += time.Since(createStart)
		ts.cfg.CreateTookString = ts.cfg.CreateTook.String()
		ts.cfg.Sync()
	}()

	if ts.cfg.Up {
		ts.lg.Info("cluster is up; no need to create cluster")
		return nil
	}

	ts.lg.Info("creating ASGs using CFN", zap.String("name", ts.cfg.Name))
	for asgName, asg := range ts.cfg.ASGs {
		timeStart := time.Now()
		ts.lg.Info("creating ASG", zap.String("name", asgName))

		// TODO: may not be necessary
		// "/aws/service/ami-amazon-linux-latest/amzn2-ami-hvm-x86_64-gp2"
		// already includes SSM agent + AWS CLI
		tg := templateASG{}
		switch asg.AMIType {
		case ec2config.AMITypeBottleRocketCPU:
			// "bottlerocket" comes with SSM agent
		case ec2config.AMITypeAL2X8664:
			tg.Metadata = installSSMAL2Metadata
			tg.UserData = installSSMAL2UserData
		case ec2config.AMITypeAL2X8664GPU:
			tg.Metadata = installSSMAL2Metadata
			tg.UserData = installSSMAL2UserData
		}
		tpl := template.Must(template.New("TemplateASG").Parse(TemplateASG))
		buf := bytes.NewBuffer(nil)
		if err := tpl.Execute(buf, tg); err != nil {
			return err
		}
		tmpl := buf.String()
		stackInput := &cloudformation.CreateStackInput{
			StackName:    aws.String(asgName),
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
					ParameterKey:   aws.String("ASGName"),
					ParameterValue: aws.String(asg.Name),
				},
				{
					ParameterKey:   aws.String("ASGLaunchTemplateName"),
					ParameterValue: aws.String(asg.Name + "-launch-template"),
				},
				{
					ParameterKey:   aws.String("RoleName"),
					ParameterValue: aws.String(ts.cfg.RoleName),
				},
				{
					ParameterKey:   aws.String("SecurityGroupID"),
					ParameterValue: aws.String(ts.cfg.SecurityGroupID),
				},
				{
					ParameterKey:   aws.String("RemoteAccessKeyName"),
					ParameterValue: aws.String(ts.cfg.RemoteAccessKeyName),
				},
				{
					ParameterKey:   aws.String("PublicSubnetIDs"),
					ParameterValue: aws.String(strings.Join(ts.cfg.PublicSubnetIDs, ",")),
				},
			},
		}
		ts.lg.Info("added image ID", zap.String("image-id", asg.ImageID))
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("ImageID"),
			ParameterValue: aws.String(asg.ImageID),
		})
		if asg.ImageIDSSMParameter != "" {
			ts.lg.Info("added image SSM parameter", zap.String("image-id-ssm-parameter", asg.ImageIDSSMParameter))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ImageIDSSMParameter"),
				ParameterValue: aws.String(asg.ImageIDSSMParameter),
			})
		}
		if len(asg.InstanceTypes) > 0 {
			ts.lg.Info("added instance type", zap.Strings("instance-types", asg.InstanceTypes))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("InstanceTypes"),
				ParameterValue: aws.String(strings.Join(asg.InstanceTypes, ",")),
			})
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("InstanceTypesCount"),
				ParameterValue: aws.String(fmt.Sprintf("%d", len(asg.InstanceTypes))),
			})
		}
		if asg.VolumeSize > 0 {
			ts.lg.Info("added volume size", zap.Int64("volume-size", asg.VolumeSize))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("VolumeSize"),
				ParameterValue: aws.String(fmt.Sprintf("%d", asg.VolumeSize)),
			})
		}
		if asg.ASGMinSize > 0 {
			ts.lg.Info("added min size", zap.Int64("min-size", asg.ASGMinSize))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ASGMinSize"),
				ParameterValue: aws.String(fmt.Sprintf("%d", asg.ASGMinSize)),
			})
		}
		if asg.ASGMaxSize > 0 {
			ts.lg.Info("added max size", zap.Int64("max-size", asg.ASGMaxSize))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ASGMaxSize"),
				ParameterValue: aws.String(fmt.Sprintf("%d", asg.ASGMaxSize)),
			})
		}
		if asg.ASGDesiredCapacity > 0 {
			ts.lg.Info("added desired size", zap.Int64("desired-capacity", asg.ASGDesiredCapacity))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ASGDesiredCapacity"),
				ParameterValue: aws.String(fmt.Sprintf("%d", asg.ASGDesiredCapacity)),
			})
		}
		stackOutput, err := ts.cfnAPI.CreateStack(stackInput)
		if err != nil {
			asg.CreateTook += time.Since(timeStart)
			asg.CreateTookString = asg.CreateTook.String()
			ts.cfg.ASGs[asgName] = asg
			ts.cfg.Sync()
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
			asg.CreateTook += time.Since(timeStart)
			asg.CreateTookString = asg.CreateTook.String()
			ts.cfg.ASGs[asgName] = asg
			ts.cfg.Sync()
			return st.Error
		}
		// update status after creating a new ASG
		for _, o := range st.Stack.Outputs {
			switch k := aws.StringValue(o.OutputKey); k {
			case "ASGName":
				ts.lg.Info("found ASGName value from CFN", zap.String("value", aws.StringValue(o.OutputValue)))
			case "InstanceProfileARN":
				ts.lg.Info("found InstanceProfileARN value from CFN", zap.String("value", aws.StringValue(o.OutputValue)))
			default:
				asg.CreateTook += time.Since(timeStart)
				asg.CreateTookString = asg.CreateTook.String()
				ts.cfg.ASGs[asgName] = asg
				ts.cfg.Sync()
				return fmt.Errorf("unexpected OutputKey %q from %q", k, asg.ASGCFNStackID)
			}
		}

		ts.lg.Info("created ASG",
			zap.String("name", asg.Name),
			zap.String("cfn-stack-id", asg.ASGCFNStackID),
			zap.String("request-started", humanize.RelTime(createStart, time.Now(), "ago", "from now")),
		)
		asg.CreateTook += time.Since(timeStart)
		asg.CreateTookString = asg.CreateTook.String()
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
			asg.CreateTook += time.Since(timeStart)
			asg.CreateTookString = asg.CreateTook.String()
			ts.cfg.ASGs[asgName] = asg
			ts.cfg.Sync()
			return err
		}
		asg.Instances = make(map[string]ec2config.Instance)
		for id, vv := range ec2Instances {
			ivv := ec2config.ConvertInstance(vv)
			ivv.RemoteAccessUserName = asg.RemoteAccessUserName
			asg.Instances[id] = ivv
		}
		asg.CreateTook += time.Since(timeStart)
		asg.CreateTookString = asg.CreateTook.String()
		ts.cfg.ASGs[asgName] = asg
		ts.cfg.RecordStatus(fmt.Sprintf("%q/%s", asgName, cloudformation.ResourceStatusCreateComplete))
		ts.cfg.Sync()
	}

	return ts.cfg.Sync()
}

func (ts *Tester) deleteASGs() error {
	deleteStart := time.Now()
	defer func() {
		ts.cfg.DeleteTook += time.Since(deleteStart)
		ts.cfg.DeleteTookString = ts.cfg.DeleteTook.String()
		ts.cfg.Sync()
	}()

	ts.lg.Info("deleting ASGs using CFN", zap.String("name", ts.cfg.Name))
	for asgName, asg := range ts.cfg.ASGs {
		if asg.ASGCFNStackID == "" {
			return fmt.Errorf("%q ASG stack ID is empty", asg.Name)
		}
		timeStart := time.Now()
		ts.lg.Info("deleting ASG", zap.String("name", asgName), zap.String("cfn-stack-id", asg.ASGCFNStackID))
		_, err := ts.cfnAPI.DeleteStack(&cloudformation.DeleteStackInput{
			StackName: aws.String(asg.ASGCFNStackID),
		})
		if err != nil {
			ts.cfg.RecordStatus(fmt.Sprintf("failed to delete ASG (%v)", err))
			asg.DeleteTook += time.Since(timeStart)
			asg.DeleteTookString = ts.cfg.DeleteTook.String()
			ts.cfg.ASGs[asgName] = asg
			ts.cfg.Sync()
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
			asg.DeleteTook += time.Since(timeStart)
			asg.DeleteTookString = ts.cfg.DeleteTook.String()
			ts.cfg.ASGs[asgName] = asg
			ts.cfg.Sync()
			return st.Error
		}
		ts.cfg.RecordStatus(fmt.Sprintf("%q/%s", asgName, ec2config.StatusDELETEDORNOTEXIST))
		asg.DeleteTook += time.Since(timeStart)
		asg.DeleteTookString = ts.cfg.DeleteTook.String()
		ts.cfg.ASGs[asgName] = asg
		ts.cfg.Sync()
	}

	return ts.cfg.Sync()
}
