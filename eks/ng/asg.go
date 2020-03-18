package ng

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

/*
https://docs.aws.amazon.com/eks/latest/userguide/launch-workers.html
https://github.com/awslabs/amazon-eks-ami/blob/master/amazon-eks-nodegroup.yaml

https://github.com/awslabs/amazon-eks-ami/blob/master/amazon-eks-nodegroup.yaml
https://raw.githubusercontent.com/awslabs/amazon-eks-ami/master/amazon-eks-nodegroup.yaml

https://aws.amazon.com/about-aws/whats-new/2019/09/amazon-eks-provides-eks-optimized-ami-metadata-via-ssm-parameters/



e.g.
aws ssm get-parameters --names /aws/service/ami-amazon-linux-latest/amzn2-ami-hvm-x86_64-gp2

e.g.
aws ssm get-parameters --names /aws/service/eks/optimized-ami/1.15/amazon-linux-2/recommended
aws ssm get-parameters --names /aws/service/bottlerocket/aws-k8s-1.15/x86_64/latest/image_id
*/

// TemplateASG is the CloudFormation template for EKS node group.
// ref. https://docs.aws.amazon.com/eks/latest/userguide/launch-workers.html
// ref. https://github.com/awslabs/amazon-eks-ami/blob/master/amazon-eks-nodegroup.yaml
const TemplateASG = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Node Group ASG'

Parameters:

  ClusterName:
    Type: String
    Description: The cluster name provided when the cluster was created. If it is incorrect, nodes will not be able to join the cluster.

  ASGName:
    Type: String
    Default: aws-k8s-tester-eks-ng-asg
    Description: EKS Node Group AutoScalingGroup name

  ASGLaunchTemplateName:
    Type: String
    Default: aws-k8s-tester-eks-ng-asg-launch-template
    Description: EKS Node Group LaunchConfiguration name

  RoleName:
    Type: String
    Description: The name of the node instance role

  PublicSubnetIDs:
    Type: List<AWS::EC2::Subnet::Id>
    Description: The public subnet IDs where workers can be created.

  NodeSecurityGroupID:
    Type: AWS::EC2::SecurityGroup::Id
    Description: The security group of the node group.

  RemoteAccessKeyName:
    Type: AWS::EC2::KeyPair::KeyName
    Description: The EC2 Key Pair to allow SSH access to the instances

  ImageID:
    Type: String
    Default: ""
    Description: (Optional) Specify your own custom image ID. This value overrides any AWS Systems Manager Parameter Store value specified above.

  ImageIDSSMParameter:
    Type: AWS::SSM::Parameter::Value<AWS::EC2::Image::Id>
    Default: /aws/service/eks/optimized-ami/1.15/amazon-linux-2/recommended/image_id
    Description: AWS Systems Manager Parameter Store parameter of the AMI ID for the worker node instances.

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
    Default: 2
    Description: Minimum size of Node Group ASG.

  ASGMaxSize:
    Type: Number
    Default: 2
    Description: Maximum size of Node Group ASG. Set to at least 1 greater than ASGDesiredCapacity.

  ASGDesiredCapacity:
    Type: Number
    Default: 2
    Description: Desired capacity of Node Group ASG.

  BootstrapArguments:
    Type: String
    Description: Arguments to pass to the bootstrap script. See files/bootstrap.sh in https://github.com/awslabs/amazon-eks-ami

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
        Value: !Ref ASGName
        PropagateAtLaunch: true
      - Key: !Sub kubernetes.io/cluster/${ClusterName}
        Value: owned
        PropagateAtLaunch: true
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

const metadataAL2InstallSSM = `    Metadata:
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

const userDataAL2InstallSSM = `        UserData:
          Fn::Base64:
            Fn::Sub: |
              #!/bin/bash
              set -o xtrace

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

`

type templateASG struct {
	Metadata string
	UserData string
}

func (ts *tester) createASGs() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() {
		ts.cfg.Logger.Info("skipping node group ASG creation")
		return nil
	}

	ts.cfg.Logger.Info("creating ASGs using CFN", zap.String("name", ts.cfg.EKSConfig.Name))
	for asgName, asg := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
		timeStart := time.Now()
		ts.cfg.Logger.Info("creating ASG", zap.String("name", asgName))

		// TODO: may not be necessary
		// "/aws/service/ami-amazon-linux-latest/amzn2-ami-hvm-x86_64-gp2"
		// already includes SSM agent + AWS CLI
		tg := templateASG{}
		switch asg.AMIType {
		case ec2config.AMITypeBottleRocketCPU:
			// "bottlerocket" comes with SSM agent
			tg.Metadata = ""
			tg.UserData = fmt.Sprintf(`        UserData:
          Fn::Base64:
            Fn::Sub: |
              [settings.kubernetes]
              api-server = "%s"
              cluster-certificate = "%s"
              cluster-name = "%s"
`,
				ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint,
				ts.cfg.EKSConfig.Status.ClusterCA,
				ts.cfg.EKSConfig.Name,
			)

		case ec2config.AMITypeAL2X8664:
			tg.Metadata = metadataAL2InstallSSM
			tg.UserData = userDataAL2InstallSSM
			tg.UserData += `              /etc/eks/bootstrap.sh ${ClusterName}`
			if ts.cfg.EKSConfig.Parameters.ResolverURL != "" {
				tg.UserData += fmt.Sprintf(` --b64-cluster-ca %s --apiserver-endpoint %s`, ts.cfg.EKSConfig.Status.ClusterCA, ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint)
				tg.UserData += "\n"
			}
			tg.UserData += `              /opt/aws/bin/cfn-signal --exit-code $? --stack ${AWS::StackName} --resource ASG --region ${AWS::Region}`

		case ec2config.AMITypeAL2X8664GPU:
			tg.Metadata = metadataAL2InstallSSM
			tg.UserData = userDataAL2InstallSSM
			tg.UserData += `              /etc/eks/bootstrap.sh ${ClusterName}`
			if ts.cfg.EKSConfig.Parameters.ResolverURL != "" {
				tg.UserData += fmt.Sprintf(` --b64-cluster-ca %s --apiserver-endpoint %s`, ts.cfg.EKSConfig.Status.ClusterCA, ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint)
				tg.UserData += "\n"
			}
			tg.UserData += `              /opt/aws/bin/cfn-signal --exit-code $? --stack ${AWS::StackName} --resource ASG --region ${AWS::Region}`
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
				"Name":                   ts.cfg.EKSConfig.Name,
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
					ParameterValue: aws.String(ts.cfg.EKSConfig.AddOnNodeGroups.RoleName),
				},
				{
					ParameterKey:   aws.String("PublicSubnetIDs"),
					ParameterValue: aws.String(strings.Join(ts.cfg.EKSConfig.Parameters.PublicSubnetIDs, ",")),
				},
				{
					ParameterKey:   aws.String("NodeSecurityGroupID"),
					ParameterValue: aws.String(ts.cfg.EKSConfig.AddOnNodeGroups.NodeGroupSecurityGroupID),
				},
				{
					ParameterKey:   aws.String("RemoteAccessKeyName"),
					ParameterValue: aws.String(ts.cfg.EKSConfig.RemoteAccessKeyName),
				},
			},
		}
		ts.cfg.Logger.Info("added image ID", zap.String("image-id", asg.ImageID))
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("ImageID"),
			ParameterValue: aws.String(asg.ImageID),
		})
		if asg.ImageIDSSMParameter != "" {
			ts.cfg.Logger.Info("added image SSM parameter", zap.String("image-id-ssm-parameter", asg.ImageIDSSMParameter))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ImageIDSSMParameter"),
				ParameterValue: aws.String(asg.ImageIDSSMParameter),
			})
		}
		if len(asg.InstanceTypes) > 0 {
			ts.cfg.Logger.Info("added instance type", zap.Strings("instance-types", asg.InstanceTypes))
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
			ts.cfg.Logger.Info("added volume size", zap.Int64("volume-size", asg.VolumeSize))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("VolumeSize"),
				ParameterValue: aws.String(fmt.Sprintf("%d", asg.VolumeSize)),
			})
		}
		if asg.ASGMinSize > 0 {
			ts.cfg.Logger.Info("added min size", zap.Int64("min-size", asg.ASGMinSize))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ASGMinSize"),
				ParameterValue: aws.String(fmt.Sprintf("%d", asg.ASGMinSize)),
			})
		}
		if asg.ASGMaxSize > 0 {
			ts.cfg.Logger.Info("added max size", zap.Int64("max-size", asg.ASGMaxSize))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ASGMaxSize"),
				ParameterValue: aws.String(fmt.Sprintf("%d", asg.ASGMaxSize)),
			})
		}
		if asg.ASGDesiredCapacity > 0 {
			ts.cfg.Logger.Info("added desired size", zap.Int64("desired-capacity", asg.ASGDesiredCapacity))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ASGDesiredCapacity"),
				ParameterValue: aws.String(fmt.Sprintf("%d", asg.ASGDesiredCapacity)),
			})
		}
		stackOutput, err := ts.cfg.CFNAPI.CreateStack(stackInput)
		if err != nil {
			asg.CreateTook += time.Since(timeStart)
			asg.CreateTookString = asg.CreateTook.String()
			ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = asg
			ts.cfg.EKSConfig.Sync()
			return err
		}
		asg.ASGCFNStackID = aws.StringValue(stackOutput.StackId)
		ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = asg
		ts.cfg.EKSConfig.Sync()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		ch := awscfn.Poll(
			ctx,
			ts.cfg.Stopc,
			ts.cfg.Sig,
			ts.cfg.Logger,
			ts.cfg.CFNAPI,
			asg.ASGCFNStackID,
			cloudformation.ResourceStatusCreateComplete,
			2*time.Minute,
			30*time.Second,
		)
		var st awscfn.StackStatus
		for st = range ch {
			if st.Error != nil {
				ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to create ASG (%v)", st.Error))
				ts.cfg.Logger.Warn("polling errror", zap.Error(st.Error))
			}
		}
		cancel()
		if st.Error != nil {
			asg.CreateTook += time.Since(timeStart)
			asg.CreateTookString = asg.CreateTook.String()
			ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = asg
			ts.cfg.EKSConfig.Sync()
			return st.Error
		}
		// update status after creating a new ASG
		for _, o := range st.Stack.Outputs {
			switch k := aws.StringValue(o.OutputKey); k {
			case "ASGName":
				ts.cfg.Logger.Info("found ASGName value from CFN", zap.String("value", aws.StringValue(o.OutputValue)))
			default:
				asg.CreateTook += time.Since(timeStart)
				asg.CreateTookString = asg.CreateTook.String()
				ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = asg
				ts.cfg.EKSConfig.Sync()
				return fmt.Errorf("unexpected OutputKey %q from %q", k, asg.ASGCFNStackID)
			}
		}

		ts.cfg.Logger.Info("created ASG",
			zap.String("name", asg.Name),
			zap.String("cfn-stack-id", asg.ASGCFNStackID),
			zap.String("request-started", humanize.RelTime(timeStart, time.Now(), "ago", "from now")),
		)
		asg.CreateTook += time.Since(timeStart)
		asg.CreateTookString = asg.CreateTook.String()
		ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = asg
		ts.cfg.EKSConfig.Sync()

		var aout *autoscaling.DescribeAutoScalingGroupsOutput
		aout, err = ts.cfg.ASGAPI.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
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
		ts.cfg.Logger.Info(
			"describing EC2 instances in ASG",
			zap.String("asg-name", asgName),
			zap.Strings("instance-ids", instanceIDs),
		)
		ec2Instances, err := awsapiec2.PollUntilRunning(
			10*time.Minute,
			ts.cfg.Logger,
			ts.cfg.EC2API,
			instanceIDs...,
		)
		if err != nil {
			asg.CreateTook += time.Since(timeStart)
			asg.CreateTookString = asg.CreateTook.String()
			ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = asg
			ts.cfg.EKSConfig.Sync()
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
		ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = asg
		ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("%q/%s", asgName, cloudformation.ResourceStatusCreateComplete))
		ts.cfg.EKSConfig.Sync()
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteASGs() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() {
		ts.cfg.Logger.Info("skipping node group ASG deletion")
		return nil
	}

	ts.cfg.Logger.Info("deleting ASGs using CFN", zap.String("name", ts.cfg.EKSConfig.Name))
	for asgName, asg := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
		if asg.ASGCFNStackID == "" {
			return fmt.Errorf("%q ASG stack ID is empty", asg.Name)
		}
		timeStart := time.Now()
		ts.cfg.Logger.Info("deleting ASG", zap.String("name", asgName), zap.String("cfn-stack-id", asg.ASGCFNStackID))
		_, err := ts.cfg.CFNAPI.DeleteStack(&cloudformation.DeleteStackInput{
			StackName: aws.String(asg.ASGCFNStackID),
		})
		if err != nil {
			asg.DeleteTook += time.Since(timeStart)
			asg.DeleteTookString = asg.DeleteTook.String()
			ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = asg
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to delete ASG (%v)", err))
			ts.cfg.EKSConfig.Sync()
			return err
		}
		ts.cfg.EKSConfig.Sync()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		ch := awscfn.Poll(
			ctx,
			make(chan struct{}),  // do not exit on stop
			make(chan os.Signal), // do not exit on stop
			ts.cfg.Logger,
			ts.cfg.CFNAPI,
			asg.ASGCFNStackID,
			cloudformation.ResourceStatusDeleteComplete,
			2*time.Minute,
			20*time.Second,
		)
		var st awscfn.StackStatus
		for st = range ch {
			if st.Error != nil {
				cancel()
				ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to delete ASG (%v)", st.Error))
				ts.cfg.Logger.Warn("polling errror", zap.Error(st.Error))
			}
		}
		cancel()
		if st.Error != nil {
			asg.DeleteTook += time.Since(timeStart)
			asg.DeleteTookString = asg.DeleteTook.String()
			ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = asg
			ts.cfg.EKSConfig.Sync()
			return st.Error
		}
		ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("%q/%s", asgName, ec2config.StatusDELETEDORNOTEXIST))
		asg.DeleteTook += time.Since(timeStart)
		asg.DeleteTookString = asg.DeleteTook.String()
		ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = asg
		ts.cfg.EKSConfig.Sync()
	}

	return ts.cfg.EKSConfig.Sync()
}
