package ec2

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	aws_ec2 "github.com/aws/aws-k8s-tester/pkg/aws/ec2"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	aws_v2 "github.com/aws/aws-sdk-go-v2/aws"
	aws_asg_v2 "github.com/aws/aws-sdk-go-v2/service/autoscaling"
	aws_asg_v2_types "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	aws_ec2_v2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	aws_ec2_v2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	aws_ssm_v2 "github.com/aws/aws-sdk-go-v2/service/ssm"
	smithy "github.com/aws/smithy-go"
	"go.uber.org/zap"
)

func (ts *Tester) createASGs() (err error) {
	tss, err := ts._createASGs()
	if err != nil {
		return err
	}
	if err = ts.waitForASGs(tss); err != nil {
		return err
	}
	return nil
}

func (ts *Tester) deleteASGs() error {
	var errs []string

	if err := ts._deleteASGs(); err != nil {
		ts.lg.Warn("failed to delete ASGs", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ","))
	}
	return nil
}

// track timestamps and check status in reverse order to minimize polling API calls
func (ts *Tester) _createASGs() (tss tupleTimes, err error) {
	ts.lg.Info("creating ASGs")

	for asgName, cur := range ts.cfg.ASGs {
		imgID := cur.ImageID
		if imgID == "" {
			imgID, err = ts.fetchImageID(cur.ImageIDSSMParameter)
			if err != nil {
				return nil, err
			}
		}
		ts.lg.Info("creating launch template",
			zap.String("launch-template-name", cur.LaunchTemplateName),
			zap.String("image-id", imgID),
		)

		userData, err := ts.generateUserData(ts.cfg.Region, cur.AMIType)
		if err != nil {
			return nil, fmt.Errorf("failed to create user data for %q (%v)", asgName, err)
		}
		userData = base64.StdEncoding.EncodeToString([]byte(userData))

		_, err = ts.ec2APIV2.CreateLaunchTemplate(
			context.Background(),
			&aws_ec2_v2.CreateLaunchTemplateInput{
				LaunchTemplateName: aws_v2.String(cur.LaunchTemplateName),

				LaunchTemplateData: &aws_ec2_v2_types.RequestLaunchTemplateData{
					IamInstanceProfile: &aws_ec2_v2_types.LaunchTemplateIamInstanceProfileSpecificationRequest{
						Arn: aws_v2.String(ts.cfg.Role.InstanceProfileARN),
					},

					KeyName: aws_v2.String(ts.cfg.RemoteAccessKeyName),

					ImageId:      aws_v2.String(imgID),
					InstanceType: aws_ec2_v2_types.InstanceType(cur.InstanceType),

					BlockDeviceMappings: []aws_ec2_v2_types.LaunchTemplateBlockDeviceMappingRequest{
						{
							DeviceName: aws_v2.String("/dev/xvda"),
							Ebs: &aws_ec2_v2_types.LaunchTemplateEbsBlockDeviceRequest{
								DeleteOnTermination: aws_v2.Bool(true),
								Encrypted:           aws_v2.Bool(true),
								VolumeType:          aws_ec2_v2_types.VolumeTypeGp3,
								VolumeSize:          aws_v2.Int32(cur.VolumeSize),
							},
						},
					},

					// for public DNS + SSH access
					NetworkInterfaces: []aws_ec2_v2_types.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest{
						{
							AssociatePublicIpAddress: aws_v2.Bool(true),
							DeleteOnTermination:      aws_v2.Bool(true),
							DeviceIndex:              aws_v2.Int32(0),
							Groups:                   []string{ts.cfg.VPC.SecurityGroupID},
						},
					},

					UserData: aws_v2.String(userData),

					Monitoring:                        &aws_ec2_v2_types.LaunchTemplatesMonitoringRequest{Enabled: aws_v2.Bool(true)},
					InstanceInitiatedShutdownBehavior: aws_ec2_v2_types.ShutdownBehaviorTerminate,
				},

				TagSpecifications: []aws_ec2_v2_types.TagSpecification{
					{
						ResourceType: aws_ec2_v2_types.ResourceTypeLaunchTemplate,
						Tags: []aws_ec2_v2_types.Tag{
							{
								Key:   aws_v2.String("Name"),
								Value: aws_v2.String(fmt.Sprintf("%s-instance-launch-template", cur.Name)),
							},
						},
					},
				},
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create launch template for %q (%v)", asgName, err)
		}

		select {
		case <-time.After(10 * time.Second):
		case <-ts.stopCreationCh:
			return nil, errors.New("stopped")
		}

		ts.lg.Info("creating ASG",
			zap.String("asg-name", asgName),
			zap.String("image-id", imgID),
		)

		// TOOD: tag instance and volume
		// Valid requests must contain either LaunchTemplate, LaunchConfigurationName, InstanceId or MixedInstancesPolicy parameter
		asgInput := &aws_asg_v2.CreateAutoScalingGroupInput{
			AutoScalingGroupName:   aws_v2.String(asgName),
			MaxSize:                aws_v2.Int32(cur.ASGMaxSize),
			MinSize:                aws_v2.Int32(cur.ASGMinSize),
			VPCZoneIdentifier:      aws_v2.String(strings.Join(ts.cfg.VPC.PublicSubnetIDs, ",")),
			HealthCheckGracePeriod: aws_v2.Int32(300),
			HealthCheckType:        aws_v2.String("EC2"),
			LaunchTemplate: &aws_asg_v2_types.LaunchTemplateSpecification{
				LaunchTemplateName: aws_v2.String(cur.LaunchTemplateName),
				Version:            aws_v2.String("$Latest"),
			},
			Tags: []aws_asg_v2_types.Tag{
				{
					Key:               aws_v2.String("Name"),
					Value:             aws_v2.String(cur.Name),
					PropagateAtLaunch: aws_v2.Bool(true),
				},
				{
					Key:               aws_v2.String(fmt.Sprintf("kubernetes.io/cluster/%s", ts.cfg.Name)),
					Value:             aws_v2.String("owned"),
					PropagateAtLaunch: aws_v2.Bool(true),
				},
				{
					Key:               aws_v2.String(fmt.Sprintf("kubernetes.io/cluster-autoscaler/%s", ts.cfg.Name)),
					Value:             aws_v2.String("owned"),
					PropagateAtLaunch: aws_v2.Bool(true),
				},
				{
					Key:               aws_v2.String("kubernetes.io/cluster-autoscaler/enabled"),
					Value:             aws_v2.String("true"),
					PropagateAtLaunch: aws_v2.Bool(true),
				},
			},
		}
		if cur.ASGDesiredCapacity > 0 {
			asgInput.DesiredCapacity = aws_v2.Int32(cur.ASGDesiredCapacity)
		}
		_, err = ts.asgAPIV2.CreateAutoScalingGroup(context.Background(), asgInput)
		if err != nil {
			return nil, fmt.Errorf("failed to create ASG for %q (%v)", asgName, err)
		}

		ts.cfg.ASGs[asgName] = cur
		ts.cfg.Sync()

		tss = append(tss, tupleTime{ts: time.Now(), name: asgName})
	}

	sort.Sort(sort.Reverse(tss))
	ts.lg.Info("created ASGs")
	return tss, nil
}

func (ts *Tester) _deleteASGs() (err error) {
	ts.lg.Info("deleting ASGs")

	for asgName, cur := range ts.cfg.ASGs {
		_, err = ts.asgAPIV2.DeleteAutoScalingGroup(
			context.Background(),
			&aws_asg_v2.DeleteAutoScalingGroupInput{
				AutoScalingGroupName: aws_v2.String(asgName),
				ForceDelete:          aws_v2.Bool(true),
			},
		)
		if err != nil {
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				if strings.Contains(apiErr.ErrorCode(), "NotFound") {
					ts.cfg.DeletedResources[asgName] = "ASGs"
					ts.cfg.Sync()
					return nil
				}
			}
			return fmt.Errorf("failed to delete ASG for %q (%v)", asgName, err)
		}

		select {
		case <-time.After(30 * time.Second):
		case <-ts.stopCreationCh:
			return errors.New("stopped")
		}

		for i := 0; i < int(cur.ASGDesiredCapacity); i++ {
			if _, ok := ts.cfg.DeletedResources[asgName]; ok {
				break
			}
			select {
			case <-time.After(5 * time.Second):
			case <-ts.stopCreationCh:
				return errors.New("stopped")
			}

			ts.lg.Info("polling ASG until deletion", zap.String("asg-name", asgName))
			aout, err := ts.asgAPIV2.DescribeAutoScalingGroups(
				context.Background(),
				&aws_asg_v2.DescribeAutoScalingGroupsInput{
					AutoScalingGroupNames: []string{asgName},
				},
			)
			if err != nil {
				ts.lg.Warn("failed to describe ASG", zap.String("asg-name", asgName), zap.Error(err))
				var apiErr smithy.APIError
				if errors.As(err, &apiErr) {
					if strings.Contains(apiErr.ErrorCode(), "NotFound") {
						ts.cfg.DeletedResources[asgName] = "ASGs"
						ts.cfg.Sync()
						break
					}
				}
				continue
			}
			ts.lg.Info("described ASG",
				zap.String("asg-name", asgName),
				zap.Int("results", len(aout.AutoScalingGroups)),
			)
			if len(aout.AutoScalingGroups) == 0 {
				ts.cfg.DeletedResources[asgName] = "ASGs"
				ts.cfg.Sync()
				break
			}
			if len(aout.AutoScalingGroups[0].Instances) == 0 {
				ts.cfg.DeletedResources[asgName] = "ASGs"
				ts.cfg.Sync()
				break
			}
			ts.lg.Info("ASG still has instances; retrying",
				zap.String("asg-name", asgName),
				zap.Int("instances", len(aout.AutoScalingGroups[0].Instances)),
			)
		}

		for i := 0; i < int(cur.ASGDesiredCapacity); i++ {
			if _, ok := ts.cfg.DeletedResources[cur.LaunchTemplateName]; ok {
				break
			}
			select {
			case <-time.After(5 * time.Second):
			case <-ts.stopCreationCh:
				return errors.New("stopped")
			}

			_, err = ts.ec2APIV2.DeleteLaunchTemplate(
				context.Background(),
				&aws_ec2_v2.DeleteLaunchTemplateInput{
					LaunchTemplateName: aws_v2.String(cur.LaunchTemplateName),
				},
			)
			if err != nil {
				ts.lg.Warn("failed to delete launch template", zap.String("name", cur.LaunchTemplateName), zap.Error(err))
				var apiErr smithy.APIError
				if errors.As(err, &apiErr) {
					if strings.Contains(apiErr.ErrorCode(), "NotFound") {
						ts.cfg.DeletedResources[cur.LaunchTemplateName] = "ASGs.LaunchTemplateName"
						ts.cfg.Sync()
						break
					}
				}
				continue
			}

			ts.cfg.DeletedResources[cur.LaunchTemplateName] = "ASGs.LaunchTemplateName"
			ts.cfg.Sync()
			break
		}
	}

	ts.lg.Info("deleted ASGs")
	return nil
}

func (ts *Tester) waitForASGs(tss tupleTimes) (err error) {
	ts.lg.Info("waiting for ASGs")

	timeStart := time.Now()
	for _, tv := range tss {
		asgName := tv.name
		cur, ok := ts.cfg.ASGs[asgName]
		if !ok {
			return fmt.Errorf("ASG name %q not found after creation", asgName)
		}

		select {
		case <-time.After(10 * time.Second):
		case <-ts.stopCreationCh:
			return errors.New("stopped")
		}

		ts.lg.Info("waiting for ASG", zap.String("asg-name", asgName))

		checkN := time.Duration(cur.ASGDesiredCapacity)
		if checkN == 0 {
			checkN = time.Duration(cur.ASGMinSize)
		}
		waitDur := 30*time.Minute + 10*time.Second*checkN
		if strings.Contains(cur.InstanceType, ".metal") { // "i3.metal" takes much longer
			ts.lg.Info("increasing wait time for metal instance", zap.String("instance-type", cur.InstanceType))
			waitDur = time.Hour + time.Minute*checkN
		}

		ctx, cancel := context.WithTimeout(context.Background(), waitDur)
		ec2Instances, err := aws_ec2.WaitUntilRunning(
			ctx,
			ts.stopCreationCh,
			ts.ec2APIV2,
			ts.asgAPIV2,
			cur.Name,
		)
		cancel()
		if err != nil {
			return err
		}

		cur, ok = ts.cfg.ASGs[asgName]
		if !ok {
			return fmt.Errorf("ASG %q not found", asgName)
		}
		cur.Instances = make(map[string]ec2config.Instance)
		for id, vv := range ec2Instances {
			ivv := ec2config.ConvertInstance(vv)
			ivv.RemoteAccessUserName = cur.RemoteAccessUserName
			cur.Instances[id] = ivv
		}

		cur.TimeFrameCreate = timeutil.NewTimeFrame(timeStart, time.Now())
		ts.cfg.ASGs[asgName] = cur
		ts.cfg.Sync()
		ts.lg.Info("waited for ASG", zap.String("asg-name", asgName), zap.Int("instances", len(cur.Instances)))
	}

	ts.lg.Info("waited for ASGs")
	return nil
}

func (ts *Tester) fetchImageID(ssmParam string) (img string, err error) {
	if ssmParam == "" {
		return "", errors.New("empty SSM parameter")
	}
	out, err := ts.ssmAPIV2.GetParameter(
		context.Background(),
		&aws_ssm_v2.GetParameterInput{
			Name: aws_v2.String(ssmParam),
		},
	)
	if err != nil {
		return "", err
	}
	return aws_v2.ToString(out.Parameter.Value), nil
}

// MUST install SSM agent, otherwise, it will "InvalidInstanceId:"
// ref. https://docs.aws.amazon.com/systems-manager/latest/userguide/agent-install-al2.html
func (ts *Tester) generateUserData(region string, amiType string) (d string, err error) {
	arch := "amd64"
	if amiType == ec2config.AMITypeAL2ARM64 {
		arch = "arm64"
	}
	d = fmt.Sprintf(`#!/bin/bash
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

sudo yum install -y https://s3.%s.amazonaws.com/amazon-ssm-%s/latest/linux_%s/amazon-ssm-agent.rpm

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
`, region, region, arch)
	return d, nil
}
