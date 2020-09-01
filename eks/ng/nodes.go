package ng

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/pkg/aws/cfn"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-k8s-tester/pkg/user"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
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
aws ssm get-parameters --names /aws/service/eks/optimized-ami/1.16/amazon-linux-2/recommended/image_id
aws ssm get-parameters --names /aws/service/bottlerocket/aws-k8s-1.16/x86_64/latest/image_id

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
    Default: 0
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
        MinInstancesInService: !Ref ASGMinSize
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
      MaxSize: !Ref ASGMaxSize{{ if ne .ASGDesiredCapacity 0 }}
      DesiredCapacity: !Ref ASGDesiredCapacity{{ end }}
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
{{ if ne .ASGTagData "" }}{{.ASGTagData}}{{ end }}
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
            - InstanceType: !Select [ 1, !Ref InstanceTypes ]
            - !Ref AWS::NoValue
          - Fn::If:
            - Has3InstanceTypes
            - InstanceType: !Select [ 2, !Ref InstanceTypes ]
            - !Ref AWS::NoValue
          - Fn::If:
            - Has4InstanceTypes
            - InstanceType: !Select [ 3, !Ref InstanceTypes ]
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
const asgTagDataNG = `      - Key: !Sub k8s.io/cluster-autoscaler/${ClusterName}
        Value: owned
        PropagateAtLaunch: true
      - Key: k8s.io/cluster-autoscaler/enabled
        Value: true
        PropagateAtLaunch: true
`

type templateASG struct {
	Metadata           string
	UserData           string
	ASGDesiredCapacity int64
	ASGTagData         string
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
		tg := templateASG{
			ASGDesiredCapacity: cur.ASGDesiredCapacity,
		}
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
              NodeType = "regular"
              AMIType = "%s"
              NGType = "custom"
              NGName = "%s"
`,
				ts.cfg.EKSConfig.Name,
				ts.cfg.EKSConfig.Status.ClusterCA,
				ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint,
				ec2config.AMITypeBottleRocketCPU,
				asgName,
			)

		case ec2config.AMITypeAL2X8664,
			ec2config.AMITypeAL2X8664GPU:
			// https://github.com/awslabs/amazon-eks-ami/blob/master/files/bootstrap.sh
			clusterVPCIP := ts.cfg.EKSConfig.Parameters.VPCCIDR
			dnsClusterIP := "10.100.0.10"
			if clusterVPCIP[:strings.IndexByte(clusterVPCIP, '.')] == "10" {
				dnsClusterIP = "172.20.0.10"
			}
			tg.Metadata = metadataAL2InstallSSM
			tg.UserData = userDataAL2InstallSSM
			tg.UserData += `              /etc/eks/bootstrap.sh ${ClusterName}`
			if ts.cfg.EKSConfig.Parameters.ResolverURL != "" {
				ts.cfg.Logger.Info("adding extra bootstrap arguments --b64-cluster-ca and --apiserver-endpoint to user data",
					zap.String("b64-cluster-ca", ts.cfg.EKSConfig.Status.ClusterCA),
					zap.String("apiserver-endpoint", ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint),
				)
				tg.UserData += fmt.Sprintf(` --b64-cluster-ca %s --apiserver-endpoint %s --dns-cluster-ip %s`, ts.cfg.EKSConfig.Status.ClusterCA, ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint, dnsClusterIP)
			}
			// https://aws.amazon.com/blogs/opensource/improvements-eks-worker-node-provisioning/
			tg.UserData += fmt.Sprintf(` --kubelet-extra-args '--node-labels=NodeType=regular,AMIType=%s,NGType=custom,NGName=%s`, cur.AMIType, asgName)
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
		tg.ASGTagData = ""
		if cur.ClusterAutoscaler != nil && cur.ClusterAutoscaler.Enable {
			tg.ASGTagData = asgTagDataNG
		}
		tpl := template.Must(template.New("TemplateASG").Parse(TemplateASG))
		buf := bytes.NewBuffer(nil)
		if err := tpl.Execute(buf, tg); err != nil {
			return err
		}

		if err := ioutil.WriteFile(cur.ASGCFNStackYAMLPath, buf.Bytes(), 0400); err != nil {
			return err
		}
		ts.cfg.Logger.Info("creating a new NG using CFN",
			zap.String("asg-name", asgName),
			zap.String("asg-cfn-file-path", cur.ASGCFNStackYAMLPath),
		)

		stackInput := &cloudformation.CreateStackInput{
			StackName:    aws.String(fmt.Sprintf("%s-%s", ts.cfg.EKSConfig.Name, cur.Name)),
			Capabilities: aws.StringSlice([]string{"CAPABILITY_NAMED_IAM"}),
			OnFailure:    aws.String(cloudformation.OnFailureDelete),
			TemplateBody: aws.String(buf.String()),
			Tags: cfn.NewTags(map[string]string{
				"Kind":                   "aws-k8s-tester",
				"Name":                   ts.cfg.EKSConfig.Name,
				"aws-k8s-tester-version": version.ReleaseVersion,
				"User":                   user.Get(),
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
		if cur.ImageID != "" {
			ts.cfg.Logger.Info("added image ID", zap.String("image-id", cur.ImageID))
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ImageID"),
				ParameterValue: aws.String(cur.ImageID),
			})
		}
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
			cur.TimeFrameCreate = timeutil.NewTimeFrame(timeStart, time.Now())
			ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
			ts.cfg.EKSConfig.Sync()
			return err
		}
		cur.ASGCFNStackID = aws.StringValue(stackOutput.StackId)
		ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
		ts.cfg.EKSConfig.AddOnNodeGroups.Created = true
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

		checkN := time.Duration(cur.ASGDesiredCapacity)
		if checkN == 0 {
			checkN = time.Duration(cur.ASGMinSize)
		}
		waitDur := 30*time.Minute + 5*time.Second*checkN
		for _, it := range cur.InstanceTypes {
			if strings.Contains(it, ".metal") { // "i3.metal" takes much longer
				ts.cfg.Logger.Info("increasing wait time for metal instance", zap.String("instance-type", it))
				waitDur = time.Hour + time.Minute*checkN
			}
		}
		timeStart := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), waitDur)
		ch := cfn.Poll(
			ctx,
			ts.cfg.Stopc,
			ts.cfg.Logger,
			ts.cfg.LogWriter,
			ts.cfg.CFNAPI,
			cur.ASGCFNStackID,
			cloudformation.ResourceStatusCreateComplete,
			2*time.Minute,
			30*time.Second,
		)
		var st cfn.StackStatus
		for st = range ch {
			if st.Error != nil {
				ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to create ASG (%v)", st.Error))
				ts.cfg.Logger.Warn("polling errror", zap.Error(st.Error))
			}
		}
		cancel()
		if st.Error != nil {
			timeEnd := time.Now()
			cur.TimeFrameCreate = timeutil.NewTimeFrame(cur.TimeFrameCreate.StartUTC, cur.TimeFrameCreate.EndUTC.Add(timeEnd.Sub(timeStart)))
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
				timeEnd := time.Now()
				cur.TimeFrameCreate = timeutil.NewTimeFrame(timeStart, timeEnd)
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
		timeEnd := time.Now()
		cur.TimeFrameCreate = timeutil.NewTimeFrame(timeStart, timeEnd)
		ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
		ts.cfg.EKSConfig.Sync()

		timeStart = time.Now()
		if err := ts.nodeWaiter.Wait(cur.Name, 3); err != nil {
			return err
		}
		cur, ok = ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName]
		if !ok {
			return fmt.Errorf("ASGs[%q] not found after creation", asgName)
		}
		timeEnd = time.Now()
		cur.TimeFrameCreate = timeutil.NewTimeFrame(cur.TimeFrameCreate.StartUTC, cur.TimeFrameCreate.EndUTC.Add(timeEnd.Sub(timeStart)))
		ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
		ts.cfg.EKSConfig.Sync()

		ts.cfg.Logger.Info("created a Node Group",
			zap.String("ng-name", cur.Name),
			zap.String("took", cur.TimeFrameCreate.TookString),
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
			timeEnd := time.Now()
			cur.TimeFrameDelete = timeutil.NewTimeFrame(timeStart, timeEnd)
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
		ch := cfn.Poll(
			ctx,
			make(chan struct{}), // do not exit on stop
			ts.cfg.Logger,
			ts.cfg.LogWriter,
			ts.cfg.CFNAPI,
			cur.ASGCFNStackID,
			cloudformation.ResourceStatusDeleteComplete,
			2*time.Minute,
			20*time.Second,
		)
		var st cfn.StackStatus
		for st = range ch {
			if st.Error != nil {
				cancel()
				ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to delete ASG (%v)", st.Error))
				ts.cfg.Logger.Warn("polling errror", zap.Error(st.Error))
			}
		}
		cancel()
		if st.Error != nil {
			timeEnd := time.Now()
			cur.TimeFrameDelete = timeutil.NewTimeFrame(cur.TimeFrameDelete.StartUTC, cur.TimeFrameDelete.EndUTC.Add(timeEnd.Sub(timeStart)))
			ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
			ts.cfg.EKSConfig.Sync()
			return st.Error
		}
		ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("%q/%s", asgName, ec2config.StatusDELETEDORNOTEXIST))
		timeEnd := time.Now()
		cur.TimeFrameDelete = timeutil.NewTimeFrame(cur.TimeFrameDelete.StartUTC, cur.TimeFrameDelete.EndUTC.Add(timeEnd.Sub(timeStart)))
		ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
		ts.cfg.EKSConfig.Sync()
	}

	return ts.cfg.EKSConfig.Sync()
}
