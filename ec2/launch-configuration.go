package ec2

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	awscfn "github.com/aws/aws-k8s-tester/pkg/aws/cloudformation"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

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

// TemplateASGLaunchConfiguration is the CloudFormation
// template for ASG launch configuration.
const TemplateASGLaunchConfiguration = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EC2 ASG launch configuration'

# https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-as-group.html
Parameters:

  LaunchConfigurationName:
    Type: String
    Default: aws-k8s-tester-ec2-asg-launch-configuration
    Description: EC2 LaunchConfiguration name

  RoleName:
    Type: String
    Default: aws-k8s-tester-ec2-role
    Description: EC2 Role name

  SecurityGroupID:
    Type: String
    Description: EC2 security group ID

  RemoteAccessKeyName:
    Type: String
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

  InstanceType:
    Type: String
    Default: c5.xlarge
    Description: EC2 instance type

  VolumeSize:
    Type: Number
    Default: 40
    MinValue: 8
    MaxValue: 1024
    Description: Size of the root disk for the EC2 instances, in GiB.

Conditions:

  HasImageID:
    Fn::Not:
      - Fn::Equals:
          - Ref: ImageID
          - ""

Resources:

  InstanceProfile:
    Type: AWS::IAM::InstanceProfile
    Properties:
      Path: "/"
      Roles:
      - !Ref RoleName

  LaunchConfiguration:
    Type: AWS::AutoScaling::LaunchConfiguration
    DependsOn:
    - InstanceProfile
    Properties:
      LaunchConfigurationName: !Ref LaunchConfigurationName
      ImageId:
        Fn::If:
        - HasImageID
        - !Ref ImageID
        - !Ref ImageIDSSMParameter
      InstanceType: !Ref InstanceType
      IamInstanceProfile: !Ref InstanceProfile
      # need this for public DNS + SSH access
      AssociatePublicIpAddress: true
      SecurityGroups:
      - !Ref SecurityGroupID
      KeyName: !Ref RemoteAccessKeyName
      BlockDeviceMappings:
      - DeviceName: '/dev/xvda'
        Ebs:
          VolumeType: gp2
          VolumeSize: !Ref VolumeSize

Outputs:

  LaunchConfigurationName:
    Value: !Ref LaunchConfiguration

`

// TemplateASGLaunchConfigurationAL2 is the CloudFormation
// template for ASG launch configuration.
const TemplateASGLaunchConfigurationAL2 = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EC2 ASG launch configuration'

# https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-as-group.html
Parameters:

  LaunchConfigurationName:
    Type: String
    Default: aws-k8s-tester-ec2-asg-launch-configuration
    Description: EC2 LaunchConfiguration name

  RoleName:
    Type: String
    Default: aws-k8s-tester-ec2-role
    Description: EC2 Role name

  SecurityGroupID:
    Type: String
    Description: EC2 security group ID

  RemoteAccessKeyName:
    Type: String
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

  InstanceType:
    Type: String
    Default: c5.xlarge
    Description: EC2 instance type

  VolumeSize:
    Type: Number
    Default: 40
    MinValue: 8
    MaxValue: 1024
    Description: Size of the root disk for the EC2 instances, in GiB.

Conditions:

  HasImageID:
    Fn::Not:
      - Fn::Equals:
          - Ref: ImageID
          - ""

Resources:

  InstanceProfile:
    Type: AWS::IAM::InstanceProfile
    Properties:
      Path: "/"
      Roles:
      - !Ref RoleName

  LaunchConfiguration:
    Type: AWS::AutoScaling::LaunchConfiguration
    DependsOn:
    - InstanceProfile
    Metadata:
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
                - Fn::Sub: 'https://s3.${AWS::Region}.${AWS::URLSuffix}/amazon-ssm-${AWS::Region}/latest/linux_amd64/amazon-ssm-agent.rpm'
    Properties:
      LaunchConfigurationName: !Ref LaunchConfigurationName
      ImageId:
        Fn::If:
        - HasImageID
        - !Ref ImageID
        - !Ref ImageIDSSMParameter
      InstanceType: !Ref InstanceType
      IamInstanceProfile: !Ref InstanceProfile
      # need this for public DNS + SSH access
      AssociatePublicIpAddress: true
      SecurityGroups:
      - !Ref SecurityGroupID
      KeyName: !Ref RemoteAccessKeyName
      BlockDeviceMappings:
      - DeviceName: '/dev/xvda'
        Ebs:
          VolumeType: gp2
          VolumeSize: !Ref VolumeSize
      UserData:
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
            sudo docker info

Outputs:

  LaunchConfigurationName:
    Value: !Ref LaunchConfiguration

`

func (ts *Tester) createLaunchConfiguration() error {
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

	ts.lg.Info("creating ASG launch configurations using CFN", zap.String("name", ts.cfg.Name))
	for asgName, asg := range ts.cfg.ASGs {
		ts.lg.Info("creating ASG launch configuration", zap.String("name", asg.LaunchConfigurationName))

		tmpl := ""
		switch asg.AMIType {
		case ec2config.AMITypeBottleRocketCPU:
			tmpl = TemplateASGLaunchConfiguration
		case ec2config.AMITypeAL2X8664,
			ec2config.AMITypeAL2X8664GPU:
			tmpl = TemplateASGLaunchConfigurationAL2
		default:
			return fmt.Errorf("unknown AMI type %q", asg.AMIType)
		}

		stackInput := &cloudformation.CreateStackInput{
			StackName:    aws.String(asg.LaunchConfigurationName),
			Capabilities: aws.StringSlice([]string{"CAPABILITY_IAM"}),
			OnFailure:    aws.String(cloudformation.OnFailureDelete),
			TemplateBody: aws.String(tmpl),
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
			},
		}
		if asg.ImageID != "" {
			ts.lg.Info("added image ID", zap.String("image-id", asg.ImageID))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ImageID"),
				ParameterValue: aws.String(asg.ImageID),
			})
		}
		if asg.ImageIDSSMParameter != "" {
			ts.lg.Info("added image ID", zap.String("image-id-ssm-parameter", asg.ImageIDSSMParameter))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ImageIDSSMParameter"),
				ParameterValue: aws.String(asg.ImageIDSSMParameter),
			})
		}
		if asg.InstanceType != "" {
			ts.lg.Info("added instance type", zap.String("instance-type", asg.InstanceType))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("InstanceType"),
				ParameterValue: aws.String(asg.InstanceType),
			})
		}
		if asg.VolumeSize > 0 {
			ts.lg.Info("added volume size", zap.Int64("volume-size", asg.VolumeSize))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("VolumeSize"),
				ParameterValue: aws.String(fmt.Sprintf("%d", asg.VolumeSize)),
			})
		}
		stackOutput, err := ts.cfnAPI.CreateStack(stackInput)
		if err != nil {
			return err
		}
		asg.ASGLaunchConfigurationCFNStackID = aws.StringValue(stackOutput.StackId)

		ts.lg.Info("created ASG Launch Configuration",
			zap.String("name", asg.LaunchConfigurationName),
			zap.String("cfn-stack-id", asg.ASGLaunchConfigurationCFNStackID),
			zap.String("request-started", humanize.RelTime(createStart, time.Now(), "ago", "from now")),
		)
		ts.cfg.ASGs[asgName] = asg
		ts.cfg.Sync()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		ch := awscfn.Poll(
			ctx,
			ts.stopCreationCh,
			ts.interruptSig,
			ts.lg,
			ts.cfnAPI,
			asg.ASGLaunchConfigurationCFNStackID,
			cloudformation.ResourceStatusCreateComplete,
			2*time.Minute,
			30*time.Second,
		)
		var st awscfn.StackStatus
		for st = range ch {
			if st.Error != nil {
				ts.cfg.RecordStatus(fmt.Sprintf("failed to create ASG Launch Configuration (%v)", st.Error))
				ts.lg.Warn("polling errror", zap.Error(st.Error))
			}
		}
		cancel()
		if st.Error != nil {
			return st.Error
		}
		// update status after creating a new cluster
		for _, o := range st.Stack.Outputs {
			switch k := aws.StringValue(o.OutputKey); k {
			case "LaunchConfigurationName":
				ts.lg.Info("found LaunchConfigurationName value from CFN", zap.String("value", aws.StringValue(o.OutputValue)))
			default:
				return fmt.Errorf("unexpected OutputKey %q from %q", k, asg.ASGLaunchConfigurationCFNStackID)
			}
		}

		var aout *autoscaling.DescribeLaunchConfigurationsOutput
		aout, err = ts.asgAPI.DescribeLaunchConfigurations(&autoscaling.DescribeLaunchConfigurationsInput{
			LaunchConfigurationNames: aws.StringSlice([]string{asg.LaunchConfigurationName}),
		})
		if err != nil {
			return fmt.Errorf("ASG %q not found (%v)", asgName, err)
		}
		if len(aout.LaunchConfigurations) != 1 {
			return fmt.Errorf("%q expected only 1 ASG Launch Configuration, got %+v", asg.LaunchConfigurationName, aout.LaunchConfigurations)
		}
		lname := aws.StringValue(aout.LaunchConfigurations[0].LaunchConfigurationName)
		if lname != asg.LaunchConfigurationName {
			ts.lg.Warn("unexpected ASG Launch Configuration name", zap.String("name", lname))
			return fmt.Errorf("ASG Launch Configuration name expected %q, got %q", asg.LaunchConfigurationName, lname)
		}
	}

	return ts.cfg.Sync()
}

func (ts *Tester) deleteLaunchConfiguration() error {
	deleteStart := time.Now()
	defer func() {
		ts.cfg.DeleteTook += time.Since(deleteStart)
		ts.cfg.DeleteTookString = ts.cfg.DeleteTook.String()
		ts.cfg.Sync()
	}()

	ts.lg.Info("deleting ASG Launch Configurations using CFN", zap.String("name", ts.cfg.Name))
	for _, asg := range ts.cfg.ASGs {
		if asg.ASGLaunchConfigurationCFNStackID == "" {
			return fmt.Errorf("%q ASG stack ID is empty", asg.Name)
		}
		ts.lg.Info("deleting ASG Launch Configuration", zap.String("name", asg.LaunchConfigurationName), zap.String("cfn-stack-id", asg.ASGLaunchConfigurationCFNStackID))
		_, err := ts.cfnAPI.DeleteStack(&cloudformation.DeleteStackInput{
			StackName: aws.String(asg.ASGLaunchConfigurationCFNStackID),
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
			asg.ASGLaunchConfigurationCFNStackID,
			cloudformation.ResourceStatusDeleteComplete,
			2*time.Minute,
			20*time.Second,
		)
		var st awscfn.StackStatus
		for st = range ch {
			if st.Error != nil {
				cancel()
				ts.cfg.RecordStatus(fmt.Sprintf("failed to delete ASG Launch Configuration (%v)", st.Error))
				ts.lg.Warn("polling errror", zap.Error(st.Error))
			}
		}
		cancel()
		if st.Error != nil {
			return st.Error
		}
		ts.cfg.RecordStatus(fmt.Sprintf("%q/%s", asg.LaunchConfigurationName, ec2config.StatusDELETEDORNOTEXIST))
	}

	return ts.cfg.Sync()
}
