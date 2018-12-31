package ec2

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"go.uber.org/zap"
)

func (md *embedded) createSecurityGroup() (err error) {
	if len(md.cfg.SecurityGroupIDs) > 0 {
		return fmt.Errorf("SecurityGroupIDs already exists (%v)", md.cfg.SecurityGroupIDs)
	}
	if md.cfg.VPCID == "" {
		return errors.New("cannot create security group without VPC ID")
	}

	var output *ec2.CreateSecurityGroupOutput
	output, err = md.ec2.CreateSecurityGroup(&ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(md.cfg.ClusterName),
		Description: aws.String(md.cfg.ClusterName),
		VpcId:       aws.String(md.cfg.VPCID),
	})
	if err != nil {
		return err
	}
	md.cfg.SecurityGroupIDs = []string{*output.GroupId}
	md.lg.Info(
		"created security group",
		zap.Strings("group-id", md.cfg.SecurityGroupIDs),
	)

	for ports, cidr := range md.cfg.IngressRulesTCP {
		ps := strings.Split(ports, "-")
		var fromPort int64
		fromPort, err = strconv.ParseInt(ps[0], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse %q", ports)
		}
		toPort := fromPort
		if len(ps) > 1 {
			toPort, err = strconv.ParseInt(ps[1], 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q", ps[1])
			}
		}
		_, err = md.ec2.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
			GroupId:    output.GroupId,
			IpProtocol: aws.String("tcp"),
			CidrIp:     aws.String(cidr),
			FromPort:   aws.Int64(fromPort),
			ToPort:     aws.Int64(toPort),
		})
		if err != nil {
			return fmt.Errorf("failed to authorize ingress for port %q and cidr %q (%v)", ports, cidr, err)
		}
		md.lg.Debug("authorized ingress", zap.String("port", ports), zap.String("cidr", cidr))
	}

	_, err = md.ec2.CreateTags(&ec2.CreateTagsInput{
		Resources: aws.StringSlice([]string{*output.GroupId}),
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String(md.cfg.ClusterName),
			},
		},
	})
	if err != nil {
		md.lg.Warn("failed to tag security group", zap.String("group-id", *output.GroupId), zap.Error(err))
	}

	return md.cfg.Sync()
}

func (md *embedded) deleteSecurityGroup() error {
	if md.cfg.KeyName == "" {
		return errors.New("cannot delete key pair without key name")
	}

	for _, id := range md.cfg.SecurityGroupIDs {
		_, err := md.ec2.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{
			GroupId: aws.String(id),
		})
		if err != nil {
			// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/errors-overview.html
			awsErr, ok := err.(awserr.Error)
			if ok && awsErr.Code() == "InvalidGroup.NotFound" {
				md.lg.Info(
					"security group does not exist",
					zap.Strings("group-ids", md.cfg.SecurityGroupIDs),
				)
				return nil
			}
			return err
		}
		md.lg.Info("deleted security group", zap.String("group-id", id))
	}

	time.Sleep(2 * time.Second)

	_, err := md.ec2.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		GroupIds: aws.StringSlice(md.cfg.SecurityGroupIDs),
	})
	if err != nil {
		// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/errors-overview.html
		awsErr, ok := err.(awserr.Error)
		if ok && awsErr.Code() == "InvalidGroup.NotFound" {
			md.lg.Info(
				"deleted all security group",
				zap.Strings("group-ids", md.cfg.SecurityGroupIDs),
			)
			return nil
		}
		return err
	}
	return fmt.Errorf("deleted security group but %q still exists", md.cfg.KeyName)
}
