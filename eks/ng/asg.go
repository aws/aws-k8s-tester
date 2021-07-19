package ng

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	aws_v2 "github.com/aws/aws-sdk-go-v2/aws"
	aws_asg_v2 "github.com/aws/aws-sdk-go-v2/service/autoscaling"
	aws_asg_v2_types "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	aws_ec2_v2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	aws_ec2_v2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	aws_eks_v2_types "github.com/aws/aws-sdk-go-v2/service/eks/types"
	aws_ssm_v2 "github.com/aws/aws-sdk-go-v2/service/ssm"
	smithy "github.com/aws/smithy-go"
	"go.uber.org/zap"
)

func (ts *tester) createASGs() (err error) {
	tss, err := ts._createASGs()
	if err != nil {
		return err
	}
	if err = ts.waitForASGs(tss); err != nil {
		return err
	}
	return nil
}

func (ts *tester) deleteASGs() error {
	var errs []string

	if err := ts._deleteASGs(); err != nil {
		ts.cfg.Logger.Warn("failed to delete ASGs", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ","))
	}
	return nil
}

// track timestamps and check status in reverse order to minimize polling API calls
func (ts *tester) _createASGs() (tss tupleTimes, err error) {
	ts.cfg.Logger.Info("creating ASGs")

	for asgName, cur := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
		imgID := cur.ImageID
		if imgID == "" {
			imgID, err = ts.fetchImageID(cur.ImageIDSSMParameter)
			if err != nil {
				return nil, err
			}
		}
		ts.cfg.Logger.Info("creating launch template",
			zap.String("launch-template-name", cur.LaunchTemplateName),
			zap.String("image-id", imgID),
		)

		userData, err := ts.generateUserData(asgName, cur.AMIType, cur.KubeletExtraArgs, cur.BootstrapArgs)
		if err != nil {
			return nil, fmt.Errorf("failed to create user data for %q (%v)", asgName, err)
		}
		userData = base64.StdEncoding.EncodeToString([]byte(userData))

		_, err = ts.cfg.EC2APIV2.CreateLaunchTemplate(
			context.Background(),
			&aws_ec2_v2.CreateLaunchTemplateInput{
				LaunchTemplateName: aws_v2.String(cur.LaunchTemplateName),

				LaunchTemplateData: &aws_ec2_v2_types.RequestLaunchTemplateData{
					IamInstanceProfile: &aws_ec2_v2_types.LaunchTemplateIamInstanceProfileSpecificationRequest{
						Arn: aws_v2.String(ts.cfg.EKSConfig.AddOnNodeGroups.Role.InstanceProfileARN),
					},

					KeyName: aws_v2.String(ts.cfg.EKSConfig.RemoteAccessKeyName),

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
							Groups:                   []string{ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID},
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
		case <-ts.cfg.Stopc:
			return nil, errors.New("stopped")
		}

		ts.cfg.Logger.Info("creating ASG",
			zap.String("asg-name", asgName),
			zap.String("image-id", imgID),
		)

		// TOOD: tag instance and volume
		// Valid requests must contain either LaunchTemplate, LaunchConfigurationName, InstanceId or MixedInstancesPolicy parameter
		asgInput := &aws_asg_v2.CreateAutoScalingGroupInput{
			AutoScalingGroupName:   aws_v2.String(asgName),
			MaxSize:                aws_v2.Int32(cur.ASGMaxSize),
			MinSize:                aws_v2.Int32(cur.ASGMinSize),
			VPCZoneIdentifier:      aws_v2.String(strings.Join(ts.cfg.EKSConfig.VPC.PublicSubnetIDs, ",")),
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
					Key:               aws_v2.String(fmt.Sprintf("kubernetes.io/cluster/%s", ts.cfg.EKSConfig.Name)),
					Value:             aws_v2.String("owned"),
					PropagateAtLaunch: aws_v2.Bool(true),
				},
				{
					Key:               aws_v2.String(fmt.Sprintf("kubernetes.io/cluster-autoscaler/%s", ts.cfg.EKSConfig.Name)),
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
		_, err = ts.cfg.ASGAPIV2.CreateAutoScalingGroup(context.Background(), asgInput)
		if err != nil {
			return nil, fmt.Errorf("failed to create ASG for %q (%v)", asgName, err)
		}

		cur.Instances = make(map[string]ec2config.Instance)
		cur.Logs = make(map[string][]string)
		ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
		ts.cfg.EKSConfig.AddOnNodeGroups.Created = true
		ts.cfg.EKSConfig.Sync()

		tss = append(tss, tupleTime{ts: time.Now(), name: asgName})
	}

	sort.Sort(sort.Reverse(tss))
	ts.cfg.Logger.Info("created ASGs")
	return tss, nil
}

func (ts *tester) _deleteASGs() (err error) {
	ts.cfg.Logger.Info("deleting ASGs")

	for asgName, cur := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
		_, err = ts.cfg.ASGAPIV2.DeleteAutoScalingGroup(
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
					ts.cfg.EKSConfig.Status.DeletedResources[asgName] = "AddOnNodeGroups.ASGs"
					ts.cfg.EKSConfig.Sync()
					return nil
				}
			}
			return fmt.Errorf("failed to delete ASG for %q (%v)", asgName, err)
		}

		select {
		case <-time.After(30 * time.Second):
		case <-ts.cfg.Stopc:
			return errors.New("stopped")
		}

		for i := 0; i < int(cur.ASGDesiredCapacity); i++ {
			if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[asgName]; ok {
				break
			}
			select {
			case <-time.After(5 * time.Second):
			case <-ts.cfg.Stopc:
				return errors.New("stopped")
			}

			ts.cfg.Logger.Info("polling ASG until deletion", zap.String("asg-name", asgName))
			aout, err := ts.cfg.ASGAPIV2.DescribeAutoScalingGroups(
				context.Background(),
				&aws_asg_v2.DescribeAutoScalingGroupsInput{
					AutoScalingGroupNames: []string{asgName},
				},
			)
			if err != nil {
				ts.cfg.Logger.Warn("failed to describe ASG", zap.String("asg-name", asgName), zap.Error(err))
				var apiErr smithy.APIError
				if errors.As(err, &apiErr) {
					if strings.Contains(apiErr.ErrorCode(), "NotFound") {
						ts.cfg.EKSConfig.Status.DeletedResources[asgName] = "AddOnNodeGroups.ASGs"
						ts.cfg.EKSConfig.Sync()
						break
					}
				}
				continue
			}
			ts.cfg.Logger.Info("described ASG",
				zap.String("asg-name", asgName),
				zap.Int("results", len(aout.AutoScalingGroups)),
			)
			if len(aout.AutoScalingGroups) == 0 {
				ts.cfg.EKSConfig.Status.DeletedResources[asgName] = "AddOnNodeGroups.ASGs"
				ts.cfg.EKSConfig.Sync()
				break
			}
			if len(aout.AutoScalingGroups[0].Instances) == 0 {
				ts.cfg.EKSConfig.Status.DeletedResources[asgName] = "AddOnNodeGroups.ASGs"
				ts.cfg.EKSConfig.Sync()
				break
			}
			ts.cfg.Logger.Info("ASG still has instances; retrying",
				zap.String("asg-name", asgName),
				zap.Int("instances", len(aout.AutoScalingGroups[0].Instances)),
			)
		}

		for i := 0; i < int(cur.ASGDesiredCapacity); i++ {
			if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[cur.LaunchTemplateName]; ok {
				break
			}
			select {
			case <-time.After(5 * time.Second):
			case <-ts.cfg.Stopc:
				return errors.New("stopped")
			}

			_, err = ts.cfg.EC2APIV2.DeleteLaunchTemplate(
				context.Background(),
				&aws_ec2_v2.DeleteLaunchTemplateInput{
					LaunchTemplateName: aws_v2.String(cur.LaunchTemplateName),
				},
			)
			if err != nil {
				ts.cfg.Logger.Warn("failed to delete launch template", zap.String("name", cur.LaunchTemplateName), zap.Error(err))
				var apiErr smithy.APIError
				if errors.As(err, &apiErr) {
					if strings.Contains(apiErr.ErrorCode(), "NotFound") {
						ts.cfg.EKSConfig.Status.DeletedResources[cur.LaunchTemplateName] = "AddOnNodeGroups.ASGs.LaunchTemplateName"
						ts.cfg.EKSConfig.Sync()
						break
					}
				}
				continue
			}

			ts.cfg.EKSConfig.Status.DeletedResources[cur.LaunchTemplateName] = "AddOnNodeGroups.ASGs.LaunchTemplateName"
			ts.cfg.EKSConfig.Sync()
			break
		}
	}

	ts.cfg.Logger.Info("deleted ASGs")
	return nil
}

func (ts *tester) waitForASGs(tss tupleTimes) (err error) {
	ts.cfg.Logger.Info("waiting for ASGs")

	for _, tv := range tss {
		asgName := tv.name
		cur, ok := ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName]
		if !ok {
			return fmt.Errorf("ASG name %q not found after creation", asgName)
		}

		select {
		case <-time.After(10 * time.Second):
		case <-ts.cfg.Stopc:
			return errors.New("stopped")
		}

		ts.cfg.Logger.Info("waiting for ASG", zap.String("asg-name", asgName))

		timeStart := time.Now()
		if err := ts.nodeWaiter.Wait(asgName, 10); err != nil {
			return err
		}
		timeEnd := time.Now()

		cur, ok = ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName]
		if !ok {
			return fmt.Errorf("ASG name %q not found after creation", asgName)
		}
		cur.TimeFrameCreate = timeutil.NewTimeFrame(timeStart, timeEnd)
		ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
		ts.cfg.EKSConfig.Sync()

		ts.cfg.Logger.Info("waited for ASG", zap.String("asg-name", asgName))
	}

	ts.cfg.Logger.Info("waited for ASGs")
	return nil
}

func (ts *tester) fetchImageID(ssmParam string) (img string, err error) {
	if ssmParam == "" {
		return "", errors.New("empty SSM parameter")
	}
	out, err := ts.cfg.SSMAPIV2.GetParameter(
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

func (ts *tester) generateUserData(asgName string, amiType string, kubeletExtraArgs string, bootstrapArgs string) (d string, err error) {
	switch amiType {
	case ec2config.AMITypeBottleRocketCPU:
		d = fmt.Sprintf(`[settings.kubernetes]
cluster-name = "%s"
cluster-certificate = "%s"
api-server = "%s"
[settings.kubernetes.node-labels]
NodeType = "regular"
AMIType = "%s"
NGType = "custom"
NGName = "%s"`,
			ts.cfg.EKSConfig.Name,
			ts.cfg.EKSConfig.Status.ClusterCA,
			ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint,
			ec2config.AMITypeBottleRocketCPU,
			asgName,
		)

	case fmt.Sprint(aws_eks_v2_types.AMITypesAl2X8664),
		fmt.Sprint(aws_eks_v2_types.AMITypesAl2Arm64),
		fmt.Sprint(aws_eks_v2_types.AMITypesAl2X8664Gpu):
		d = fmt.Sprintf(`#!/bin/bash
set -xeu

/etc/eks/bootstrap.sh %s`, ts.cfg.EKSConfig.Name)
		if ts.cfg.EKSConfig.ResolverURL != "" {
			clusterVPCIP := ts.cfg.EKSConfig.VPC.CIDRs[0]
			dnsClusterIP := "10.100.0.10"
			if clusterVPCIP[:strings.IndexByte(clusterVPCIP, '.')] == "10" {
				dnsClusterIP = "172.20.0.10"
			}
			ts.cfg.Logger.Info("adding extra bootstrap arguments --b64-cluster-ca and --apiserver-endpoint to user data",
				zap.String("b64-cluster-ca", ts.cfg.EKSConfig.Status.ClusterCA),
				zap.String("apiserver-endpoint", ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint),
				zap.String("dns-cluster-ip", dnsClusterIP),
			)
			d += fmt.Sprintf(` --b64-cluster-ca %s --apiserver-endpoint %s --dns-cluster-ip %s`,
				ts.cfg.EKSConfig.Status.ClusterCA,
				ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint,
				dnsClusterIP,
			)
		}
		// https://aws.amazon.com/blogs/opensource/improvements-eks-worker-node-provisioning/
		d += fmt.Sprintf(` --kubelet-extra-args '--node-labels=NodeType=regular,AMIType=%s,NGType=custom,NGName=%s`, amiType, asgName)
		if kubeletExtraArgs != "" {
			ts.cfg.Logger.Info("adding extra bootstrap arguments --kubelet-extra-args to user data",
				zap.String("kubelet-extra-args", kubeletExtraArgs),
			)
			d += fmt.Sprintf(` %s`, kubeletExtraArgs)
		}
		d += "'"
		if bootstrapArgs != "" {
			ts.cfg.Logger.Info("adding further additional bootstrap arguments to user data",
				zap.String("bootstrap-args", bootstrapArgs),
			)
			d += fmt.Sprintf(` %s`, bootstrapArgs)
		}
	}

	return d, nil
}
