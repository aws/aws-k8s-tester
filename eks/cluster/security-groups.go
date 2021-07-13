package cluster

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

// AWS::EC2::SecurityGroup
func (ts *tester) createSecurityGroups() error {
	ts.cfg.Logger.Info("creating security group")

	sout, err := ts.cfg.EC2APIV2.CreateSecurityGroup(
		context.Background(),
		&aws_ec2_v2.CreateSecurityGroupInput{
			GroupName:   aws_v2.String(fmt.Sprintf("%s-security-group", ts.cfg.EKSConfig.Name)),
			Description: aws_v2.String("Communication between EKS Kubernetes control plane and worker nodes"),
			VpcId:       aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to create security group", zap.Error(err))
		return err
	}

	ts.cfg.EKSConfig.VPC.SecurityGroupID = aws_v2.ToString(sout.GroupId)
	ts.cfg.EKSConfig.Sync()
	ts.cfg.Logger.Info("created security group", zap.String("security-group-id", ts.cfg.EKSConfig.VPC.SecurityGroupID))

	return nil
}

func (ts *tester) deleteSecurityGroups() (err error) {
	ts.cfg.Logger.Info("deleting security group")
	if ts.cfg.EKSConfig.VPC.ID == "" {
		return nil
	}
	if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.SecurityGroupID]; ok {
		return nil
	}

	_, err = ts.cfg.EC2APIV2.DeleteSecurityGroup(
		context.Background(),
		&aws_ec2_v2.DeleteSecurityGroupInput{
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.SecurityGroupID),
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to delete security group", zap.Error(err))
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.SecurityGroupID] = "VPC.SecurityGroupID"
				ts.cfg.EKSConfig.Sync()
				return nil
			}
		}
		return err
	}

	ts.cfg.Logger.Info("deleted security group")
	ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.SecurityGroupID] = "VPC.SecurityGroupID"
	ts.cfg.EKSConfig.Sync()

	return nil
}

func (ts *tester) deleteOtherSecurityGroups() (err error) {
	ts.cfg.Logger.Info("deleting other security groups for the VPC", zap.String("vpc-id", ts.cfg.EKSConfig.VPC.ID))
	if ts.cfg.EKSConfig.VPC.ID == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	sout, err := ts.cfg.EC2APIV2.DescribeSecurityGroups(
		ctx,
		&aws_ec2_v2.DescribeSecurityGroupsInput{
			Filters: []aws_ec2_v2_types.Filter{
				{
					Name:   aws_v2.String("vpc-id"),
					Values: []string{ts.cfg.EKSConfig.VPC.ID},
				},
			},
		})
	cancel()
	if err != nil {
		ts.cfg.Logger.Warn("failed to describe security groups", zap.Error(err))
		return err
	}

	deleted := false
	for _, sg := range sout.SecurityGroups {
		sgID, sgGroupName := aws_v2.ToString(sg.GroupId), aws_v2.ToString(sg.GroupName)
		if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[sgID]; ok {
			continue
		}
		ts.cfg.Logger.Info("cleaning security group",
			zap.String("security-group-id", sgID),
			zap.String("security-group-name", sgGroupName),
		)

		for _, ipPerm := range sg.IpPermissions {
			ts.cfg.Logger.Info("revoking ingress", zap.String("ip-perm", fmt.Sprintf("%+v", ipPerm)))
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupIngress(
				ctx,
				&aws_ec2_v2.RevokeSecurityGroupIngressInput{
					GroupId:       aws_v2.String(sgID),
					IpPermissions: []aws_ec2_v2_types.IpPermission{ipPerm},
				})
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("failed to revoke ingress", zap.Error(err))
			} else {
				ts.cfg.Logger.Info("revoked ingress")
				deleted = true
			}

			if len(ipPerm.UserIdGroupPairs) != 1 {
				continue
			}
			sgIDEgress := aws_v2.ToString(ipPerm.UserIdGroupPairs[0].GroupId)
			sgNameEgress := aws_v2.ToString(ipPerm.UserIdGroupPairs[0].GroupName)
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			sgEgress, err := ts.cfg.EC2APIV2.DescribeSecurityGroups(
				ctx,
				&aws_ec2_v2.DescribeSecurityGroupsInput{
					GroupIds: []string{sgIDEgress},
				})
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("failed to describe egress security group", zap.Error(err))
				continue
			}
			if len(sgEgress.SecurityGroups) != 1 {
				ts.cfg.Logger.Warn("expected only 1 security group",
					zap.String("egress-security-group-id", sgIDEgress),
					zap.String("egress-security-group-name", sgNameEgress),
					zap.Int("total", len(sgEgress.SecurityGroups)),
				)
				continue
			}
			for _, ipPermEg := range sgEgress.SecurityGroups[0].IpPermissionsEgress {
				ts.cfg.Logger.Info("revoking egress", zap.String("ip-perm", fmt.Sprintf("%+v", ipPermEg)))
				ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
				_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupEgress(
					ctx,
					&aws_ec2_v2.RevokeSecurityGroupEgressInput{
						GroupId:       aws_v2.String(sgIDEgress),
						IpPermissions: []aws_ec2_v2_types.IpPermission{ipPermEg},
					})
				if err != nil {
					ts.cfg.Logger.Warn("failed to revoke egress", zap.Error(err))
				} else {
					ts.cfg.Logger.Info("revoked egress")
					deleted = true
				}
				cancel()
			}
		}

		for _, ipPerm := range sg.IpPermissionsEgress {
			ts.cfg.Logger.Info("revoking egress",
				zap.String("security-group-id", sgID),
				zap.String("ip-perm", fmt.Sprintf("%+v", ipPerm)),
			)
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupEgress(
				ctx,
				&aws_ec2_v2.RevokeSecurityGroupEgressInput{
					GroupId:       aws_v2.String(sgID),
					IpPermissions: []aws_ec2_v2_types.IpPermission{ipPerm},
				})
			cancel()
			if err != nil {
				var apiErr smithy.APIError
				if errors.As(err, &apiErr) {
					if apiErr.ErrorCode() == "InvalidPermission.NotFound" {
						ts.cfg.Logger.Warn("egress ip permission does not exist", zap.Error(err))
						deleted = true
					} else {
						ts.cfg.Logger.Warn("failed to revoke egress", zap.String("error-code", apiErr.ErrorCode()), zap.Error(err))
					}
				} else {
					ts.cfg.Logger.Warn("failed to revoke egress", zap.Error(err))
				}
			} else {
				ts.cfg.Logger.Info("revoked egress",
					zap.String("security-group-id", sgID),
					zap.Error(err),
				)
				deleted = true
			}

			if len(ipPerm.UserIdGroupPairs) != 1 {
				continue
			}
			sgIDIngress := aws_v2.ToString(ipPerm.UserIdGroupPairs[0].GroupId)
			sgNameIngress := aws_v2.ToString(ipPerm.UserIdGroupPairs[0].GroupName)
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			sgIngress, err := ts.cfg.EC2APIV2.DescribeSecurityGroups(
				ctx,
				&aws_ec2_v2.DescribeSecurityGroupsInput{
					GroupIds: []string{sgIDIngress},
				})
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("failed to describe egress security group", zap.Error(err))
				continue
			}
			if len(sgIngress.SecurityGroups) != 1 {
				ts.cfg.Logger.Warn("expected only 1 security group",
					zap.String("ingress-security-group-id", sgIDIngress),
					zap.String("ingress-security-group-name", sgNameIngress),
					zap.Int("total", len(sgIngress.SecurityGroups)),
				)
				continue
			}
			for _, ipPermEg := range sgIngress.SecurityGroups[0].IpPermissionsEgress {
				ts.cfg.Logger.Info("revoking ingress", zap.String("ip-perm", fmt.Sprintf("%+v", ipPermEg)))
				ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
				_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupIngress(
					ctx,
					&aws_ec2_v2.RevokeSecurityGroupIngressInput{
						GroupId:       aws_v2.String(sgIDIngress),
						IpPermissions: []aws_ec2_v2_types.IpPermission{ipPermEg},
					})
				cancel()
				if err != nil {
					var apiErr smithy.APIError
					if errors.As(err, &apiErr) {
						if apiErr.ErrorCode() == "InvalidPermission.NotFound" {
							ts.cfg.Logger.Warn("ingress ip permission does not exist", zap.Error(err))
							deleted = true
						} else {
							ts.cfg.Logger.Warn("failed to revoke ingress", zap.String("error-code", apiErr.ErrorCode()), zap.Error(err))
						}
					} else {
						ts.cfg.Logger.Warn("failed to revoke ingress", zap.Error(err))
					}
				} else {
					ts.cfg.Logger.Info("revoked ingress",
						zap.String("security-group-id", sgID),
						zap.Error(err),
					)
					deleted = true
				}
			}
		}

		deleted = false
		if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[sgID]; !ok {
			ts.cfg.Logger.Info("deleting security group",
				zap.String("security-group-id", sgID),
				zap.String("security-group-name", sgGroupName),
			)
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			_, err = ts.cfg.EC2APIV2.DeleteSecurityGroup(
				ctx,
				&aws_ec2_v2.DeleteSecurityGroupInput{
					GroupId: aws_v2.String(sgID),
				})
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("failed to delete security group", zap.Error(err))
				var apiErr smithy.APIError
				if errors.As(err, &apiErr) {
					if strings.Contains(apiErr.ErrorCode(), "NotFound") {
						ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.SecurityGroupID] = "VPC.SecurityGroupID"
						ts.cfg.EKSConfig.Sync()
						deleted = true
					}
				}
			} else {
				ts.cfg.Logger.Info("deleted security group",
					zap.String("security-group-id", sgID),
					zap.String("security-group-name", sgGroupName),
				)
				ts.cfg.EKSConfig.Status.DeletedResources[sgID] = "VPC.SecurityGroupID"
				ts.cfg.EKSConfig.Sync()
				deleted = true
			}
		}

		if deleted {
			retryStart := time.Now()
			for time.Since(retryStart) < 5*time.Minute {
				ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
				sout, err := ts.cfg.EC2APIV2.DescribeSecurityGroups(
					ctx,
					&aws_ec2_v2.DescribeSecurityGroupsInput{
						GroupIds: []string{sgID},
					})
				cancel()
				if err != nil {
					var apiErr smithy.APIError
					if errors.As(err, &apiErr) {
						if strings.Contains(apiErr.ErrorCode(), ".NotFound") {
							ts.cfg.Logger.Info("successfully deleted security group",
								zap.String("security-group-id", sgID),
								zap.String("security-group-name", sgGroupName),
							)
							break
						}
						ts.cfg.Logger.Warn("failed to describe securituy group",
							zap.String("security-group-id", sgID),
							zap.String("security-group-name", sgGroupName),
							zap.String("error-code", apiErr.ErrorCode()),
							zap.Error(err),
						)
					} else {
						ts.cfg.Logger.Warn("failed to describe securituy group",
							zap.String("security-group-id", sgID),
							zap.String("security-group-name", sgGroupName),
							zap.Error(err),
						)
					}
					continue
				}
				if len(sout.SecurityGroups) == 0 {
					ts.cfg.Logger.Warn("deleted security group",
						zap.String("security-group-id", sgID),
						zap.String("security-group-name", sgGroupName),
					)
					break
				}
				ts.cfg.Logger.Warn("still deleting security group",
					zap.String("security-group-id", sgID),
					zap.String("security-group-name", sgGroupName),
					zap.Error(err),
				)
				time.Sleep(5 * time.Second)
			}
		}
	}

	_ = deleted
	return nil
}
