package mng

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"go.uber.org/zap"
)

func (ts *tester) deleteENIs(name string) bool {
	cur, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[name]
	if !ok {
		return false
	}
	if cur.RemoteAccessSecurityGroupID == "" {
		return false
	}
	ts.cfg.Logger.Info("deleting ENIs for security group", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
	enis := make([]*ec2.NetworkInterface, 0)
	if err := ts.cfg.EC2API.DescribeNetworkInterfacesPages(
		&ec2.DescribeNetworkInterfacesInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("group-id"),
					Values: aws.StringSlice([]string{cur.RemoteAccessSecurityGroupID}),
				},
			},
		},
		func(out *ec2.DescribeNetworkInterfacesOutput, lastPage bool) bool {
			for _, eni := range out.NetworkInterfaces {
				enis = append(enis, eni)
				ts.cfg.Logger.Info("found ENI", zap.String("eni", aws.StringValue(eni.NetworkInterfaceId)))
			}
			return true
		},
	); err != nil {
		ts.cfg.Logger.Warn("failed to describe ENIs", zap.Error(err))
		return false
	}

	// detacth and delete ENIs
	deleted := false
	for _, eni := range enis {
		eniID := aws.StringValue(eni.NetworkInterfaceId)

		ts.cfg.Logger.Warn("detaching ENI", zap.String("eni", eniID))
		out, err := ts.cfg.EC2API.DescribeNetworkInterfaces(
			&ec2.DescribeNetworkInterfacesInput{
				NetworkInterfaceIds: aws.StringSlice([]string{eniID}),
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
				_, err = ts.cfg.EC2API.DetachNetworkInterface(&ec2.DetachNetworkInterfaceInput{
					AttachmentId: out.NetworkInterfaces[0].Attachment.AttachmentId,
					Force:        aws.Bool(true),
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
			_, err = ts.cfg.EC2API.DeleteNetworkInterface(&ec2.DeleteNetworkInterfaceInput{
				NetworkInterfaceId: aws.String(eniID),
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
		for time.Now().Sub(retryStart) < 5*time.Minute {
			time.Sleep(5 * time.Second)
			_, err = ts.cfg.EC2API.DescribeNetworkInterfaces(&ec2.DescribeNetworkInterfacesInput{
				NetworkInterfaceIds: aws.StringSlice([]string{eniID}),
			})
			if err == nil {
				_, derr := ts.cfg.EC2API.DeleteNetworkInterface(&ec2.DeleteNetworkInterfaceInput{
					NetworkInterfaceId: aws.String(eniID),
				})
				ts.cfg.Logger.Warn("ENI still exists", zap.String("eni", eniID), zap.Error(derr))
				continue
			}
			if awsErr, ok := err.(awserr.Error); ok {
				if strings.Contains(awsErr.Code(), "InvalidNetworkInterfaceID.NotFound") {
					ts.cfg.Logger.Info("confirmed ENI deletion", zap.String("eni", eniID))
					deleted = true
					break
				}
			}

			_, derr := ts.cfg.EC2API.DeleteNetworkInterface(&ec2.DeleteNetworkInterfaceInput{
				NetworkInterfaceId: aws.String(eniID),
			})
			ts.cfg.Logger.Warn("ENI still exists", zap.String("eni", eniID), zap.String("errors", fmt.Sprintf("%v, %v", err, derr)))
		}
	}
	return deleted
}
