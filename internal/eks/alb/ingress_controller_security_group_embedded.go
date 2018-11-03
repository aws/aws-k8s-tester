package alb

import (
	"errors"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

func (md *embedded) CreateSecurityGroup() error {
	if md.cfg.VPCID == "" {
		return errors.New("cannot create security group without VPC stack VPC ID")
	}
	if md.cfg.SecurityGroupID == "" {
		return errors.New("cannot create security group without VPC stack Security Group ID")
	}
	if len(md.cfg.SubnetIDs) == 0 {
		return errors.New("cannot create security group without VPC stack Subnet IDs")
	}

	name := md.cfg.ClusterName + "-alb-open-80-443"
	so, err := md.ec2.CreateSecurityGroup(&ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(name),
		VpcId:       aws.String(md.cfg.VPCID),
		Description: aws.String(name),
	})
	if err != nil {
		md.cfg.ALBIngressController.ELBv2SecurityGroupStatus = err.Error()
		md.cfg.Sync()
		return err
	}
	md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen = *so.GroupId
	md.lg.Info("created security group",
		zap.String("name", name),
		zap.String("created-security-group-id", md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen),
	)

	for _, port := range []int64{80, 443} {
		_, err = md.ec2.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
			GroupId:    aws.String(md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen),
			IpProtocol: aws.String("tcp"),
			CidrIp:     aws.String("0.0.0.0/0"),
			FromPort:   aws.Int64(port),
			ToPort:     aws.Int64(port),
		})
		if err != nil {
			md.cfg.ALBIngressController.ELBv2SecurityGroupStatus = err.Error()
			md.cfg.Sync()
			return err
		}
		md.lg.Info("authorized ingress for ALB", zap.Int64("port", port))
	}

	_, err = md.ec2.CreateTags(&ec2.CreateTagsInput{
		Resources: aws.StringSlice([]string{md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen}),
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String(md.cfg.ClusterName),
			},
		},
	})
	if err != nil {
		md.lg.Warn("failed to tag security group", zap.String("group-id", md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen), zap.Error(err))
	}

	md.cfg.ALBIngressController.ELBv2SecurityGroupStatus = "READY"
	return md.cfg.Sync()
}

func (md *embedded) DeleteSecurityGroup() error {
	if md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen == "" {
		return errors.New("ALB Ingress Controller security group ID does not exist")
	}

	now := time.Now().UTC()

	md.cfg.ALBIngressController.ELBv2SecurityGroupStatus = "DELETING"

	_, err := md.ec2.RevokeSecurityGroupIngress(&ec2.RevokeSecurityGroupIngressInput{
		GroupId:    aws.String(md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen),
		IpProtocol: aws.String("tcp"),
		CidrIp:     aws.String("0.0.0.0/0"),
		FromPort:   aws.Int64(80),
		ToPort:     aws.Int64(80),
	})
	if err != nil {
		// do not fail the whole function, just logging errors
		// following delete security group or ALB Ingress Controller
		// should clean up this resources anyway
		if !isSecurityGroupHasDependencyGoClient(err, md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen) &&
			!isSecurityGroupDeletedGoClient(err) {
			md.cfg.ALBIngressController.ELBv2SecurityGroupStatus = err.Error() + " for port 80"
			md.cfg.Sync()
		}
	}
	_, err = md.ec2.RevokeSecurityGroupIngress(&ec2.RevokeSecurityGroupIngressInput{
		GroupId:    aws.String(md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen),
		IpProtocol: aws.String("tcp"),
		CidrIp:     aws.String("0.0.0.0/0"),
		FromPort:   aws.Int64(443),
		ToPort:     aws.Int64(443),
	})
	if err != nil {
		// do not fail the whole function, just logging errors
		// following delete security group or ALB Ingress Controller
		// should clean up this resources anyway
		if !isSecurityGroupHasDependencyGoClient(err, md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen) &&
			!isSecurityGroupDeletedGoClient(err) {
			md.cfg.ALBIngressController.ELBv2SecurityGroupStatus = err.Error() + " for port 443"
			md.cfg.Sync()
		}
	}
	md.lg.Info("revoked ALB Ingress Controller security group ingress",
		zap.String("id", md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)

	_, err = md.ec2.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{
		GroupId: aws.String(md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen),
	})
	if err != nil {
		if !isSecurityGroupHasDependencyGoClient(err, md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen) &&
			!isSecurityGroupDeletedGoClient(err) {
			md.cfg.ALBIngressController.ELBv2SecurityGroupStatus = err.Error()
			md.cfg.Sync()
			return err
		}
	}
	md.lg.Info("deleted ALB Ingress Controller security group",
		zap.String("id", md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)

	md.cfg.ALBIngressController.ELBv2SecurityGroupStatus = "DELETE_COMPLETE"
	return md.cfg.Sync()
}
