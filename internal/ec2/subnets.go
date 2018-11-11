package ec2

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
	"go.uber.org/zap"
)

const (
	subnetCIDR1 = "192.168.64.0/18"
	subnetCIDR2 = "192.168.128.0/18"
	subnetCIDR3 = "192.168.192.0/18"
)

var zoneSfx = []string{"a", "b", "c"}

func (md *embedded) createSubnets() (err error) {
	if md.cfg.VPCID == "" {
		return errors.New("cannot create subnets without VPC ID")
	}
	if len(md.cfg.SubnetIDs) > 0 {
		return fmt.Errorf("subnets already exist (%q)", md.cfg.SubnetIDs)
	}

	md.cfg.SubnetIDs = make([]string, 0, 3)
	md.cfg.SubnetIDToAvailibilityZone = make(map[string]string, 3)
	for i, cidr := range []string{subnetCIDR1, subnetCIDR2, subnetCIDR3} {
		var output *ec2.CreateSubnetOutput
		output, err = md.ec2.CreateSubnet(&ec2.CreateSubnetInput{
			VpcId:            aws.String(md.cfg.VPCID),
			CidrBlock:        aws.String(cidr),
			AvailabilityZone: aws.String(fmt.Sprintf("%s%s", md.cfg.AWSRegion, zoneSfx[i%len(zoneSfx)])),
		})
		if err != nil {
			return err
		}
		id := *output.Subnet.SubnetId
		az := *output.Subnet.AvailabilityZone

		md.cfg.SubnetIDs = append(md.cfg.SubnetIDs, id)
		md.cfg.SubnetIDToAvailibilityZone[id] = az

		md.lg.Info(
			"created subnet",
			zap.String("vpc-id", md.cfg.VPCID),
			zap.String("subnet-id", id),
			zap.String("availability-zone", az),
		)
	}

	if md.cfg.AssociatePublicIPAddress {
		if err = md.associatePublicIP(); err != nil {
			return err
		}
	}
	sort.Strings(md.cfg.SubnetIDs)
	return md.cfg.Sync()
}

func (md *embedded) deleteSubnet() (err error) {
	if md.cfg.VPCID == "" {
		return errors.New("cannot delete subnets without VPC ID")
	}
	if len(md.cfg.SubnetIDs) == 0 {
		return errors.New("cannot delete subnets without Subnet IDs")
	}

	for _, id := range md.cfg.SubnetIDs {
		for i := 0; i < 10; i++ {
			// TODO: handle "DependencyViolation: The subnet 'subnet-034524cbada087b8d' has dependencies and cannot be deleted"
			_, err = md.ec2.DeleteSubnet(&ec2.DeleteSubnetInput{
				SubnetId: aws.String(id),
			})
			if err == nil {
				break
			}

			if request.IsErrorRetryable(err) || request.IsErrorThrottle(err) {
				md.lg.Warn("failed to delete subnet, retrying...", zap.Error(err))
				time.Sleep(5 * time.Second)
				continue
			}

			// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/errors-overview.html
			awsErr, ok := err.(awserr.Error)
			if ok {
				if awsErr.Code() == "InvalidSubnetID.NotFound" {
					md.lg.Info(
						"subnet does not exist",
						zap.String("subnet-id", id),
					)
					return nil
				}
			}
			return err
		}
		if err != nil {
			return err
		}
		md.lg.Info(
			"deleted subnet",
			zap.String("vpc-id", md.cfg.VPCID),
			zap.String("subnet-id", id),
		)
	}

	time.Sleep(2 * time.Second)

	_, err = md.ec2.DescribeSubnets(&ec2.DescribeSubnetsInput{
		SubnetIds: aws.StringSlice(md.cfg.SubnetIDs),
	})
	if err != nil {
		// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/errors-overview.html
		awsErr, ok := err.(awserr.Error)
		if ok && awsErr.Code() == "InvalidSubnetID.NotFound" {
			return nil
		}
		return err
	}
	return fmt.Errorf("deleted subnets but %v still exists", md.cfg.SubnetIDs)
}

func (md *embedded) getSubnets() (err error) {
	if md.cfg.VPCID == "" {
		return errors.New("cannot get subnets without VPC ID")
	}
	if len(md.cfg.SubnetIDs) > 0 {
		return fmt.Errorf("subnets already exist (%q)", md.cfg.SubnetIDs)
	}

	if err = md.enableDNSHostnames(); err != nil {
		return err
	}

	var output *ec2.DescribeSubnetsOutput
	output, err = md.ec2.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: aws.StringSlice([]string{md.cfg.VPCID}),
			},
		},
	})
	if err != nil {
		return err
	}

	md.cfg.SubnetIDs = make([]string, 0, len(output.Subnets))
	md.cfg.SubnetIDToAvailibilityZone = make(map[string]string)
	for _, sv := range output.Subnets {
		md.cfg.SubnetIDs = append(md.cfg.SubnetIDs, *sv.SubnetId)
		md.cfg.SubnetIDToAvailibilityZone[*sv.SubnetId] = *sv.AvailabilityZone
	}

	md.lg.Info(
		"found subnets",
		zap.String("vpc-id", md.cfg.VPCID),
		zap.Strings("subnets", md.cfg.SubnetIDs),
	)
	return md.cfg.Sync()
}

func (md *embedded) associatePublicIP() (err error) {
	for _, id := range md.cfg.SubnetIDs {
		_, err = md.ec2.ModifySubnetAttribute(&ec2.ModifySubnetAttributeInput{
			MapPublicIpOnLaunch: &ec2.AttributeBooleanValue{
				Value: aws.Bool(true),
			},
			SubnetId: aws.String(id),
		})
		if err != nil {
			return err
		}
		md.lg.Info("allowed public IP assign for subnet", zap.String("subnet-id", id))
	}
	return nil
}
