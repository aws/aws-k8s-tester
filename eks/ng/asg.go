package ng

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
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
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
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
aws ssm get-parameters --names /aws/service/eks/optimized-ami/1.15/amazon-linux-2/recommended/image_id
aws ssm get-parameters --names /aws/service/bottlerocket/aws-k8s-1.15/x86_64/latest/image_id


TODO

  BootstrapArguments:
    Type: String
    Description: Arguments to pass to the bootstrap script. See files/bootstrap.sh in https://github.com/awslabs/amazon-eks-ami
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

  NodeGroupSecurityGroupID:
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
          - !Ref NodeGroupSecurityGroupID
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

	// track timestamps and check status in reverse order
	// to minimize polling API calls
	tss := make(tupleTimes, 0)

	// https://github.com/awslabs/amazon-eks-ami/blob/master/files/bootstrap.sh
	ts.cfg.Logger.Info("creating ASGs using CFN", zap.String("name", ts.cfg.EKSConfig.Name))
	for asgName, cur := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
		timeStart := time.Now()
		ts.cfg.Logger.Info("creating ASG", zap.String("name", asgName))

		// TODO: may not be necessary
		// "/aws/service/ami-amazon-linux-latest/amzn2-ami-hvm-x86_64-gp2"
		// already includes SSM agent + AWS CLI
		tg := templateASG{}
		switch cur.AMIType {
		case ec2config.AMITypeBottleRocketCPU:
			// "bottlerocket" comes with SSM agent
			tg.Metadata = ""
			tg.UserData = fmt.Sprintf(`        UserData:
          Fn::Base64:
            Fn::Sub: |
              [settings.kubernetes]
              cluster-name = "%s"
              cluster-certificate = "%s"
              api-server = "%s"
              [settings.kubernetes.node-labels]
              AMIType = "%s"
              NGType = "custom"
`,
				ts.cfg.EKSConfig.Name,
				ts.cfg.EKSConfig.Status.ClusterCA,
				ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint,
				ec2config.AMITypeBottleRocketCPU,
			)

		case ec2config.AMITypeAL2X8664,
			ec2config.AMITypeAL2X8664GPU:
			// https://github.com/awslabs/amazon-eks-ami/blob/master/files/bootstrap.sh
			tg.Metadata = metadataAL2InstallSSM
			tg.UserData = userDataAL2InstallSSM
			tg.UserData += `              /etc/eks/bootstrap.sh ${ClusterName}`
			if ts.cfg.EKSConfig.Parameters.ResolverURL != "" {
				ts.cfg.Logger.Info("adding extra bootstrap arguments --b64-cluster-ca and --apiserver-endpoint to user data",
					zap.String("b64-cluster-ca", ts.cfg.EKSConfig.Status.ClusterCA),
					zap.String("apiserver-endpoint", ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint),
				)
				tg.UserData += fmt.Sprintf(` --b64-cluster-ca %s --apiserver-endpoint %s`, ts.cfg.EKSConfig.Status.ClusterCA, ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint)
			}
			// https://aws.amazon.com/blogs/opensource/improvements-eks-worker-node-provisioning/
			tg.UserData += fmt.Sprintf(` --kubelet-extra-args '--node-labels=AMIType=%s,NGType=custom`, cur.AMIType)
			if cur.KubeletExtraArgs != "" {
				ts.cfg.Logger.Info("adding extra bootstrap arguments --kubelet-extra-args to user data",
					zap.String("kubelet-extra-args", cur.KubeletExtraArgs),
				)
				tg.UserData += fmt.Sprintf(` %s`, cur.KubeletExtraArgs)
			}
			tg.UserData += "'"
			tg.UserData += "\n"
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
					ParameterKey:   aws.String("ClusterName"),
					ParameterValue: aws.String(ts.cfg.EKSConfig.Name),
				},
				{
					ParameterKey:   aws.String("ASGName"),
					ParameterValue: aws.String(cur.Name),
				},
				{
					ParameterKey:   aws.String("ASGLaunchTemplateName"),
					ParameterValue: aws.String(cur.Name + "-launch-template"),
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
					ParameterKey:   aws.String("NodeGroupSecurityGroupID"),
					ParameterValue: aws.String(ts.cfg.EKSConfig.AddOnNodeGroups.NodeGroupSecurityGroupID),
				},
				{
					ParameterKey:   aws.String("RemoteAccessKeyName"),
					ParameterValue: aws.String(ts.cfg.EKSConfig.RemoteAccessKeyName),
				},
			},
		}
		ts.cfg.Logger.Info("added image ID", zap.String("image-id", cur.ImageID))
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("ImageID"),
			ParameterValue: aws.String(cur.ImageID),
		})
		if cur.ImageIDSSMParameter != "" {
			ts.cfg.Logger.Info("added image SSM parameter", zap.String("image-id-ssm-parameter", cur.ImageIDSSMParameter))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ImageIDSSMParameter"),
				ParameterValue: aws.String(cur.ImageIDSSMParameter),
			})
		}
		if len(cur.InstanceTypes) > 0 {
			ts.cfg.Logger.Info("added instance type", zap.Strings("instance-types", cur.InstanceTypes))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("InstanceTypes"),
				ParameterValue: aws.String(strings.Join(cur.InstanceTypes, ",")),
			})
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("InstanceTypesCount"),
				ParameterValue: aws.String(fmt.Sprintf("%d", len(cur.InstanceTypes))),
			})
		}
		if cur.VolumeSize > 0 {
			ts.cfg.Logger.Info("added volume size", zap.Int64("volume-size", cur.VolumeSize))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("VolumeSize"),
				ParameterValue: aws.String(fmt.Sprintf("%d", cur.VolumeSize)),
			})
		}
		if cur.ASGMinSize > 0 {
			ts.cfg.Logger.Info("added min size", zap.Int64("min-size", cur.ASGMinSize))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ASGMinSize"),
				ParameterValue: aws.String(fmt.Sprintf("%d", cur.ASGMinSize)),
			})
		}
		if cur.ASGMaxSize > 0 {
			ts.cfg.Logger.Info("added max size", zap.Int64("max-size", cur.ASGMaxSize))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ASGMaxSize"),
				ParameterValue: aws.String(fmt.Sprintf("%d", cur.ASGMaxSize)),
			})
		}
		if cur.ASGDesiredCapacity > 0 {
			ts.cfg.Logger.Info("added desired size", zap.Int64("desired-capacity", cur.ASGDesiredCapacity))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ASGDesiredCapacity"),
				ParameterValue: aws.String(fmt.Sprintf("%d", cur.ASGDesiredCapacity)),
			})
		}
		stackOutput, err := ts.cfg.CFNAPI.CreateStack(stackInput)
		if err != nil {
			cur.CreateTook += time.Since(timeStart)
			cur.CreateTookString = cur.CreateTook.String()
			ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
			ts.cfg.EKSConfig.Sync()
			return err
		}
		cur.ASGCFNStackID = aws.StringValue(stackOutput.StackId)
		ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
		ts.cfg.EKSConfig.Sync()

		tss = append(tss, tupleTime{ts: time.Now(), name: asgName})
	}

	sort.Sort(sort.Reverse(tss))

	// wait for ASG EC2 instances + Kubernetes nodes ready
	for _, tv := range tss {
		asgName := tv.name
		cur, ok := ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName]
		if !ok {
			return fmt.Errorf("ASG name %q not found after creation", asgName)
		}

		now := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		ch := awscfn.Poll(
			ctx,
			ts.cfg.Stopc,
			ts.cfg.Sig,
			ts.cfg.Logger,
			ts.cfg.CFNAPI,
			cur.ASGCFNStackID,
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
			cur.CreateTook += time.Since(now)
			cur.CreateTookString = cur.CreateTook.String()
			ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
			ts.cfg.EKSConfig.Sync()
			return st.Error
		}
		// update status after creating a new ASG
		for _, o := range st.Stack.Outputs {
			switch k := aws.StringValue(o.OutputKey); k {
			case "ASGName":
				ts.cfg.Logger.Info("found ASGName value from CFN", zap.String("value", aws.StringValue(o.OutputValue)))
			case "InstanceProfileARN":
				ts.cfg.Logger.Info("found InstanceProfileARN value from CFN", zap.String("value", aws.StringValue(o.OutputValue)))
			default:
				cur.CreateTook += time.Since(now)
				cur.CreateTookString = cur.CreateTook.String()
				ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
				ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("unexpected key from ASG stack (%v)", k))
				ts.cfg.EKSConfig.Sync()
				return fmt.Errorf("unexpected OutputKey %q from %q", k, cur.ASGCFNStackID)
			}
		}

		ts.cfg.Logger.Info("created ASG",
			zap.String("name", cur.Name),
			zap.String("cfn-stack-id", cur.ASGCFNStackID),
		)
		cur.CreateTook += time.Since(now)
		cur.CreateTookString = cur.CreateTook.String()
		ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
		ts.cfg.EKSConfig.Sync()

		if err := ts.waitForNodes(cur.Name); err != nil {
			return err
		}
		ts.cfg.Logger.Info("created a Node Group",
			zap.String("ng-name", cur.Name),
			zap.String("took", cur.CreateTookString),
		)
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteASGs() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() {
		ts.cfg.Logger.Info("skipping node group ASG deletion")
		return nil
	}

	ts.cfg.Logger.Info("deleting ASGs using CFN", zap.String("name", ts.cfg.EKSConfig.Name))
	for asgName, cur := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
		if cur.ASGCFNStackID == "" {
			return fmt.Errorf("%q ASG stack ID is empty", cur.Name)
		}
		timeStart := time.Now()
		ts.cfg.Logger.Info("deleting ASG", zap.String("name", asgName), zap.String("cfn-stack-id", cur.ASGCFNStackID))
		_, err := ts.cfg.CFNAPI.DeleteStack(&cloudformation.DeleteStackInput{
			StackName: aws.String(cur.ASGCFNStackID),
		})
		if err != nil {
			cur.DeleteTook += time.Since(timeStart)
			cur.DeleteTookString = cur.DeleteTook.String()
			ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to delete ASG (%v)", err))
			ts.cfg.EKSConfig.Sync()
			return err
		}
		ts.cfg.EKSConfig.Sync()
	}
	for asgName, cur := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
		timeStart := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		ch := awscfn.Poll(
			ctx,
			make(chan struct{}),  // do not exit on stop
			make(chan os.Signal), // do not exit on stop
			ts.cfg.Logger,
			ts.cfg.CFNAPI,
			cur.ASGCFNStackID,
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
			cur.DeleteTook += time.Since(timeStart)
			cur.DeleteTookString = cur.DeleteTook.String()
			ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
			ts.cfg.EKSConfig.Sync()
			return st.Error
		}
		ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("%q/%s", asgName, ec2config.StatusDELETEDORNOTEXIST))
		cur.DeleteTook += time.Since(timeStart)
		cur.DeleteTookString = cur.DeleteTook.String()
		ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
		ts.cfg.EKSConfig.Sync()
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitForNodes(asgName string) error {
	cur, ok := ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName]
	if !ok {
		return fmt.Errorf("Node Group %q not found", asgName)
	}
	waitDur := 3*time.Minute + time.Duration(15*cur.ASGDesiredCapacity)*time.Second

	aout, err := ts.cfg.ASGAPI.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: aws.StringSlice([]string{cur.Name}),
	})
	if err != nil {
		return fmt.Errorf("ASG %q not found (%v)", cur.Name, err)
	}
	if len(aout.AutoScalingGroups) != 1 {
		return fmt.Errorf("%q expected only 1 ASG, got %+v", cur.Name, aout.AutoScalingGroups)
	}

	av := aout.AutoScalingGroups[0]
	instanceIDs := make([]string, 0, len(av.Instances))
	for _, iv := range av.Instances {
		instanceIDs = append(instanceIDs, aws.StringValue(iv.InstanceId))
	}

	ts.cfg.Logger.Info(
		"describing EC2 instances in ASG",
		zap.String("asg-name", cur.Name),
		zap.Strings("instance-ids", instanceIDs),
	)
	ec2Instances, err := awsapiec2.PollUntilRunning(
		waitDur,
		ts.cfg.Logger,
		ts.cfg.EC2API,
		instanceIDs...,
	)
	if err != nil {
		return err
	}
	cur, ok = ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName]
	if !ok {
		return fmt.Errorf("Node Group %q not found", asgName)
	}
	cur.Instances = make(map[string]ec2config.Instance)
	for id, vv := range ec2Instances {
		ivv := ec2config.ConvertInstance(vv)
		ivv.RemoteAccessUserName = cur.RemoteAccessUserName
		cur.Instances[id] = ivv
	}
	ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
	ts.cfg.EKSConfig.Sync()

	cur, ok = ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName]
	if !ok {
		return fmt.Errorf("Node Group %q not found", asgName)
	}

	// Hostname/InternalDNS == EC2 private DNS
	// TODO: handle DHCP option domain name
	ec2PrivateDNS := make(map[string]struct{})
	for _, v := range cur.Instances {
		ts.cfg.Logger.Info("found private DNS for an EC2 instance", zap.String("instance-id", v.InstanceID), zap.String("private-dns-name", v.PrivateDNSName))
		ec2PrivateDNS[v.PrivateDNSName] = struct{}{}
		// "ip-192-168-81-186" from "ip-192-168-81-186.my-private-dns"
		ec2PrivateDNS[strings.Split(v.PrivateDNSName, ".")[0]] = struct{}{}
	}

	ts.cfg.Logger.Info("checking nodes readiness")
	var items []v1.Node
	retryStart := time.Now()
	ready := false
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("checking node aborted")
		case <-ts.cfg.Sig:
			return errors.New("checking node aborted")
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		nodes, err := ts.cfg.K8SClient.KubernetesClientSet().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("get nodes failed", zap.Error(err))
			continue
		}
		items = nodes.Items

		readies := 0
		for _, node := range items {
			nodeName := node.GetName()

			// e.g. given node name ip-192-168-81-186.us-west-2.compute.internal + DHCP option my-private-dns
			// InternalIP == 192.168.81.186
			// ExternalIP == 52.38.118.149
			// Hostname == my-private-dns (without DHCP option, it's "ip-192-168-81-186.my-private-dns", private DNS, InternalDNS)
			// InternalDNS == ip-192-168-81-186.my-private-dns
			// ExternalDNS == ec2-52-38-118-149.us-west-2.compute.amazonaws.com
			ts.cfg.Logger.Info("checking node address with EC2 Private DNS",
				zap.String("name", nodeName),
				zap.String("labels", fmt.Sprintf("%v", node.Labels)),
			)
			hostName := ""
			for _, av := range node.Status.Addresses {
				ts.cfg.Logger.Info("node status address",
					zap.String("name", nodeName),
					zap.String("type", string(av.Type)),
					zap.String("address", string(av.Address)),
				)
				if av.Type != v1.NodeHostName && av.Type != v1.NodeInternalDNS {
					continue
				}
				// handle when node is configured DHCP
				hostName = av.Address
				_, ok := ec2PrivateDNS[hostName]
				if !ok {
					// "ip-192-168-81-186" from "ip-192-168-81-186.my-private-dns"
					_, ok = ec2PrivateDNS[strings.Split(hostName, ".")[0]]
				}
				if ok {
					break
				}
			}
			if hostName == "" {
				return fmt.Errorf("%q not found for node %q", v1.NodeHostName, nodeName)
			}
			_, ok := ec2PrivateDNS[hostName]
			if !ok {
				// "ip-192-168-81-186" from "ip-192-168-81-186.my-private-dns"
				_, ok = ec2PrivateDNS[strings.Split(hostName, ".")[0]]
			}
			if !ok {
				ts.cfg.Logger.Warn("node may not belong to this ASG", zap.String("host-name", hostName), zap.String("ec2-private-dnss", fmt.Sprintf("%v", ec2PrivateDNS)))
				continue
			}
			ts.cfg.Logger.Info("checked node host name with EC2 Private DNS", zap.String("name", nodeName), zap.String("host-name", hostName))
			ts.cfg.Logger.Info("checking node readiness", zap.String("name", nodeName))
			for _, cond := range node.Status.Conditions {
				if cond.Status != v1.ConditionTrue {
					continue
				}
				if cond.Type != v1.NodeReady {
					continue
				}
				ts.cfg.Logger.Info("checked node readiness",
					zap.String("name", nodeName),
					zap.String("type", fmt.Sprintf("%s", cond.Type)),
					zap.String("status", fmt.Sprintf("%s", cond.Status)),
				)
				readies++
				break
			}
		}
		ts.cfg.Logger.Info("nodes",
			zap.Int("current-ready-nodes", readies),
			zap.Int64("desired-ready-nodes", cur.ASGDesiredCapacity),
		)

		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		output, err := exec.New().CommandContext(
			ctx,
			ts.cfg.EKSConfig.KubectlPath,
			"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
			"get",
			"csr",
			"-o=wide",
		).CombinedOutput()
		cancel()
		out := string(output)
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl get csr' failed", zap.Error(err))
		}
		fmt.Printf("\n\n\"%s get csr\":\n%s\n", ts.cfg.EKSConfig.KubectlCommand(), out)

		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		output, err = exec.New().CommandContext(
			ctx,
			ts.cfg.EKSConfig.KubectlPath,
			"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
			"get",
			"nodes",
			"--show-labels",
			"-o=wide",
		).CombinedOutput()
		cancel()
		out = string(output)
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl get nodes -o=wide' failed", zap.Error(err))
		}
		fmt.Printf("\n\"%s get nodes -o=wide\":\n%s\n\n", ts.cfg.EKSConfig.KubectlCommand(), out)

		if int64(readies) >= cur.ASGDesiredCapacity { // TODO: check per node group
			ready = true
			break
		}
	}
	if !ready {
		return fmt.Errorf("NG %q not ready", asgName)
	}

	println()
	fmt.Printf("%q nodes are ready!\n", asgName)
	for _, v := range items {
		fmt.Printf("node %q address: %+v\n", v.GetName(), v.Status.Addresses)
	}
	println()
	return ts.cfg.EKSConfig.Sync()
}
