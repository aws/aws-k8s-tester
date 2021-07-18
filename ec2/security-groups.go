package ec2

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
func (ts *Tester) createSecurityGroups() error {
	ts.lg.Info("creating security group")

	sout, err := ts.ec2APIV2.CreateSecurityGroup(
		context.Background(),
		&aws_ec2_v2.CreateSecurityGroupInput{
			GroupName:   aws_v2.String(fmt.Sprintf("%s-security-group", ts.cfg.Name)),
			Description: aws_v2.String("Communication between EKS Kubernetes control plane and worker nodes"),
			VpcId:       aws_v2.String(ts.cfg.VPC.ID),
		},
	)
	if err != nil {
		ts.lg.Warn("failed to create security group", zap.Error(err))
		return err
	}

	ts.cfg.VPC.SecurityGroupID = aws_v2.ToString(sout.GroupId)
	ts.cfg.Sync()
	ts.lg.Info("created security group", zap.String("security-group-id", ts.cfg.VPC.SecurityGroupID))

	return nil
}

func (ts *Tester) deleteSecurityGroups() (err error) {
	ts.lg.Info("deleting security group")
	if ts.cfg.VPC.ID == "" {
		return nil
	}
	if _, ok := ts.cfg.DeletedResources[ts.cfg.VPC.SecurityGroupID]; ok {
		return nil
	}

	_, err = ts.ec2APIV2.DeleteSecurityGroup(
		context.Background(),
		&aws_ec2_v2.DeleteSecurityGroupInput{
			GroupId: aws_v2.String(ts.cfg.VPC.SecurityGroupID),
		},
	)
	if err != nil {
		ts.lg.Warn("failed to delete security group", zap.Error(err))
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				ts.cfg.DeletedResources[ts.cfg.VPC.SecurityGroupID] = "VPC.SecurityGroupID"
				ts.cfg.Sync()
				return nil
			}
		}
		return err
	}

	ts.lg.Info("deleted security group")
	ts.cfg.DeletedResources[ts.cfg.VPC.SecurityGroupID] = "VPC.SecurityGroupID"
	ts.cfg.Sync()

	return nil
}

func (ts *Tester) deleteOtherSecurityGroups() (err error) {
	ts.lg.Info("deleting other security groups for the VPC", zap.String("vpc-id", ts.cfg.VPC.ID))
	if ts.cfg.VPC.ID == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	sout, err := ts.ec2APIV2.DescribeSecurityGroups(
		ctx,
		&aws_ec2_v2.DescribeSecurityGroupsInput{
			Filters: []aws_ec2_v2_types.Filter{
				{
					Name:   aws_v2.String("vpc-id"),
					Values: []string{ts.cfg.VPC.ID},
				},
			},
		})
	cancel()
	if err != nil {
		ts.lg.Warn("failed to describe security groups", zap.Error(err))
		return err
	}

	deleted := false
	for _, sg := range sout.SecurityGroups {
		sgID, sgGroupName := aws_v2.ToString(sg.GroupId), aws_v2.ToString(sg.GroupName)
		if _, ok := ts.cfg.DeletedResources[sgID]; ok {
			continue
		}
		ts.lg.Info("cleaning security group",
			zap.String("security-group-id", sgID),
			zap.String("security-group-name", sgGroupName),
		)

		for _, ipPerm := range sg.IpPermissions {
			ts.lg.Info("revoking ingress", zap.String("ip-perm", fmt.Sprintf("%+v", ipPerm)))
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			_, err = ts.ec2APIV2.RevokeSecurityGroupIngress(
				ctx,
				&aws_ec2_v2.RevokeSecurityGroupIngressInput{
					GroupId:       aws_v2.String(sgID),
					IpPermissions: []aws_ec2_v2_types.IpPermission{ipPerm},
				})
			cancel()
			if err != nil {
				ts.lg.Warn("failed to revoke ingress", zap.Error(err))
				var apiErr smithy.APIError
				if errors.As(err, &apiErr) {
					if strings.Contains(apiErr.ErrorCode(), "NotFound") {
						deleted = true
					}
				}
			} else {
				ts.lg.Info("revoked ingress")
				deleted = true
			}

			if len(ipPerm.UserIdGroupPairs) != 1 {
				continue
			}
			sgIDEgress := aws_v2.ToString(ipPerm.UserIdGroupPairs[0].GroupId)
			sgNameEgress := aws_v2.ToString(ipPerm.UserIdGroupPairs[0].GroupName)
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			sgEgress, err := ts.ec2APIV2.DescribeSecurityGroups(
				ctx,
				&aws_ec2_v2.DescribeSecurityGroupsInput{
					GroupIds: []string{sgIDEgress},
				})
			cancel()
			if err != nil {
				ts.lg.Warn("failed to describe egress security group", zap.Error(err))
				continue
			}
			if len(sgEgress.SecurityGroups) != 1 {
				ts.lg.Warn("expected only 1 security group",
					zap.String("egress-security-group-id", sgIDEgress),
					zap.String("egress-security-group-name", sgNameEgress),
					zap.Int("total", len(sgEgress.SecurityGroups)),
				)
				continue
			}
			for _, ipPermEg := range sgEgress.SecurityGroups[0].IpPermissionsEgress {
				ts.lg.Info("revoking egress", zap.String("ip-perm", fmt.Sprintf("%+v", ipPermEg)))
				ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
				_, err = ts.ec2APIV2.RevokeSecurityGroupEgress(
					ctx,
					&aws_ec2_v2.RevokeSecurityGroupEgressInput{
						GroupId:       aws_v2.String(sgIDEgress),
						IpPermissions: []aws_ec2_v2_types.IpPermission{ipPermEg},
					})
				if err != nil {
					ts.lg.Warn("failed to revoke egress", zap.Error(err))
					var apiErr smithy.APIError
					if errors.As(err, &apiErr) {
						if strings.Contains(apiErr.ErrorCode(), "NotFound") {
							deleted = true
						}
					}
				} else {
					ts.lg.Info("revoked egress")
					deleted = true
				}
				cancel()
			}
		}

		for _, ipPerm := range sg.IpPermissionsEgress {
			ts.lg.Info("revoking egress",
				zap.String("security-group-id", sgID),
				zap.String("ip-perm", fmt.Sprintf("%+v", ipPerm)),
			)
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			_, err = ts.ec2APIV2.RevokeSecurityGroupEgress(
				ctx,
				&aws_ec2_v2.RevokeSecurityGroupEgressInput{
					GroupId:       aws_v2.String(sgID),
					IpPermissions: []aws_ec2_v2_types.IpPermission{ipPerm},
				})
			cancel()
			if err != nil {
				ts.lg.Warn("failed to revoke egress", zap.Error(err))
				var apiErr smithy.APIError
				if errors.As(err, &apiErr) {
					if strings.Contains(apiErr.ErrorCode(), "NotFound") {
						deleted = true
					}
				}
			} else {
				ts.lg.Info("revoked egress")
				deleted = true
			}

			if len(ipPerm.UserIdGroupPairs) != 1 {
				continue
			}
			sgIDIngress := aws_v2.ToString(ipPerm.UserIdGroupPairs[0].GroupId)
			sgNameIngress := aws_v2.ToString(ipPerm.UserIdGroupPairs[0].GroupName)
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			sgIngress, err := ts.ec2APIV2.DescribeSecurityGroups(
				ctx,
				&aws_ec2_v2.DescribeSecurityGroupsInput{
					GroupIds: []string{sgIDIngress},
				})
			cancel()
			if err != nil {
				ts.lg.Warn("failed to describe egress security group", zap.Error(err))
				continue
			}
			if len(sgIngress.SecurityGroups) != 1 {
				ts.lg.Warn("expected only 1 security group",
					zap.String("ingress-security-group-id", sgIDIngress),
					zap.String("ingress-security-group-name", sgNameIngress),
					zap.Int("total", len(sgIngress.SecurityGroups)),
				)
				continue
			}
			for _, ipPermEg := range sgIngress.SecurityGroups[0].IpPermissionsEgress {
				ts.lg.Info("revoking ingress", zap.String("ip-perm", fmt.Sprintf("%+v", ipPermEg)))
				ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
				_, err = ts.ec2APIV2.RevokeSecurityGroupIngress(
					ctx,
					&aws_ec2_v2.RevokeSecurityGroupIngressInput{
						GroupId:       aws_v2.String(sgIDIngress),
						IpPermissions: []aws_ec2_v2_types.IpPermission{ipPermEg},
					})
				cancel()
				if err != nil {
					ts.lg.Warn("failed to revoke ingress", zap.Error(err))
					var apiErr smithy.APIError
					if errors.As(err, &apiErr) {
						if strings.Contains(apiErr.ErrorCode(), "NotFound") {
							deleted = true
						}
					}
				} else {
					ts.lg.Info("revoked ingress")
					deleted = true
				}
			}
		}

		deleted = false
		if _, ok := ts.cfg.DeletedResources[sgID]; !ok {
			ts.lg.Info("deleting security group",
				zap.String("security-group-id", sgID),
				zap.String("security-group-name", sgGroupName),
			)
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			_, err = ts.ec2APIV2.DeleteSecurityGroup(
				ctx,
				&aws_ec2_v2.DeleteSecurityGroupInput{
					GroupId: aws_v2.String(sgID),
				})
			cancel()
			if err != nil {
				ts.lg.Warn("failed to delete security group", zap.Error(err))
				var apiErr smithy.APIError
				if errors.As(err, &apiErr) {
					if strings.Contains(apiErr.ErrorCode(), "NotFound") {
						ts.cfg.DeletedResources[ts.cfg.VPC.SecurityGroupID] = "VPC.SecurityGroupID"
						ts.cfg.Sync()
						deleted = true
					}
				}
			} else {
				ts.lg.Info("deleted security group",
					zap.String("security-group-id", sgID),
					zap.String("security-group-name", sgGroupName),
				)
				ts.cfg.DeletedResources[sgID] = "VPC.SecurityGroupID"
				ts.cfg.Sync()
				deleted = true
			}
		}

		if deleted {
			retryStart := time.Now()
			for time.Since(retryStart) < 5*time.Minute {
				ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
				sout, err := ts.ec2APIV2.DescribeSecurityGroups(
					ctx,
					&aws_ec2_v2.DescribeSecurityGroupsInput{
						GroupIds: []string{sgID},
					})
				cancel()
				if err != nil {
					var apiErr smithy.APIError
					if errors.As(err, &apiErr) {
						if strings.Contains(apiErr.ErrorCode(), ".NotFound") {
							ts.lg.Info("successfully deleted security group",
								zap.String("security-group-id", sgID),
								zap.String("security-group-name", sgGroupName),
							)
							break
						}
						ts.lg.Warn("failed to describe securituy group",
							zap.String("security-group-id", sgID),
							zap.String("security-group-name", sgGroupName),
							zap.String("error-code", apiErr.ErrorCode()),
							zap.Error(err),
						)
					} else {
						ts.lg.Warn("failed to describe securituy group",
							zap.String("security-group-id", sgID),
							zap.String("security-group-name", sgGroupName),
							zap.Error(err),
						)
					}
					continue
				}
				if len(sout.SecurityGroups) == 0 {
					ts.lg.Warn("deleted security group",
						zap.String("security-group-id", sgID),
						zap.String("security-group-name", sgGroupName),
					)
					break
				}
				ts.lg.Warn("still deleting security group",
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

func (ts *Tester) authorizeSecurityGroup() error {
	ts.lg.Info("authorizing security group",
		zap.String("security-group-id", ts.cfg.VPC.SecurityGroupID),
	)

	ts.lg.Info("authorizing Ingress22ForSSH", zap.String("sg-id", ts.cfg.VPC.SecurityGroupID))
	_, err := ts.ec2APIV2.AuthorizeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.AuthorizeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(ts.cfg.VPC.SecurityGroupID),
			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("tcp"),
					IpRanges: []aws_ec2_v2_types.IpRange{
						{
							CidrIp: aws_v2.String("0.0.0.0/0"),
						},
					},
					FromPort: aws_v2.Int32(22),
					ToPort:   aws_v2.Int32(22),
				},
			},
		},
	)
	if err != nil {
		ts.lg.Warn("failed to authorize ingress", zap.Error(err))
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "Duplicate") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.lg.Info("authorized Ingress22ForSSH")

	ts.lg.Info("authorized security group")
	return nil
}

func (ts *Tester) revokeSecurityGroups() (err error) {
	ts.lg.Info("revoking security group")
	if ts.cfg.VPC.SecurityGroupID == "" {
		return nil
	}

	ts.lg.Info("revoking Ingress22ForSSH", zap.String("sg-id", ts.cfg.VPC.SecurityGroupID))
	_, err = ts.ec2APIV2.RevokeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.RevokeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(ts.cfg.VPC.SecurityGroupID),
			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("tcp"),
					IpRanges: []aws_ec2_v2_types.IpRange{
						{
							CidrIp: aws_v2.String("0.0.0.0/0"),
						},
					},
					FromPort: aws_v2.Int32(22),
					ToPort:   aws_v2.Int32(22),
				},
			},
		},
	)
	if err != nil {
		ts.lg.Warn("failed to revoke ingress", zap.Error(err))
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.lg.Info("revoked Ingress22ForSSH")

	ts.lg.Info("revoked security group")
	return nil
}
