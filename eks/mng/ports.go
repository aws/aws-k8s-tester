package mng

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"go.uber.org/zap"
)

/*
// 'SourceSecurityGroups' does not open to '0.0.0.0/0'
RemoteAccess:
  Ec2SshKey: !Ref RemoteAccessKeyName
  SourceSecurityGroups:
  - !Ref ManagedNodeGroupSecurityGroup
*/

/*
// Not supported yet
// Fn::GetAtt: Resource type AWS::EKS::Nodegroup does not support attribute {Resources.RemoteAccessSecurityGroup}

  ManagedNodeGroupSecurityGroupIngress22:
    Type: AWS::EC2::SecurityGroupIngress
    Properties:
      GroupId: !GetAtt ManagedNodeGroup.Resources.RemoteAccessSecurityGroup
      IpProtocol: 'tcp'
      FromPort: '22'
      ToPort: '22'
      CidrIp: '0.0.0.0/0'

  ManagedNodeGroupSecurityGroupIngress1024:
    Type: AWS::EC2::SecurityGroupIngress
    Properties:
      GroupId: !GetAtt ManagedNodeGroup.Resources.RemoteAccessSecurityGroup
      IpProtocol: 'tcp'
      FromPort: '1'
      ToPort: '1024'
      CidrIp: '0.0.0.0/0'

  ManagedNodeGroupSecurityGroupEgress1024:
    Type: AWS::EC2::SecurityGroupIngress
    Properties:
      GroupId: !Ref ControlPlaneSecurityGroupID
      IpProtocol: 'tcp'
      FromPort: '1'
      ToPort: '1024'
      CidrIp: '0.0.0.0/0'

Outputs:

  ManagedNodeGroupRemoteAccessSecurityGroupID:
    Description: Security group ID for the node group SSH
    Value: !GetAtt ManagedNodeGroup.Resources.RemoteAccessSecurityGroup
*/

func (ts *tester) openPorts(name string) error {
	sv, ok := ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes[name]
	if !ok {
		return fmt.Errorf("Managed Node Group %q not found", name)
	}

	sgID := sv.RemoteAccessSecurityGroupID
	ts.cfg.Logger.Info("opening ports", zap.String("name", name), zap.String("security-group-id", sgID))

	sout, err := ts.cfg.EC2API.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		GroupIds: aws.StringSlice([]string{sgID}),
	})
	if err != nil {
		return err
	}
	if len(sout.SecurityGroups) != 1 {
		return fmt.Errorf("expected 1 security group, got %+v", sout.SecurityGroups)
	}

	foundSSHAccess := false
	sg := sout.SecurityGroups[0]
	for _, perm := range sg.IpPermissions {
		if perm.FromPort == nil || perm.ToPort == nil {
			ts.cfg.Logger.Info(
				"found security IP permission",
				zap.String("security-group-id", sgID),
				zap.String("permission", fmt.Sprintf("%+v", perm)),
			)
			continue
		}
		fromPort, toPort := *perm.FromPort, *perm.ToPort
		rg := ""
		if len(perm.IpRanges) == 1 {
			rg = *perm.IpRanges[0].CidrIp
		}
		ts.cfg.Logger.Info(
			"found security IP permission",
			zap.String("security-group-id", sgID),
			zap.Int64("from-port", fromPort),
			zap.Int64("to-port", toPort),
			zap.String("cidr-ip", rg),
		)
		if fromPort == 22 && toPort == 22 && rg == "0.0.0.0/0" {
			foundSSHAccess = true
			break
		}
	}

	if !foundSSHAccess {
		ts.cfg.Logger.Warn("authorizing SSH access",
			zap.String("name", name),
			zap.String("security-group-id", sgID),
			zap.Int64("port", 22),
		)
		_, aerr := ts.cfg.EC2API.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
			GroupId:    aws.String(sgID),
			IpProtocol: aws.String("tcp"),
			FromPort:   aws.Int64(22),
			ToPort:     aws.Int64(22),
			CidrIp:     aws.String("0.0.0.0/0"),
		})
		if aerr != nil {
			return aerr
		}
		ts.cfg.Logger.Info("authorized SSH access ingress",
			zap.String("security-group-id", sgID),
			zap.Int64("port", 22),
		)
	}

	ts.cfg.Logger.Info("authorizing worker node privileged port 'ingress' access for control plane",
		zap.String("security-group-id", sgID),
		zap.String("port-range", "1-1024"),
	)
	_, err = ts.cfg.EC2API.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:    aws.String(sgID),
		IpProtocol: aws.String("tcp"),
		FromPort:   aws.Int64(1),
		ToPort:     aws.Int64(1024),
		CidrIp:     aws.String("0.0.0.0/0"),
	})
	if err != nil {
		// security group may be shared between node groups
		skip := false
		if ev, ok := err.(awserr.Error); ok {
			if ev.Code() == "InvalidPermission.Duplicate" && strings.Contains(ev.Message(), "already exists") {
				skip = true
			}
		}
		if !skip {
			return err
		}
	}

	ts.cfg.Logger.Info("authorizing worker node privileged port 'egress' access for control plane",
		zap.String("security-group-id", sgID),
		zap.String("port-range", "1-1024"),
	)
	_, err = ts.cfg.EC2API.AuthorizeSecurityGroupEgress(&ec2.AuthorizeSecurityGroupEgressInput{
		GroupId: aws.String(ts.cfg.EKSConfig.Parameters.ControlPlaneSecurityGroupID),
		IpPermissions: []*ec2.IpPermission{
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int64(1),
				ToPort:     aws.Int64(1024),
				IpRanges: []*ec2.IpRange{
					{CidrIp: aws.String("0.0.0.0/0")},
				},
			},
		},
	})
	if err != nil {
		// security group may be shared between node groups
		skip := false
		if ev, ok := err.(awserr.Error); ok {
			if ev.Code() == "InvalidPermission.Duplicate" && strings.Contains(ev.Message(), "already exists") {
				skip = true
			}
		}
		if !skip {
			return err
		}
	}

	return ts.cfg.EKSConfig.Sync()
}
