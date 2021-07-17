package ng

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	aws_v2 "github.com/aws/aws-sdk-go-v2/aws"
	aws_ec2_v2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	aws_ec2_v2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	smithy "github.com/aws/smithy-go"
	"go.uber.org/zap"
)

func (ts *tester) deleteENIs() bool {
	if ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID == "" {
		ts.cfg.Logger.Warn("empty security group ID; returning")
		return false
	}

	ts.cfg.Logger.Info("deleting ENIs for security group", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	out, err := ts.cfg.EC2APIV2.DescribeNetworkInterfaces(
		context.Background(),
		&aws_ec2_v2.DescribeNetworkInterfacesInput{
			Filters: []aws_ec2_v2_types.Filter{
				{
					Name:   aws_v2.String("group-id"),
					Values: []string{ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID},
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to describe ENIs", zap.Error(err))
		return false
	}

	enis := make([]aws_ec2_v2_types.NetworkInterface, 0)
	for _, eni := range out.NetworkInterfaces {
		enis = append(enis, eni)
		ts.cfg.Logger.Info("found ENI", zap.String("eni", aws_v2.ToString(eni.NetworkInterfaceId)))
	}

	// detacth and delete ENIs
	deleted := false
	for _, eni := range enis {
		eniID := aws_v2.ToString(eni.NetworkInterfaceId)
		if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[eniID]; ok {
			continue
		}

		ts.cfg.Logger.Warn("detaching ENI", zap.String("eni", eniID))
		out, err := ts.cfg.EC2APIV2.DescribeNetworkInterfaces(
			context.Background(),
			&aws_ec2_v2.DescribeNetworkInterfacesInput{
				NetworkInterfaceIds: []string{eniID},
			},
		)
		if err != nil {
			ts.cfg.Logger.Warn("failed to describe ENI", zap.Error(err))
			continue
		}
		if len(out.NetworkInterfaces) != 1 {
			ts.cfg.Logger.Warn("expected 1 ENI", zap.String("eni", eniID), zap.Int("enis", len(out.NetworkInterfaces)))
			continue
		}
		if out.NetworkInterfaces[0].Attachment == nil {
			ts.cfg.Logger.Warn("no attachment found for ENI", zap.String("eni", eniID))
		} else {
			for i := 0; i < 5; i++ {
				time.Sleep(5 * time.Second)
				_, err = ts.cfg.EC2APIV2.DetachNetworkInterface(
					context.Background(),
					&aws_ec2_v2.DetachNetworkInterfaceInput{
						AttachmentId: out.NetworkInterfaces[0].Attachment.AttachmentId,
						Force:        aws_v2.Bool(true),
					})
				if err == nil {
					ts.cfg.Logger.Info("successfully detached ENI", zap.String("eni", eniID))
					break
				}
				ts.cfg.Logger.Warn("failed to detach ENI", zap.String("eni", eniID), zap.Error(err))
			}
		}

		for i := 0; i < 5; i++ {
			//  may take awhile for delete to success upon detach
			time.Sleep(10 * time.Second)
			ts.cfg.Logger.Info("deleting ENI", zap.String("eni", eniID))
			_, err = ts.cfg.EC2APIV2.DeleteNetworkInterface(
				context.Background(),
				&aws_ec2_v2.DeleteNetworkInterfaceInput{
					NetworkInterfaceId: aws_v2.String(eniID),
				})
			if err == nil {
				ts.cfg.Logger.Info("successfully deleted ENI", zap.String("eni", eniID))
				deleted = true
				break
			}
			ts.cfg.Logger.Warn("failed to delete ENI", zap.String("eni", eniID), zap.Error(err))
		}

		// confirm ENI deletion
		retryStart := time.Now()
		for time.Since(retryStart) < 5*time.Minute {
			time.Sleep(5 * time.Second)
			_, err = ts.cfg.EC2APIV2.DescribeNetworkInterfaces(
				context.Background(),
				&aws_ec2_v2.DescribeNetworkInterfacesInput{
					NetworkInterfaceIds: []string{eniID},
				})
			if err == nil {
				_, derr := ts.cfg.EC2APIV2.DeleteNetworkInterface(
					context.Background(),
					&aws_ec2_v2.DeleteNetworkInterfaceInput{
						NetworkInterfaceId: aws_v2.String(eniID),
					})
				ts.cfg.Logger.Warn("ENI still exists", zap.String("eni", eniID), zap.Error(derr))
				continue
			}
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				if strings.Contains(apiErr.ErrorCode(), "NotFound") {
					ts.cfg.EKSConfig.Status.DeletedResources[eniID] = "AddOnNodeGroups.ENI"
					ts.cfg.EKSConfig.Sync()
					deleted = true
					break
				}
			}

			_, derr := ts.cfg.EC2APIV2.DeleteNetworkInterface(
				context.Background(),
				&aws_ec2_v2.DeleteNetworkInterfaceInput{
					NetworkInterfaceId: aws_v2.String(eniID),
				})
			ts.cfg.Logger.Warn("ENI still exists", zap.String("eni", eniID), zap.String("errors", fmt.Sprintf("%v, %v", err, derr)))
		}
	}
	return deleted
}
