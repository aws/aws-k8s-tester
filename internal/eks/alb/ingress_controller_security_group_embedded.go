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
	if md.cfg.ClusterState.CFStackVPCID == "" {
		return errors.New("cannot create security group without VPC stack VPC ID")
	}
	if md.cfg.ClusterState.CFStackVPCSecurityGroupID == "" {
		return errors.New("cannot create security group without VPC stack Security Group ID")
	}
	if len(md.cfg.ClusterState.CFStackVPCSubnetIDs) == 0 {
		return errors.New("cannot create security group without VPC stack Subnet IDs")
	}

	now := time.Now().UTC()

	so, err := md.ec2.CreateSecurityGroup(&ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(md.cfg.ClusterName + "-alb-sg"),
		VpcId:       aws.String(md.cfg.ClusterState.CFStackVPCID),
		Description: aws.String("ALB 80 443 open"),
	})
	if err != nil {
		md.cfg.ALBIngressController.ELBv2SecurityGroupStatus = err.Error()
		md.cfg.Sync()
		return err
	}
	md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen = *so.GroupId
	md.lg.Info("created security group",
		zap.String("name", "alb-sg"),
		zap.String("created-security-group-id", md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)

	_, err = md.ec2.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:    aws.String(md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen),
		IpProtocol: aws.String("tcp"),
		CidrIp:     aws.String("0.0.0.0/0"),
		FromPort:   aws.Int64(80),
		ToPort:     aws.Int64(80),
	})
	if err != nil {
		md.cfg.ALBIngressController.ELBv2SecurityGroupStatus = err.Error()
		md.cfg.Sync()
		return err
	}
	_, err = md.ec2.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:    aws.String(md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen),
		IpProtocol: aws.String("tcp"),
		CidrIp:     aws.String("0.0.0.0/0"),
		FromPort:   aws.Int64(443),
		ToPort:     aws.Int64(443),
	})
	if err != nil {
		md.cfg.ALBIngressController.ELBv2SecurityGroupStatus = err.Error()
		md.cfg.Sync()
		return err
	}
	md.lg.Info(
		"authorized security group",
		zap.String("name", "alb-sg"),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)

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
